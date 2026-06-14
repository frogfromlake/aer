package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// MetricRow represents a single aggregated metric data point from ClickHouse.
type MetricRow struct {
	TS         time.Time
	Value      float64
	Source     string
	MetricName string
	// Count is the number of gold-layer rows contributing to Value in this
	// bucket. Required for any caller that needs a document rate (e.g.
	// the Atmosphere's per-probe pulse): Value is an average of the
	// per-document metric, not a document tally. UInt64 in ClickHouse.
	Count uint64
	// Stddev is the Bessel-corrected sample standard deviation of the
	// per-document metric values in the bucket. Populated only by
	// GetMetricsWithSpread (Phase 131) — the default GetMetrics path and
	// the normalized/percentile paths leave it zero. Powers the
	// time-series ±1σ uncertainty band. Zero for buckets with n<2.
	Stddev float64
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
//
// Used by callers that aggregate against `aer_gold.metrics` directly
// (normalized + percentile metrics) — those queries cannot route to the
// pre-aggregated MVs because they JOIN per-row against
// language_detections / metric_baselines, which the MVs do not carry.
// Plain GetMetrics uses [Resolution.queryShape] instead, which selects
// the correct physical table per resolution (Phase 122c).
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

// metricsQueryShape captures the physical-table choice and the
// SQL-fragment shape for a given Resolution against the metrics layer.
// Phase 122c activated three pre-aggregated materialized views
// (metrics_hourly / metrics_daily / metrics_monthly) backed by
// AggregatingMergeTree state columns; queries against them must use
// avgMerge / countMerge to combine the partial states. The raw
// aer_gold.metrics table continues to back 5-minute resolution where
// per-document precision is required.
type metricsQueryShape struct {
	// Table is the FROM target — `aer_gold.metrics` for raw or one of
	// the pre-aggregated MVs.
	Table string
	// TimestampColumn is the column the WHERE clause filters on.
	// `timestamp` for raw; `bucket` for the MVs (which pre-bucket at
	// write time).
	TimestampColumn string
	// BucketExpr is the SELECT-side expression that yields the time
	// bucket. For raw 5-minute it is `toStartOfFiveMinute(timestamp)`;
	// for the MVs it is just `bucket` (passthrough — the MV already
	// bucketed at write time). The weekly resolution rebuckets the
	// daily MV via `toStartOfWeek(bucket)`, avoiding a fourth MV.
	BucketExpr string
	// ValueExpr aggregates `value` (raw) or merges the AggregatingMergeTree
	// `avgState` partial state (MV).
	ValueExpr string
	// CountExpr counts rows (raw) or merges the AggregatingMergeTree
	// `countState` partial state (MV).
	CountExpr string
}

// queryShape returns the physical-table routing for this resolution
// against the metrics layer (Phase 122c). The shape contract is
// byte-equivalence: a query built from the returned shape produces the
// same `[]MetricRow` as the pre-activation handler at any single
// resolution, regardless of which physical table backed the query.
func (r Resolution) queryShape() metricsQueryShape {
	switch r {
	case ResolutionHourly:
		return metricsQueryShape{
			Table:           "aer_gold.metrics_hourly",
			TimestampColumn: "bucket",
			BucketExpr:      "bucket",
			ValueExpr:       "avgMerge(value_avg_state)",
			CountExpr:       "countMerge(sample_count_state)",
		}
	case ResolutionDaily:
		return metricsQueryShape{
			Table:           "aer_gold.metrics_daily",
			TimestampColumn: "bucket",
			BucketExpr:      "bucket",
			ValueExpr:       "avgMerge(value_avg_state)",
			CountExpr:       "countMerge(sample_count_state)",
		}
	case ResolutionWeekly:
		// Weekly bins the daily MV via toStartOfWeek at query time —
		// no fourth MV. The daily MV's 1825-day TTL still bounds the
		// queryable window for weekly.
		return metricsQueryShape{
			Table:           "aer_gold.metrics_daily",
			TimestampColumn: "bucket",
			BucketExpr:      "toStartOfWeek(bucket)",
			ValueExpr:       "avgMerge(value_avg_state)",
			CountExpr:       "countMerge(sample_count_state)",
		}
	case ResolutionMonthly:
		return metricsQueryShape{
			Table:           "aer_gold.metrics_monthly",
			TimestampColumn: "bucket",
			BucketExpr:      "bucket",
			ValueExpr:       "avgMerge(value_avg_state)",
			CountExpr:       "countMerge(sample_count_state)",
		}
	default: // ResolutionFiveMinute — raw, full-precision.
		return metricsQueryShape{
			Table:           "aer_gold.metrics",
			TimestampColumn: "timestamp",
			BucketExpr:      "toStartOfFiveMinute(timestamp)",
			ValueExpr:       "avg(value)",
			CountExpr:       "count()",
		}
	}
}

// GetMetrics retrieves aggregated time-series data from the gold layer.
// Phase 122c routes the query to the physical table backing the
// requested resolution (raw `aer_gold.metrics` for 5-minute; the
// pre-aggregated `metrics_hourly` / `metrics_daily` / `metrics_monthly`
// MVs otherwise — see [Resolution.queryShape]). A hard LIMIT scaled by
// the resolution keeps memory bounded. Optional source and metricName
// filters narrow results to specific dimensions.
func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) ([]MetricRow, error) {
	var results []MetricRow

	shape := resolution.queryShape()
	query := fmt.Sprintf(`
		SELECT
			%s as TS,
			%s as Value,
			source as Source,
			metric_name as MetricName,
			%s as Count
		FROM %s
		WHERE %s >= $1 AND %s <= $2
	`, shape.BucketExpr, shape.ValueExpr, shape.CountExpr,
		shape.Table, shape.TimestampColumn, shape.TimestampColumn)
	args := []any{start, end}
	argIdx := 3

	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			argIdx++
			args = append(args, src)
		}
		query += fmt.Sprintf(" AND source IN (%s)", strings.Join(placeholders, ", "))
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

