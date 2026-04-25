package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

// SourceStore serves the BFF API's /sources endpoint directly from the
// PostgreSQL `sources` table (the single source of truth as of Phase 87).
// Reads go through a short-lived in-memory cache so the hot path does not
// hit Postgres on every request. On a read failure the last successful
// snapshot is returned so a transient Postgres hiccup does not turn into
// a /sources outage — a missing row is never worse than a stale one for
// an SSoT list that mutates at migration cadence.
type SourceStore struct {
	db  *sql.DB
	ttl time.Duration

	mu        sync.RWMutex
	cached    []config.SourceEntry
	cachedAt  time.Time
	populated bool
}

// NewSourceStore constructs a SourceStore over an existing read-only
// PostgreSQL pool. The caller retains ownership of the pool's lifecycle.
func NewSourceStore(db *sql.DB, ttl time.Duration) *SourceStore {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &SourceStore{db: db, ttl: ttl}
}

// List returns the cached source list, refreshing from Postgres if the
// cache is empty or older than the configured TTL. On query failure it
// falls back to the last successful snapshot and logs the error; if no
// snapshot exists yet the error propagates so the handler can emit 500.
func (s *SourceStore) List(ctx context.Context) ([]config.SourceEntry, error) {
	s.mu.RLock()
	if s.populated && time.Since(s.cachedAt) < s.ttl {
		snapshot := s.cached
		s.mu.RUnlock()
		return snapshot, nil
	}
	s.mu.RUnlock()

	rows, err := s.fetch(ctx)
	if err != nil {
		s.mu.RLock()
		defer s.mu.RUnlock()
		if s.populated {
			slog.Warn("SourceStore.List falling back to stale cache", "error", err)
			return s.cached, nil
		}
		return nil, fmt.Errorf("query sources: %w", err)
	}

	s.mu.Lock()
	s.cached = rows
	s.cachedAt = time.Now()
	s.populated = true
	s.mu.Unlock()
	return rows, nil
}

func (s *SourceStore) fetch(ctx context.Context) ([]config.SourceEntry, error) {
	const query = `
		SELECT name, type, url, documentation_url,
		       silver_eligible,
		       silver_review_reviewer, silver_review_date,
		       silver_review_rationale, silver_review_reference
		  FROM sources
		 ORDER BY name
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []config.SourceEntry
	for rows.Next() {
		var (
			name, sourceType string
			url              sql.NullString
			docURL           sql.NullString
			eligible         bool
			reviewer         sql.NullString
			reviewDate       sql.NullTime
			rationale        sql.NullString
			reference        sql.NullString
		)
		if err := rows.Scan(
			&name, &sourceType, &url, &docURL,
			&eligible, &reviewer, &reviewDate, &rationale, &reference,
		); err != nil {
			return nil, fmt.Errorf("scan sources row: %w", err)
		}
		entry := config.SourceEntry{Name: name, Type: sourceType, SilverEligible: eligible}
		if url.Valid {
			v := url.String
			entry.URL = &v
		}
		if docURL.Valid {
			v := docURL.String
			entry.DocumentationURL = &v
		}
		if reviewer.Valid {
			v := reviewer.String
			entry.SilverReviewReviewer = &v
		}
		if reviewDate.Valid {
			t := reviewDate.Time
			entry.SilverReviewDate = &t
		}
		if rationale.Valid {
			v := rationale.String
			entry.SilverReviewRationale = &v
		}
		if reference.Valid {
			v := reference.String
			entry.SilverReviewReference = &v
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources rows: %w", err)
	}
	return out, nil
}
