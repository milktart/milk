package miles

import (
	"encoding/json"
	_ "embed"
	"fmt"
	"math"
	"strings"
)

// Airport represents an airport with its geographical coordinates
type Airport struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// FareClassEarnings represents the earnings for a specific fare class
type FareClassEarnings struct {
	MQD   float64 `json:"mqd"`
	Miles float64 `json:"miles"`
	Bonus int     `json:"bonus"`
	Cabin string  `json:"cabin"`
}

// EarningsData holds the earnings information for all airlines and fare classes
type EarningsData map[string]map[string]FareClassEarnings

var (
	//go:embed airports.json
	airportsJSON []byte

	//go:embed fareclasses.json
	fareClassesJSON []byte

	airports      map[string]Airport
	earningsData  EarningsData
	statusBonuses = map[string]float64{
		"DM":   1.2,
		"PM":   0.8,
		"GM":   0.6,
		"SM":   0.4,
		"None": 0.0,
	}
)

func init() {
	// Parse airports JSON
	if err := json.Unmarshal(airportsJSON, &airports); err != nil {
		panic(fmt.Sprintf("failed to parse airports.json: %v", err))
	}

	// Parse fare classes JSON
	if err := json.Unmarshal(fareClassesJSON, &earningsData); err != nil {
		panic(fmt.Sprintf("failed to parse fareclasses.json: %v", err))
	}
}

// Earnings represents calculated miles and MQD earnings
type Earnings struct {
	MQD   float64
	Miles float64
}

// calculateDistance computes the distance in miles between two airports using Haversine formula
func calculateDistance(from, to Airport) float64 {
	const earthRadiusMeters = 6371000.0
	const metersToMiles = 0.000621371

	// Convert to radians
	lat1 := degreesToRadians(from.Latitude)
	lon1 := degreesToRadians(from.Longitude)
	lat2 := degreesToRadians(to.Latitude)
	lon2 := degreesToRadians(to.Longitude)

	// Haversine formula
	dlat := lat2 - lat1
	dlon := lon2 - lon1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dlon/2)*math.Sin(dlon/2)

	c := 2 * math.Asin(math.Sqrt(a))
	distMeters := earthRadiusMeters * c

	return distMeters * metersToMiles
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180.0
}

// calculateEarnings computes MQD and miles for a given airline fare class and distance
func calculateEarnings(airlineFare string, distance float64, loyaltyStatus string) Earnings {
	if airlineFare == "" {
		return Earnings{MQD: 0, Miles: 0}
	}

	// Parse airline and fare class
	airline := ""
	fareClass := ""
	for i, c := range airlineFare {
		if c == '.' {
			airline = airlineFare[:i]
			fareClass = airlineFare[i+1:]
			break
		}
	}

	if airline == "" || fareClass == "" {
		return Earnings{MQD: 0, Miles: 0}
	}

	// Look up earnings
	airlineEarnings, ok := earningsData[airline]
	if !ok {
		return Earnings{MQD: 0, Miles: 0}
	}

	earnings, ok := airlineEarnings[fareClass]
	if !ok {
		return Earnings{MQD: 0, Miles: 0}
	}

	mqd := distance * earnings.MQD
	baseMiles := distance * earnings.Miles
	bonus := 0.0
	if earnings.Bonus == 1 {
		statusBonus := statusBonuses[loyaltyStatus]
		bonus = distance * statusBonus
	}

	totalMiles := baseMiles + bonus

	return Earnings{MQD: mqd, Miles: totalMiles}
}

const (
	colReset  = "\033[0m"
	colBold   = "\033[1m"
	colDim    = "\033[2m"
	colCyan   = "\033[1;96m"
	colYellow = "\033[1;93m"
	colGreen  = "\033[1;92m"
)

type legResult struct {
	segment  string
	distance float64
	mqd      float64
	miles    float64
	hasFare  bool
}

