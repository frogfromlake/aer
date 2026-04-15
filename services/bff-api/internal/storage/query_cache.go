package storage

import (
	"fmt"
	"sync"
	"time"
)

// singleSlot is a typed one-entry TTL cache for hot-path query results.
// Unlike a multi-entry LRU, each query family owns its own slot — a second
// request with different parameters replaces the cached entry in place.
// Dashboard refreshes (identical params, tight loop) are absorbed; arbitrary
// drill-down queries fall through and repopulate the slot for the next caller.
type singleSlot[T any] struct {
	mu     sync.RWMutex
	key    string
	value  T
	stored time.Time
	filled bool
}

func (s *singleSlot[T]) get(key string, ttl time.Duration) (T, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var zero T
	if !s.filled || s.key != key {
		return zero, false
	}
	if time.Since(s.stored) >= ttl {
		return zero, false
	}
	return s.value, true
}

func (s *singleSlot[T]) put(key string, v T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	s.value = v
	s.stored = time.Now()
	s.filled = true
}

// normalizedMetricsCacheEntry bundles the two return values of
// GetNormalizedMetrics so both can be served from a single cache slot.
type normalizedMetricsCacheEntry struct {
	rows     []MetricRow
	excluded int64
}

// hotQueryKey renders a deterministic cache key for a hot-path query.
// The key embeds every parameter that influences the query result so a
// different filter combination forces a cache miss.
func hotQueryKey(endpoint string, parts ...any) string {
	return fmt.Sprintf("%s|%v", endpoint, parts)
}

// derefString renders an optional string pointer for hotQueryKey without
// leaking the pointer's address into the key.
func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return "*" + *p
}
