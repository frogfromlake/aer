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
	mu        sync.RWMutex
	names     []string
	cachedAt  time.Time
}

type ClickHouseStorage struct {
	conn           clickhouse.Conn
	rowLimit       int
	metricsCacheTTL time.Duration
	metricsCache   metricsCache
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
		backoff.WithMaxElapsedTime(30*time.Second),
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

// GetMetrics retrieves aggregated time-series data from the gold layer.
// It downsamples the data to 5-minute intervals to prevent OOM errors on large time ranges.
// Optional source and metricName filters narrow results to specific dimensions.
// MetricRow represents a single aggregated metric data point from ClickHouse.
type MetricRow struct {
	TS         time.Time
	Value      float64
	Source     string
	MetricName string
}

func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time, source, metricName *string) ([]MetricRow, error) {
	var results []MetricRow

	// Use toStartOfFiveMinute and avg() to aggregate data on the DB level.
	// We also apply a hard limit to guarantee memory safety.
	// Build dynamic WHERE clause based on optional dimension filters.
	query := `
		SELECT
			toStartOfFiveMinute(timestamp) as TS,
			avg(value) as Value,
			source as Source,
			metric_name as MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if metricName != nil {
		query += fmt.Sprintf(" AND metric_name = $%d", argIdx)
		args = append(args, *metricName)
	}

	// The ClickHouse Go driver (clickhouse-go/v2) does not support parameterized
	// LIMIT clauses via the $N positional syntax. rowLimit is validated at
	// initialization (NewClickHouseStorage) and is never negative.
	query += fmt.Sprintf(`
		GROUP BY TS, Source, MetricName
		ORDER BY TS ASC
		LIMIT %d
	`, s.rowLimit)

	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query metrics from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}

// EntityRow represents an aggregated entity result from ClickHouse.
type EntityRow struct {
	EntityText  string
	EntityLabel string
	Count       uint64
	Sources     []string
}

// GetEntities retrieves aggregated named entities from the gold layer.
func (s *ClickHouseStorage) GetEntities(ctx context.Context, start, end time.Time, source, label *string, limit int) ([]EntityRow, error) {
	query := `
		SELECT
			entity_text as EntityText,
			entity_label as EntityLabel,
			count() as Count,
			groupArray(DISTINCT source) as Sources
		FROM aer_gold.entities
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if label != nil {
		query += fmt.Sprintf(" AND entity_label = $%d", argIdx)
		args = append(args, *label)
	}

	// The ClickHouse Go driver (clickhouse-go/v2) does not support parameterized
	// LIMIT clauses via the $N positional syntax. limit is validated in the handler
	// layer (1–1000) before reaching this function.
	query += fmt.Sprintf(`
		GROUP BY EntityText, EntityLabel
		ORDER BY Count DESC
		LIMIT %d
	`, limit)

	var results []EntityRow
	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query entities from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}

// LanguageDetectionRow represents an aggregated language detection result from ClickHouse.
type LanguageDetectionRow struct {
	DetectedLanguage string
	Count            uint64
	AvgConfidence    float64
	Sources          []string
}

// GetLanguageDetections retrieves aggregated language detections from the gold layer.
// Only rank=1 (top candidate per document) detections are included.
func (s *ClickHouseStorage) GetLanguageDetections(ctx context.Context, start, end time.Time, source, language *string, limit int) ([]LanguageDetectionRow, error) {
	query := `
		SELECT
			detected_language as DetectedLanguage,
			count() as Count,
			avg(confidence) as AvgConfidence,
			groupArray(DISTINCT source) as Sources
		FROM aer_gold.language_detections
		WHERE timestamp >= $1 AND timestamp <= $2
		  AND rank = 1
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if language != nil {
		query += fmt.Sprintf(" AND detected_language = $%d", argIdx)
		args = append(args, *language)
	}

	// The ClickHouse Go driver (clickhouse-go/v2) does not support parameterized
	// LIMIT clauses via the $N positional syntax. limit is validated in the handler
	// layer (1–1000) before reaching this function.
	query += fmt.Sprintf(`
		GROUP BY DetectedLanguage
		ORDER BY Count DESC
		LIMIT %d
	`, limit)

	var results []LanguageDetectionRow
	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query language detections from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}

// GetAvailableMetrics returns the distinct metric names present in the gold layer.
// The start/end params are accepted to satisfy the Store interface but are not used
// in the query — metric names change only when new extractors are deployed, making
// time-range scoping unnecessary. Results are served from an in-process TTL cache
// (default 60 s) to avoid a full table scan on every call.
func (s *ClickHouseStorage) GetAvailableMetrics(ctx context.Context, _, _ time.Time) ([]string, error) {
	s.metricsCache.mu.RLock()
	if s.metricsCache.names != nil && time.Since(s.metricsCache.cachedAt) < s.metricsCacheTTL {
		cached := s.metricsCache.names
		s.metricsCache.mu.RUnlock()
		return cached, nil
	}
	s.metricsCache.mu.RUnlock()

	// Cache miss or expired — fetch from ClickHouse.
	var results []struct {
		MetricName string
	}
	err := s.conn.Select(ctx, &results, `
		SELECT DISTINCT metric_name as MetricName
		FROM aer_gold.metrics
		ORDER BY MetricName
	`)
	if err != nil {
		slog.Error("Failed to query available metrics from ClickHouse", "error", err)
		return nil, err
	}

	names := make([]string, len(results))
	for i, r := range results {
		names[i] = r.MetricName
	}

	s.metricsCache.mu.Lock()
	s.metricsCache.names = names
	s.metricsCache.cachedAt = time.Now()
	s.metricsCache.mu.Unlock()

	return names, nil
}
