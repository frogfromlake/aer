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
func (s *ClickHouseStorage) GetMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) ([]MetricRow, error) {
	var results []MetricRow

	// Bucket on the DB side via resolution.bucketExpr; aggregate with avg().
	// A hard LIMIT — scaled by the resolution — keeps memory bounded.
	query := fmt.Sprintf(`
		SELECT
			%s as TS,
			avg(value) as Value,
			source as Source,
			metric_name as MetricName,
			count() as Count
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`, resolution.bucketExpr("timestamp"))
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

// GetMetricValidationStatus returns the validation status for a single metric
// name: "validated" when a current entry exists in metric_validity whose
// valid_until is in the future, "expired" when the most recent entry has
// already expired, and "unvalidated" when no entry exists at all.
func (s *ClickHouseStorage) GetMetricValidationStatus(ctx context.Context, metricName string) (string, error) {
	var result []struct {
		Status string
	}
	err := s.conn.Select(ctx, &result, `
		SELECT if(max(valid_until) > now(), 'validated', 'expired') AS Status
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

// GetNormalizedMetrics retrieves z-score normalized time-series data and the
// count of source rows dropped because their article lacks a language detection.
//
// Metrics are LEFT JOINed against language_detections (rank=1) so rows with no
// detection can be counted and surfaced to the client as excludedCount instead
// of vanishing silently. The subsequent INNER JOIN against metric_baselines
// still excludes rows whose (source, language) pair has no baseline — those are
// out of scope for excludedCount because the baseline-precondition gate in the
// handler (CheckBaselineExists) ensures the requested (metric, source) has at
// least one baseline before this query runs.
//
// The `SETTINGS join_use_nulls = 1` clause makes LEFT-JOIN misses produce true
// NULLs instead of ClickHouse's default zero-values, so `IS NULL` / `IS NOT NULL`
// discriminate correctly regardless of the detected_language string domain.
func (s *ClickHouseStorage) GetNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) ([]MetricRow, int64, error) {
	cacheKey := hotQueryKey("normalized_metrics",
		start.UnixNano(), end.UnixNano(), strings.Join(sources, ","), derefString(metricName), int(resolution))
	if cached, ok := s.normalizedMetricsCache.get(cacheKey, s.metricsCacheTTL); ok {
		return cached.rows, cached.excluded, nil
	}

	baseWhere := "m.timestamp >= $1 AND m.timestamp <= $2"
	args := []any{start, end}
	argIdx := 3
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			argIdx++
			args = append(args, src)
		}
		baseWhere += fmt.Sprintf(" AND m.source IN (%s)", strings.Join(placeholders, ", "))
	}
	if metricName != nil {
		baseWhere += fmt.Sprintf(" AND m.metric_name = $%d", argIdx)
		args = append(args, *metricName)
	}

	// Excluded-count query: source rows in-window with no matching language detection.
	excludedQuery := `
		SELECT count() AS Cnt
		FROM aer_gold.metrics AS m
		LEFT JOIN aer_gold.language_detections AS ld
			ON m.article_id = ld.article_id AND ld.rank = 1
		WHERE ` + baseWhere + ` AND ld.detected_language IS NULL
		SETTINGS join_use_nulls = 1
	`
	var excludedResult []struct{ Cnt uint64 }
	if err := s.conn.Select(ctx, &excludedResult, excludedQuery, args...); err != nil {
		slog.Error("Failed to count excluded normalized metrics", "error", err)
		return nil, 0, err
	}
	var excluded int64
	if len(excludedResult) > 0 {
		excluded = int64(excludedResult[0].Cnt)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS TS,
			avg((m.value - b.baseline_value) / b.baseline_std) AS Value,
			m.source AS Source,
			m.metric_name AS MetricName,
			count() AS Count
		FROM aer_gold.metrics AS m
		LEFT JOIN aer_gold.language_detections AS ld
			ON m.article_id = ld.article_id AND ld.rank = 1
		INNER JOIN aer_gold.metric_baselines AS b
			ON m.metric_name = b.metric_name
			AND m.source = b.source
			AND ld.detected_language = b.language
		WHERE %s
		  AND ld.detected_language IS NOT NULL
		  AND b.baseline_std > 0
		GROUP BY TS, Source, MetricName
		ORDER BY TS ASC
		LIMIT %d
		SETTINGS join_use_nulls = 1
	`, resolution.bucketExpr("m.timestamp"), baseWhere, s.rowLimit*resolution.rowLimitMultiplier())

	var results []MetricRow
	if err := s.conn.Select(ctx, &results, query, args...); err != nil {
		slog.Error("Failed to query normalized metrics from ClickHouse", "error", err)
		return nil, 0, err
	}

	if excluded > 0 {
		slog.Warn("normalized metrics query excluded rows lacking language detection",
			"excluded", excluded,
			"start", start,
			"end", end,
			"sources", sources,
			"metric", metricName,
		)
	}

	s.normalizedMetricsCache.put(cacheKey, normalizedMetricsCacheEntry{rows: results, excluded: excluded})
	return results, excluded, nil
}
