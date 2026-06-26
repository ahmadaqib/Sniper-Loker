package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type SearchQuery struct {
	ID            bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Keyword       string        `bson:"keyword" json:"keyword"`
	Location      string        `bson:"location" json:"location"`
	Enabled       bool          `bson:"enabled" json:"enabled"`
	LastScrapedAt *time.Time    `bson:"last_scraped_at,omitempty" json:"last_scraped_at,omitempty"`
	CreatedAt     time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time     `bson:"updated_at" json:"updated_at"`
	ExpiresAt     time.Time     `bson:"expires_at" json:"expires_at"`
}
