package util

import (
	"fmt"
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI color codes from a string
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// PrintNumbers prints a formatted list of phone numbers
func PrintNumbers(title string, numbers []string) {
	if len(numbers) == 0 {
		return
	}
	fmt.Println(title)
	for _, n := range numbers {
		if len(n) < 10 {
			continue
		}
		fmt.Printf("  +1 (%s) %s-%s ///// +1-%s-%s%s ///// %s\n",
			n[:3], n[3:6], n[6:10], n[:3], n[3:6], n[6:10], n)
	}
}

// SplitList parses a comma or space separated list into a slice
func SplitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Fields(strings.ReplaceAll(s, ",", " "))
}
