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

// AvailableMetricRow represents a metric name with its validation status.
type AvailableMetricRow struct {
	MetricName       string
	ValidationStatus string // "unvalidated", "validated", or "expired"
}

// GetAvailableMetrics returns the distinct metric names that have data in the given
// time range, along with their validation status from the metric_validity table.
// Results are served from an in-process TTL cache (default 60 s) keyed on (start, end)
// to avoid a full table scan on every call. A request with a different date range
// bypasses and replaces the cached entry.
func (s *ClickHouseStorage) GetAvailableMetrics(ctx context.Context, start, end time.Time) ([]AvailableMetricRow, error) {
	s.metricsCache.mu.RLock()
	if s.metricsCache.entries != nil &&
		time.Since(s.metricsCache.cachedAt) < s.metricsCacheTTL &&
		s.metricsCache.cachedStart.Equal(start) &&
		s.metricsCache.cachedEnd.Equal(end) {
		cached := s.metricsCache.entries
		s.metricsCache.mu.RUnlock()
		return cached, nil
	}
	s.metricsCache.mu.RUnlock()

	// Cache miss, expired, or different date range — fetch from ClickHouse.
	// Step 1: Get distinct metric names.
	var metricResults []struct {
		MetricName string
	}
	err := s.conn.Select(ctx, &metricResults, `
		SELECT DISTINCT metric_name as MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY MetricName
	`, start, end)
	if err != nil {
		slog.Error("Failed to query available metrics from ClickHouse", "error", err)
		return nil, err
	}

	// Step 2: Get validation status from metric_validity.
	// Deduplicates by (metric_name, context_key) keeping the latest validation_date,
	// compatible with both ReplacingMergeTree (production) and Memory (tests).
	var validityResults []struct {
		MetricName       string
		ValidationStatus string
	}
	err = s.conn.Select(ctx, &validityResults, `
		SELECT
			metric_name AS MetricName,
			if(max(valid_until) > now(), 'validated', 'expired') AS ValidationStatus
		FROM aer_gold.metric_validity
		GROUP BY metric_name
	`)
	if err != nil {
		slog.Error("Failed to query metric validity from ClickHouse", "error", err)
		return nil, err
	}

	// Build lookup map from validity results.
	validityMap := make(map[string]string, len(validityResults))
	for _, v := range validityResults {
		validityMap[v.MetricName] = v.ValidationStatus
	}

	// Step 3: Combine — metrics without validity entries are "unvalidated".
	entries := make([]AvailableMetricRow, len(metricResults))
	for i, r := range metricResults {
		status := "unvalidated"
		if s, ok := validityMap[r.MetricName]; ok {
			status = s
		}
		entries[i] = AvailableMetricRow{
			MetricName:       r.MetricName,
			ValidationStatus: status,
		}
	}

	s.metricsCache.mu.Lock()
	s.metricsCache.entries = entries
	s.metricsCache.cachedAt = time.Now()
	s.metricsCache.cachedStart = start
	s.metricsCache.cachedEnd = end
	s.metricsCache.mu.Unlock()

	return entries, nil
}
