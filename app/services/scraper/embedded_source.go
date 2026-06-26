package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type embeddedSource struct {
	client        *http.Client
	antiDetection *AntiDetection
	config        SourceConfig
	searchPath    string
	queryParams   func(SearchQuery) map[string]string
	parser        func(*goquery.Document, string, string) []ScrapedJob
}

func (s *embeddedSource) Name() string {
	return s.config.Name
}

func (s *embeddedSource) Config() SourceConfig {
	return s.config
}

func (s *embeddedSource) ApplyConfig(config SourceConfig) {
	s.config = mergeSourceConfig(s.config, config)
	if s.config.UseUTLS {
		s.client = NewUTLSHTTPClient(s.config.RequestTimeout)
	}
}

func (s *embeddedSource) Scrape(ctx context.Context, query SearchQuery) ([]ScrapedJob, error) {
	resp, err := s.antiDetection.NavigateTo(ctx, s.client, s.config.BaseURL, "")
	if err != nil {
		return nil, fmt.Errorf("warm up %s session: %w", s.config.DisplayName, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if err := s.antiDetection.HumanDelay(ctx, s.config.BaseDelay, s.config.Jitter); err != nil {
		return nil, err
	}

	endpoint, err := s.searchURL(query)
	if err != nil {
		return nil, err
	}
	resp, err = s.antiDetection.NavigateTo(ctx, s.client, endpoint, s.config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", s.config.DisplayName, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", s.config.DisplayName, err)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || IsChallengePage(string(body)) {
		s.antiDetection.ResetSession(resp.Request.URL.Hostname())
		return nil, ErrChallengeDetected
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse %s response: %w", s.config.DisplayName, err)
	}
	if result := IsValidJobPage(resp.StatusCode, doc); !result.Valid {
		return nil, fmt.Errorf("invalid %s page: %s", s.config.DisplayName, result.Reason)
	}

	jobs := s.parser(doc, s.config.BaseURL, s.Name())
	if len(jobs) == 0 {
		jobs = fallbackCardJobs(doc, s.config.BaseURL, s.Name())
	}
	return jobs, nil
}

func (s *embeddedSource) searchURL(query SearchQuery) (string, error) {
	base, err := url.Parse(s.config.BaseURL + s.searchPath)
	if err != nil {
		return "", err
	}
	values := base.Query()
	for key, value := range s.queryParams(query) {
		if strings.TrimSpace(value) != "" {
			values.Set(key, strings.TrimSpace(value))
		}
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func fallbackCardJobs(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
	var jobs []ScrapedJob
	doc.Find(".job, .job-card, .jobsearch-SerpJobCard, article[class*='job'], [data-testid*='job'], [class*='job-card']").Each(func(_ int, card *goquery.Selection) {
		titleSel := firstNonEmptySelection(card, []string{"h2 a", "h3 a", ".job-title a", ".jobTitle a", "[data-testid='job-title']", "a[href*='job']"})
		companySel := firstNonEmptySelection(card, []string{".company", ".company-name", ".companyName", "[data-testid='company-name']", "[class*='company']"})
		locationSel := firstNonEmptySelection(card, []string{".location", ".companyLocation", "[data-testid='text-location']", "[class*='location']"})
		title := cleanText(titleSel.Text())
		company := cleanText(companySel.Text())
		if title == "" || company == "" {
			return
		}
		sourceURL := absolutizeURL(baseURL, titleSel.AttrOr("href", ""))
		jobs = append(jobs, ScrapedJob{
			Title:       title,
			Company:     company,
			Location:    cleanText(locationSel.Text()),
			Description: cleanText(card.Find(".description, .summary, [class*='description'], [class*='snippet']").First().Text()),
			ApplyURL:    sourceURL,
			SourceURL:   sourceURL,
			Source:      sourceName,
			ExternalID:  externalIDFromURL(sourceURL),
			Raw:         map[string]any{"source_selector": "fallback_card"},
		})
	})
	return jobs
}
