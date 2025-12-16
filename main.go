package main

import (
  "flag"
  "fmt"
  "net/http"
  "regexp"
  "os"
  "sort"
  "strings"
  "time"
  "unicode/utf8"

  "golang.org/x/net/html"
  "golang.org/x/term"
  "github.com/dlclark/regexp2"
)

const (
  TOOLNAME  = "milk"
  RED       = "\033[1;91m"
  GREEN     = "\033[1;92m"
  YELLOW    = "\033[1;93m"
  BLUE      = "\033[1;94m"
  BLUEBLINK = "\033[1;5;94m"
  NC        = "\033[0m"
)

// Regex patterns
var (
  VIPPatterns = []string{
    `\d{3}\d000$`,
    `(\d)(\d)0\1\2[0]{2}$`,
    `(\d{3})\1\1`,
    `(\d{5})\1`,
    `\d+(\d)\1\1\1$`,
    `(\d)\1\1[0]\1{3}$`,
    `.*8675309.*`,
    `(\d)(\d)\1\2\1\2$`,
    `^212.+`,
  }

  PlatinumPatterns = []string{
    `.*(\d){3}\d(\d)\2\2$`,
    `.*(\d{2})\1[0]0$`,
    `.*\d{3}(\d{3})[0]\1$`,
  }

  NotablePatterns = []string{
    `(\d)(\d)(\d)(\d)(\d)\5\4\3\2\1`,
    `(\d)(\d)\1\2\1\2.+`,
    `((\d)(\d)(\2|\3){3})\1`,
    `(\d)(\d)\1\2\1.*(\d)(\d)\3\4\3`,
    `.*(\d)\1\1(\d)(\d)\2\3$`,
    `.*(\d)\1(\d)(\1|\2)\1\2\2$`,
    `.*(\d)(\d)(\d)\1\2\3(\1|\2|\3)$`,
    `.*(\d)\1\1(\d)\2\2\d$`,
    `.*\d{3}(\d{3})[1-9]\1$`,
    `.*(246)8\1$`,
    `.*(258)\1[08]$`,
    `.*\d\d(\d)(\d)(\d)(\d)\1\2\3\4$`,
    `.*(\d{2})(?!\1)(\d{2})00$`,
    `.*8449988.*`,
  }

  compiledVIP      []*regexp2.Regexp
  compiledPlatinum []*regexp2.Regexp
  compiledNotable  []*regexp2.Regexp
)

// Precompile regexes
func init() {
  for _, p := range VIPPatterns {
    compiledVIP = append(compiledVIP, regexp2.MustCompile(p, 0))
  }
  for _, p := range PlatinumPatterns {
    compiledPlatinum = append(compiledPlatinum, regexp2.MustCompile(p, 0))
  }
  for _, p := range NotablePatterns {
    compiledNotable = append(compiledNotable, regexp2.MustCompile(p, 0))
  }
}

// Default area codes by region
var regionCodes = map[string][]string{
  "default": {
    "202", "212", "213", 
    "303", "310", "313",
    "415", "416", "418", 
    "512", "514", 
    "617", 
    "808", 
    "907", 
    "~8449988",
  },
  "Canada": {
    "204", "226", "236", "249", "250", "289", 
    "306", "343", "365", "367",
    "403", "416", "418", "428", "431", "437", "438", "450", 
    "506", "514", "519", "548", "579", "581", "587", 
    "604", "613", "639", "647", "672",
    "705", "709", "778", "780", "782", 
    "807", "819", "825", "867", 
    "902", "905",
  },
  "CA": {
    "209", "213", 
    "310", "323", 
    "408", "415", "424", "442", 
    "510", "530", "559", "562", 
    "619", "626", "650", "661", "669", 
    "707", "714", "760", 
    "805", "818", "831", "858", 
    "909", "916", "925", "949", "951",
  },
  "NY": {
    "212", 
    "315", "332", "347", 
    "516", "518", "585", 
    "607", "631", "646", 
    "716", "718", 
    "845", 
    "914", "917", "929", "934",
  },
  "NYC": {
    "212", "332", "347", "646", "718", "917", "929",
  },
  "TX": {
    "210", "214", "254", "281", 
    "325", "346", "361", 
    "409", "430", "432", "469", 
    "512", 
    "682", 
    "713", 
    "806", "817", "830", 
    "903", "915", "936", "940", "956", "972", "979",
  },
}