// GetMetricsWithSpread retrieves aggregated time-series data together with the
// per-bucket sample standard deviation (Phase 131). Unlike GetMetrics it always
// reads the raw `aer_gold.metrics` table — the pre-aggregated resolution MVs
// carry an avg/count state but no variance state, so the spread can only be
// reconstructed from the raw rows. Bucketing still honours the requested
// resolution via bucketExpr, so the ±1σ band is available at any resolution at
// the cost of a raw scan (bounded by the same resolution-scaled row cap the
// normalized path uses).
//
// stddevSamp is undefined for single-row buckets (n-1 == 0 → NaN, which would
// break JSON encoding); the `if(count() > 1, …, 0)` guard collapses those to 0.
func (s *ClickHouseStorage) GetMetricsWithSpread(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) ([]MetricRow, error) {
	var results []MetricRow

	bucket := resolution.bucketExpr("timestamp")
	query := fmt.Sprintf(`
		SELECT
			%s as TS,
			avg(value) as Value,
			source as Source,
			metric_name as MetricName,
			count() as Count,
			if(count() > 1, stddevSamp(value), 0) as Stddev
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`, bucket)
	args := []any{start, end}
	argIdx := 3

	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			argIdx++
			args = append(args, src)
		}
		query += fmt.Sprintf(" AND source IN (%s)", strings.Join(placeholders, ", "))
	}
	if metricName != nil {
		query += fmt.Sprintf(" AND metric_name = $%d", argIdx)
		args = append(args, *metricName)
	}

	query += fmt.Sprintf(`
		GROUP BY TS, Source, MetricName
		ORDER BY TS ASC
		LIMIT %d
	`, s.rowLimit*resolution.rowLimitMultiplier())

	if err := s.conn.Select(ctx, &results, query, args...); err != nil {
		slog.Error("Failed to query metrics with spread from ClickHouse", "error", err)
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
	// Phase 115: structured equivalence status. Populated alongside the
	// deprecated EquivalenceLevel field so the deprecation alias remains
	// honest. Nil when no equivalence entry exists.
	EquivalenceStatus *EquivalenceStatusRow
}

