package numbers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/milktart/milk/pkg/tui"
)

type numbersConfig struct {
	DefaultCodes []string `json:"default_codes"`
}

func numbersConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "milk", "numbers")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "milk", "numbers")
}

func numbersConfigFile() string {
	return filepath.Join(numbersConfigDir(), "config.json")
}

func loadNumbersConfig() (*numbersConfig, error) {
	path := numbersConfigFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg numbersConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse numbers config: %w", err)
	}
	return &cfg, nil
}

func saveNumbersConfig(cfg *numbersConfig) error {
	path := numbersConfigFile()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// AddDefaultCodes adds one or more codes to the user's default list.
func AddDefaultCodes(codes []string) error {
	cfg, err := loadNumbersConfig()
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &numbersConfig{}
	}
	existing := make(map[string]bool, len(cfg.DefaultCodes))
	for _, c := range cfg.DefaultCodes {
		existing[c] = true
	}
	var added []string
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if existing[code] {
			fmt.Printf("%s is already in the default list.\n", code)
			continue
		}
		cfg.DefaultCodes = append(cfg.DefaultCodes, code)
		existing[code] = true
		added = append(added, code)
	}
	if len(added) == 0 {
		return nil
	}
	if err := saveNumbersConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Added %s to default codes.\n", strings.Join(added, ", "))
	return nil
}

// RemoveDefaultCodesInteractive presents an interactive multi-select list of
// the current default codes and removes the ones the user selects.
func RemoveDefaultCodesInteractive(builtinDefaults []string) error {
	cfg, err := loadNumbersConfig()
	if err != nil {
		return err
	}

	codes := builtinDefaults
	usingCustom := false
	if cfg != nil && len(cfg.DefaultCodes) > 0 {
		codes = cfg.DefaultCodes
		usingCustom = true
	}
	if len(codes) == 0 {
		fmt.Println("No codes configured.")
		return nil
	}

	items := make([]tui.Item, len(codes))
	for i, c := range codes {
		items[i] = tui.Item{Label: c, Group: "Area Codes"}
	}

	indices, err := tui.MultiSelect("Select codes to remove:", items)
	if err != nil {
		return err
	}
	if len(indices) == 0 {
		fmt.Println("Nothing removed.")
		return nil
	}

	remove := make(map[int]bool, len(indices))
	for _, i := range indices {
		remove[i] = true
	}

	var kept, removed []string
	for i, c := range codes {
		if remove[i] {
			removed = append(removed, c)
		} else {
			kept = append(kept, c)
		}
	}

	if usingCustom {
		if cfg == nil {
			cfg = &numbersConfig{}
		}
		cfg.DefaultCodes = kept
		if err := saveNumbersConfig(cfg); err != nil {
			return err
		}
	} else {
		// Promote built-ins minus the removed ones to a custom config.
		newCfg := &numbersConfig{DefaultCodes: kept}
		if err := saveNumbersConfig(newCfg); err != nil {
			return err
		}
	}

	fmt.Printf("Removed %s from default codes.\n", strings.Join(removed, ", "))
	return nil
}

// ListDefaultCodes prints the user's configured default codes, or a note if none are set.
// Kept for internal use; prefer ListCodes for the CLI.
func ListDefaultCodes(builtinDefaults []string) error {
	return ListCodes(builtinDefaults)
}

// ListCodes prints the user's configured default codes.
func ListCodes(builtinDefaults []string) error {
	cfg, err := loadNumbersConfig()
	if err != nil {
		return err
	}
	if cfg == nil || len(cfg.DefaultCodes) == 0 {
		fmt.Println("No custom default codes configured. Using built-in defaults:")
		for _, c := range builtinDefaults {
			fmt.Printf("  %s\n", c)
		}
		fmt.Printf("\nConfig file: %s\n", numbersConfigFile())
		return nil
	}
	fmt.Println("Custom default codes:")
	for _, c := range cfg.DefaultCodes {
		fmt.Printf("  %s\n", c)
	}
	fmt.Printf("\nConfig file: %s\n", numbersConfigFile())
	return nil
}
