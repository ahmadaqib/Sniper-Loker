package scraper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDefaultProfilesUseCurrentMajorVersion(t *testing.T) {
	for _, profile := range defaultProfiles {
		if strings.Contains(profile.UserAgent, "Chrome/119.") || strings.Contains(profile.UserAgent, "Chrome/120.") ||
			strings.Contains(profile.SecCHUA, `v="119"`) || strings.Contains(profile.SecCHUA, `v="120"`) {
			t.Fatalf("stale browser profile should not be used: %+v", profile)
		}
	}
}

func TestApplyHeadersKeepsClientHintsAlignedWithProfile(t *testing.T) {
	profile := UAProfile{
		UserAgent:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
		SecCHUA:     `"Not_A Brand";v="8", "Chromium";v="120", "Microsoft Edge";v="120"`,
		SecCHUAFull: `"Not_A Brand";v="8.0.0.0", "Chromium";v="120.0.6099.130", "Microsoft Edge";v="120.0.2210.91"`,
		Platform:    "Windows",
		PlatformVer: "15.0.0",
	}
	antiDetection := NewAntiDetection([]UAProfile{profile})

	req := httptest.NewRequest(http.MethodGet, "https://www.loker.id/lowongan", nil)
	antiDetection.ApplyHeaders(req, "https://www.loker.id")

	if got := req.Header.Get("User-Agent"); got != profile.UserAgent {
		t.Fatalf("unexpected user-agent: %q", got)
	}
	if got := req.Header.Get("Sec-CH-UA"); got != profile.SecCHUA {
		t.Fatalf("unexpected sec-ch-ua: %q", got)
	}
	if got := req.Header.Get("Sec-Fetch-Site"); got != "same-origin" {
		t.Fatalf("expected same-origin referer, got %q", got)
	}
}

func TestNavigateToAttachesCookieJar(t *testing.T) {
	antiDetection := NewAntiDetection(nil)
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Set-Cookie": []string{"session=abc; Path=/"}},
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    req,
			}, nil
		}),
	}

	resp, err := antiDetection.NavigateTo(context.Background(), client, "https://example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if client.Jar == nil {
		t.Fatal("expected navigate to attach cookie jar")
	}
	parsedURL, _ := url.Parse("https://example.test")
	if got := client.Jar.Cookies(parsedURL); len(got) != 1 || got[0].Name != "session" {
		t.Fatalf("expected stored session cookie, got %+v", got)
	}
}

func TestSessionExpiresAndRotatesProfile(t *testing.T) {
	profiles := []UAProfile{
		{UserAgent: "first", SecCHUA: `"First";v="1"`, SecCHUAFull: `"First";v="1.0.0.0"`, Platform: "Windows", PlatformVer: "15.0.0"},
		{UserAgent: "second", SecCHUA: `"Second";v="1"`, SecCHUAFull: `"Second";v="1.0.0.0"`, Platform: "Windows", PlatformVer: "15.0.0"},
	}
	antiDetection := NewAntiDetection(profiles, time.Nanosecond)
	first := antiDetection.UserAgentForSession("example.test")
	time.Sleep(time.Millisecond)
	second := antiDetection.UserAgentForSession("example.test")

	if first == second {
		t.Fatalf("expected rotated profile after session ttl, still got %q", first)
	}
}

func TestSessionCacheEvictsOldestWhenFull(t *testing.T) {
	antiDetection := NewAntiDetection([]UAProfile{
		{UserAgent: "one", SecCHUA: `"One";v="1"`, SecCHUAFull: `"One";v="1.0.0.0"`, Platform: "Windows", PlatformVer: "15.0.0"},
	})
	antiDetection.maxSessions = 2

	_ = antiDetection.CookieJarFor("a.example")
	_ = antiDetection.CookieJarFor("b.example")
	_ = antiDetection.CookieJarFor("c.example")

	antiDetection.mu.Lock()
	defer antiDetection.mu.Unlock()
	if len(antiDetection.sessions) != 2 {
		t.Fatalf("expected capped session cache, got %d sessions", len(antiDetection.sessions))
	}
	if _, ok := antiDetection.sessions["a.example"]; ok {
		t.Fatal("expected oldest session to be evicted")
	}
}

func TestDefaultSourcesUseUTLSClient(t *testing.T) {
	source := NewLokerIDSource(nil, nil)
	if !source.Config().UseUTLS {
		t.Fatal("expected uTLS to be enabled by default")
	}
	transport, ok := source.client.Transport.(*http.Transport)
	if !ok || transport.DialTLSContext == nil {
		t.Fatal("expected default source client to use uTLS transport")
	}
}

func TestIsChallengePage(t *testing.T) {
	if !IsChallengePage("<html><title>Just a moment...</title><script>cf_chl_opt={}</script></html>") {
		t.Fatal("expected cloudflare challenge to be detected")
	}
	if IsChallengePage(stringsOfLength("normal job content ", 80)) {
		t.Fatal("expected large normal page to pass")
	}
}

func stringsOfLength(part string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += part
	}
	return out
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
