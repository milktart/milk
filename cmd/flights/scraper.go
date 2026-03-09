package flights

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

const maxConcurrent = 4

// FetchBooking opens a headless browser, navigates Delta's My Trips form,
// intercepts the auth token from the browser's own API call, then fetches
// data directly from the Delta API using that token.
func FetchBooking(b Booking, headless bool) *CacheEntry {
	entry := &CacheEntry{
		PNR:       b.PNR,
		Passenger: formatName(b.First, b.Last),
		FetchedAt: time.Now().UTC(),
	}

	l := launcher.New().
		Headless(headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox", "")
	u := l.MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	page.MustSetViewport(1280, 800, 1, false)
	page.MustSetExtraHeaders(
		"Accept-Language", "en-US,en;q=0.9",
		"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	)

	// Intercept all requests to the mytrips API to capture the bearer token.
	var authToken string
	var authMu sync.Mutex
	router := page.HijackRequests()
	router.MustAdd("*mytrips-api.delta.com*", func(ctx *rod.Hijack) {
		h := ctx.Request.Header("Authorization")
		fmt.Printf("  [hijack] %s auth=%q\n", ctx.Request.URL().String(), h[:min(len(h), 40)])
		if strings.HasPrefix(h, "Bearer ") {
			authMu.Lock()
			authToken = h
			authMu.Unlock()
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	go router.Run()

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

	// 7. Wait for the auth token to be intercepted (browser makes its own API call
	// once the trip-details page loads, which gives us the bearer token).
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		authMu.Lock()
		tok := authToken
		authMu.Unlock()
		if tok != "" {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	authMu.Lock()
	tok := authToken
	authMu.Unlock()

	if tok == "" {
		entry.RawError = "auth token not captured — browser did not call the API within 30s"
		return entry
	}

	// 8. Call the API directly with the captured token.
	entry.AuthToken = tok
	apiEntry := FetchBookingFromAPI(b, tok)
	if apiEntry.RawError != "" || len(apiEntry.Flights) == 0 {
		if apiEntry.RawError != "" {
			entry.RawError = fmt.Sprintf("API: %s", apiEntry.RawError)
		} else {
			entry.RawError = "API returned no flights"
		}
		return entry
	}
	return apiEntry
}
