package flights

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// passengerEntry mirrors the bookings.json schema.
type passengerEntry struct {
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	PNRs      []string `json:"pnrs"`
}

// Booking is a flat, scraper-ready record: one PNR + one passenger.
type Booking struct {
	PNR   string
	First string
	Last  string
}

func flightsConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "milk", "flights")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "milk", "flights")
}

func bookingsFile() string {
	return filepath.Join(flightsConfigDir(), "bookings.json")
}

// ListBookings prints all bookings grouped by passenger name.
func ListBookings() error {
	path := bookingsFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		fmt.Println("No bookings found. Add one with: milk flights --add PNR LASTNAME/FIRSTNAME")
		return nil
	}
	if err != nil {
		return err
	}
	var entries []passengerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse bookings.json: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("No bookings found. Add one with: milk flights --add PNR LASTNAME/FIRSTNAME")
		return nil
	}
	for _, e := range entries {
		fmt.Printf("%s/%s\n", e.LastName, e.FirstName)
		for _, pnr := range e.PNRs {
			fmt.Printf("  %s\n", pnr)
		}
	}
	return nil
}

// formatName returns a name in LASTNAME/FIRSTNAME format, uppercased.
func formatName(first, last string) string {
	return strings.ToUpper(last) + "/" + strings.ToUpper(first)
}

// LoadBookings reads bookings.json and expands the grouped schema into flat Booking slices.
func LoadBookings() ([]Booking, error) {
	path := bookingsFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []passengerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse bookings.json: %w", err)
	}
	var out []Booking
	for _, e := range entries {
		first := strings.TrimSpace(e.FirstName)
		last := strings.TrimSpace(e.LastName)
		for _, pnr := range e.PNRs {
			out = append(out, Booking{
				PNR:   strings.ToUpper(strings.TrimSpace(pnr)),
				First: first,
				Last:  last,
			})
		}
	}
	return out, nil
}

// AddBooking appends a PNR to bookings.json, grouping by passenger name.
func AddBooking(pnr, first, last string) error {
	pnr = strings.ToUpper(strings.TrimSpace(pnr))
	first = strings.TrimSpace(first)
	last = strings.TrimSpace(last)

	path := bookingsFile()
	var entries []passengerEntry
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &entries); err != nil {
			return fmt.Errorf("parse bookings.json: %w", err)
		}
	}

	for i, e := range entries {
		if strings.EqualFold(e.FirstName, first) && strings.EqualFold(e.LastName, last) {
			for _, p := range e.PNRs {
				if strings.EqualFold(p, pnr) {
					fmt.Printf("PNR %s already exists for %s.\n", pnr, formatName(first, last))
					return nil
				}
			}
			entries[i].PNRs = append(entries[i].PNRs, pnr)
			fmt.Printf("Added PNR %s to %s.\n", pnr, formatName(first, last))
			return saveBookings(path, entries)
		}
	}

	entries = append(entries, passengerEntry{
		FirstName: first,
		LastName:  last,
		PNRs:      []string{pnr},
	})
	fmt.Printf("Added %s with PNR %s.\n", formatName(first, last), pnr)
	return saveBookings(path, entries)
}

func saveBookings(path string, entries []passengerEntry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}
