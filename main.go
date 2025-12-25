package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milktart/milk/cmd/numbers"
	"github.com/milktart/milk/pkg/config"
)

const (
	TOOLNAME = "milk"
	VERSION  = "1.0.0"
)

func printMainMenu() {
	fmt.Printf("%s - A Swiss Army Knife CLI tool\n\n", TOOLNAME)
	fmt.Println("Usage:")
	fmt.Printf("  %s <command> [options]\n", TOOLNAME)
	fmt.Printf("  %s --help\n\n", TOOLNAME)
	fmt.Println("Commands:")
	fmt.Println("  numbers    Search for special phone numbers by area code and pattern")
	fmt.Println("  distance   Calculate distances between locations")
	fmt.Println()
	fmt.Printf("Use \"%s <command> --help\" for more information about a command.\n\n", TOOLNAME)
	fmt.Println("Examples:")
	fmt.Printf("  %s numbers -c 212 415 808 -r Canada -p VIP\n", TOOLNAME)
	fmt.Printf("  %s numbers --Canada -c 416 604\n", TOOLNAME)
	fmt.Printf("  %s distance --from \"New York\" --to \"Los Angeles\"\n", TOOLNAME)
}

func loadConfig() (*config.Config, error) {
	// Find config directory relative to executable
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	configDir := filepath.Join(exeDir, "config")

	// Try to load from same directory as executable
	cfg, err := config.Load(configDir)
	if err == nil {
		return cfg, nil
	}

	// Try to load from parent directory (for development)
	configDir = filepath.Join(exeDir, "..", "config")
	cfg, err = config.Load(configDir)
	if err == nil {
		return cfg, nil
	}

	// Try from current working directory
	configDir = "config"
	cfg, err = config.Load(configDir)
	if err == nil {
		return cfg, nil
	}

	return nil, fmt.Errorf("failed to load configuration from any location")
}

func main() {
	if len(os.Args) < 2 {
		printMainMenu()
		os.Exit(0)
	}

	subcommand := os.Args[1]

	// Handle help flags at main level
	if subcommand == "--help" || subcommand == "-h" || subcommand == "help" {
		printMainMenu()
		os.Exit(0)
	}

	// Load configuration once at startup
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Route to subcommands
	switch strings.ToLower(subcommand) {
	case "numbers":
		handler := numbers.NewHandler(cfg)
		if err := handler.Execute(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "distance":
		fmt.Println("Distance command is not yet implemented")
		os.Exit(1)

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown command '%s'\n\n", subcommand)
		printMainMenu()
		os.Exit(1)
	}
}
