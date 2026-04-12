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

// Resolution selects the temporal bucketing applied at query time.
// The zero value (ResolutionFiveMinute) preserves backward-compatible behavior.
type Resolution int

const (
	ResolutionFiveMinute Resolution = iota
	ResolutionHourly
	ResolutionDaily
	ResolutionWeekly
	ResolutionMonthly
)

// bucketExpr returns the ClickHouse expression that buckets the supplied
// timestamp column for this resolution.
func (r Resolution) bucketExpr(column string) string {
	switch r {
	case ResolutionHourly:
		return "toStartOfHour(" + column + ")"
	case ResolutionDaily:
		return "toStartOfDay(" + column + ")"
	case ResolutionWeekly:
		return "toStartOfWeek(" + column + ")"
	case ResolutionMonthly:
		return "toStartOfMonth(" + column + ")"
	default:
		return "toStartOfFiveMinute(" + column + ")"
	}
}

// rowLimitMultiplier scales the per-request hard cap by the size ratio
// between the requested bucket and the 5-minute baseline. Wider buckets
// produce proportionally fewer rows for the same time range, so we relax
// the OOM guard to keep long ranges queryable.
func (r Resolution) rowLimitMultiplier() int {
	switch r {
	case ResolutionHourly:
		return 12
	case ResolutionDaily:
		return 288
	case ResolutionWeekly:
		return 2016
	case ResolutionMonthly:
		return 8640
	default:
		return 1
	}
}

// GetMetrics retrieves aggregated time-series data from the gold layer.
// It downsamples the data to the requested resolution (default 5-minute)
// to prevent OOM errors on large time ranges. Optional source and metricName
// filters narrow results to specific dimensions.
func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time, source, metricName *string, resolution Resolution) ([]MetricRow, error) {
	var results []MetricRow

	// Bucket on the DB side via resolution.bucketExpr; aggregate with avg().
	// A hard LIMIT — scaled by the resolution — keeps memory bounded.
	query := fmt.Sprintf(`
		SELECT
			%s as TS,
			avg(value) as Value,
			source as Source,
			metric_name as MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`, resolution.bucketExpr("timestamp"))
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
	`, s.rowLimit*resolution.rowLimitMultiplier())

	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query metrics from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}

// AvailableMetricRow represents a metric name with its validation status and optional equivalence metadata.
type AvailableMetricRow struct {
	MetricName              string
	ValidationStatus        string // "unvalidated", "validated", or "expired"
	EticConstruct           *string
	EquivalenceLevel        *string
	MinMeaningfulResolution *string // resolution string ("hourly", "daily", ...) or nil
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

	// Step 3: Get equivalence metadata from metric_equivalence.
	var equivalenceResults []struct {
		MetricName       string
		EticConstruct    string
		EquivalenceLevel string
	}
	err = s.conn.Select(ctx, &equivalenceResults, `
		SELECT
			metric_name AS MetricName,
			etic_construct AS EticConstruct,
			equivalence_level AS EquivalenceLevel
		FROM aer_gold.metric_equivalence
		GROUP BY metric_name, etic_construct, equivalence_level
	`)
	if err != nil {
		slog.Error("Failed to query metric equivalence from ClickHouse", "error", err)
		return nil, err
	}

	type equivInfo struct {
		eticConstruct    string
		equivalenceLevel string
	}
	equivalenceMap := make(map[string]equivInfo, len(equivalenceResults))
	for _, e := range equivalenceResults {
		equivalenceMap[e.MetricName] = equivInfo{
			eticConstruct:    e.EticConstruct,
			equivalenceLevel: e.EquivalenceLevel,
		}
	}

	// Step 4: Combine — metrics without validity entries are "unvalidated".
	entries := make([]AvailableMetricRow, len(metricResults))
	for i, r := range metricResults {
		status := "unvalidated"
		if s, ok := validityMap[r.MetricName]; ok {
			status = s
		}
		row := AvailableMetricRow{
			MetricName:       r.MetricName,
			ValidationStatus: status,
		}
		if eq, ok := equivalenceMap[r.MetricName]; ok {
			row.EticConstruct = &eq.eticConstruct
			row.EquivalenceLevel = &eq.equivalenceLevel
		}
		entries[i] = row
	}

	s.metricsCache.mu.Lock()
	s.metricsCache.entries = entries
	s.metricsCache.cachedAt = time.Now()
	s.metricsCache.cachedStart = start
	s.metricsCache.cachedEnd = end
	s.metricsCache.mu.Unlock()

	return entries, nil
}

// CheckBaselineExists returns true if at least one baseline row exists for the
// given (metricName, source) pair.  When source is nil it checks for any source.
func (s *ClickHouseStorage) CheckBaselineExists(ctx context.Context, metricName string, source *string) (bool, error) {
	query := `SELECT count() AS Cnt FROM aer_gold.metric_baselines WHERE metric_name = $1`
	args := []any{metricName}
	if source != nil {
		query += ` AND source = $2`
		args = append(args, *source)
	}

	var result []struct{ Cnt uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		slog.Error("Failed to check baseline existence", "error", err)
		return false, err
	}
	return len(result) > 0 && result[0].Cnt > 0, nil
}

// CheckEquivalenceExists returns true if at least one equivalence entry with
// at least "deviation"-level equivalence exists for the given metricName.
func (s *ClickHouseStorage) CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error) {
	var result []struct{ Cnt uint64 }
	err := s.conn.Select(ctx, &result, `
		SELECT count() AS Cnt
		FROM aer_gold.metric_equivalence
		WHERE metric_name = $1
		  AND equivalence_level IN ('deviation', 'absolute')
	`, metricName)
	if err != nil {
		slog.Error("Failed to check equivalence existence", "error", err)
		return false, err
	}
	return len(result) > 0 && result[0].Cnt > 0, nil
}

// GetNormalizedMetrics retrieves z-score normalized time-series data.
// It joins metrics with language_detections (rank=1) to resolve language,
// then with metric_baselines to compute (value - baseline_value) / baseline_std.
func (s *ClickHouseStorage) GetNormalizedMetrics(ctx context.Context, start, end time.Time, source, metricName *string, resolution Resolution) ([]MetricRow, error) {
	var results []MetricRow

	query := fmt.Sprintf(`
		SELECT
			%s AS TS,
			avg((m.value - b.baseline_value) / b.baseline_std) AS Value,
			m.source AS Source,
			m.metric_name AS MetricName
		FROM aer_gold.metrics AS m
		INNER JOIN aer_gold.language_detections AS ld
			ON m.article_id = ld.article_id AND ld.rank = 1
		INNER JOIN aer_gold.metric_baselines AS b
			ON m.metric_name = b.metric_name
			AND m.source = b.source
			AND ld.detected_language = b.language
		WHERE m.timestamp >= $1 AND m.timestamp <= $2
		  AND b.baseline_std > 0
	`, resolution.bucketExpr("m.timestamp"))
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND m.source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if metricName != nil {
		query += fmt.Sprintf(" AND m.metric_name = $%d", argIdx)
		args = append(args, *metricName)
	}

	query += fmt.Sprintf(`
		GROUP BY TS, Source, MetricName
		ORDER BY TS ASC
		LIMIT %d
	`, s.rowLimit*resolution.rowLimitMultiplier())

	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query normalized metrics from ClickHouse", "error", err)
		return nil, err
	}

	return results, nil
}
