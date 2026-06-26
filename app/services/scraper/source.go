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
	Title       string
	Company     string
	Location    string
	Description string
	ApplyURL    string
	SourceURL   string
	Source      string
	ExternalID  string
	PostedAt    *time.Time
	Raw         map[string]any
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
}

type JobSource interface {
	Name() string
	Config() SourceConfig
	Scrape(ctx context.Context, query SearchQuery) ([]ScrapedJob, error)
}
