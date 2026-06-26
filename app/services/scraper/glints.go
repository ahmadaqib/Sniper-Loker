package scraper

import (
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const GlintsSourceName = "glints"

type GlintsSource struct {
	*embeddedSource
}

func NewGlintsSource(client *http.Client, antiDetection *AntiDetection) *GlintsSource {
	config := SourceConfig{
		Name:             GlintsSourceName,
		DisplayName:      "Glints",
		BaseURL:          "https://glints.com",
		Enabled:          true,
		MaxPerHour:       18,
		BaseDelay:        4 * time.Second,
		Jitter:           2 * time.Second,
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
	return &GlintsSource{embeddedSource: &embeddedSource{
		client:        client,
		antiDetection: antiDetection,
		config:        config,
		searchPath:    "/en-id/jobs",
		queryParams: func(query SearchQuery) map[string]string {
			return map[string]string{"keyword": query.Keyword, "country": "ID", "locationName": query.Location}
		},
		parser: func(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
			if jobs := extractNextDataJobs(doc, baseURL, sourceName); len(jobs) > 0 {
				return jobs
			}
			return extractJSONLDJobs(doc, baseURL, sourceName)
		},
	}}
}
