package flights

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

const maxConcurrent = 4

// sessionHeaderKeys are the headers from the browser's API call that we capture
// and replay in subsequent direct API requests.
var sessionHeaderKeys = []string{"Cookie", "appId", "channelId", "transactionid"}

// CaptureSessionHeaders opens a headless browser, navigates Delta's My Trips
// form for one booking, intercepts the session headers from the browser's API
// call, and returns them. The headers can then be reused for any number of
// subsequent direct API calls.
func CaptureSessionHeaders(b Booking, headless bool) (map[string]string, error) {
	l := launcher.New().
		Headless(headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox", "")
	u, launchErr := l.Launch()
	if launchErr != nil {
		return nil, fmt.Errorf("browser launch failed: %v", launchErr)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	page := stealth.MustPage(browser)
	page.MustSetViewport(1280, 800, 1, false)
	page.MustSetExtraHeaders(
		"Accept-Language", "en-US,en;q=0.9",
		"User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	)

	var sessionHeaders map[string]string
	var mu sync.Mutex
	router := page.HijackRequests()
	router.MustAdd("*mytrips-api.delta.com/v1/mytrips/travelreservations*", func(ctx *rod.Hijack) {
		if ctx.Request.Method() == "POST" {
			mu.Lock()
			if sessionHeaders == nil {
				captured := make(map[string]string)
				for _, k := range sessionHeaderKeys {
					if v := ctx.Request.Header(k); v != "" {
						captured[k] = v
					}
				}
				if len(captured) > 0 {
					sessionHeaders = captured
				}
			}
			mu.Unlock()
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	go router.Run()

	if err := rod.Try(func() {
		page.MustNavigate("https://www.delta.com").MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("homepage load: %v", err)
	}
	time.Sleep(800 * time.Millisecond)

	_ = rod.Try(func() {
		page.Timeout(4 * time.Second).MustElement("#onetrust-accept-btn-handler").MustClick()
		time.Sleep(500 * time.Millisecond)
	})
	page.MustEval(`() => {
		document.querySelectorAll('modal-container[aria-modal="true"]').forEach(m => m.style.display = 'none');
		document.querySelectorAll('.modal-backdrop').forEach(b => b.remove());
		document.body.classList.remove('modal-open');
	}`)

	if err := rod.Try(func() {
		page.Timeout(15 * time.Second).MustElement("#headPrimary3").MustClick()
	}); err != nil {
		return nil, fmt.Errorf("MY TRIPS tab: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	if err := rod.Try(func() {
		page.Timeout(15 * time.Second).MustElement("#confirmationNo")
	}); err != nil {
		return nil, fmt.Errorf("form not found: %v", err)
	}

	page.MustElement("#confirmationNo").MustInput(b.PNR)
	time.Sleep(400 * time.Millisecond)
	page.MustElement("#firstName").MustInput(b.First)
	time.Sleep(400 * time.Millisecond)
	page.MustElement("#lastName").MustInput(b.Last)
	time.Sleep(600 * time.Millisecond)

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
		return nil, fmt.Errorf("submit button not found")
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		h := sessionHeaders
		mu.Unlock()
		if h != nil {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}

	mu.Lock()
	headers := sessionHeaders
	mu.Unlock()

	if headers == nil {
		return nil, fmt.Errorf("session headers not captured — browser did not call the API within 30s")
	}
	return headers, nil
}

// loyaltyDigitKeys maps digit runes to their rod input.Key values.
// Only digits are needed for Delta SkyMiles username (which is a phone number).
var loyaltyDigitKeys = map[rune]input.Key{
	'0': input.Digit0, '1': input.Digit1, '2': input.Digit2,
	'3': input.Digit3, '4': input.Digit4, '5': input.Digit5,
	'6': input.Digit6, '7': input.Digit7, '8': input.Digit8,
	'9': input.Digit9,
}

// captureLoyaltySession opens a browser and captures the loyalty session headers.
// When automated is true it attempts to fill in credentials and submit the form.
// When automated is false (or headless is false) it waits for the user to log in
// manually, then navigates to the SkyMiles overview to trigger the loyalty API call.
func captureLoyaltySession(username, password string, headless, automated bool) (map[string]string, error) {
	l := launcher.New().
		Headless(headless).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "").
		Delete("enable-automation").
		Set("exclude-switches", "enable-automation")
	u, launchErr := l.Launch()
	if launchErr != nil {
		return nil, fmt.Errorf("browser launch failed: %v", launchErr)
	}

	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.MustClose()

	incognito, _ := browser.Incognito()
	page := stealth.MustPage(incognito)
	page.MustSetViewport(1280, 800, 1, false)

	captured := make(chan map[string]string, 1)

	// Intercept any Delta API call that carries an Authorization header.
	router := page.HijackRequests()
	router.MustAdd("*api.delta.com*", func(ctx *rod.Hijack) {
		if auth := ctx.Request.Header("Authorization"); auth != "" {
			headers := map[string]string{"Authorization": auth}
			for _, k := range sessionHeaderKeys {
				if v := ctx.Request.Header(k); v != "" {
					headers[k] = v
				}
			}
			select {
			case captured <- headers:
			default:
			}
		}
		ctx.ContinueRequest(&proto.FetchContinueRequest{})
	})
	go router.Run()

	if err := rod.Try(func() {
		page.MustNavigate("https://www.delta.com/skymiles/login").MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("login page load: %v", err)
	}

	// Dismiss cookie banner if present.
	_ = rod.Try(func() {
		page.Timeout(5 * time.Second).MustElement("#onetrust-accept-btn-handler").MustClick()
	})

	if automated {
		// Delta's login page shows #userId (username) and #password on the same page.
		// The Angular form only renders after the PingFederate IS_USER_REMEMBERED flow
		// completes — wait up to 20s for #userId to appear.
		if err := rod.Try(func() {
			page.Timeout(20 * time.Second).MustElement("#userId").MustClick()
		}); err != nil {
			return nil, fmt.Errorf("username field not found: %v", err)
		}
		time.Sleep(500 * time.Millisecond)

		// Type username digit-by-digit via keyboard events (required for Angular
		// reactive form validation to register the value).
		for _, ch := range username {
			if k, ok := loyaltyDigitKeys[ch]; ok {
				page.Keyboard.MustType(k)
			}
			time.Sleep(80 * time.Millisecond)
		}
		time.Sleep(600 * time.Millisecond)

		// Click the password field and type it character-by-character via
		// JavaScript input events so Angular registers the value correctly.
		if err := rod.Try(func() {
			page.Timeout(5 * time.Second).MustElement("#password").MustClick()
		}); err != nil {
			return nil, fmt.Errorf("password field not found — re-run with --visible to log in manually: %v", err)
		}
		time.Sleep(500 * time.Millisecond)
		for _, ch := range password {
			page.MustElement("#password").MustInput(string(ch))
			time.Sleep(80 * time.Millisecond)
		}
		time.Sleep(600 * time.Millisecond)

		// Click the Login button explicitly.
		submitted := false
		for _, sel := range []string{`.loginModal-button`, `button[type="submit"]`, `#btn-login`} {
			if err := rod.Try(func() {
				page.Timeout(3 * time.Second).MustElement(sel).MustClick()
				submitted = true
			}); err == nil && submitted {
				break
			}
		}
		if !submitted {
			// Fall back to Enter key
			page.Keyboard.MustType(input.Enter)
		}
		time.Sleep(3 * time.Second)

		// Wait for post-login redirect (up to 20s).
		_ = rod.Try(func() {
			page.Timeout(20 * time.Second).MustWaitNavigation()
		})
		time.Sleep(1 * time.Second)

		// If still on the login page, Akamai blocked the automated submission.
		if info, _ := page.Info(); info != nil && strings.Contains(info.URL, "/skymiles/login") {
			// Save a screenshot for debugging.
			if img, err := page.Screenshot(false, nil); err == nil {
				_ = os.WriteFile("/tmp/milk-login-debug.png", img, 0644)
				fmt.Println("Debug screenshot saved to /tmp/milk-login-debug.png")
			}
			return nil, fmt.Errorf("login blocked by bot-detection — re-run with --visible to log in manually")
		}
	} else {
		// Interactive mode: wait up to 3 minutes for the user to log in.
		fmt.Println("Please log in to Delta SkyMiles in the browser window that just opened.")
		fmt.Println("The window will close automatically once your session is captured.")
		deadline := time.Now().Add(3 * time.Minute)
		for time.Now().Before(deadline) {
			if info, _ := page.Info(); info != nil && !strings.Contains(info.URL, "/skymiles/login") {
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Navigate to SkyMiles overview to trigger the loyalty API call.
	if err := rod.Try(func() {
		page.MustNavigate("https://www.delta.com/us/en/skymiles/overview").MustWaitLoad()
	}); err != nil {
		return nil, fmt.Errorf("skymiles overview load: %v", err)
	}

	// Wait up to 30s for an intercepted API call with Authorization header.
	select {
	case headers := <-captured:
		return headers, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("loyalty session headers not captured — browser did not call the loyalty API within 30s")
	}
}

// CaptureLoyaltySessionHeaders captures a Delta SkyMiles session.
// When headless is false (--visible flag), it opens a visible browser for
// automated login (which bypasses Akamai bot-detection more reliably).
// When headless is true, it tries automated headless first, then falls back
// to a visible browser with manual login if bot-detection fires.
func CaptureLoyaltySessionHeaders(username, password string, headless bool) (map[string]string, error) {
	if !headless {
		// Visible mode: try automated login in a visible browser (user can
		// intervene if needed), which avoids Akamai headless fingerprinting.
		return captureLoyaltySession(username, password, false, true)
	}
	headers, err := captureLoyaltySession(username, password, true, true)
	if err != nil && strings.Contains(err.Error(), "bot-detection") {
		// Automated headless login was blocked — retry with a non-headless browser
		// (avoids Akamai's headless fingerprint check). Works whether running under
		// Xvfb or a real display.
		fmt.Println("Automated headless login blocked. Retrying with visible browser...")
		headers, err = captureLoyaltySession(username, password, false, true)
	}
	return headers, err
}

// FetchBooking captures session headers via a browser session for b, then
// immediately uses them to fetch b from the API.
func FetchBooking(b Booking, headless bool) *CacheEntry {
	entry := &CacheEntry{
		PNR:       b.PNR,
		Passenger: formatName(b.First, b.Last),
		FetchedAt: time.Now().UTC(),
	}
	headers, err := CaptureSessionHeaders(b, headless)
	if err != nil {
		entry.RawError = err.Error()
		return entry
	}
	entry.SessionHeaders = headers
	apiEntry := FetchBookingFromAPI(b, headers)
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

// HasAPICredentials reports whether the entry has session headers for direct API calls.
func HasAPICredentials(e *CacheEntry) bool {
	return e != nil && len(e.SessionHeaders) > 0 && e.SessionHeaders["Cookie"] != ""
}

// cookieWithoutBmSs returns the Cookie header with bm_ss removed, since that
// short-lived Akamai cookie causes rejections when replayed later.
func cookieWithoutBmSs(cookie string) string {
	var parts []string
	for _, p := range strings.Split(cookie, "; ") {
		if !strings.HasPrefix(p, "bm_ss=") {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "; ")
}
