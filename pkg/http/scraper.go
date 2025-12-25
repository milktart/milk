package http

import (
	"net/http"
	"sort"

	"github.com/dlclark/regexp2"
	"golang.org/x/net/html"
)

// ExtractNumbers extracts phone numbers from an HTTP response
// Returns: (notable numbers, platinum numbers, VIP numbers, error)
func ExtractNumbers(
	resp *http.Response,
	compiledNotable []*regexp2.Regexp,
	compiledPlatinum []*regexp2.Regexp,
	compiledVIP []*regexp2.Regexp,
) ([]string, []string, []string, error) {
	defer resp.Body.Close()
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, nil, nil, err
	}

	var numbers, platinum, VIP []string
	for _, num := range getHrefNumbers(doc) {
		if matchesAny(num, compiledNotable) {
			numbers = append(numbers, num)
		}
		if matchesAny(num, compiledPlatinum) {
			platinum = append(platinum, num)
		}
		if matchesAny(num, compiledVIP) {
			VIP = append(VIP, num)
		}
	}
	return numbers, platinum, VIP, nil
}

// getHrefNumbers recursively extracts phone numbers from href attributes
func getHrefNumbers(n *html.Node) []string {
	var nums []string
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key == "href" && len(attr.Val) >= 10 {
				nums = append(nums, attr.Val[len(attr.Val)-10:])
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		nums = append(nums, getHrefNumbers(c)...)
	}
	return nums
}

// matchesAny checks if a number matches any regex in a slice
func matchesAny(number string, regexes []*regexp2.Regexp) bool {
	for _, re := range regexes {
		if match, _ := re.MatchString(number); match {
			return true
		}
	}
	return false
}

// DeduplicateAndSort removes duplicates and sorts a slice of strings
func DeduplicateAndSort(numbers []string) []string {
	set := make(map[string]struct{}, len(numbers))
	for _, n := range numbers {
		set[n] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for n := range set {
		result = append(result, n)
	}
	sort.Strings(result)
	return result
}
