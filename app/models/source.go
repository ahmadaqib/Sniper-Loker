package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

type Source struct {
	ID                    bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name                  string        `bson:"name" json:"name"`
	DisplayName           string        `bson:"display_name" json:"display_name"`
	Enabled               bool          `bson:"enabled" json:"enabled"`
	MaxPerHour            int           `bson:"max_per_hour" json:"max_per_hour"`
	BaseDelayMillis       int           `bson:"base_delay_millis" json:"base_delay_millis"`
	JitterMillis          int           `bson:"jitter_millis" json:"jitter_millis"`
	RequestTimeoutMillis  int           `bson:"request_timeout_millis" json:"request_timeout_millis"`
	CircuitThreshold      int           `bson:"circuit_threshold" json:"circuit_threshold"`
	CircuitCooldownMillis int           `bson:"circuit_cooldown_millis" json:"circuit_cooldown_millis"`
	UseUTLS               *bool         `bson:"use_utls,omitempty" json:"use_utls,omitempty"`
	CircuitState          CircuitState  `bson:"circuit_state" json:"circuit_state"`
	ErrorCount            int           `bson:"error_count" json:"error_count"`
	LastError             string        `bson:"last_error,omitempty" json:"last_error,omitempty"`
	LastSuccessAt         *time.Time    `bson:"last_success_at,omitempty" json:"last_success_at,omitempty"`
	LastFailureAt         *time.Time    `bson:"last_failure_at,omitempty" json:"last_failure_at,omitempty"`
	OpenedUntil           *time.Time    `bson:"opened_until,omitempty" json:"opened_until,omitempty"`
	CreatedAt             time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt             time.Time     `bson:"updated_at" json:"updated_at"`
}

type SourceStatusUpdate struct {
	State       CircuitState
	ErrorCount  int
	LastError   string
	Success     bool
	OpenedUntil *time.Time
}
