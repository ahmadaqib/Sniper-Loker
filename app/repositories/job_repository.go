package repositories

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"Goravel-learn/app/models"
	"Goravel-learn/app/services/scraper"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	DedupTierContentHash = "content_hash"
	DedupTierSourceRef   = "source_ref"
	DedupTierNormalized  = "normalized_key"
)

type UpsertResult struct {
	Inserted   int
	Updated    int
	Duplicated int
}

type JobRepository interface {
	UpsertJobs(ctx context.Context, jobs []scraper.ScrapedJob) (UpsertResult, error)
	UpdateSourceStatus(ctx context.Context, source string, update models.SourceStatusUpdate) error
	ListJobs(ctx context.Context, filter JobFilter) ([]models.Job, error)
	SourceConfigs(ctx context.Context) (map[string]scraper.SourceConfig, error)
}

type JobFilter struct {
	Keyword  string
	Location string
	Since    time.Time
	Limit    int64
}

type MongoJobRepository struct {
	database      *mongo.Database
	jobs          *mongo.Collection
	sources       *mongo.Collection
	searchQueries *mongo.Collection
	jobTTL        time.Duration
	now           func() time.Time
}

func NewMongoJobRepository(client *mongo.Client, databaseName string, jobTTL time.Duration) *MongoJobRepository {
	db := client.Database(databaseName)
	if jobTTL <= 0 {
		jobTTL = 7 * 24 * time.Hour
	}

	return &MongoJobRepository{
		database:      db,
		jobs:          db.Collection("jobs"),
		sources:       db.Collection("sources"),
		searchQueries: db.Collection("search_queries"),
		jobTTL:        jobTTL,
		now:           time.Now,
	}
}

func NewMongoClient(uri string) (*mongo.Client, error) {
	if strings.TrimSpace(uri) == "" {
		uri = "mongodb://localhost:27017"
	}
	return mongo.Connect(options.Client().ApplyURI(uri))
}

func (r *MongoJobRepository) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "content_hash", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_content_hash")},
		{Keys: bson.D{{Key: "source", Value: 1}, {Key: "external_id", Value: 1}}, Options: options.Index().SetSparse(true).SetName("idx_source_external_id")},
		{Keys: bson.D{{Key: "normalized_key", Value: 1}}, Options: options.Index().SetName("idx_normalized_key")},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_jobs")},
	}
	if _, err := r.jobs.Indexes().CreateMany(ctx, indexes); err != nil {
		return err
	}

	sourceIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "name", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_source_name")},
	}
	if _, err := r.sources.Indexes().CreateMany(ctx, sourceIndexes); err != nil {
		return err
	}

	queryIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "keyword", Value: 1}, {Key: "location", Value: 1}}, Options: options.Index().SetUnique(true).SetName("uniq_search_query")},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0).SetName("ttl_search_queries")},
	}
	_, err := r.searchQueries.Indexes().CreateMany(ctx, queryIndexes)
	return err
}

func (r *MongoJobRepository) UpsertJobs(ctx context.Context, scraped []scraper.ScrapedJob) (UpsertResult, error) {
	var result UpsertResult
	for _, scrapedJob := range scraped {
		job := BuildJob(scrapedJob, r.now(), r.jobTTL)
		duplicate, tier, err := r.findDuplicate(ctx, job)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return result, err
		}

		if duplicate != nil {
			job.DuplicateTier = tier
			job.DuplicateJobID = duplicate.ID
			update := bson.M{
				"$set": bson.M{
					"last_seen_at": job.LastSeenAt,
					"expires_at":   job.ExpiresAt,
				},
			}
			if _, err := r.jobs.UpdateOne(ctx, bson.M{"_id": duplicate.ID}, update); err != nil {
				return result, err
			}
			result.Duplicated++
			result.Updated++
			continue
		}

		if _, err := r.jobs.InsertOne(ctx, job); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				result.Duplicated++
				continue
			}
			return result, err
		}
		result.Inserted++
	}

	return result, nil
}

