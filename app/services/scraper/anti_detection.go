package scraper

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

type UAProfile struct {
	UserAgent   string
	SecCHUA     string
	SecCHUAFull string
	Platform    string
	PlatformVer string
}

var defaultProfiles = []UAProfile{
	{
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="149", "Google Chrome";v="149"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="149.0.0.0", "Google Chrome";v="149.0.0.0"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	},
	{
		UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="149", "Google Chrome";v="149"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="149.0.0.0", "Google Chrome";v="149.0.0.0"`,
		Platform:    "macOS",
		PlatformVer: "15.5.0",
	},
	{
		UserAgent:   "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="149", "Google Chrome";v="149"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="149.0.0.0", "Google Chrome";v="149.0.0.0"`,
		Platform:    "Linux",
		PlatformVer: "6.8.0",
	},
	{
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36 Edg/149.0.0.0",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="149", "Microsoft Edge";v="149"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="149.0.0.0", "Microsoft Edge";v="149.0.0.0"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	},
}

type sessionState struct {
	profile   UAProfile
	jar       *cookiejar.Jar
	createdAt time.Time
	lastUsed  time.Time
}

type AntiDetection struct {
	mu          sync.Mutex
	random      *rand.Rand
	sessions    map[string]*sessionState
	profiles    []UAProfile
	sessionTTL  time.Duration
	maxSessions int
	now         func() time.Time
}

func NewAntiDetection(profiles []UAProfile, sessionTTL ...time.Duration) *AntiDetection {
	if len(profiles) == 0 {
		profiles = defaultProfiles
	}

	ttl := 2 * time.Hour
	if len(sessionTTL) > 0 && sessionTTL[0] > 0 {
		ttl = sessionTTL[0]
	}

	return &AntiDetection{
		random:      rand.New(rand.NewSource(time.Now().UnixNano())),
		sessions:    make(map[string]*sessionState),
		profiles:    append([]UAProfile(nil), profiles...),
		sessionTTL:  ttl,
		maxSessions: 128,
		now:         time.Now,
	}
}

func (a *AntiDetection) UserAgentForSession(domain string) string {
	return a.sessionFor(domain).profile.UserAgent
}

func (a *AntiDetection) CookieJarFor(domain string) http.CookieJar {
	return a.sessionFor(domain).jar
}

func (a *AntiDetection) ResetSession(domain string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, domain)
}

func (a *AntiDetection) ApplyBrowserHeaders(req *http.Request, referer string) {
	a.ApplyHeaders(req, referer)
}

func (a *AntiDetection) ApplyHeaders(req *http.Request, referer string) {
	domain := req.URL.Hostname()
	profile := a.sessionFor(domain).profile

	req.Header.Set("User-Agent", profile.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", secFetchSite(domain, referer))
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-CH-UA", profile.SecCHUA)
	req.Header.Set("Sec-CH-UA-Full-Version-List", profile.SecCHUAFull)
	req.Header.Set("Sec-CH-UA-Mobile", "?0")
	req.Header.Set("Sec-CH-UA-Platform", `"`+profile.Platform+`"`)
	req.Header.Set("Sec-CH-UA-Platform-Version", `"`+profile.PlatformVer+`"`)
	req.Header.Set("Cache-Control", "max-age=0")

	if referer != "" {
		req.Header.Set("Referer", referer)
	}
}

func (a *AntiDetection) ApplyJSONHeaders(req *http.Request, referer string) {
	a.ApplyHeaders(req, referer)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Del("Upgrade-Insecure-Requests")
	req.Header.Del("Sec-Fetch-User")
}

func (a *AntiDetection) NavigateTo(ctx context.Context, client *http.Client, targetURL, referer string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, err
	}

	a.ApplyHeaders(req, referer)
	parsedURL, err := url.Parse(targetURL)
	if err == nil {
		client.Jar = a.CookieJarFor(parsedURL.Hostname())
	}

	return client.Do(req)
}

func (a *AntiDetection) JitterDelay(base, jitter time.Duration) time.Duration {
	if jitter <= 0 {
		return base
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	offset := time.Duration(a.random.Int63n(int64(jitter*2))) - jitter
	delay := base + offset
	if delay < 0 {
		return 0
	}

	return delay
}

func (a *AntiDetection) HumanDelay(ctx context.Context, base, jitter time.Duration) error {
	timer := time.NewTimer(a.JitterDelay(base, jitter))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *AntiDetection) sessionFor(domain string) *sessionState {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.now()
	previous := a.sessions[domain]
	a.pruneExpiredLocked(now)
	if session, ok := a.sessions[domain]; ok && now.Sub(session.createdAt) < a.sessionTTL {
		session.lastUsed = now
		return session
	}

	next := a.profiles[a.random.Intn(len(a.profiles))]
	if previous != nil && len(a.profiles) > 1 {
		for next.UserAgent == previous.profile.UserAgent {
			next = a.profiles[a.random.Intn(len(a.profiles))]
		}
	}

	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	a.evictOldestIfFullLocked()
	session := &sessionState{profile: next, jar: jar, createdAt: now, lastUsed: now}
	a.sessions[domain] = session
	return session
}

func (a *AntiDetection) pruneExpiredLocked(now time.Time) {
	for domain, session := range a.sessions {
		if now.Sub(session.createdAt) >= a.sessionTTL {
			delete(a.sessions, domain)
		}
	}
}

func (a *AntiDetection) evictOldestIfFullLocked() {
	if a.maxSessions <= 0 || len(a.sessions) < a.maxSessions {
		return
	}

	var oldestDomain string
	var oldestTime time.Time
	for domain, session := range a.sessions {
		if oldestDomain == "" || session.lastUsed.Before(oldestTime) {
			oldestDomain = domain
			oldestTime = session.lastUsed
		}
	}
	if oldestDomain != "" {
		delete(a.sessions, oldestDomain)
	}
}

func secFetchSite(domain, referer string) string {
	if referer == "" {
		return "none"
	}

	refURL, err := url.Parse(referer)
	if err != nil || refURL.Hostname() == "" {
		return "cross-site"
	}
	if refURL.Hostname() == domain {
		return "same-origin"
	}
	return "cross-site"
}
