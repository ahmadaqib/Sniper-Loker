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
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="120.0.6099.130", "Google Chrome";v="120.0.6099.130"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	},
	{
		UserAgent:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="120.0.6099.130", "Google Chrome";v="120.0.6099.130"`,
		Platform:    "macOS",
		PlatformVer: "13.6.3",
	},
	{
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="119", "Google Chrome";v="119"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="119.0.6045.160", "Google Chrome";v="119.0.6045.160"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	},
	{
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="120.0.6099.130", "Microsoft Edge";v="120.0.2210.91"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	},
}

type sessionState struct {
	profile   UAProfile
	jar       *cookiejar.Jar
	createdAt time.Time
}

type AntiDetection struct {
	mu         sync.Mutex
	random     *rand.Rand
	sessions   map[string]*sessionState
	profiles   []UAProfile
	sessionTTL time.Duration
	now        func() time.Time
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
		random:     rand.New(rand.NewSource(time.Now().UnixNano())),
		sessions:   make(map[string]*sessionState),
		profiles:   append([]UAProfile(nil), profiles...),
		sessionTTL: ttl,
		now:        time.Now,
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
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
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
	if session, ok := a.sessions[domain]; ok && now.Sub(session.createdAt) < a.sessionTTL {
		return session
	}

	next := a.profiles[a.random.Intn(len(a.profiles))]
	if previous, ok := a.sessions[domain]; ok && len(a.profiles) > 1 {
		for next.UserAgent == previous.profile.UserAgent {
			next = a.profiles[a.random.Intn(len(a.profiles))]
		}
	}

	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	session := &sessionState{profile: next, jar: jar, createdAt: now}
	a.sessions[domain] = session
	return session
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
