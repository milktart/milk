package flights

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

var (
	seatRE   = regexp.MustCompile(`\b(\d{1,3}[A-F])\b`)
	aircraftRE = regexp.MustCompile(`(Airbus|Boeing|Embraer|McDonnell|CRJ|ATR)\s[\w\s-]+`)
	flightRE = regexp.MustCompile(`\b([A-Z]{1,3}\d{1,4})\b`)
	iataRE   = regexp.MustCompile(`\(([A-Z]{3})\)`)
	timeRE   = regexp.MustCompile(`\b(\d{1,2}:\d{2}\s*[AaPp][Mm])\b`)
	dateRE   = regexp.MustCompile(`(?i)(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)[a-z]*,\s+([A-Za-z]{3}\s+\d{1,2})`)
	fareRE   = regexp.MustCompile(`\(([A-Z]{1,3})\)\s*$`)
)

func extractFlights(page *rod.Page, fallbackName string) ([]Flight, error) {
	cards, err := page.Elements(".td-flight-card")
	if err != nil || len(cards) == 0 {
		return nil, nil
	}

	var flights []Flight

	for _, card := range cards {
		visible, _ := card.Visible()
		if !visible {
			continue
		}

		// --- Status ---
		status := ""
		if el, err := card.Element(".td-segment-status__text, .td-segment-status"); err == nil {
			status = strings.TrimSpace(el.MustText())
		}

		// --- Flight numbers ---
		fnEls, _ := card.Elements(".td-flight-number")
		var flightNumbers []string
		for _, el := range fnEls {
			raw := strings.TrimSpace(el.MustText())
			if m := flightRE.FindStringSubmatch(raw); m != nil {
				flightNumbers = append(flightNumbers, m[1])
			} else if raw != "" {
				flightNumbers = append(flightNumbers, raw)
			}
		}

		// --- Aircraft types ---
		summaries, _ := card.Elements("idp-flight-summary")
		var aircraftList []string
		for _, s := range summaries {
			text := strings.TrimSpace(s.MustText())
			if m := aircraftRE.FindString(text); m != "" {
				// Trim anything after newline or "(Enhanced"
				ac := strings.SplitN(m, "\n", 2)[0]
				ac = strings.SplitN(ac, "(Enhanced", 2)[0]
				aircraftList = append(aircraftList, strings.TrimSpace(ac))
			} else {
				aircraftList = append(aircraftList, "")
			}
		}

		// --- Seat + cabin blocks (one per leg) ---
		type legSeats struct {
			seats []string
			cabin string
		}
		seatBlocks, _ := card.Elements(".td-card-header-seats-legs")
		var legsPax []legSeats
		for _, block := range seatBlocks {
			vis, _ := block.Visible()
			if !vis {
				continue
			}
			text := strings.TrimSpace(block.MustText())
			seats := seatRE.FindAllString(text, -1)
			cabin := ""
			if cabinEl, err := block.Element(".td-cabin-name"); err == nil {
				cabin = strings.TrimSpace(cabinEl.MustText())
			}
			if len(seats) == 0 {
				seats = []string{"—"}
			}
			legsPax = append(legsPax, legSeats{seats: seats, cabin: cabin})
		}

		nLegs := len(flightNumbers)
		if nLegs == 0 {
			nLegs = 1
		}
		for len(legsPax) < nLegs {
			legsPax = append(legsPax, legSeats{seats: []string{"—"}, cabin: ""})
		}
		for len(aircraftList) < nLegs {
			aircraftList = append(aircraftList, "")
		}

		nPax := 1
		for _, lp := range legsPax {
			if len(lp.seats) > nPax {
				nPax = len(lp.seats)
			}
		}

		// --- Expand "Show Flight Details" toggle for times + pax names ---
		_ = rod.Try(func() {
			toggle := card.MustElement(".td-toggle-details")
			label, _ := toggle.Attribute("aria-label")
			if label != nil && strings.Contains(strings.ToUpper(*label), "SHOW") {
				toggle.MustClick()
				time.Sleep(500 * time.Millisecond)
			}
		})

		// --- Per-leg dep/arr times ---
		type legTime struct{ dep, arr string }
		legEls, _ := card.Elements(".td-flight-card__leg")
		var legTimes []legTime
		for _, legEl := range legEls {
			noPrint, err := legEl.Element(".td-no-print")
			if err != nil {
				continue
			}
			segPoint, err := noPrint.Element(".td-flight-segment-point")
			if err != nil {
				continue
			}
			dep := legPointText(segPoint, "idp-flight-point.td-flight-segment--left")
			arr := legPointText(segPoint, "idp-flight-point.td-flight-segment-right")
			if dep != "" || arr != "" {
				legTimes = append(legTimes, legTime{dep, arr})
			}
		}

		// Fallback: card-level origin/dest
		if len(legTimes) == 0 {
			dep, arr := "", ""
			if pc, err := card.Element(".td-flight-point-container"); err == nil {
				if el, err := pc.Element("idp-departure-arrival-info.left-alignment"); err == nil {
					dep = strings.TrimSpace(el.MustText())
				}
				if el, err := pc.Element("idp-departure-arrival-info.right-alignment"); err == nil {
					arr = strings.TrimSpace(el.MustText())
				}
			}
			for i := 0; i < nLegs; i++ {
				legTimes = append(legTimes, legTime{dep, arr})
			}
		}
		for len(legTimes) < nLegs {
			legTimes = append(legTimes, legTime{})
		}

		// --- Passenger names ---
		paxNames := []string{}
		nameEls, _ := card.Elements(".td-no-print .td-passenger__title--name")
		for _, el := range nameEls {
			vis, _ := el.Visible()
			if vis {
				paxNames = append(paxNames, strings.ToUpper(strings.TrimSpace(el.MustText())))
			}
		}
		for len(paxNames) < nPax {
			paxNames = append(paxNames, strings.ToUpper(fallbackName))
		}

		// --- Emit one row per leg per PAX ---
		for legIdx, fltNum := range flightNumbers {
			lp := legsPax[legIdx]
			ac := aircraftList[legIdx]
			lt := legTimes[legIdx]

			for paxIdx := 0; paxIdx < nPax; paxIdx++ {
				seat := "—"
				if paxIdx < len(lp.seats) {
					seat = lp.seats[paxIdx]
				}
				pax := fallbackName
				if paxIdx < len(paxNames) {
					pax = paxNames[paxIdx]
				}
				flights = append(flights, Flight{
					FlightNumber:  fltNum,
					Departure:     lt.dep,
					Arrival:       lt.arr,
					DepartureTime: parseDepartureTime(lt.dep),
					Seat:          seat,
					Cabin:         lp.cabin,
					Aircraft:      ac,
					Status:        status,
					PaxIndex:      paxIdx,
					NPax:          nPax,
					PassengerName: pax,
				})
			}
		}
	}

	return flights, nil
}