// Utility functions
func splitList(s string) []string {
  s = strings.TrimSpace(s)
  if s == "" {
    return nil
  }
  return strings.Fields(strings.ReplaceAll(s, ",", " "))
}

func matchesAny(number string, regexes []*regexp2.Regexp) bool {
  for _, re := range regexes {
    if match, _ := re.MatchString(number); match {
      return true
    }
  }
  return false
}

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

func extractNumbers(resp *http.Response) ([]string, []string, []string, error) {
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

func deduplicateAndSort(numbers []string) []string {
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

func printNumbers(title string, numbers []string) {
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

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
  return ansiRE.ReplaceAllString(s, "")
}

func getNumbersFiltered(codes []string, patternTypes []string) {
  fmt.Println("Searching these area codes or patterns:")

  client := &http.Client{Timeout: 10 * time.Second}
  var allNumbers, allPlatinum, allVIP []string
  var results []string

  for i, code := range codes {
    // Prepare done/current/todo strings
    done := strings.Join(results, ", ")
    current := BLUEBLINK + code + NC
    todo := ""
    if i < len(codes)-1 {
      todo = BLUE + strings.Join(codes[i+1:], ", ") + NC
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
    cols := utf8.RuneCountInString(stripANSI(line))

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
      result = RED + code + NC
    } else {
      nums, platinum, VIP, err := extractNumbers(resp)
      if err != nil {
        result = RED + code + NC
      } else {
        result = GREEN + code + NC
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
  printNumbers("VIP Numbers found:", deduplicateAndSort(allVIP))
  printNumbers("\nPlatinum Numbers found:", deduplicateAndSort(allPlatinum))
  printNumbers("\nNotable pattern matches found:", deduplicateAndSort(allNumbers))
  fmt.Println("")
}

func main() {
  codeFlag := flag.String("c", "", "Comma or space separated list of area codes (ex. -c 212,415,808)")
  flag.StringVar(codeFlag, "code", "", "Same as -c")

  regionFlag := flag.String("r", "", "Region filter (ex. -r Canada)")
  flag.StringVar(regionFlag, "region", "", "Same as -r")

  patternFlag := flag.String("p", "", "Pattern type(s) to search (ex. -p VIP,platinum)")
  flag.StringVar(patternFlag, "pattern", "", "Same as -p")

  canadaFlag := flag.Bool("Canada", false, "Shorthand for -r Canada")

  CAFlag := flag.Bool("CA", false, "Shorthand for -r CA")
  NYFlag := flag.Bool("NY", false, "Shorthand for -r NY")
  NYCFlag := flag.Bool("NYC", false, "Shorthand for -r NYC")
  TXFlag := flag.Bool("TX", false, "Shorthand for -r TX")

  flag.Usage = func() {
    fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [options]\n\n", os.Args[0])
    fmt.Println("Options:")
    flag.PrintDefaults()
    fmt.Println("\nExamples:")
    fmt.Println("  " + TOOLNAME + " -c 212 415 808 -r Canada -p VIP,platinum")
    fmt.Println("  " + TOOLNAME + " --code 212,415,808 --region TX --pattern VIP")
    fmt.Println("  " + TOOLNAME + " --Canada -c 416 604")
  }

  flag.Parse()

  region := *regionFlag
  if *canadaFlag { region = "Canada" }
  if *CAFlag { region = "CA" }
  if *NYFlag { region = "NY" }
  if *NYCFlag { region = "NYC" }
  if *TXFlag { region = "TX" }

  codes := splitList(*codeFlag)
  patternTypes := splitList(*patternFlag)

  for _, arg := range flag.Args() {
    if !strings.HasPrefix(arg, "-") && len(arg) <= 5 {
      codes = append(codes, arg)
    }
  }

  if region != "" && len(codes) == 0 {
    if rc, ok := regionCodes[region]; ok {
      codes = rc
    }
  }

  if len(codes) == 0 {
    codes = regionCodes["default"]
  }

  getNumbersFiltered(codes, patternTypes)
}
