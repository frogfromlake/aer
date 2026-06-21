// Package storage is the BFF's data-access layer: read-only ClickHouse queries
// against aer_gold.* (the bulk — roughly one file per query family) plus the
// PostgreSQL-backed auth/analyses/webauthn stores that run under the bff_auth
// write role. ClickHouseStorage carries the hot-path caches and is the single
// ClickHouse entry point; the *Store types wrap *sql.DB. No business logic
// lives here — handlers compose these queries.
package storage

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/cenkalti/backoff/v5"
)

type metricsCache struct {
	mu          sync.RWMutex
	entries     []AvailableMetricRow
	cachedAt    time.Time
	cachedStart time.Time
	cachedEnd   time.Time
}

// ClickHouseStorage is the read-only query client against aer_gold.*, with
// short-TTL hot-path caches that absorb dashboard refresh bursts on identical
// parameters. It is the BFF's single ClickHouse entry point; every query family
// hangs off it.
type ClickHouseStorage struct {
	conn            clickhouse.Conn
	rowLimit        int
	metricsCacheTTL time.Duration
	metricsCache    metricsCache
	// Phase 85 hot-path single-slot caches. Each slot absorbs dashboard
	// refresh bursts on identical parameters; differing parameters replace
	// the slot in place. TTL reuses metricsCacheTTL so operators have a
	// single knob (BFF_METRICS_CACHE_TTL_SECONDS) for all data endpoints.
	normalizedMetricsCache singleSlot[normalizedMetricsCacheEntry]
	entitiesCache          singleSlot[[]EntityRow]
	languagesCache         singleSlot[[]LanguageDetectionRow]
}

// NewClickHouseStorage opens the ClickHouse connection with bounded retry so a
// cold database doesn't fail BFF startup. rowLimit caps result-set size;
// metricsCacheTTL is the single operator knob (BFF_METRICS_CACHE_TTL_SECONDS)
// shared by every hot-path cache.
func NewClickHouseStorage(ctx context.Context, addr, user, password, db string, rowLimit int, metricsCacheTTL time.Duration) (*ClickHouseStorage, error) {
	operation := func() (clickhouse.Conn, error) {
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{addr},
			Auth: clickhouse.Auth{
				Database: db,
				Username: user,
				Password: password,
			},
			DialTimeout: 5 * time.Second,
			// SEC-089 — explicit pool bounds + a server-side query ceiling, for
			// parity with the PG pools and the worker. max_execution_time is
			// defense-in-depth behind the 30s request-context timeout (ctx-cancel
			// already tears the query down); it caps a query that somehow outlives
			// its context. Pool bounds keep a burst of dashboard refreshes from
			// opening unbounded ClickHouse connections.
			Settings: clickhouse.Settings{
				"max_execution_time": 30,
			},
			MaxOpenConns:    10,
			MaxIdleConns:    5,
			ConnMaxLifetime: time.Hour,
		})
		if err != nil {
			return nil, err
		}

		if err := conn.Ping(ctx); err != nil {
			return nil, err
		}
		return conn, nil
	}

	notify := func(err error, d time.Duration) {
		slog.Warn("ClickHouse not ready, retrying...", "error", err, "backoff", d)
	}

	conn, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(60*time.Second),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clickhouse after retries: %w", err)
	}

	if rowLimit <= 0 {
		rowLimit = 10000
	}
	if metricsCacheTTL <= 0 {
		metricsCacheTTL = 60 * time.Second
	}
	return &ClickHouseStorage{conn: conn, rowLimit: rowLimit, metricsCacheTTL: metricsCacheTTL}, nil
}

// Ping checks if the ClickHouse connection is alive.
func (s *ClickHouseStorage) Ping(ctx context.Context) error {
	return s.conn.Ping(ctx)
}

// Close releases the underlying ClickHouse connection pool. Safe to call on a
// nil receiver so graceful-shutdown paths can call it unconditionally.
func (s *ClickHouseStorage) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// Conn returns the underlying ClickHouse connection. Exposed so cross-
// store callers (e.g. DossierStore for ADR-022 article resolution) can
// share the pool rather than opening a second one.
func (s *ClickHouseStorage) Conn() clickhouse.Conn {
	return s.conn
}
