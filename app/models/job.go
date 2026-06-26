package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Job struct {
	ID             bson.ObjectID  `bson:"_id,omitempty" json:"id"`
	Title          string         `bson:"title" json:"title"`
	Company        string         `bson:"company" json:"company"`
	Location       string         `bson:"location" json:"location"`
	Description    string         `bson:"description,omitempty" json:"description,omitempty"`
	ApplyURL       string         `bson:"apply_url,omitempty" json:"apply_url,omitempty"`
	SourceURL      string         `bson:"source_url,omitempty" json:"source_url,omitempty"`
	Source         string         `bson:"source" json:"source"`
	ExternalID     string         `bson:"external_id,omitempty" json:"external_id,omitempty"`
	ContentHash    string         `bson:"content_hash" json:"content_hash"`
	NormalizedKey  string         `bson:"normalized_key" json:"normalized_key"`
	PostedAt       *time.Time     `bson:"posted_at,omitempty" json:"posted_at,omitempty"`
	FirstSeenAt    time.Time      `bson:"first_seen_at" json:"first_seen_at"`
	LastSeenAt     time.Time      `bson:"last_seen_at" json:"last_seen_at"`
	ExpiresAt      time.Time      `bson:"expires_at" json:"expires_at"`
	Raw            map[string]any `bson:"raw,omitempty" json:"raw,omitempty"`
	DuplicateTier  string         `bson:"duplicate_tier,omitempty" json:"duplicate_tier,omitempty"`
	DuplicateJobID bson.ObjectID  `bson:"duplicate_job_id,omitempty" json:"duplicate_job_id,omitempty"`
}
