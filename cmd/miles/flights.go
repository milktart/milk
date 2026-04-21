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
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CountryCode string  `json:"country_code"`
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
	colRed    = "\033[1;91m"
)

type cpmRange struct {
	greatLo, greatHi float64 // great value band
	fairLo, fairHi   float64 // fair value band
	// above fairHi = bad value
}


// routeCPM returns CPM ranges for a given distance (miles) and whether the route is domestic.
// Returns economy and business ranges. Values calibrated from real market fares.
func routeCPM(distMi float64, domestic bool) (eco, biz cpmRange) {
	type band struct {
		ecoGreatLo, ecoGreatHi float64
		ecoFairLo, ecoFairHi   float64
		bizGreatLo, bizGreatHi float64
		bizFairLo, bizFairHi   float64
	}
	var b band
	if domestic {
		switch {
		case distMi < 500:
			// Short-haul domestic: higher per-mile due to fixed costs
			b = band{0.18, 0.28, 0.28, 0.50, 0.80, 1.20, 1.20, 2.00}
		case distMi < 1500:
			// Medium-haul domestic (e.g. AUS-LAX at 1242mi: real range $0.14–$0.39)
			b = band{0.14, 0.20, 0.20, 0.39, 0.65, 1.00, 1.00, 1.80}
		default:
			// Long-haul domestic: slightly lower per-mile than medium
			b = band{0.12, 0.18, 0.18, 0.32, 0.55, 0.85, 0.85, 1.50}
		}
	} else {
		switch {
		case distMi < 4000:
			// Intl short-haul (e.g. BOS/JFK-AMS ~3500mi)
			// Eco: ~0.13–0.28; Biz: real BOS $1950–$3450 → 0.57–1.00, JFK outlier to 2.34
			b = band{0.13, 0.18, 0.18, 0.28, 0.55, 0.80, 0.80, 1.20}
		case distMi < 7000:
			// Intl medium-haul (e.g. AUS-AMS 5429mi, LAX-LHR 5456mi)
			// Eco: real 0.127–0.249; Biz: real AUS-AMS $2200–$8900→0.41–1.64, LAX-LHR $3300–$8700→0.61–1.60
			b = band{0.12, 0.17, 0.17, 0.25, 0.40, 0.75, 0.75, 1.60}
		default:
			// Intl long-haul (7000+mi): lower CPM due to distance
			b = band{0.09, 0.14, 0.14, 0.20, 0.28, 0.55, 0.55, 1.10}
		}
	}
	eco = cpmRange{b.ecoGreatLo, b.ecoGreatHi, b.ecoFairLo, b.ecoFairHi}
	biz = cpmRange{b.bizGreatLo, b.bizGreatHi, b.bizFairLo, b.bizFairHi}
	return
}

// isBusiness returns true if the cabin string maps to business or first class.
func isBusiness(cabin string) bool {
	lower := strings.ToLower(cabin)
	return strings.Contains(lower, "business") || strings.Contains(lower, "first")
}

type priceInfo struct {
	greatLo, greatHi float64 // absolute prices
	fairLo, fairHi   float64
}

// cpmPriceInfo returns absolute price breakpoints for a leg.
func cpmPriceInfo(distMi float64, domestic bool, cabin string) priceInfo {
	eco, biz := routeCPM(distMi, domestic)
	r := eco
	if isBusiness(cabin) {
		r = biz
	}
	return priceInfo{
		greatLo: r.greatLo * distMi,
		greatHi: r.greatHi * distMi,
		fairLo:  r.fairLo * distMi,
		fairHi:  r.fairHi * distMi,
	}
}


func fmtPrice(v float64) string {
	n := int(math.Round(v))
	if n >= 1000 {
		return "$" + commaSep(n)
	}
	return fmt.Sprintf("$%d", n)
}

func commaSep(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	return commaSep(n/1000) + "," + fmt.Sprintf("%03d", n%1000)
}

type legResult struct {
	from       string
	to         string
	airline    string
	fareClass  string
	cabin      string
	distance   float64
	baseMiles  float64
	bonusMiles float64
	mqd        float64
	hasFare    bool
	domestic   bool
	price      priceInfo
}

