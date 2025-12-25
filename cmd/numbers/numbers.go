package numbers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/milktart/milk/pkg/config"
	httplib "github.com/milktart/milk/pkg/http"
	"github.com/milktart/milk/pkg/util"
	"golang.org/x/term"
)

// GetNumbersFiltered searches for numbers matching specified patterns and area codes
func GetNumbersFiltered(codes []string, patternTypes []string) {
	fmt.Println("Searching these area codes or patterns:")

	cfg := config.Get()
	client := &http.Client{Timeout: 10 * time.Second}
	var allNumbers, allPlatinum, allVIP []string
	var results []string

	for i, code := range codes {
		// Prepare done/current/todo strings
		done := strings.Join(results, ", ")
		current := util.BLUEBLINK + code + util.NC
		todo := ""
		if i < len(codes)-1 {
			todo = util.BLUE + strings.Join(codes[i+1:], ", ") + util.NC
		}

		// Print blinking line
		lineParts := []string{}
		lineClear := ""

		if done != "" {
			lineParts = append(lineParts, done)
		}
		lineParts = append(lineParts, current)
		if todo != "" {
			lineParts = append(lineParts, todo)
		}
		line := strings.Join(lineParts, ", ")

		width, _, err := term.GetSize(int(os.Stdout.Fd()))
		cols := utf8.RuneCountInString(util.StripANSI(line))

		if cols >= width {
			lineClear = "\r\033[A  %s"
		} else {
			lineClear = "\r  %s"
		}

		fmt.Printf(lineClear, line)

		time.Sleep(500 * time.Millisecond) // simulate work before HTTP

		// Fetch numbers
		url := "https://jmp.chat/tels?q=" + code
		resp, err := client.Get(url)
		result := ""
		if err != nil {
			result = util.RED + code + util.NC
		} else {
			nums, platinum, VIP, err := httplib.ExtractNumbers(
				resp,
				cfg.CompiledNotable,
				cfg.CompiledPlatinum,
				cfg.CompiledVIP,
			)
			if err != nil {
				result = util.RED + code + util.NC
			} else {
				result = util.GREEN + code + util.NC
				// Append pattern results
				if len(patternTypes) == 0 {
					allNumbers = append(allNumbers, nums...)
					allPlatinum = append(allPlatinum, platinum...)
					allVIP = append(allVIP, VIP...)
				} else {
					for _, pt := range patternTypes {
						switch strings.ToLower(pt) {
						case "vip":
							allVIP = append(allVIP, VIP...)
						case "platinum":
							allPlatinum = append(allPlatinum, platinum...)
						case "notable", "all":
							allNumbers = append(allNumbers, nums...)
						}
					}
				}
			}
		}
		results = append(results, result)

		// Update line with completed result
		lineParts = []string{}
		lineParts = append(lineParts, results...)
		if i < len(codes)-1 && len(todo) > 0 {
			lineParts = append(lineParts, todo)
		}
		line = strings.Join(lineParts, ", ")

		fmt.Printf(lineClear, line)
		time.Sleep(200 * time.Millisecond) // small delay so change is visible
	}

	fmt.Println("\n")
	util.PrintNumbers("VIP Numbers found:", httplib.DeduplicateAndSort(allVIP))
	util.PrintNumbers("\nPlatinum Numbers found:", httplib.DeduplicateAndSort(allPlatinum))
	util.PrintNumbers("\nNotable pattern matches found:", httplib.DeduplicateAndSort(allNumbers))
	fmt.Println("")
}
