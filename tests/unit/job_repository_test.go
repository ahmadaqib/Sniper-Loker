package unit

import (
	"context"
	"testing"

	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services/scraper"
)

func TestContentHashDeterministicAndSensitive(t *testing.T) {
	job := scraper.ScrapedJob{
		Title:       "Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: "Build APIs",
		SourceURL:   "https://example.test/jobs/1",
		Source:      "loker_id",
	}

	first := repositories.ContentHash(job)
	second := repositories.ContentHash(job)
	if first != second {
		t.Fatalf("expected stable hash, got %s and %s", first, second)
	}

	job.Description = "Build APIs and workers"
	if first == repositories.ContentHash(job) {
		t.Fatal("expected hash to change when content changes")
	}
}

func TestContentHashDistinctJobsDoNotCollide(t *testing.T) {
	first := scraper.ScrapedJob{
		Title:       "Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: "Build APIs",
		SourceURL:   "https://example.test/jobs/backend",
		Source:      "loker_id",
	}
	second := scraper.ScrapedJob{
		Title:       "Finance Staff",
		Company:     "Beta",
		Location:    "Bandung",
		Description: "Manage invoices",
		SourceURL:   "https://example.test/jobs/finance",
		Source:      "loker_id",
	}

	if repositories.ContentHash(first) == repositories.ContentHash(second) {
		t.Fatal("expected different jobs to produce different hashes")
	}
}

func TestInMemoryJobRepositoryDeduplicatesByContentHash(t *testing.T) {
	repo := repositories.NewInMemoryJobRepository(0)
	job := scraper.ScrapedJob{
		Title:       "Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: "Build APIs",
		SourceURL:   "https://example.test/jobs/1",
		Source:      "loker_id",
		ExternalID:  "1",
	}

	first, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{job})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{job})
	if err != nil {
		t.Fatal(err)
	}

	if first.Inserted != 1 {
		t.Fatalf("expected first insert, got %+v", first)
	}
	if second.Duplicated != 1 || second.Updated != 1 {
		t.Fatalf("expected duplicate update, got %+v", second)
	}
	if got := len(repo.Jobs()); got != 1 {
		t.Fatalf("expected one stored job, got %d", got)
	}
}

func TestInMemoryJobRepositoryDeduplicatesBySourceReference(t *testing.T) {
	repo := repositories.NewInMemoryJobRepository(0)
	firstJob := scraper.ScrapedJob{
		Title:       "Backend Engineer",
		Company:     "Acme",
		Location:    "Jakarta",
		Description: "Build APIs",
		SourceURL:   "https://example.test/jobs/1",
		Source:      "loker_id",
		ExternalID:  "same-id",
	}
	secondJob := firstJob
	secondJob.Description = "Updated body from source"

	if _, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{firstJob}); err != nil {
		t.Fatal(err)
	}
	result, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{secondJob})
	if err != nil {
		t.Fatal(err)
	}

	if result.Duplicated != 1 {
		t.Fatalf("expected source ref duplicate, got %+v", result)
	}
	if got := repo.Jobs()[0].DuplicateTier; got != repositories.DedupTierSourceRef {
		t.Fatalf("expected duplicate tier %q, got %q", repositories.DedupTierSourceRef, got)
	}
}

func TestInMemoryJobRepositoryDeduplicatesByNormalizedKey(t *testing.T) {
	repo := repositories.NewInMemoryJobRepository(0)
	firstJob := scraper.ScrapedJob{
		Title:       "Backend Engineer",
		Company:     "Acme Indonesia",
		Location:    "Jakarta Selatan",
		Description: "Build APIs",
		SourceURL:   "https://example.test/jobs/1",
		Source:      "loker_id",
	}
	secondJob := scraper.ScrapedJob{
		Title:       " backend   engineer ",
		Company:     "ACME Indonesia",
		Location:    "Jakarta-Selatan",
		Description: "Different body and URL",
		SourceURL:   "https://example.test/jobs/2",
		Source:      "loker_id",
	}

	if _, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{firstJob}); err != nil {
		t.Fatal(err)
	}
	result, err := repo.UpsertJobs(context.Background(), []scraper.ScrapedJob{secondJob})
	if err != nil {
		t.Fatal(err)
	}

	if result.Duplicated != 1 {
		t.Fatalf("expected normalized duplicate, got %+v", result)
	}
	if got := repo.Jobs()[0].DuplicateTier; got != repositories.DedupTierNormalized {
		t.Fatalf("expected duplicate tier %q, got %q", repositories.DedupTierNormalized, got)
	}
}
