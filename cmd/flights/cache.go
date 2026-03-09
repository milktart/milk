package flights

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheMaxAge       = 6 * time.Hour
	nearDepartureWindow = 24 * time.Hour
)

// CacheEntry is the persisted result for one PNR.
type CacheEntry struct {
	PNR       string    `json:"pnr"`
	Passenger string    `json:"passenger"`
	FetchedAt time.Time `json:"fetched_at"`
	Flights   []Flight  `json:"flights"`
	RawError  string    `json:"raw_error,omitempty"`
}

func cacheFile() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(exe), "cache.json")
}

// LoadCache reads cache.json. Returns an empty map on any error.
func LoadCache() map[string]*CacheEntry {
	out := make(map[string]*CacheEntry)
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		return out
	}
	_ = json.Unmarshal(data, &out)
	return out
}

// SaveCache writes the cache map to disk atomically.
func SaveCache(cache map[string]*CacheEntry) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp := cacheFile() + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, cacheFile())
}

// IsCacheFresh returns true if the entry is recent enough to skip a re-fetch.
func IsCacheFresh(e *CacheEntry) bool {
	if e == nil {
		return false
	}
	if time.Since(e.FetchedAt) > cacheMaxAge {
		return false
	}
	// Always re-fetch if any leg departs within the near-departure window.
	now := time.Now().UTC()
	for _, f := range e.Flights {
		if !f.DepartureTime.IsZero() {
			until := f.DepartureTime.Sub(now)
			if until > 0 && until < nearDepartureWindow {
				return false
			}
		}
	}
	return true
}

// Flight is the structured data for a single flight leg × passenger.
type Flight struct {
	FlightNumber  string    `json:"flight_number"`
	Departure     string    `json:"departure"`
	Arrival       string    `json:"arrival"`
	DepartureTime time.Time `json:"departure_datetime,omitempty"`
	Seat          string    `json:"seat"`
	Cabin         string    `json:"cabin"`
	Aircraft      string    `json:"aircraft"`
	Status        string    `json:"status"`
	PaxIndex      int       `json:"pax_index"`
	NPax          int       `json:"n_pax"`
	PassengerName string    `json:"passenger_name"`
}
