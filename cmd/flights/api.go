package flights

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const apiURL = "https://mytrips-api.delta.com/v1/mytrips/travelreservations"

type apiRequest struct {
	Using           string `json:"using"`
	ConfirmationNum string `json:"confirmationNum"`
	GivenNames      string `json:"givenNames"`
	Surname         string `json:"surname"`
}

// FetchBookingFromAPI fetches booking data directly from the Delta API using
// session headers captured from a prior browser session.
// Returns a new CacheEntry; on error the entry will have RawError set.
func FetchBookingFromAPI(b Booking, sessionHeaders map[string]string) *CacheEntry {
	entry := &CacheEntry{
		PNR:            b.PNR,
		Passenger:      formatName(b.First, b.Last),
		FetchedAt:      time.Now().UTC(),
		SessionHeaders: sessionHeaders,
	}

	payload, err := json.Marshal(apiRequest{
		Using:           "CONFIRMATION",
		ConfirmationNum: b.PNR,
		GivenNames:      b.First,
		Surname:         b.Last,
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
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Origin", "https://www.delta.com")
	req.Header.Set("Referer", "https://www.delta.com/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for _, k := range []string{"Cookie", "appId", "channelId", "transactionid"} {
		if v, ok := sessionHeaders[k]; ok && v != "" {
			if k == "Cookie" {
				v = cookieWithoutBmSs(v)
			}
			req.Header.Set(k, v)
		}
	}

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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
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

// parseAPIResponse maps the Delta API JSON response into Flight slices.
func parseAPIResponse(body []byte, fallbackPassenger string) ([]Flight, error) {
	var raw struct {
		TravelReservations []struct {
			Passengers []struct {
				GivenNames string `json:"givenNames"`
				Surname    string `json:"surname"`
				PassengerTrips []struct {
					TripID           int `json:"tripId"`
					PassengerSegments []struct {
						SegmentID        int    `json:"segmentId"`
						BookedCabinClass struct {
							Code string `json:"code"`
						} `json:"bookedCabinClass"`
						PassengerLegs []struct {
							LegID           int `json:"legId"`
							SeatAssignments []struct {
								Seat struct {
									Number string `json:"number"`
								} `json:"seat"`
							} `json:"seatAssignments"`
						} `json:"passengerLegs"`
					} `json:"passengerSegments"`
				} `json:"passengerTrips"`
			} `json:"passengers"`
			Trips []struct {
				TripID        int `json:"tripId"`
				DistanceMiles int `json:"distanceMiles"`
				Segments []struct {
					SegmentID int `json:"segmentId"`
					FlightNum string `json:"flightNum"`
					MarketingSegment struct {
						FareBasisCode string `json:"fareBasisCode"`
					} `json:"marketingSegment"`
					Legs []struct {
						LegID   int `json:"legId"`
						FlightNum string `json:"flightNum"`
						TransportOrigin struct {
							ScheduledDepartureLocalDateTime string `json:"scheduledDepartureLocalDateTime"`
							Station struct {
								Code string `json:"code"`
							} `json:"station"`
						} `json:"transportOrigin"`
						TransportDestination struct {
							ScheduledArrivalLocalDateTime string `json:"scheduledArrivalLocalDateTime"`
							Station struct {
								Code string `json:"code"`
							} `json:"station"`
						} `json:"transportDestination"`
						TransportEquipment struct {
							Name string `json:"name"`
						} `json:"transportEquipment"`
						Status          string `json:"status"`
						DistanceMiles   int    `json:"distanceMiles"`
						MealServiceCode string `json:"mealServiceCode"`
					} `json:"legs"`
				} `json:"segments"`
			} `json:"trips"`
		} `json:"travelReservations"`
	}

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var flights []Flight
	for _, res := range raw.TravelReservations {
		nPax := len(res.Passengers)
		if nPax == 0 {
			nPax = 1
		}

		for _, trip := range res.Trips {
			for _, seg := range trip.Segments {
				fareBasisCode := seg.MarketingSegment.FareBasisCode

				for _, leg := range seg.Legs {
					dep := formatAPIAirportTime(
						leg.TransportOrigin.Station.Code,
						leg.TransportOrigin.ScheduledDepartureLocalDateTime,
					)
					arr := formatAPIAirportTime(
						leg.TransportDestination.Station.Code,
						leg.TransportDestination.ScheduledArrivalLocalDateTime,
					)
					status := strings.ReplaceAll(leg.Status, "_", " ")
					dist := leg.DistanceMiles
					if dist == 0 {
						dist = trip.DistanceMiles
					}
					fltNum := "DL" + leg.FlightNum

					for paxIdx, pax := range res.Passengers {
						paxName := formatName(pax.GivenNames, pax.Surname)
						seat := "—"
						cabin := ""

						for _, pt := range pax.PassengerTrips {
							if pt.TripID != trip.TripID {
								continue
							}
							for _, ps := range pt.PassengerSegments {
								if ps.SegmentID != seg.SegmentID {
									continue
								}
								cabin = ps.BookedCabinClass.Code
								if fareBasisCode != "" {
									cabin = cabin + " (" + fareBasisCode + ")"
								}
								for _, pl := range ps.PassengerLegs {
									if pl.LegID != leg.LegID {
										continue
									}
									if len(pl.SeatAssignments) > 0 {
										seat = pl.SeatAssignments[0].Seat.Number
									}
								}
							}
						}

						if seat == "" {
							seat = "—"
						}

						flights = append(flights, Flight{
							FlightNumber:  fltNum,
							Departure:     dep,
							Arrival:       arr,
							DepartureTime: parseAPITime(leg.TransportOrigin.ScheduledDepartureLocalDateTime),
							Seat:          seat,
							Cabin:         cabin,
							Aircraft:      leg.TransportEquipment.Name,
							Status:        status,
							PaxIndex:      paxIdx,
							NPax:          nPax,
							PassengerName: paxName,
							DistanceMiles: dist,
						})
					}

					// No passengers in response — emit one row with fallback name.
					if nPax == 0 {
						flights = append(flights, Flight{
							FlightNumber:  fltNum,
							Departure:     dep,
							Arrival:       arr,
							DepartureTime: parseAPITime(leg.TransportOrigin.ScheduledDepartureLocalDateTime),
							Seat:          "—",
							Aircraft:      leg.TransportEquipment.Name,
							Status:        status,
							NPax:          1,
							PassengerName: fallbackPassenger,
							DistanceMiles: dist,
						})
					}
				}
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
	for _, layout := range []string{
		"2006-01-02T15:04:05.0",
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
