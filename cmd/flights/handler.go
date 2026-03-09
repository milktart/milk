package flights

import (
	"flag"
	"fmt"
	"strings"
	"sync"
)

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
	addFlag := h.FlagSet.Bool("add", false, "Add a booking: --add PNR LASTNAME/FIRSTNAME")
	listFlag := h.FlagSet.Bool("list", false, "List all saved bookings grouped by passenger")
	pullFlag := h.FlagSet.Bool("pull", false, "Fetch a PNR without saving: --pull PNR LASTNAME/FIRSTNAME")
	refreshFlag := h.FlagSet.Bool("refresh", false, "Force re-fetch bookings (ignore cache); optionally pass a PNR as the next argument to refresh only that booking")
	cachedFlag := h.FlagSet.Bool("show-cached", false, "Display cached data only, do not hit delta.com")
	visibleFlag := h.FlagSet.Bool("visible", false, "Run browser in visible (non-headless) mode")

	h.FlagSet.Usage = func() {
		fmt.Fprintf(h.FlagSet.Output(), "Usage: milk flights [options]\n\n")
		fmt.Println("Fetch and display your Delta flight bookings from delta.com.\n")
		fmt.Println("Booking data is stored in bookings.json next to the binary.")
		fmt.Println("Results are cached for 6 hours (always re-fetched within 24h of departure).\n")
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

	// --list: show all saved bookings grouped by passenger
	if *listFlag {
		return ListBookings()
	}

	// --pull PNR LASTNAME/FIRSTNAME: one-off fetch, no cache or bookings written
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
		fmt.Printf("Fetching %s (%s)…\n", pnr, formatName(first, last))
		entry := FetchBooking(Booking{PNR: pnr, First: first, Last: last}, !*visibleFlag)
		DisplayAll([]*CacheEntry{entry})
		return nil
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
		fmt.Println("Add one with: milk flights --add CONFIRMATION FIRSTNAME LASTNAME")
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
		cached := cache[b.PNR]
		forceRefresh := *refreshFlag && (refreshPNR == "" || b.PNR == refreshPNR)
		if !forceRefresh && IsCacheFresh(cached) {
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
				fmt.Printf("Fetching %s (%s)…\n", booking.PNR, formatName(booking.First, booking.Last))
				var entry *CacheEntry
				if prior := cache[booking.PNR]; HasAPICredentials(prior) {
					entry = FetchBookingFromAPI(booking, prior.AuthToken)
				} else {
					entry = FetchBooking(booking, !*visibleFlag)
				}
				mu.Lock()
				if entry.RawError != "" || len(entry.Flights) == 0 {
					if prior := cache[booking.PNR]; prior != nil {
						if entry.RawError != "" {
							fmt.Printf("Warning: refresh failed for %s (%s), keeping cached data: %s\n", booking.PNR, formatName(booking.First, booking.Last), entry.RawError)
						} else {
							fmt.Printf("Warning: refresh returned no flights for %s (%s), keeping cached data\n", booking.PNR, formatName(booking.First, booking.Last))
						}
						entry = prior
					}
				}
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