// EquivalenceStatusRow mirrors the structured equivalenceStatus object
// returned on /metrics/available (Phase 115). Carries the new `notes`
// column added by ClickHouse migration 000014.
type EquivalenceStatusRow struct {
	Level          *string
	ValidatedBy    *string
	ValidationDate *time.Time
	Notes          string
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

	// Step 3: Get equivalence metadata from metric_equivalence including
	// the Phase-115 notes column. Picks the highest-ranked equivalence
	// row per metric so the structured status carries the strongest
	// guarantee on record.
	equivalenceMap, err := s.fetchEquivalenceByMetric(ctx)
	if err != nil {
		return nil, err
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
			ec := eq.EticConstruct
			lvl := eq.Level
			row.EticConstruct = &ec
			row.EquivalenceLevel = &lvl
			levelCopy := lvl
			row.EquivalenceStatus = &EquivalenceStatusRow{
				Level: &levelCopy,
				Notes: eq.Notes,
			}
			if eq.ValidatedBy != "" {
				vb := eq.ValidatedBy
				row.EquivalenceStatus.ValidatedBy = &vb
			}
			if !eq.ValidationDate.IsZero() {
				vd := eq.ValidationDate
				row.EquivalenceStatus.ValidationDate = &vd
			}
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

// GetMetricValidationStatus returns the validation status for a single metric
// name: "validated" when a current entry exists in metric_validity whose
// valid_until is in the future, "expired" when the most recent entry has
// already expired, and "unvalidated" when no entry exists at all.
func (s *ClickHouseStorage) GetMetricValidationStatus(ctx context.Context, metricName string) (string, error) {
	var result []struct {
		Status string
	}
	// ClickHouse aggregations always return one row even when the source is
	// empty (max() yields NULL, which `if(NULL > now(), ...)` resolves to the
	// `'expired'` branch). Guard with `count() > 0` so an absent metric reads
	// back as the empty string and the caller can map it to "unvalidated".
	err := s.conn.Select(ctx, &result, `
		SELECT
			if(count() > 0,
			   if(max(valid_until) > now(), 'validated', 'expired'),
			   '') AS Status
		FROM aer_gold.metric_validity
		WHERE metric_name = $1
	`, metricName)
	if err != nil {
		slog.Error("Failed to query metric validity", "error", err, "metric", metricName)
		return "", err
	}
	if len(result) == 0 || result[0].Status == "" {
		return "unvalidated", nil
	}
	return result[0].Status, nil
}

// GetMetricCulturalContextNotes returns a human-readable summary of any
// equivalence entries registered for the metric, or empty string when none
// exists. The summary lists the highest-ranked equivalence level together
// with the etic construct it maps to.
func (s *ClickHouseStorage) GetMetricCulturalContextNotes(ctx context.Context, metricName string) (string, error) {
	var result []struct {
		EticConstruct    string
		EquivalenceLevel string
	}
	err := s.conn.Select(ctx, &result, `
		SELECT
			etic_construct AS EticConstruct,
			equivalence_level AS EquivalenceLevel
		FROM aer_gold.metric_equivalence
		WHERE metric_name = $1
		GROUP BY etic_construct, equivalence_level
	`, metricName)
	if err != nil {
		slog.Error("Failed to query metric equivalence", "error", err, "metric", metricName)
		return "", err
	}
	if len(result) == 0 {
		return "", nil
	}
	// Pick the strongest equivalence level on record.
	rank := map[string]int{"temporal": 1, "deviation": 2, "absolute": 3}
	best := result[0]
	for _, r := range result[1:] {
		if rank[r.EquivalenceLevel] > rank[best.EquivalenceLevel] {
			best = r
		}
	}
	return fmt.Sprintf(
		"Cross-cultural equivalence established at %q level for etic construct %q.",
		best.EquivalenceLevel, best.EticConstruct,
	), nil
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
