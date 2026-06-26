package scraper

import (
	"net/http"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const IndeedSourceName = "indeed"

type IndeedSource struct {
	*embeddedSource
}

func NewIndeedSource(client *http.Client, antiDetection *AntiDetection) *IndeedSource {
	config := SourceConfig{
		Name:             IndeedSourceName,
		DisplayName:      "Indeed",
		BaseURL:          "https://id.indeed.com",
		Enabled:          true,
		MaxPerHour:       20,
		BaseDelay:        4 * time.Second,
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
	return &IndeedSource{embeddedSource: &embeddedSource{
		client:        client,
		antiDetection: antiDetection,
		config:        config,
		searchPath:    "/jobs",
		queryParams: func(query SearchQuery) map[string]string {
			return map[string]string{"q": query.Keyword, "l": query.Location}
		},
		parser: func(doc *goquery.Document, baseURL, sourceName string) []ScrapedJob {
			if jobs := extractJSONLDJobs(doc, baseURL, sourceName); len(jobs) > 0 {
				return jobs
			}
			return extractMosaicJobs(doc, baseURL, sourceName)
		},
	}}
}