// legPointText extracts combined time+date+city text from a dep or arr flight point element.
func legPointText(parent *rod.Element, selector string) string {
	point, err := parent.Element(selector)
	if err != nil {
		return ""
	}
	var parts []string
	for _, sel := range []string{".td-flight-point-time", ".td-flight-point-date", ".td-flight-point-city, .td-train-point-city"} {
		if el, err := point.Element(sel); err == nil {
			if t := strings.TrimSpace(el.MustText()); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}

// parseDepartureTime attempts to parse the raw departure string into a time.Time
// for cache freshness checks. Returns zero value if unparseable.
func parseDepartureTime(raw string) time.Time {
	timeM := timeRE.FindString(raw)
	dateM := dateRE.FindStringSubmatch(raw)
	if timeM == "" || dateM == nil {
		return time.Time{}
	}
	timeNorm := strings.ToUpper(strings.ReplaceAll(timeM, " ", ""))
	dateParts := strings.Fields(strings.ToUpper(dateM[1]))
	if len(dateParts) != 2 {
		return time.Time{}
	}
	s := fmt.Sprintf("%s %s %d %s", dateParts[0], dateParts[1], time.Now().Year(), timeNorm)
	t, err := time.Parse("Jan 2 2006 3:04PM", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

// ParseIATA extracts the 3-letter airport code from a raw dep/arr string.
func ParseIATA(raw string) string {
	if m := iataRE.FindStringSubmatch(raw); m != nil {
		return m[1]
	}
	return "—"
}

// ParseDT formats the raw dep/arr string to "DDMMM HHMM".
func ParseDT(raw string) string {
	timeM := timeRE.FindString(raw)
	dateM := dateRE.FindStringSubmatch(raw)
	if timeM == "" || dateM == nil {
		return "—"
	}
	timeNorm := strings.ToUpper(strings.ReplaceAll(timeM, " ", ""))
	t, err := time.Parse("3:04PM", timeNorm)
	if err != nil {
		t2, err2 := time.Parse("15:04PM", timeNorm)
		if err2 != nil {
			return timeM
		}
		t = t2
	}
	hhmm := t.Format("1504")

	dateParts := strings.Fields(strings.ToUpper(dateM[1]))
	if len(dateParts) != 2 {
		return "—"
	}
	day := dateParts[1]
	mon := dateParts[0]
	if len(day) == 1 {
		day = "0" + day
	}
	return day + mon + " " + hhmm
}

// ParseFare extracts the booking class from a cabin string like "Delta First Classic (Z)".
func ParseFare(cabin string) string {
	if m := fareRE.FindStringSubmatch(cabin); m != nil {
		return m[1]
	}
	return "—"
}

var acAbbrev = map[string]string{
	"Airbus A220-100":   "A220-100",
	"Airbus A220-300":   "A220-300",
	"Airbus A319":       "A319",
	"Airbus A320":       "A320",
	"Airbus A321":       "A321",
	"Boeing 717":        "B717",
	"Boeing 737-800":    "B738",
	"Boeing 737-900":    "B739",
	"Boeing 757-200":    "B752",
	"Boeing 757-300":    "B753",
	"Boeing 767-300":    "B763",
	"Boeing 767-300ER":  "B763",
	"Boeing 767-400":    "B764",
	"Boeing 777-200":    "B772",
	"Boeing 777-200LR":  "B77L",
	"Boeing 777-200ER":  "B772",
	"Boeing 787-8":      "B788",
	"Boeing 787-9":      "B789",
	"Boeing 787-10":     "B78X",
	"Embraer 170":       "E170",
	"Embraer 175":       "E175",
	"Embraer 190":       "E190",
	"Airbus A330-900":   "A339",
	"Airbus A350-900":   "A359",
	"Airbus A350-1000":  "A35K",
}

// AbbrevAC shortens aircraft type strings.
func AbbrevAC(ac string) string {
	if short, ok := acAbbrev[ac]; ok {
		return short
	}
	return ac
}
