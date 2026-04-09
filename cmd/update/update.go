package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	owner      = "milktart"
	repo       = "milk"
	binaryName = "milk"
	apiURL     = "https://api.github.com/repos/" + owner + "/" + repo + "/releases/latest"
)

type release struct {
	TagName string `json:"tag_name"`
}

// LatestVersion fetches the latest release tag from GitHub, with a timeout.
// Returns empty string on any error.
func LatestVersion(timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(apiURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	var r release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return ""
	}
	return strings.TrimPrefix(r.TagName, "v")
}

const (
	yellow = "\033[1;93m"
	nc     = "\033[0m"
)

// CheckAndNotify prints an upgrade notice if a newer version is available.
// It runs the network check with a short timeout so it never blocks the user.
func CheckAndNotify(current string) {
	latest := LatestVersion(3 * time.Second)
	if latest == "" || latest == current {
		return
	}
	if newerThan(latest, current) {
		fmt.Printf("%s⚠  New version available: v%s → v%s. Run 'milk update' to upgrade.%s\n\n", yellow, current, latest, nc)
	}
}

// Run checks for a newer version and, if found, downloads and installs it.
func Run(current string) error {
	fmt.Printf("Checking for updates (current: v%s)...\n", current)

	latest := LatestVersion(10 * time.Second)
	if latest == "" {
		return fmt.Errorf("could not fetch latest version from GitHub")
	}

	if !newerThan(latest, current) {
		fmt.Printf("Already up to date (v%s).\n", current)
		return nil
	}

	fmt.Printf("New version available: v%s\n", latest)
	fmt.Printf("Downloading...\n")

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	assetName := fmt.Sprintf("%s_%s_%s_%s", binaryName, latest, goos, goarch)
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/v%s/%s",
		owner, repo, latest, assetName,
	)

	tmpFile, err := os.CreateTemp("", binaryName+"-*")
	if err != nil {
		return fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath)
	}()

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d from %s", resp.StatusCode, downloadURL)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return fmt.Errorf("could not write download: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("could not chmod binary: %w", err)
	}

	// Determine where the current binary lives so we can replace it.
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine current binary path: %w", err)
	}

	// Try a direct rename first (works when tmp and dest are on the same filesystem).
	if err := os.Rename(tmpPath, selfPath); err != nil {
		// Fall back to sudo mv for system-wide installs (e.g. /usr/local/bin).
		cmd := exec.Command("sudo", "mv", tmpPath, selfPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("could not install binary (tried mv and sudo mv): %w", err2)
		}
	}

	fmt.Printf("Updated to v%s. Run 'milk --version' to confirm.\n", latest)
	return nil
}

// newerThan returns true if a > b using simple semver integer comparison.
func newerThan(a, b string) bool {
	av := parseVer(a)
	bv := parseVer(b)
	for i := range av {
		if i >= len(bv) {
			return av[i] > 0
		}
		if av[i] != bv[i] {
			return av[i] > bv[i]
		}
	}
	return false
}

func parseVer(v string) []int {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n := 0
		fmt.Sscanf(p, "%d", &n)
		nums[i] = n
	}
	return nums
}
