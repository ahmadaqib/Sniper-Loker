package scraper

import (
	"errors"
	"sync"
	"time"

	"Goravel-learn/app/models"
)

var ErrCircuitOpen = errors.New("source circuit breaker is open")

type CircuitSnapshot struct {
	State       models.CircuitState
	ErrorCount  int
	LastError   string
	OpenedUntil *time.Time
}

type CircuitBreaker struct {
	mu        sync.Mutex
	threshold int
	cooldown  time.Duration
	sources   map[string]*CircuitSnapshot
	now       func() time.Time
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = 3
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Minute
	}

	return &CircuitBreaker{
		threshold: threshold,
		cooldown:  cooldown,
		sources:   make(map[string]*CircuitSnapshot),
		now:       time.Now,
	}
}

func (b *CircuitBreaker) BeforeRequest(source string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := b.snapshot(source)
	if s.State != models.CircuitOpen {
		return nil
	}

	if s.OpenedUntil != nil && b.now().After(*s.OpenedUntil) {
		s.State = models.CircuitHalfOpen
		return nil
	}

	return ErrCircuitOpen
}

func (b *CircuitBreaker) RecordSuccess(source string) CircuitSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := b.snapshot(source)
	s.State = models.CircuitClosed
	s.ErrorCount = 0
	s.LastError = ""
	s.OpenedUntil = nil
	return *s
}

func (b *CircuitBreaker) RecordFailure(source string, err error) CircuitSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	s := b.snapshot(source)
	s.ErrorCount++
	if err != nil {
		s.LastError = err.Error()
	}
	if s.ErrorCount >= b.threshold {
		until := b.now().Add(b.cooldown)
		s.State = models.CircuitOpen
		s.OpenedUntil = &until
	}
	return *s
}

func (b *CircuitBreaker) Snapshot(source string) CircuitSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	return *b.snapshot(source)
}

func (b *CircuitBreaker) snapshot(source string) *CircuitSnapshot {
	if s, ok := b.sources[source]; ok {
		return s
	}

	s := &CircuitSnapshot{State: models.CircuitClosed}
	b.sources[source] = s
	return s
}
