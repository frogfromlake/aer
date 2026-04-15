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
