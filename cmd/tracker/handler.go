package tracker

import (
	"flag"
	"fmt"
	"sync"
)

// Handler processes the tracker subcommand.
type Handler struct {
	FlagSet *flag.FlagSet
}

// NewHandler creates a Handler for the tracker command.
func NewHandler() *Handler {
	return &Handler{
		FlagSet: flag.NewFlagSet("tracker", flag.ExitOnError),
	}
}

// Execute runs the tracker command with the provided arguments.
func (h *Handler) Execute(args []string) error {
	addFlag := h.FlagSet.Bool("add", false, "Add a booking: --add PNR FIRSTNAME LASTNAME")
	refreshFlag := h.FlagSet.Bool("refresh", false, "Force re-fetch all bookings (ignore cache)")
	cachedFlag := h.FlagSet.Bool("show-cached", false, "Display cached data only, do not hit delta.com")
	visibleFlag := h.FlagSet.Bool("visible", false, "Run browser in visible (non-headless) mode")

	h.FlagSet.Usage = func() {
		fmt.Fprintf(h.FlagSet.Output(), "Usage: milk tracker [options]\n\n")
		fmt.Println("Fetch and display your Delta flight bookings from delta.com.\n")
		fmt.Println("Booking data is stored in bookings.json next to the binary.")
		fmt.Println("Results are cached for 6 hours (always re-fetched within 24h of departure).\n")
		fmt.Println("Options:")
		h.FlagSet.PrintDefaults()
		fmt.Println("\nExamples:")
		fmt.Println("  milk tracker")
		fmt.Println("  milk tracker --refresh")
		fmt.Println("  milk tracker --show-cached")
		fmt.Println("  milk tracker --add ABC123 Jane Doe")
		fmt.Println("  milk tracker --visible")
	}

	if err := h.FlagSet.Parse(args); err != nil {
		return err
	}

	// --add PNR FIRST LAST (extra args after the flag)
	if *addFlag {
		rest := h.FlagSet.Args()
		if len(rest) < 3 {
			return fmt.Errorf("--add requires three arguments: PNR FIRSTNAME LASTNAME")
		}
		return AddBooking(rest[0], rest[1], rest[2])
	}

	// --show-cached: display without fetching
	if *cachedFlag {
		cache := LoadCache()
		if len(cache) == 0 {
			fmt.Println("Cache is empty. Run without --show-cached to fetch data.")
			return nil
		}
		var entries []*CacheEntry
		for _, e := range cache {
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
		fmt.Println("Add one with: milk tracker --add CONFIRMATION FIRSTNAME LASTNAME")
		return nil
	}

	cache := LoadCache()
	var toFetch []Booking
	results := make(map[string]*CacheEntry, len(bookings))

	for _, b := range bookings {
		cached := cache[b.PNR]
		if !*refreshFlag && IsCacheFresh(cached) {
			results[b.PNR] = cached
		} else {
			toFetch = append(toFetch, b)
			results[b.PNR] = nil // placeholder
		}
	}

	if len(toFetch) > 0 {
		sem := make(chan struct{}, maxConcurrent)
		var mu sync.Mutex
		var wg sync.WaitGroup

		for _, b := range toFetch {
			wg.Add(1)
			sem <- struct{}{}
			go func(booking Booking) {
				defer wg.Done()
				defer func() { <-sem }()
				fmt.Printf("Fetching %s (%s %s)…\n", booking.PNR, booking.First, booking.Last)
				entry := FetchBooking(booking, !*visibleFlag)
				mu.Lock()
				results[booking.PNR] = entry
				cache[booking.PNR] = entry
				mu.Unlock()
			}(b)
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
