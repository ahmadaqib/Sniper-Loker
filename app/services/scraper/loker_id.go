package scraper

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const LokerIDSourceName = "loker_id"

var ErrChallengeDetected = errors.New("challenge page detected")

type LokerIDSource struct {
	client        *http.Client
	antiDetection *AntiDetection
	config        SourceConfig
}

func NewLokerIDSource(client *http.Client, antiDetection *AntiDetection) *LokerIDSource {
	config := SourceConfig{
		Name:             LokerIDSourceName,
		DisplayName:      "Loker.id",
		BaseURL:          "https://www.loker.id",
		Enabled:          true,
		MaxPerHour:       30,
		BaseDelay:        3 * time.Second,
		Jitter:           time.Second,
		RequestTimeout:   10 * time.Second,
		CircuitThreshold: 3,
		CircuitCooldown:  30 * time.Minute,
	}
	if client == nil {
		client = &http.Client{Timeout: config.RequestTimeout}
	}
	if antiDetection == nil {
		antiDetection = NewAntiDetection(nil)
	}

	return &LokerIDSource{client: client, antiDetection: antiDetection, config: config}
}

func (s *LokerIDSource) Name() string {
	return s.config.Name
}

func (s *LokerIDSource) Config() SourceConfig {
	return s.config
}

func (s *LokerIDSource) Scrape(ctx context.Context, query SearchQuery) ([]ScrapedJob, error) {
	homeResp, err := s.antiDetection.NavigateTo(ctx, s.client, s.config.BaseURL, "")
	if err != nil {
		return nil, fmt.Errorf("warm up loker.id session: %w", err)
	}
	_, _ = io.Copy(io.Discard, homeResp.Body)
	_ = homeResp.Body.Close()

	if err := s.antiDetection.HumanDelay(ctx, s.config.BaseDelay, s.config.Jitter); err != nil {
		return nil, err
	}

	endpoint, err := s.searchURL(query)
	if err != nil {
		return nil, err
	}

	resp, err := s.antiDetection.NavigateTo(ctx, s.client, endpoint, s.config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("request loker.id: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read loker.id response: %w", err)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || IsChallengePage(string(body)) {
		s.antiDetection.ResetSession(resp.Request.URL.Hostname())
		return nil, ErrChallengeDetected
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse loker.id response: %w", err)
	}

	if result := IsValidJobPage(resp.StatusCode, doc); !result.Valid {
		return nil, fmt.Errorf("invalid loker.id page: %s", result.Reason)
	}

	return s.extractJobs(doc), nil
}

func (s *LokerIDSource) searchURL(query SearchQuery) (string, error) {
	base, err := url.Parse(s.config.BaseURL + "/lowongan")
	if err != nil {
		return "", err
	}

	values := base.Query()
	if strings.TrimSpace(query.Keyword) != "" {
		values.Set("q", strings.TrimSpace(query.Keyword))
	}
	if strings.TrimSpace(query.Location) != "" {
		values.Set("l", strings.TrimSpace(query.Location))
	}
	base.RawQuery = values.Encode()

	return base.String(), nil
}

func (s *LokerIDSource) extractJobs(doc *goquery.Document) []ScrapedJob {
	var jobs []ScrapedJob
	selectors := []string{
		".job-list .job, .job-list .job-card, article.job, article[class*='job']",
		".job-card, .vacancy-card, .lowongan, [class*='lowongan']",
	}

	for _, selector := range selectors {
		doc.Find(selector).Each(func(_ int, card *goquery.Selection) {
			if job, ok := s.extractJobCard(card); ok {
				jobs = append(jobs, job)
			}
		})
		if len(jobs) > 0 {
			break
		}
	}

	return jobs
}

func (s *LokerIDSource) extractJobCard(card *goquery.Selection) (ScrapedJob, bool) {
	titleSel := firstNonEmptySelection(card, []string{
		".job-title a", ".job-title", "h2 a", "h3 a", "a[href*='lowongan']",
	})
	companySel := firstNonEmptySelection(card, []string{
		".company-name", ".company", "[class*='company']", ".employer",
	})
	locationSel := firstNonEmptySelection(card, []string{
		".location", "[class*='location']", ".job-location",
	})

	title := cleanText(titleSel.Text())
	company := cleanText(companySel.Text())
	location := cleanText(locationSel.Text())
	if title == "" || company == "" {
		return ScrapedJob{}, false
	}

	href := titleSel.AttrOr("href", "")
	sourceURL := absolutizeURL(s.config.BaseURL, href)
	externalID := externalIDFromURL(sourceURL)

	return ScrapedJob{
		Title:       title,
		Company:     company,
		Location:    location,
		Description: cleanText(card.Find(".description, .job-description, [class*='description']").First().Text()),
		ApplyURL:    sourceURL,
		SourceURL:   sourceURL,
		Source:      s.Name(),
		ExternalID:  externalID,
		Raw: map[string]any{
			"source_selector": "loker_id_job_card",
		},
	}, true
}

func firstNonEmptySelection(parent *goquery.Selection, selectors []string) *goquery.Selection {
	for _, selector := range selectors {
		match := parent.Find(selector).First()
		if cleanText(match.Text()) != "" || match.AttrOr("href", "") != "" {
			return match
		}
	}
	return parent.Find("__missing__")
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func absolutizeURL(baseURL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}
	return base.ResolveReference(parsed).String()
}

func externalIDFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	path := strings.Trim(parsed.Path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