// calculateAndDisplay performs all calculations and displays the results
func calculateAndDisplay(legs []Leg, loyaltyStatus string) error {
	results := make([]legResult, 0, len(legs))
	var totalDistance, totalMQD, totalMiles float64
	showEarnings := false

	for _, leg := range legs {
		fromAirport, ok := airports[leg.From]
		if !ok {
			return fmt.Errorf("unknown airport code: %s", leg.From)
		}
		toAirport, ok := airports[leg.To]
		if !ok {
			return fmt.Errorf("unknown airport code: %s", leg.To)
		}

		distance := calculateDistance(fromAirport, toAirport)
		earnings := calculateEarnings(leg.AirlineFare, distance, loyaltyStatus)
		hasFare := leg.AirlineFare != "" && (earnings.MQD > 0 || earnings.Miles > 0)

		if hasFare {
			showEarnings = true
		}

		totalDistance += distance
		totalMQD += earnings.MQD
		totalMiles += earnings.Miles

		results = append(results, legResult{
			segment:  leg.From + " → " + leg.To,
			distance: distance,
			mqd:      earnings.MQD,
			miles:    earnings.Miles,
			hasFare:  hasFare,
		})
	}

	// Column widths
	const segW, distW, earnW = 11, 14, 12

	divider := func(left, mid, right, horiz string) string {
		seg := strings.Repeat(horiz, segW+2)
		dist := strings.Repeat(horiz, distW+2)
		if showEarnings {
			earn := strings.Repeat(horiz, earnW+2)
			return left + seg + mid + dist + mid + earn + mid + earn + right
		}
		return left + seg + mid + dist + right
	}

	pad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return s + strings.Repeat(" ", w-len(s))
	}
	lpad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return strings.Repeat(" ", w-len(s)) + s
	}

	fmt.Println()
	fmt.Println(colCyan + colBold + "  Flight Summary" + colReset)
	fmt.Println()

	// Top border
	fmt.Println(colDim + divider("┌", "┬", "┐", "─") + colReset)

	// Header
	if showEarnings {
		fmt.Printf(colCyan+"│ %s │ %s │ %s │ %s │\n"+colReset,
			colBold+pad("Segment", segW)+colCyan,
			lpad("Distance (mi)", distW),
			lpad("MQDs", earnW),
			lpad("SkyMiles", earnW),
		)
	} else {
		fmt.Printf(colCyan+"│ %s │ %s │\n"+colReset,
			colBold+pad("Segment", segW)+colCyan,
			lpad("Distance (mi)", distW),
		)
	}

	// Header divider
	fmt.Println(colDim + divider("├", "┼", "┤", "─") + colReset)

	// Rows
	for _, r := range results {
		dist := fmt.Sprintf("%.0f mi", r.distance)
		if showEarnings {
			mqd := "—"
			skm := "—"
			if r.hasFare {
				mqd = fmt.Sprintf("%.0f", r.mqd)
				skm = fmt.Sprintf("%.0f", r.miles)
			}
			fmt.Printf("│ %s │ %s │ %s │ %s │\n",
				colYellow+pad(r.segment, segW)+colReset,
				lpad(dist, distW),
				lpad(mqd, earnW),
				lpad(skm, earnW),
			)
		} else {
			fmt.Printf("│ %s │ %s │\n",
				colYellow+pad(r.segment, segW)+colReset,
				lpad(dist, distW),
			)
		}
	}

	// Footer divider
	fmt.Println(colDim + divider("├", "┼", "┤", "─") + colReset)

	// Totals row
	totalDist := fmt.Sprintf("%.0f mi", totalDistance)
	if showEarnings {
		fmt.Printf("│ %s │ %s │ %s │ %s │\n",
			colBold+colGreen+pad("Total", segW)+colReset,
			colBold+lpad(totalDist, distW)+colReset,
			colBold+lpad(fmt.Sprintf("%.0f", totalMQD), earnW)+colReset,
			colBold+lpad(fmt.Sprintf("%.0f", totalMiles), earnW)+colReset,
		)
	} else {
		fmt.Printf("│ %s │ %s │\n",
			colBold+colGreen+pad("Total", segW)+colReset,
			colBold+lpad(totalDist, distW)+colReset,
		)
	}

	// Bottom border
	fmt.Println(colDim + divider("└", "┴", "┘", "─") + colReset)
	fmt.Println()

	return nil
}
