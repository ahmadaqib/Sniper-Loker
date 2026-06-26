package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
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
		UseUTLS:          true,
	}
	if client == nil {
		client = NewUTLSHTTPClient(config.RequestTimeout)
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

func (s *LokerIDSource) ApplyConfig(config SourceConfig) {
	s.config = mergeSourceConfig(s.config, config)
	if s.config.UseUTLS {
		s.client = NewUTLSHTTPClient(s.config.RequestTimeout)
	}
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
		// As fallback for Loker.id Remix SPA, just check status
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("invalid loker.id page: %s", result.Reason)
		}
	}

	return s.extractJobs(string(body)), nil
}

func (s *LokerIDSource) searchURL(query SearchQuery) (string, error) {
	base, err := url.Parse(s.config.BaseURL + "/cari-lowongan-kerja")
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

func (s *LokerIDSource) extractJobs(body string) []ScrapedJob {
	re := regexp.MustCompile(`window\.__remixContext\s*=\s*(\{.*?\});\s*</script>`)
	match := re.FindStringSubmatch(body)
	if len(match) < 2 {
		return nil
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
		return nil
	}

	state, ok := data["state"].(map[string]any)
	if !ok {
		return nil
	}

	loaderData, ok := state["loaderData"].(map[string]any)
	if !ok {
		return nil
	}

	var jobsArray []any
	for k, v := range loaderData {
		if strings.Contains(k, "cari-lowongan-kerja") {
			if route, ok := v.(map[string]any); ok {
				if jobs, ok := route["jobs"].([]any); ok {
					jobsArray = jobs
					break
				}
			}
		}
	}

	var result []ScrapedJob
	for _, j := range jobsArray {
		if job, ok := j.(map[string]any); ok {
			title := cleanText(asString(job["title"]))
			company := cleanText(asString(job["company_name"]))
			
			var location string
			if locs, ok := job["locations"].([]any); ok && len(locs) > 0 {
				if l, ok := locs[0].(map[string]any); ok {
					location = cleanText(asString(l["name"]))
				}
			}

			salary := cleanText(asString(job["job_salary"]))
			if salary == "" {
				if salObj, ok := job["salary"].(map[string]any); ok {
					salary = cleanText(asString(salObj["name"]))
				}
			}
			
			slug := asString(job["slug"])

			var postedAt *time.Time
			if pubStr := asString(job["published_at"]); pubStr != "" {
				if t, err := time.Parse("2006-01-02 15:04:05", pubStr); err == nil {
					postedAt = &t
				}
			}

			if title == "" || company == "" {
				continue
			}

			desc := cleanText(asString(job["short_description"]))
			if desc == "" {
				var parts []string
				if cat := cleanText(asString(job["category"])); cat != "" {
					parts = append(parts, "Kategori: "+cat)
				}
				if exp := cleanText(asString(job["job_experience"])); exp != "" {
					parts = append(parts, "Pengalaman: "+exp)
				}
				if jt := cleanText(asString(job["job_type"])); jt != "" {
					parts = append(parts, "Tipe: "+jt)
				}
				if bens := cleanText(asString(job["job_benefits"])); bens != "" {
					parts = append(parts, "Benefit: "+strings.ReplaceAll(bens, "\n", ", "))
				}
				desc = strings.Join(parts, " | ")
			}

			sourceURL := s.config.BaseURL + "/lowongan-kerja/" + slug
			
			result = append(result, ScrapedJob{
				Title:       title,
				Company:     company,
				Location:    location,
				Description: desc,
				Salary:      salary,
				ApplyURL:    sourceURL,
				SourceURL:   sourceURL,
				Source:      s.Name(),
				ExternalID:  asString(job["id"]),
				PostedAt:    postedAt,
				Raw: map[string]any{
					"source_selector": "remix_context",
				},
			})
		}
	}

	return result
}

func (s *LokerIDSource) extractJobCard(card *goquery.Selection) (ScrapedJob, bool) {
	// Deprecated: Loker.id now uses Remix SPA data in script tags.
	return ScrapedJob{}, false
}