func (r *MongoJobRepository) findDuplicate(ctx context.Context, job models.Job) (*models.Job, string, error) {
	tiers := []struct {
		name   string
		filter bson.M
	}{
		{DedupTierContentHash, bson.M{"content_hash": job.ContentHash}},
		{DedupTierSourceRef, bson.M{"source": job.Source, "external_id": job.ExternalID}},
		{DedupTierNormalized, bson.M{"normalized_key": job.NormalizedKey}},
	}

	for _, tier := range tiers {
		if tier.name == DedupTierSourceRef && strings.TrimSpace(job.ExternalID) == "" {
			continue
		}

		var existing models.Job
		err := r.jobs.FindOne(ctx, tier.filter).Decode(&existing)
		if err == nil {
			return &existing, tier.name, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, "", err
		}
	}

	return nil, "", mongo.ErrNoDocuments
}

func (r *MongoJobRepository) UpdateSourceStatus(ctx context.Context, source string, update models.SourceStatusUpdate) error {
	now := r.now()
	set := bson.M{
		"updated_at":    now,
		"circuit_state": update.State,
		"error_count":   update.ErrorCount,
		"enabled":       true,
		"name":          source,
		"display_name":  source,
		"last_error":    update.LastError,
		"opened_until":  update.OpenedUntil,
	}
	if update.Success {
		set["last_success_at"] = now
		set["last_error"] = ""
	} else if update.LastError != "" {
		set["last_failure_at"] = now
	}

	_, err := r.sources.UpdateOne(
		ctx,
		bson.M{"name": source},
		bson.M{"$set": set, "$setOnInsert": bson.M{"created_at": now}},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (r *MongoJobRepository) SourceConfigs(ctx context.Context) (map[string]scraper.SourceConfig, error) {
	cursor, err := r.sources.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	configs := make(map[string]scraper.SourceConfig)
	for cursor.Next(ctx) {
		var source models.Source
		if err := cursor.Decode(&source); err != nil {
			return nil, err
		}
		useUTLS := true
		if source.UseUTLS != nil {
			useUTLS = *source.UseUTLS
		}
		configs[source.Name] = scraper.SourceConfig{
			Name:             source.Name,
			DisplayName:      source.DisplayName,
			Enabled:          source.Enabled,
			MaxPerHour:       source.MaxPerHour,
			BaseDelay:        time.Duration(source.BaseDelayMillis) * time.Millisecond,
			Jitter:           time.Duration(source.JitterMillis) * time.Millisecond,
			RequestTimeout:   time.Duration(source.RequestTimeoutMillis) * time.Millisecond,
			CircuitThreshold: source.CircuitThreshold,
			CircuitCooldown:  time.Duration(source.CircuitCooldownMillis) * time.Millisecond,
			UseUTLS:          useUTLS,
		}
	}
	return configs, cursor.Err()
}

func (r *MongoJobRepository) ListJobs(ctx context.Context, filter JobFilter) ([]models.Job, error) {
	query := bson.M{}
	if !filter.Since.IsZero() {
		query["last_seen_at"] = bson.M{"$gt": filter.Since}
	}
	if filter.Keyword != "" {
		query["$or"] = []bson.M{
			{"title": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			{"company": bson.M{"$regex": filter.Keyword, "$options": "i"}},
			{"description": bson.M{"$regex": filter.Keyword, "$options": "i"}},
		}
	}
	if filter.Location != "" {
		query["location"] = bson.M{"$regex": filter.Location, "$options": "i"}
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	cursor, err := r.jobs.Find(ctx, query, options.Find().SetSort(bson.D{{Key: "last_seen_at", Value: -1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var jobs []models.Job
	for cursor.Next(ctx) {
		var job models.Job
		if err := cursor.Decode(&job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, cursor.Err()
}

type InMemoryJobRepository struct {
	mu      sync.Mutex
	jobs    []models.Job
	sources map[string]models.SourceStatusUpdate
	ttl     time.Duration
	now     func() time.Time
}

func NewInMemoryJobRepository(ttl time.Duration) *InMemoryJobRepository {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &InMemoryJobRepository{
		sources: make(map[string]models.SourceStatusUpdate),
		ttl:     ttl,
		now:     time.Now,
	}
}

func (r *InMemoryJobRepository) UpsertJobs(_ context.Context, scraped []scraper.ScrapedJob) (UpsertResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result UpsertResult
	for _, scrapedJob := range scraped {
		job := BuildJob(scrapedJob, r.now(), r.ttl)
		if duplicate, tier := r.findDuplicate(job); duplicate != nil {
			duplicate.LastSeenAt = job.LastSeenAt
			duplicate.ExpiresAt = job.ExpiresAt
			duplicate.DuplicateTier = tier
			result.Duplicated++
			result.Updated++
			continue
		}

		job.ID = bson.NewObjectID()
		r.jobs = append(r.jobs, job)
		result.Inserted++
	}

	return result, nil
}

func (r *InMemoryJobRepository) UpdateSourceStatus(_ context.Context, source string, update models.SourceStatusUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[source] = update
	return nil
}

func (r *InMemoryJobRepository) SourceConfigs(_ context.Context) (map[string]scraper.SourceConfig, error) {
	return map[string]scraper.SourceConfig{}, nil
}

func (r *InMemoryJobRepository) ListJobs(_ context.Context, filter JobFilter) ([]models.Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var jobs []models.Job
	for _, job := range r.jobs {
		if !filter.Since.IsZero() && !job.LastSeenAt.After(filter.Since) {
			continue
		}
		if filter.Keyword != "" {
			needle := Normalize(filter.Keyword)
			haystack := Normalize(job.Title + " " + job.Company + " " + job.Description)
			if !strings.Contains(haystack, needle) {
				continue
			}
		}
		if filter.Location != "" && !strings.Contains(Normalize(job.Location), Normalize(filter.Location)) {
			continue
		}
		jobs = append(jobs, job)
	}
	if filter.Limit > 0 && int64(len(jobs)) > filter.Limit {
		jobs = jobs[:filter.Limit]
	}
	return jobs, nil
}

func (r *InMemoryJobRepository) Jobs() []models.Job {
	r.mu.Lock()
	defer r.mu.Unlock()

	jobs := make([]models.Job, len(r.jobs))
	copy(jobs, r.jobs)
	return jobs
}

func (r *InMemoryJobRepository) findDuplicate(job models.Job) (*models.Job, string) {
	for i := range r.jobs {
		if r.jobs[i].ContentHash == job.ContentHash {
			return &r.jobs[i], DedupTierContentHash
		}
	}
	if strings.TrimSpace(job.ExternalID) != "" {
		for i := range r.jobs {
			if r.jobs[i].Source == job.Source && r.jobs[i].ExternalID == job.ExternalID {
				return &r.jobs[i], DedupTierSourceRef
			}
		}
	}
	for i := range r.jobs {
		if r.jobs[i].NormalizedKey == job.NormalizedKey {
			return &r.jobs[i], DedupTierNormalized
		}
	}
	return nil, ""
}

func BuildJob(scrapedJob scraper.ScrapedJob, now time.Time, ttl time.Duration) models.Job {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return models.Job{
		Title:         strings.TrimSpace(scrapedJob.Title),
		Company:       strings.TrimSpace(scrapedJob.Company),
		Location:      strings.TrimSpace(scrapedJob.Location),
		Description:   strings.TrimSpace(scrapedJob.Description),
		ApplyURL:      strings.TrimSpace(scrapedJob.ApplyURL),
		SourceURL:     strings.TrimSpace(scrapedJob.SourceURL),
		Source:        strings.TrimSpace(scrapedJob.Source),
		ExternalID:    strings.TrimSpace(scrapedJob.ExternalID),
		ContentHash:   ContentHash(scrapedJob),
		NormalizedKey: NormalizedJobKey(scrapedJob.Title, scrapedJob.Company, scrapedJob.Location),
		PostedAt:      scrapedJob.PostedAt,
		FirstSeenAt:   now,
		LastSeenAt:    now,
		ExpiresAt:     now.Add(ttl),
		Raw:           scrapedJob.Raw,
	}
}

func ContentHash(job scraper.ScrapedJob) string {
	payload := strings.Join([]string{
		Normalize(job.Title),
		Normalize(job.Company),
		Normalize(job.Location),
		Normalize(job.Description),
		Normalize(job.ApplyURL),
		Normalize(job.SourceURL),
		Normalize(job.Source),
	}, "\x00")

	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func NormalizedJobKey(title, company, location string) string {
	return fmt.Sprintf("%s|%s|%s", Normalize(title), Normalize(company), Normalize(location))
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func Normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonWord.ReplaceAllString(value, " ")
	return strings.Join(strings.Fields(value), " ")
}
