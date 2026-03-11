package flights

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheMaxAge           = 6 * time.Hour
	nearDepartureWindow   = 24 * time.Hour
	loyaltySessionMaxAge  = 4 * time.Hour
)

// CacheFile is the top-level structure persisted to cache.json.
type CacheFile struct {
	Entries         map[string]*CacheEntry          `json:"entries"`
	LoyaltySessions map[string]*LoyaltySessionCache `json:"loyalty_sessions,omitempty"` // keyed by member_id
}

// LoyaltySessionCache holds a captured JWT and related headers for a SkyMiles account.
type LoyaltySessionCache struct {
	Headers    map[string]string `json:"headers"`
	CapturedAt time.Time         `json:"captured_at"`
}

// IsLoyaltySessionFresh returns true if ls was captured within loyaltySessionMaxAge.
func IsLoyaltySessionFresh(ls *LoyaltySessionCache) bool {
	return ls != nil && time.Since(ls.CapturedAt) < loyaltySessionMaxAge
}

// CacheEntry is the persisted result for one PNR.
type CacheEntry struct {
	PNR            string            `json:"pnr"`
	Passenger      string            `json:"passenger"`
	FetchedAt      time.Time         `json:"fetched_at"`
	Flights        []Flight          `json:"flights"`
	RawError       string            `json:"raw_error,omitempty"`
	SessionHeaders map[string]string `json:"session_headers,omitempty"`
}

func cacheFile() string {
	return filepath.Join(flightsConfigDir(), "cache.json")
}

// LoadCache reads cache.json and returns a *CacheFile.
// Supports backward-compat migration from the old flat map[string]*CacheEntry format.
func LoadCache() *CacheFile {
	out := &CacheFile{
		Entries:         make(map[string]*CacheEntry),
		LoyaltySessions: make(map[string]*LoyaltySessionCache),
	}
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		return out
	}

	// Try new format first.
	if err := json.Unmarshal(data, out); err == nil && out.Entries != nil {
		if out.LoyaltySessions == nil {
			out.LoyaltySessions = make(map[string]*LoyaltySessionCache)
		}
		return out
	}

	// Fall back to old flat format: map[string]*CacheEntry
	var old map[string]*CacheEntry
	if err := json.Unmarshal(data, &old); err == nil {
		out.Entries = old
		return out
	}

	return out
}

// SaveCache writes the CacheFile to disk atomically.
func SaveCache(cache *CacheFile) error {
	path := cacheFile()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
	DistanceMiles int       `json:"distance_miles,omitempty"`
}
