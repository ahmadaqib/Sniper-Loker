package services

import (
	"context"
	"os"
	"strconv"
	"time"

	"Goravel-learn/app/facades"
	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services/scraper"
)

func NewDefaultScraperService(ctx context.Context) (*ScraperService, error) {
	client, err := repositories.NewMongoClient(envString("MONGODB_URI", "mongodb://localhost:27017"))
	if err != nil {
		return nil, err
	}

	repo := repositories.NewMongoJobRepository(
		client,
		envString("MONGODB_DATABASE", "loker_radar"),
		time.Duration(envInt("JOB_TTL_DAYS", 7))*24*time.Hour,
	)
	if err := repo.EnsureIndexes(ctx); err != nil {
		return nil, err
	}

	registry := scraper.NewSourceRegistry()
	if err := registry.Register(scraper.NewLokerIDSource(nil, nil)); err != nil {
		return nil, err
	}

	breaker := scraper.NewCircuitBreaker(
		envInt("SCRAPER_CIRCUIT_BREAKER_THRESHOLD", 3),
		time.Duration(envInt("SCRAPER_CIRCUIT_COOLDOWN_MINUTES", 30))*time.Minute,
	)

	return NewScraperService(registry, breaker, repo), nil
}

func envString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return facades.Config().EnvString(key, fallback)
}

func envInt(key string, fallback int) int {
	if value, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return value
	}
	return facades.Config().GetInt(key, fallback)
}
