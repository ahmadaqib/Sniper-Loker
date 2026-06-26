package services

import (
	"context"
	"errors"

	"Goravel-learn/app/models"
	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services/scraper"
)

type ScraperService struct {
	registry  *scraper.SourceRegistry
	breaker   *scraper.CircuitBreaker
	repo      repositories.JobRepository
	broadcast *BroadcastService
}

func NewScraperService(registry *scraper.SourceRegistry, breaker *scraper.CircuitBreaker, repo repositories.JobRepository) *ScraperService {
	return &ScraperService{registry: registry, breaker: breaker, repo: repo, broadcast: DefaultBroadcastService}
}

type ScrapeSummary struct {
	SourceResults []SourceScrapeResult
	Inserted      int
	Updated       int
	Duplicated    int
}

type SourceScrapeResult struct {
	Source    string
	Fetched   int
	Inserted  int
	Updated   int
	Duplicate int
	Error     string
}

func (s *ScraperService) Scrape(ctx context.Context, query scraper.SearchQuery) ScrapeSummary {
	var summary ScrapeSummary
	for _, source := range s.registry.Enabled() {
		result := SourceScrapeResult{Source: source.Name()}
		if err := s.breaker.BeforeRequest(source.Name()); err != nil {
			result.Error = err.Error()
			summary.SourceResults = append(summary.SourceResults, result)
			continue
		}

		jobs, err := source.Scrape(ctx, query)
		if err != nil {
			snapshot := s.breaker.RecordFailure(source.Name(), err)
			result.Error = err.Error()
			_ = s.repo.UpdateSourceStatus(ctx, source.Name(), models.SourceStatusUpdate{
				State:       snapshot.State,
				ErrorCount:  snapshot.ErrorCount,
				LastError:   snapshot.LastError,
				OpenedUntil: snapshot.OpenedUntil,
			})
			summary.SourceResults = append(summary.SourceResults, result)
			continue
		}

		upsert, err := s.repo.UpsertJobs(ctx, jobs)
		if err != nil {
			snapshot := s.breaker.RecordFailure(source.Name(), err)
			result.Error = err.Error()
			_ = s.repo.UpdateSourceStatus(ctx, source.Name(), models.SourceStatusUpdate{
				State:       snapshot.State,
				ErrorCount:  snapshot.ErrorCount,
				LastError:   snapshot.LastError,
				OpenedUntil: snapshot.OpenedUntil,
			})
			summary.SourceResults = append(summary.SourceResults, result)
			continue
		}

		snapshot := s.breaker.RecordSuccess(source.Name())
		_ = s.repo.UpdateSourceStatus(ctx, source.Name(), models.SourceStatusUpdate{
			State:      snapshot.State,
			ErrorCount: snapshot.ErrorCount,
			Success:    true,
		})

		result.Fetched = len(jobs)
		result.Inserted = upsert.Inserted
		result.Updated = upsert.Updated
		result.Duplicate = upsert.Duplicated
		summary.Inserted += upsert.Inserted
		summary.Updated += upsert.Updated
		summary.Duplicated += upsert.Duplicated
		summary.SourceResults = append(summary.SourceResults, result)
		if s.broadcast != nil {
			s.broadcast.BroadcastJobs(query.Keyword, query.Location, jobs)
		}
	}

	return summary
}

func (s ScrapeSummary) Err() error {
	if len(s.SourceResults) == 0 {
		return errors.New("no enabled scraper sources")
	}
	for _, result := range s.SourceResults {
		if result.Error == "" {
			return nil
		}
	}
	return errors.New("all scraper sources failed")
}
