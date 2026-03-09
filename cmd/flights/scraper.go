package flights

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/stealth"
)

const maxConcurrent = 4

// FetchBooking opens a headless browser, navigates Delta's My Trips form,
// submits the PNR + name, and returns a CacheEntry with extracted flight data.
func FetchBooking(b Booking, headless bool) *CacheEntry {
	entry := &CacheEntry{
		PNR:       b.PNR,
		Passenger: b.First + " " + b.Last,
		FetchedAt: time.Now().UTC(),
	}

	l := launcher.New().
		Headless(headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox", "")
	if !headless {
		l = l.Headless(false)
	}
	u := l.MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	page.MustSetViewport(1280, 800, 1, false)
	page.MustSetExtraHeaders(
		"Accept-Language", "en-US,en;q=0.9",
		"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	)

	// 1. Load homepage
	if err := rod.Try(func() {
		page.MustNavigate("https://www.delta.com").MustWaitLoad()
	}); err != nil {
		entry.RawError = fmt.Sprintf("homepage load: %v", err)
		return entry
	}
	time.Sleep(800 * time.Millisecond)

	// 2. Dismiss cookie modal
	_ = rod.Try(func() {
		page.Timeout(4 * time.Second).MustElement("#onetrust-accept-btn-handler").MustClick()
		time.Sleep(500 * time.Millisecond)
	})
	page.MustEval(`() => {
		document.querySelectorAll('modal-container[aria-modal="true"]').forEach(m => m.style.display = 'none');
		document.querySelectorAll('.modal-backdrop').forEach(b => b.remove());
		document.body.classList.remove('modal-open');
	}`)

	// 3. Click MY TRIPS tab
	if err := rod.Try(func() {
		page.Timeout(15 * time.Second).MustElement("#headPrimary3").MustClick()
	}); err != nil {
		entry.RawError = fmt.Sprintf("MY TRIPS tab: %v", err)
		return entry
	}
	time.Sleep(1500 * time.Millisecond)

	// 4. Wait for confirmation number field
	if err := rod.Try(func() {
		page.Timeout(15 * time.Second).MustElement("#confirmationNo")
	}); err != nil {
		entry.RawError = fmt.Sprintf("form not found: %v", err)
		return entry
	}

	// 5. Fill form
	page.MustElement("#confirmationNo").MustInput(b.PNR)
	time.Sleep(400 * time.Millisecond)
	page.MustElement("#firstName").MustInput(b.First)
	time.Sleep(400 * time.Millisecond)
	page.MustElement("#lastName").MustInput(b.Last)
	time.Sleep(600 * time.Millisecond)

	// 6. Submit
	submitted := false
	for _, sel := range []string{"#btn-mytrip-submit", `button[type="submit"]`} {
		if err := rod.Try(func() {
			page.Timeout(5 * time.Second).MustElement(sel).MustClick()
		}); err == nil {
			submitted = true
			break
		}
	}
	if !submitted {
		entry.RawError = "submit button not found"
		return entry
	}

	// 7. Wait for trip-details URL
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(page.MustInfo().URL, "trip-details") {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}

	// 8. Check for CAPTCHA
	bodyText := page.MustElement("body").MustText()
	if strings.Contains(strings.ToLower(bodyText), "captcha") ||
		strings.Contains(strings.ToLower(bodyText), "are you a robot") {
		entry.RawError = "CAPTCHA detected — run with --visible to solve manually"
		return entry
	}

	// 9. Wait for flight cards to render (Angular SPA skeleton)
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		cards, _ := page.Elements(".td-flight-card")
		if len(cards) > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 10. Extract
	flights, err := extractFlights(page, b.First+" "+b.Last)
	if err != nil {
		entry.RawError = fmt.Sprintf("extraction: %v", err)
		return entry
	}
	entry.Flights = flights
	return entry
}
