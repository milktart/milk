package flights

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const apiURL = "https://mytrips-api.delta.com/v1/mytrips/travelreservations"

type apiRequest struct {
	Using           string `json:"using"`
	ConfirmationNum string `json:"confirmationNum"`
	GivenNames      string `json:"givenNames"`
	Surname         string `json:"surname"`
}

// HasAPICredentials reports whether the entry has an auth token for direct API calls.
func HasAPICredentials(e *CacheEntry) bool {
	return e != nil && e.AuthToken != ""
}

// FetchBookingFromAPI fetches booking data directly from the Delta API using a
// bearer token captured from a prior browser session.
// Returns a new CacheEntry; on error the entry will have RawError set.
func FetchBookingFromAPI(b Booking, authToken string) *CacheEntry {
	entry := &CacheEntry{
		PNR:       b.PNR,
		Passenger: formatName(b.First, b.Last),
		FetchedAt: time.Now().UTC(),
		AuthToken: authToken,
	}

	payload, err := json.Marshal(apiRequest{
		Using:          "CONFIRMATION",
		ConfirmationNum: b.PNR,
		GivenNames:     b.First,
		Surname:        b.Last,
	})
	if err != nil {
		entry.RawError = fmt.Sprintf("api marshal: %v", err)
		return entry
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		entry.RawError = fmt.Sprintf("api request: %v", err)
		return entry
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Origin", "https://www.delta.com")
	req.Header.Set("Referer", "https://www.delta.com/us/en/my-trips/trip-details")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Authorization", authToken)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		entry.RawError = fmt.Sprintf("api call: %v", err)
		return entry
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		entry.RawError = fmt.Sprintf("api read: %v", err)
		return entry
	}

	if resp.StatusCode != http.StatusOK {
		entry.RawError = fmt.Sprintf("api status %d: %s", resp.StatusCode, truncate(string(body), 120))
		return entry
	}

	flights, err := parseAPIResponse(body, entry.Passenger)
	if err != nil {
		entry.RawError = fmt.Sprintf("api parse: %v", err)
		return entry
	}
	entry.Flights = flights
	return entry
}

// parseAPIResponse maps the Delta API JSON response into the existing Flight slice.
func parseAPIResponse(body []byte, fallbackPassenger string) ([]Flight, error) {
	var raw struct {
		TravelReservations []struct {
			Segments []struct {
				FlightNumber  string `json:"flightNumber"`
				DepartureCode string `json:"departureAirportCode"`
				ArrivalCode   string `json:"arrivalAirportCode"`
				DepartureTime string `json:"scheduledDepartureLocalTs"`
				ArrivalTime   string `json:"scheduledArrivalLocalTs"`
				Aircraft      string `json:"aircraftType"`
				Status        string `json:"segmentStatus"`
				DistanceMiles int    `json:"distanceMiles"`
				Travelers     []struct {
					FirstName       string `json:"firstName"`
					LastName        string `json:"lastName"`
					Seat            string `json:"seatNumber"`
					Cabin           string `json:"cabinName"`
					FareClass     string `json:"fareClass"`
					FareBasisCode string `json:"fareBasisCode"`
				} `json:"travelers"`
			} `json:"segments"`
		} `json:"travelReservations"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var flights []Flight
	for _, res := range raw.TravelReservations {
		for _, seg := range res.Segments {
			nPax := len(seg.Travelers)
			if nPax == 0 {
				nPax = 1
			}
			dep := formatAPIAirportTime(seg.DepartureCode, seg.DepartureTime)
			arr := formatAPIAirportTime(seg.ArrivalCode, seg.ArrivalTime)

			for i := 0; i < nPax; i++ {
				seat, cabin, paxName := "—", "", fallbackPassenger
				if i < len(seg.Travelers) {
					t := seg.Travelers[i]
					if t.Seat != "" {
						seat = t.Seat
					}
					fareCode := t.FareBasisCode
					if fareCode == "" {
						fareCode = t.FareClass
					}
					if fareCode != "" {
						cabin = t.Cabin + " (" + fareCode + ")"
					} else {
						cabin = t.Cabin
					}
					if t.FirstName != "" || t.LastName != "" {
						paxName = formatName(t.FirstName, t.LastName)
					}
				}
				flights = append(flights, Flight{
					FlightNumber:  seg.FlightNumber,
					Departure:     dep,
					Arrival:       arr,
					DepartureTime: parseAPITime(seg.DepartureTime),
					Seat:          seat,
					Cabin:         cabin,
					Aircraft:      seg.Aircraft,
					Status:        seg.Status,
					PaxIndex:      i,
					NPax:          nPax,
					PassengerName: paxName,
					DistanceMiles: seg.DistanceMiles,
				})
			}
		}
	}
	return flights, nil
}

// formatAPIAirportTime builds the same "HH:MM DDMMM\n(XXX)" style string
// the scraper produces, so the existing display/parse code works unchanged.
func formatAPIAirportTime(iata, ts string) string {
	t := parseAPITime(ts)
	if t.IsZero() || iata == "" {
		return "—"
	}
	return fmt.Sprintf("%s\n(%s)", t.Format("02Jan 1504"), iata)
}

func parseAPITime(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	// Try common ISO-like formats Delta may return.
	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}
