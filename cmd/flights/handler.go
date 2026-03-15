package flights

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ensureDisplay re-execs the current process under xvfb-run if $DISPLAY is
// not set and xvfb-run is available. This allows the browser launcher to work
// on headless servers without the caller needing to wrap the command manually.
func ensureDisplay() error {
	if os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "" {
		return nil
	}
	xvfbRun, err := exec.LookPath("xvfb-run")
	if err != nil {
		return nil // no xvfb-run available; let the browser fail naturally
	}
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	// Re-exec: xvfb-run -a <self> <original args...>
	cmd := exec.Command(xvfbRun, append([]string{"-a", self}, os.Args[1:]...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
	os.Exit(0)
	return nil // unreachable
}

// Handler processes the flights subcommand.
type Handler struct {
	FlagSet *flag.FlagSet
}

// NewHandler creates a Handler for the flights command.
func NewHandler() *Handler {
	return &Handler{
		FlagSet: flag.NewFlagSet("flights", flag.ExitOnError),
	}
}

// Execute runs the flights command with the provided arguments.
func (h *Handler) Execute(args []string) error {
	ensureDisplay()

	addFlag := h.FlagSet.Bool("add", false, "Add a booking: --add PNR LASTNAME/FIRSTNAME")
	removeFlag := h.FlagSet.Bool("remove", false, "Remove a booking by PNR: --remove PNR")
	listFlag := h.FlagSet.Bool("list", false, "List all saved bookings grouped by passenger")
	pullFlag := h.FlagSet.Bool("pull", false, "Fetch a PNR without saving: --pull PNR LASTNAME/FIRSTNAME")
	refreshFlag := h.FlagSet.Bool("refresh", false, "Force re-fetch bookings (ignore cache); optionally pass a PNR to refresh only that booking")
	cachedFlag := h.FlagSet.Bool("show-cached", false, "Display cached data only, do not hit delta.com")
	visibleFlag := h.FlagSet.Bool("visible", false, "Run browser in visible (non-headless) mode")

	h.FlagSet.Usage = func() {
		fmt.Fprintf(h.FlagSet.Output(), "Usage: milk flights [options]\n\n")
		fmt.Println("Fetch and display Delta flight bookings from delta.com.")
		fmt.Println()
		fmt.Println("Booking data is stored in bookings.json in ~/.config/milk/flights/.")
		fmt.Println("Results are cached for 6 hours (always re-fetched within 24h of departure).")
		fmt.Println()
		fmt.Println("Options:")
		h.FlagSet.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  milk flights")
		fmt.Println("  milk flights --refresh")
		fmt.Println("  milk flights --refresh Z0Y3X5")
		fmt.Println("  milk flights --show-cached")
		fmt.Println("  milk flights --add Z0Y3X5 DOE/JANEALICE")
		fmt.Println("  milk flights --list")
		fmt.Println("  milk flights --pull Z0Y3X5 DOE/ALICEJANE")
		fmt.Println("  milk flights --remove Z0Y3X5")
		fmt.Println("  milk flights --visible")
	}

	if err := h.FlagSet.Parse(args); err != nil {
		return err
	}

	// --add PNR LASTNAME/FIRSTNAME
	if *addFlag {
		rest := h.FlagSet.Args()
		if len(rest) < 2 {
			return fmt.Errorf("--add requires two arguments: PNR LASTNAME/FIRSTNAME")
		}
		pnr := strings.ToUpper(strings.TrimSpace(rest[0]))
		name := rest[1]
		slash := strings.Index(name, "/")
		if slash < 0 {
			return fmt.Errorf("--add name must be in LASTNAME/FIRSTNAME format")
		}
		last := strings.TrimSpace(name[:slash])
		first := strings.TrimSpace(name[slash+1:])
		if pnr == "" || last == "" || first == "" {
			return fmt.Errorf("--add requires non-empty PNR, last name, and first name")
		}
		return AddBooking(pnr, first, last)
	}

	// --remove PNR
	if *removeFlag {
		rest := h.FlagSet.Args()
		if len(rest) < 1 {
			return fmt.Errorf("--remove requires one argument: PNR")
		}
		pnr := strings.ToUpper(strings.TrimSpace(rest[0]))
		if pnr == "" {
			return fmt.Errorf("--remove requires a non-empty PNR")
		}
		return RemoveBooking(pnr)
	}

	// --list: show all saved bookings grouped by passenger
	if *listFlag {
		return ListBookings()
	}

	// --pull PNR LASTNAME/FIRSTNAME: one-off fetch, not saved to bookings or cache
	if *pullFlag {
		rest := h.FlagSet.Args()
		if len(rest) < 2 {
			return fmt.Errorf("--pull requires two arguments: PNR LASTNAME/FIRSTNAME")
		}
		pnr := strings.ToUpper(strings.TrimSpace(rest[0]))
		name := rest[1]
		slash := strings.Index(name, "/")
		if slash < 0 {
			return fmt.Errorf("--pull name must be in LASTNAME/FIRSTNAME format")
		}
		last := strings.TrimSpace(name[:slash])
		first := strings.TrimSpace(name[slash+1:])
		if pnr == "" || last == "" || first == "" {
			return fmt.Errorf("--pull requires non-empty PNR, last name, and first name")
		}
		b := Booking{PNR: pnr, First: first, Last: last}
		fmt.Printf("Fetching %s (%s)…\n", pnr, formatName(first, last))
		entry := fetchWithBrowser(b, !*visibleFlag)
		DisplayAll([]*CacheEntry{entry})
		return nil
	}

	// --show-cached: display without fetching
	if *cachedFlag {
		cache := LoadCache()
		if len(cache.Entries) == 0 {
			fmt.Println("Cache is empty. Run without --show-cached to fetch data.")
			return nil
		}
		var entries []*CacheEntry
		for _, e := range cache.Entries {
			entries = append(entries, e)
		}
		DisplayAll(entries)
		return nil
	}

	// Default: load bookings and fetch as needed
	bookings, err := LoadBookings()
	if err != nil {
		return fmt.Errorf("loading bookings: %w", err)
	}

	if len(bookings) == 0 {
		fmt.Println("No bookings found in bookings.json.")
		fmt.Println("Add one with: milk flights --add PNR LASTNAME/FIRSTNAME")
		return nil
	}

	cache := LoadCache()

	var toFetch []Booking
	results := make(map[string]*CacheEntry, len(bookings))

	var refreshPNR string
	if *refreshFlag && len(h.FlagSet.Args()) > 0 {
		refreshPNR = strings.ToUpper(strings.TrimSpace(h.FlagSet.Args()[0]))
	}

	for _, b := range bookings {
		cached := cache.Entries[b.PNR]
		forceRefresh := *refreshFlag && (refreshPNR == "" || b.PNR == refreshPNR)
		if !forceRefresh && IsCacheFresh(cached) {
			results[b.PNR] = cached
		} else {
			toFetch = append(toFetch, b)
			results[b.PNR] = nil
		}
	}

	if len(toFetch) > 0 {
		// Partition: bookings with cached headers can go straight to the API;
		// the rest need a browser session to capture headers first.
		var needBrowser, hasHeaders []Booking
		for _, b := range toFetch {
			if HasAPICredentials(cache.Entries[b.PNR]) {
				hasHeaders = append(hasHeaders, b)
			} else {
				needBrowser = append(needBrowser, b)
			}
		}

		// Capture session headers once for all browser-needed bookings.
		var freshHeaders map[string]string
		if len(needBrowser) > 0 {
			fmt.Println("Opening browser to capture session…")
			var err error
			freshHeaders, err = CaptureSessionHeaders(needBrowser[0], !*visibleFlag)
			if err != nil {
				fmt.Printf("Warning: could not capture session headers (%v); falling back to per-PNR browser\n", err)
			}
		}

		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, maxConcurrent)

		fetch := func(booking Booking, headers map[string]string) {
			defer wg.Done()
			defer func() { <-sem }()
			fmt.Printf("Fetching %s (%s)…\n", booking.PNR, formatName(booking.First, booking.Last))
			var entry *CacheEntry
			if headers != nil {
				entry = FetchBookingFromAPI(booking, headers)
				if entry.RawError == "" && len(entry.Flights) > 0 {
					entry.SessionHeaders = headers
				}
			} else {
				entry = fetchWithBrowser(booking, !*visibleFlag)
			}
			mu.Lock()
			if entry.RawError != "" || len(entry.Flights) == 0 {
				if prior := cache.Entries[booking.PNR]; prior != nil {
					if entry.RawError != "" {
						fmt.Printf("Warning: refresh failed for %s (%s), keeping cached data: %s\n", booking.PNR, formatName(booking.First, booking.Last), entry.RawError)
					} else {
						fmt.Printf("Warning: refresh returned no flights for %s (%s), keeping cached data\n", booking.PNR, formatName(booking.First, booking.Last))
					}
					entry = prior
				}
			}
			results[booking.PNR] = entry
			cache.Entries[booking.PNR] = entry
			mu.Unlock()
		}

		for _, b := range hasHeaders {
			wg.Add(1)
			sem <- struct{}{}
			go fetch(b, cache.Entries[b.PNR].SessionHeaders)
		}
		for _, b := range needBrowser {
			wg.Add(1)
			sem <- struct{}{}
			go fetch(b, freshHeaders) // nil if capture failed → falls back to per-PNR browser
		}
		wg.Wait()

		if err := SaveCache(cache); err != nil {
			fmt.Printf("Warning: could not save cache: %v\n", err)
		}
	}

	// Emit cached notice
	var cachedPNRs []string
	for _, b := range bookings {
		if results[b.PNR] != nil && !containsBooking(toFetch, b.PNR) {
			cachedPNRs = append(cachedPNRs, b.PNR)
		}
	}
	if len(cachedPNRs) > 0 {
		fmt.Printf("\033[2mCached: %s\033[0m\n", joinStrings(cachedPNRs, ", "))
	}

	// Build ordered slice (preserve bookings.json order)
	var ordered []*CacheEntry
	seen := map[string]bool{}
	for _, b := range bookings {
		if !seen[b.PNR] {
			ordered = append(ordered, results[b.PNR])
			seen[b.PNR] = true
		}
	}

	DisplayAll(ordered)
	return nil
}

// fetchWithBrowser captures session headers via a browser session for b, then
// immediately uses them to fetch b from the API.
func fetchWithBrowser(b Booking, headless bool) *CacheEntry {
	entry := &CacheEntry{
		PNR:       b.PNR,
		Passenger: formatName(b.First, b.Last),
	}
	headers, err := CaptureSessionHeaders(b, headless)
	if err != nil {
		entry.RawError = err.Error()
		return entry
	}
	entry.SessionHeaders = headers
	apiEntry := FetchBookingFromAPI(b, headers)
	if apiEntry.RawError != "" || len(apiEntry.Flights) == 0 {
		if apiEntry.RawError != "" {
			entry.RawError = fmt.Sprintf("API: %s", apiEntry.RawError)
		} else {
			entry.RawError = "API returned no flights"
		}
		return entry
	}
	return apiEntry
}

func containsBooking(list []Booking, pnr string) bool {
	for _, b := range list {
		if b.PNR == pnr {
			return true
		}
	}
	return false
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	out := ss[0]
	for _, s := range ss[1:] {
		out += sep + s
	}
	return out
}