// calculateAndDisplay performs all calculations and displays the results
func calculateAndDisplay(legs []Leg, loyaltyStatus string, userPrice float64) error {
	results := make([]legResult, 0, len(legs))
	var totalDistance, totalMQD, totalBase, totalBonus float64
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

		airline, fareClass, cabin := "", "", ""
		var baseMiles, bonusMiles, mqd float64
		hasFare := false

		if leg.AirlineFare != "" {
			for i, c := range leg.AirlineFare {
				if c == '.' {
					airline = leg.AirlineFare[:i]
					fareClass = leg.AirlineFare[i+1:]
					break
				}
			}
			if al, ok := earningsData[airline]; ok {
				if fe, ok := al[fareClass]; ok {
					baseMiles = distance * fe.Miles
					mqd = distance * fe.MQD
					cabin = fe.Cabin
					if fe.Bonus == 1 {
						bonusMiles = distance * statusBonuses[loyaltyStatus]
					}
					hasFare = true
					showEarnings = true
				}
			}
		}

		domestic := fromAirport.CountryCode == "US" && toAirport.CountryCode == "US"
		price := cpmPriceInfo(distance, domestic, cabin)

		totalDistance += distance
		totalMQD += mqd
		totalBase += baseMiles
		totalBonus += bonusMiles

		results = append(results, legResult{
			from:       leg.From,
			to:         leg.To,
			airline:    airline,
			fareClass:  fareClass,
			cabin:      cabin,
			distance:   distance,
			baseMiles:  baseMiles,
			bonusMiles: bonusMiles,
			mqd:        mqd,
			hasFare:    hasFare,
			domestic: domestic,
			price:    price,
		})
	}

	lpad := func(s string, w int) string {
		if len(s) >= w {
			return s
		}
		return strings.Repeat(" ", w-len(s)) + s
	}

	// Sum price ranges across all legs for a total bar
	var totalPrice priceInfo
	for _, r := range results {
		totalPrice.greatLo += r.price.greatLo
		totalPrice.greatHi += r.price.greatHi
		totalPrice.fairLo += r.price.fairLo
		totalPrice.fairHi += r.price.fairHi
	}
	showTotalBar := len(results) > 1

	// Compute max label widths to keep bar column consistent
	const barWidth = 12
	loW, hiW := 0, 0
	allPrices := make([]priceInfo, 0, len(results)+1)
	allPrices = append(allPrices, results[0].price) // at least one
	for _, r := range results {
		allPrices = append(allPrices, r.price)
	}
	if showTotalBar {
		allPrices = append(allPrices, totalPrice)
	}
	for _, p := range allPrices {
		lo := fmtPrice(p.greatLo)
		hi := fmtPrice(p.fairHi) + "+"
		if len(lo) > loW {
			loW = len(lo)
		}
		if len(hi) > hiW {
			hiW = len(hi)
		}
	}
	// bar column total visible width: loW + 1 + barWidth + 1 + hiW
	barColW := loW + 1 + barWidth + 1 + hiW

	priceBarStr := func(p priceInfo, plotPrice float64) string {
		// Allocate bar cells proportional to price zone widths within [greatLo, fairHi].
		totalRange := p.fairHi - p.greatLo
		g := int(math.Round(float64(barWidth) * (p.greatHi - p.greatLo) / totalRange))
		f := int(math.Round(float64(barWidth) * (p.fairHi - p.fairLo) / totalRange))
		b := barWidth - g - f
		if g < 1 {
			g = 1
		}
		if f < 1 {
			f = 1
		}
		if b < 1 {
			b = 1
			// re-trim from the largest segment
			if g >= f && g >= 1 {
				g--
			} else if f >= 1 {
				f--
			}
		}

		// Segments as rune slices so we can overwrite a character for the marker
		greens := []rune(strings.Repeat("█", g))
		fairs := []rune(strings.Repeat("█", f))
		bads := []rune(strings.Repeat("█", b))

		if plotPrice > 0 {
			// Bar scale: greatLo (left edge) → fairHi (right edge).
			// Anything at or beyond fairHi pins to the last cell.
			t := (plotPrice - p.greatLo) / (p.fairHi - p.greatLo)
			pos := int(math.Round(t * float64(barWidth-1)))
			if pos < 0 {
				pos = 0
			}
			if pos >= barWidth {
				pos = barWidth - 1
			}
			switch {
			case pos < g:
				greens[pos] = '▼'
			case pos < g+f:
				fairs[pos-g] = '▼'
			default:
				bads[pos-g-f] = '▼'
			}
		}

		bar := colGreen + string(greens) +
			colYellow + string(fairs) +
			colRed + string(bads) +
			colReset

		lo := fmtPrice(p.greatLo)
		hi := fmtPrice(p.fairHi) + "+"
		loPad := strings.Repeat(" ", loW-len(lo))
		hiPad := strings.Repeat(" ", hiW-len(hi))
		return loPad + lo + "  " + bar + "  " + hi + hiPad
	}

	const priceHdr = "PRICE RANGE"
	if barColW < len(priceHdr) {
		barColW = len(priceHdr)
	}

	sepLen := 0
	if showEarnings {
		sepLen = 4 + 7 + 2 + 4 + 2 + 7 + 2 + 8 + 2 + 6 + 2 + barColW + 4
	} else {
		sepLen = 4 + 7 + 2 + 7 + 2 + barColW + 4
	}
	sep := strings.Repeat("─", sepLen)

	fmt.Println()

	// Header line
	if showEarnings {
		fmt.Printf(colBold+" %-2s  %-7s  %-4s  %7s  %8s  %6s  %-*s\n"+colReset,
			"#", "SEG", "F/C", "DIST MI", "SKYMILES", "MQD",
			barColW+4, priceHdr)
	} else {
		fmt.Printf(colBold+" %-2s  %-7s  %7s  %-*s\n"+colReset,
			"#", "SEG", "DIST MI", barColW+4, priceHdr)
	}
	fmt.Println(colDim + sep + colReset)

	for i, r := range results {
		seg := r.from + "/" + r.to
		dist := fmt.Sprintf("%s", commaSep(int(math.Round(r.distance))))
		// Only plot the price marker on per-leg bars for single-leg trips
		barPrice := userPrice
		if showTotalBar {
			barPrice = 0
		}
		bar := priceBarStr(r.price, barPrice)

		if showEarnings {
			fc := "--"
			if r.airline != "" && r.fareClass != "" {
				fc = r.airline + "/" + r.fareClass
			}
			skm := "--"
			mqdStr := "--"
			if r.hasFare {
				skm = fmt.Sprintf("%s", commaSep(int(math.Round(r.baseMiles+r.bonusMiles))))
				mqdStr = fmt.Sprintf("%s", commaSep(int(math.Round(r.mqd))))
			}
			fmt.Printf(colCyan+" %2d"+colReset+"  %-7s  %-4s  %7s  %8s  %6s  %s\n",
				i+1, seg, fc, dist, skm, mqdStr, bar)
		} else {
			fmt.Printf(colCyan+" %2d"+colReset+"  %-7s  %7s  %s\n",
				i+1, seg, dist, bar)
		}
	}

	fmt.Println(colDim + sep + colReset)

	// Totals
	totalDist := fmt.Sprintf("%s", commaSep(int(math.Round(totalDistance))))
	if showEarnings {
		totalMiles := totalBase + totalBonus
		totalSkm := fmt.Sprintf("%s", commaSep(int(math.Round(totalBase+totalBonus))))
		totalMQDStr := fmt.Sprintf("%s", commaSep(int(math.Round(totalMQD))))
		if showTotalBar {
			totalBar := priceBarStr(totalPrice, userPrice)
			fmt.Printf(colBold+" %-2s  %-7s  %-4s  %7s  %8s  %6s  %s\n"+colReset,
				"", lpad("TOTALS", 7), "",
				totalDist, totalSkm, totalMQDStr, totalBar,
			)
		} else {
			fmt.Printf(colBold+" %-2s  %-7s  %-4s  %7s  %8s  %6s\n"+colReset,
				"", lpad("TOTALS", 7), "",
				totalDist, totalSkm, totalMQDStr,
			)
		}
		fmt.Println()
		fmt.Printf(colGreen+colBold+"  TOTAL SKYMILES  %s\n"+colReset,
			lpad(fmt.Sprintf("%s", commaSep(int(math.Round(totalMiles)))), 10))
		fmt.Printf(colGreen+colBold+"  TOTAL MQD       %s\n"+colReset,
			lpad(fmt.Sprintf("%s", commaSep(int(math.Round(totalMQD)))), 10))
		fmt.Printf(colDim+"  STATUS BONUS     %s  (%s)\n"+colReset,
			lpad(loyaltyStatus, 10), fmt.Sprintf("+%.0f%%", statusBonuses[loyaltyStatus]*100))
	} else {
		if showTotalBar {
			totalBar := priceBarStr(totalPrice, userPrice)
			fmt.Printf(colBold+" %-2s  %-7s  %7s  %s\n"+colReset,
				"", lpad("TOTALS", 7), totalDist, totalBar)
		} else {
			fmt.Printf(colBold+" %-2s  %-7s  %7s\n"+colReset,
				"", lpad("TOTALS", 7), totalDist)
		}
	}
	fmt.Println()

	return nil
}
