package scraper

import (
	"context"
	"time"
)

type SearchQuery struct {
	Keyword  string
	Location string
}

type ScrapedJob struct {
	Title       string         `json:"title"`
	Company     string         `json:"company"`
	Location    string         `json:"location"`
	Description string         `json:"description"`
	Salary      string         `json:"salary"`
	ApplyURL    string         `json:"apply_url"`
	SourceURL   string         `json:"source_url"`
	Source      string         `json:"source"`
	ExternalID  string         `json:"external_id"`
	PostedAt    *time.Time     `json:"posted_at"`
	Raw         map[string]any `json:"raw"`
}

type SourceConfig struct {
	Name             string
	DisplayName      string
	BaseURL          string
	Enabled          bool
	MaxPerHour       int
	BaseDelay        time.Duration
	Jitter           time.Duration
	RequestTimeout   time.Duration
	CircuitThreshold int
	CircuitCooldown  time.Duration
	UseUTLS          bool
}

type JobSource interface {
	Name() string
	Config() SourceConfig
	Scrape(ctx context.Context, query SearchQuery) ([]ScrapedJob, error)
}

type ConfigurableSource interface {
	JobSource
	ApplyConfig(config SourceConfig)
}
