package flights

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// passengerEntry mirrors the bookings.json schema.
// Username is kept for backward-compat with existing bookings.json files but
// is always the same as MemberID — use MemberID as the login username.
type passengerEntry struct {
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	PNRs      []string `json:"pnrs"`
	Password  string   `json:"password,omitempty"`
	MemberID  string   `json:"member_id,omitempty"`
	Source    string   `json:"source,omitempty"` // "manual" | "loyalty"

	// Deprecated: same as MemberID. Kept for backward compat when reading old files.
	Username string `json:"username,omitempty"`
}

// Booking is a flat, scraper-ready record: one PNR + one passenger.
type Booking struct {
	PNR      string
	First    string
	Last     string
	MemberID string
	Password string
	Source   string
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
		memberSuffix := ""
		if e.MemberID != "" {
			memberSuffix = fmt.Sprintf(" [member: %s]", e.MemberID)
		}
		fmt.Printf("%s/%s%s\n", e.LastName, e.FirstName, memberSuffix)
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
				PNR:      strings.ToUpper(strings.TrimSpace(pnr)),
				First:    first,
				Last:     last,
				MemberID: e.MemberID,
				Password: e.Password,
				Source:   e.Source,
			})
		}
	}
	return out, nil
}

// LoadPassengers reads bookings.json and returns the raw passenger entries.
func LoadPassengers() ([]passengerEntry, error) {
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
	return entries, nil
}

// AddBooking appends a PNR to bookings.json, grouping by passenger name.
func AddBooking(pnr, first, last, source string) error {
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
		Source:    source,
	})
	fmt.Printf("Added %s with PNR %s.\n", formatName(first, last), pnr)
	return saveBookings(path, entries)
}

// RemoveBooking removes a PNR from bookings.json.
// If the passenger has no remaining PNRs, the passenger entry is also removed.
func RemoveBooking(pnr string) error {
	pnr = strings.ToUpper(strings.TrimSpace(pnr))

	path := bookingsFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("PNR %s not found (no bookings file)", pnr)
	}
	if err != nil {
		return err
	}
	var entries []passengerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse bookings.json: %w", err)
	}

	found := false
	var updated []passengerEntry
	for _, e := range entries {
		var remaining []string
		for _, p := range e.PNRs {
			if strings.EqualFold(p, pnr) {
				found = true
			} else {
				remaining = append(remaining, p)
			}
		}
		if len(remaining) > 0 {
			e.PNRs = remaining
			updated = append(updated, e)
		}
	}

	if !found {
		return fmt.Errorf("PNR %s not found in bookings", pnr)
	}

	fmt.Printf("Removed PNR %s.\n", pnr)
	return saveBookings(path, updated)
}

// SetAccountCredentials sets the password and member ID on the passenger
// matching last/first (case-insensitive), then saves.
func SetAccountCredentials(last, first, password, memberID string) error {
	path := bookingsFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("passenger %s not found (no bookings file)", formatName(first, last))
	}
	if err != nil {
		return err
	}
	var entries []passengerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("parse bookings.json: %w", err)
	}

	for i, e := range entries {
		if strings.EqualFold(e.FirstName, first) && strings.EqualFold(e.LastName, last) {
			entries[i].Password = password
			entries[i].MemberID = memberID
			if err := saveBookings(path, entries); err != nil {
				return err
			}
			fmt.Printf("Account updated for %s.\n", formatName(first, last))
			return nil
		}
	}
	return fmt.Errorf("passenger %s not found in bookings", formatName(first, last))
}

// MergeLoyaltyPNRs adds newly discovered PNRs for the given passenger.
// It deduplicates against all existing PNRs across all passengers.
// Returns the count of newly added PNRs.
func MergeLoyaltyPNRs(pnrs []string, ownerFirst, ownerLast string) (int, error) {
	path := bookingsFile()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, fmt.Errorf("passenger %s not found (no bookings file)", formatName(ownerFirst, ownerLast))
	}
	if err != nil {
		return 0, err
	}
	var entries []passengerEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 0, fmt.Errorf("parse bookings.json: %w", err)
	}

	// Collect all existing PNRs across all passengers for deduplication.
	existing := make(map[string]bool)
	for _, e := range entries {
		for _, p := range e.PNRs {
			existing[strings.ToUpper(strings.TrimSpace(p))] = true
		}
	}

	// Find the matching passenger.
	passengerIdx := -1
	for i, e := range entries {
		if strings.EqualFold(e.FirstName, ownerFirst) && strings.EqualFold(e.LastName, ownerLast) {
			passengerIdx = i
			break
		}
	}
	if passengerIdx < 0 {
		return 0, fmt.Errorf("passenger %s not found in bookings", formatName(ownerFirst, ownerLast))
	}

	added := 0
	for _, pnr := range pnrs {
		pnr = strings.ToUpper(strings.TrimSpace(pnr))
		if pnr == "" || existing[pnr] {
			continue
		}
		entries[passengerIdx].PNRs = append(entries[passengerIdx].PNRs, pnr)
		existing[pnr] = true
		added++
	}

	if added > 0 {
		if entries[passengerIdx].Source == "" {
			entries[passengerIdx].Source = "loyalty"
		}
		if err := saveBookings(path, entries); err != nil {
			return 0, err
		}
	}
	return added, nil
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
