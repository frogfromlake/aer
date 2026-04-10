package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// MetricRow represents a single aggregated metric data point from ClickHouse.
type MetricRow struct {
	TS         time.Time
	Value      float64
	Source     string
	MetricName string
}

// GetMetrics retrieves aggregated time-series data from the gold layer.
// It downsamples the data to 5-minute intervals to prevent OOM errors on large time ranges.
// Optional source and metricName filters narrow results to specific dimensions.
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

// GetAvailableMetrics returns the distinct metric names that have data in the given
// time range. Results are served from an in-process TTL cache (default 60 s) keyed
// on (start, end) to avoid a full table scan on every call. A request with a
// different date range bypasses and replaces the cached entry.
func (s *ClickHouseStorage) GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]string, error) {
	s.metricsCache.mu.RLock()
	if s.metricsCache.names != nil &&
		time.Since(s.metricsCache.cachedAt) < s.metricsCacheTTL &&
		s.metricsCache.cachedStart.Equal(start) &&
		s.metricsCache.cachedEnd.Equal(end) {
		cached := s.metricsCache.names
		s.metricsCache.mu.RUnlock()
		return cached, nil
	}
	s.metricsCache.mu.RUnlock()

	// Cache miss, expired, or different date range — fetch from ClickHouse.
	var results []struct {
		MetricName string
	}
	err := s.conn.Select(ctx, &results, `
		SELECT DISTINCT metric_name as MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY MetricName
	`, start, end)
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
	s.metricsCache.cachedStart = start
	s.metricsCache.cachedEnd = end
	s.metricsCache.mu.Unlock()

	return names, nil
}
