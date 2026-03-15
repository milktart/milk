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
