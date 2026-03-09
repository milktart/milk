package tracker

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

// statusColor maps a flight status string to an ANSI color escape.
func statusColor(status string) (string, string) {
	lower := strings.ToLower(status)
	reset := "\033[0m"
	switch {
	case strings.Contains(lower, "on time"), strings.Contains(lower, "on_time"):
		return "\033[1;92m", reset // bright green
	case strings.Contains(lower, "schedule change"), strings.Contains(lower, "delayed"):
		return "\033[1;93m", reset // bright yellow
	case strings.Contains(lower, "cancel"):
		return "\033[1;91m", reset // bright red
	default:
		return "\033[1;96m", reset // bright cyan
	}
}

type displayRow struct {
	sortKey time.Time
	pnr     string
	flt     string
	org     string
	dst     string
	dep     string
	arr     string
	pax     string
	seat    string
	cls     string
	ac      string
	status  string
}

// DisplayAll renders a unified, departure-sorted table across all cache entries.
func DisplayAll(entries []*CacheEntry) {
	var rows []displayRow
	var errors []string

	for _, e := range entries {
		if e == nil {
			continue
		}
		if e.RawError != "" {
			errors = append(errors, fmt.Sprintf("  %s: %s", e.PNR, e.RawError))
			continue
		}
		if len(e.Flights) == 0 {
			errors = append(errors, fmt.Sprintf("  %s: no flight data", e.PNR))
			continue
		}

		for _, f := range e.Flights {
			blank := map[string]bool{"": true, "—": true}
			depIATA, arrIATA := "—", "—"
			depDT, arrDT := "—", "—"
			if !blank[f.Departure] {
				depIATA = ParseIATA(f.Departure)
				depDT = ParseDT(f.Departure)
			}
			if !blank[f.Arrival] {
				arrIATA = ParseIATA(f.Arrival)
				arrDT = ParseDT(f.Arrival)
			}

			sortStr := depDT
			if sortStr == "—" {
				sortStr = arrDT
			}
			sortKey := parseSortKey(sortStr)

			paxLabel := strings.ToUpper(f.PassengerName)
			if paxLabel == "" {
				paxLabel = strings.ToUpper(e.Passenger)
			}

			fltNum := f.FlightNumber
			if m := flightRE.FindStringSubmatch(fltNum); m != nil {
				fltNum = m[1]
			}

			status := strings.ReplaceAll(f.Status, "_", " ")
			if status == "" {
				status = "—"
			}

			rows = append(rows, displayRow{
				sortKey: sortKey,
				pnr:     e.PNR,
				flt:     fltNum,
				org:     depIATA,
				dst:     arrIATA,
				dep:     depDT,
				arr:     arrDT,
				pax:     paxLabel,
				seat:    f.Seat,
				cls:     ParseFare(f.Cabin),
				ac:      AbbrevAC(f.Aircraft),
				status:  status,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].sortKey.Before(rows[j].sortKey)
	})

	if len(rows) == 0 && len(errors) == 0 {
		fmt.Println("No flight data to display.")
		return
	}

	// Column widths
	cols := []struct {
		header string
		width  int
	}{
		{"PNR", 7},
		{"FLT", 7},
		{"ORG", 5},
		{"DST", 5},
		{"DEP", 13},
		{"ARR", 13},
		{"PAX", 24},
		{"SEAT", 6},
		{"CLS", 5},
		{"A/C", 9},
		{"STATUS", 15},
	}

	termW, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || termW < 80 {
		termW = 160
	}
	_ = termW

	bold := "\033[1m"
	dim := "\033[2m"
	cyan := "\033[1;96m"
	reset := "\033[0m"

	// Header
	fmt.Println()
	fmt.Print(bold)
	for _, c := range cols {
		fmt.Printf(" %-*s", c.width, c.header)
	}
	fmt.Println(reset)

	// Separator
	fmt.Print(dim)
	total := 1
	for _, c := range cols {
		total += c.width + 1
	}
	fmt.Println(strings.Repeat("─", total) + reset)

	// Rows — suppress repeated PNR/FLT/times for same-key rows (multi-PAX)
	type rowKey struct{ pnr, flt, dep string }
	prevKey := rowKey{}

	for _, r := range rows {
		key := rowKey{r.pnr, r.flt, r.dep}
		same := key == prevKey

		pnrStr := r.pnr
		if same {
			pnrStr = ""
		}
		fltStr := r.flt
		orgStr, dstStr, depStr, arrStr := r.org, r.dst, r.dep, r.arr
		if same {
			fltStr = ""
			orgStr, dstStr, depStr, arrStr = "", "", "", ""
		}

		sc, sr := statusColor(r.status)
		statusStr := r.status
		if same {
			statusStr = ""
			sc, sr = "", ""
		}

		clsStr := r.cls
		acStr := r.ac
		if same {
			clsStr = ""
			acStr = ""
		}

		fmt.Printf(" %s%-*s%s", cyan, cols[0].width, pnrStr, reset)
		fmt.Printf(" %-*s", cols[1].width, fltStr)
		fmt.Printf(" %-*s", cols[2].width, orgStr)
		fmt.Printf(" %-*s", cols[3].width, dstStr)
		fmt.Printf(" %-*s", cols[4].width, depStr)
		fmt.Printf(" %-*s", cols[5].width, arrStr)
		fmt.Printf(" %-*s", cols[6].width, truncate(r.pax, cols[6].width))
		fmt.Printf(" %-*s", cols[7].width, r.seat)
		fmt.Printf(" %-*s", cols[8].width, clsStr)
		fmt.Printf(" %-*s", cols[9].width, acStr)
		fmt.Printf(" %s%-*s%s", sc, cols[10].width, statusStr, sr)
		fmt.Println()

		prevKey = key
	}

	fmt.Println()

	if len(errors) > 0 {
		for _, e := range errors {
			fmt.Printf("\033[1;91m%s\033[0m\n", e)
		}
		fmt.Println()
	}
}

func parseSortKey(depDT string) time.Time {
	if depDT == "—" || depDT == "" {
		return time.Time{}
	}
	// Format: "HH:MM DDMMM", e.g. "06:03 10MAR"
	parts := strings.Fields(depDT)
	if len(parts) != 2 {
		return time.Time{}
	}
	s := fmt.Sprintf("%s %s %d", parts[0], parts[1], time.Now().Year())
	t, err := time.Parse("15:04 02Jan 2006", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
