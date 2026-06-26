package scraper

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const KarirComSourceName = "karir_com"

type KarirComSource struct {
	client        *http.Client
	antiDetection *AntiDetection
	config        SourceConfig
}

func NewKarirComSource(client *http.Client, antiDetection *AntiDetection) *KarirComSource {
	config := SourceConfig{
		Name:             KarirComSourceName,
		DisplayName:      "Karir.com",
		BaseURL:          "https://www.karir.com",
		Enabled:          true,
		MaxPerHour:       25,
		BaseDelay:        3 * time.Second,
		Jitter:           1500 * time.Millisecond,
		RequestTimeout:   10 * time.Second,
		CircuitThreshold: 3,
		CircuitCooldown:  30 * time.Minute,
		UseUTLS:          true,
	}
	if client == nil {
		client = NewUTLSHTTPClient(config.RequestTimeout)
	}
	if antiDetection == nil {
		antiDetection = NewAntiDetection(nil)
	}
	return &KarirComSource{client: client, antiDetection: antiDetection, config: config}
}

func (s *KarirComSource) Name() string {
	return s.config.Name
}

func (s *KarirComSource) Config() SourceConfig {
	return s.config
}

func (s *KarirComSource) ApplyConfig(config SourceConfig) {
	s.config = mergeSourceConfig(s.config, config)
	if s.config.UseUTLS {
		s.client = NewUTLSHTTPClient(s.config.RequestTimeout)
	}
}

func (s *KarirComSource) Scrape(ctx context.Context, query SearchQuery) ([]ScrapedJob, error) {
	if _, err := s.warmUp(ctx); err != nil {
		return nil, err
	}
	if err := s.antiDetection.HumanDelay(ctx, s.config.BaseDelay, s.config.Jitter); err != nil {
		return nil, err
	}

	endpoint, err := s.searchURL(query)
	if err != nil {
		return nil, err
	}
	resp, err := s.antiDetection.NavigateTo(ctx, s.client, endpoint, s.config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("request karir.com: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read karir.com response: %w", err)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || IsChallengePage(string(body)) {
		s.antiDetection.ResetSession(resp.Request.URL.Hostname())
		return nil, ErrChallengeDetected
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse karir.com response: %w", err)
	}
	if result := IsValidJobPage(resp.StatusCode, doc); !result.Valid {
		return nil, fmt.Errorf("invalid karir.com page: %s", result.Reason)
	}

	return s.extractJobs(doc), nil
}

func (s *KarirComSource) warmUp(ctx context.Context) (*http.Response, error) {
	resp, err := s.antiDetection.NavigateTo(ctx, s.client, s.config.BaseURL, "")
	if err != nil {
		return nil, fmt.Errorf("warm up karir.com session: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp, nil
}

func (s *KarirComSource) searchURL(query SearchQuery) (string, error) {
	base, err := url.Parse(s.config.BaseURL + "/job-search")
	if err != nil {
		return "", err
	}
	values := base.Query()
	if strings.TrimSpace(query.Keyword) != "" {
		values.Set("keywords", strings.TrimSpace(query.Keyword))
	}
	if strings.TrimSpace(query.Location) != "" {
		values.Set("location", strings.TrimSpace(query.Location))
	}
	base.RawQuery = values.Encode()
	return base.String(), nil
}

func (s *KarirComSource) extractJobs(doc *goquery.Document) []ScrapedJob {
	if jobs := extractJSONLDJobs(doc, s.config.BaseURL, s.Name()); len(jobs) > 0 {
		return jobs
	}

	var jobs []ScrapedJob
	doc.Find(".job, .job-card, .job-list-item, article[class*='job'], [class*='job-list'] [class*='item']").Each(func(_ int, card *goquery.Selection) {
		titleSel := firstNonEmptySelection(card, []string{"h2 a", "h3 a", ".job-title a", ".job-title", "a[href*='/lowongan']", "a[href*='/job']"})
		companySel := firstNonEmptySelection(card, []string{".company", ".company-name", "[class*='company']", ".employer"})
		locationSel := firstNonEmptySelection(card, []string{".location", "[class*='location']", ".job-location"})
		title := cleanText(titleSel.Text())
		company := cleanText(companySel.Text())
		if title == "" || company == "" {
			return
		}
		sourceURL := absolutizeURL(s.config.BaseURL, titleSel.AttrOr("href", ""))
		jobs = append(jobs, ScrapedJob{
			Title:       title,
			Company:     company,
			Location:    cleanText(locationSel.Text()),
			Description: cleanText(card.Find(".description, .job-description, [class*='description']").First().Text()),
			ApplyURL:    sourceURL,
			SourceURL:   sourceURL,
			Source:      s.Name(),
			ExternalID:  externalIDFromURL(sourceURL),
			Raw:         map[string]any{"source_selector": "karir_com_card"},
		})
	})
	return jobs
}
