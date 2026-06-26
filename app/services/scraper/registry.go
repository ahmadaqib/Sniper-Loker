package scraper

import (
	"fmt"
	"sort"
	"sync"
)

type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]JobSource
}

func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{sources: make(map[string]JobSource)}
}

func (r *SourceRegistry) Register(source JobSource) error {
	if source == nil {
		return fmt.Errorf("source cannot be nil")
	}
	if source.Name() == "" {
		return fmt.Errorf("source name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[source.Name()] = source
	return nil
}

func (r *SourceRegistry) Get(name string) (JobSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	source, ok := r.sources[name]
	return source, ok
}

func (r *SourceRegistry) Enabled() []JobSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]JobSource, 0, len(r.sources))
	for _, source := range r.sources {
		if source.Config().Enabled {
			sources = append(sources, source)
		}
	}
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Name() < sources[j].Name()
	})
	return sources
}
