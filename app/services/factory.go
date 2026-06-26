package services

import (
	"context"
	"os"
	"strconv"
	"time"

	"Goravel-learn/app/facades"
	"Goravel-learn/app/repositories"
	"Goravel-learn/app/services/scraper"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func NewDefaultScraperService(ctx context.Context) (*ScraperService, error) {
	client, err := repositories.NewMongoClient(envString("MONGODB_URI", "mongodb://localhost:27017"))
	if err != nil {
		return nil, err
	}

	repo := newMongoRepository(client)
	if err := repo.EnsureIndexes(ctx); err != nil {
		return nil, err
	}

	registry := scraper.NewSourceRegistry()
	sources := []scraper.JobSource{
		scraper.NewLokerIDSource(nil, nil),
		scraper.NewKarirComSource(nil, nil),
		scraper.NewIndeedSource(nil, nil),
		scraper.NewGlintsSource(nil, nil),
	}
	configs, err := repo.SourceConfigs(ctx)
	if err != nil {
		return nil, err
	}
	for _, source := range sources {
		if config, ok := configs[source.Name()]; ok {
			if configurable, ok := source.(scraper.ConfigurableSource); ok {
				configurable.ApplyConfig(config)
			}
		}
		if err := registry.Register(source); err != nil {
			return nil, err
		}
	}

	breaker := scraper.NewCircuitBreaker(
		envInt("SCRAPER_CIRCUIT_BREAKER_THRESHOLD", 3),
		time.Duration(envInt("SCRAPER_CIRCUIT_COOLDOWN_MINUTES", 30))*time.Minute,
	)

	return NewScraperService(registry, breaker, repo), nil
}

func NewDefaultJobRepository(ctx context.Context, ensureIndexes ...bool) (repositories.JobRepository, error) {
	client, err := repositories.NewMongoClientWithTimeout(
		envString("MONGODB_URI", "mongodb://localhost:27017"),
		time.Duration(envInt("MONGODB_TIMEOUT_SECONDS", 2))*time.Second,
	)
	if err != nil {
		return nil, err
	}
	repo := newMongoRepository(client)
	shouldEnsure := true
	if len(ensureIndexes) > 0 {
		shouldEnsure = ensureIndexes[0]
	}
	if shouldEnsure {
		if err := repo.EnsureIndexes(ctx); err != nil {
			return nil, err
		}
	}
	return repo, nil
}

func newMongoRepository(client *mongo.Client) *repositories.MongoJobRepository {
	return repositories.NewMongoJobRepository(
		client,
		envString("MONGODB_DATABASE", "loker_radar"),
		time.Duration(envInt("JOB_TTL_DAYS", 7))*24*time.Hour,
	)
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
