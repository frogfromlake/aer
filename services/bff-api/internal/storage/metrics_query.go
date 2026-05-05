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

// equivalenceEntry is the storage-layer view of one
// `aer_gold.metric_equivalence` row, joined with the Phase-115 `notes`
// column added by ClickHouse migration 000014.
type equivalenceEntry struct {
	MetricName     string
	EticConstruct  string
	Level          string
	ValidatedBy    string
	ValidationDate time.Time
	Notes          string
	Languages      []string
}

// equivalenceLevelRank maps the three Phase-65 equivalence levels onto
// an ordered rank so the strongest grant on record can be picked.
var equivalenceLevelRank = map[string]int{
	"temporal":  1,
	"deviation": 2,
	"absolute":  3,
}

// fetchEquivalenceByMetric returns one entry per metric, holding the
// strongest equivalence level on record together with its provenance
// metadata and the set of languages the equivalence has been validated
// across. Used by /metrics/available (Phase-115 structured status) and by
// the cross-frame gate (multi-language coverage check).
func (s *ClickHouseStorage) fetchEquivalenceByMetric(ctx context.Context) (map[string]equivalenceEntry, error) {
	var rows []struct {
		MetricName       string    `ch:"MetricName"`
		EticConstruct    string    `ch:"EticConstruct"`
		EquivalenceLevel string    `ch:"EquivalenceLevel"`
		Language         string    `ch:"Language"`
		ValidatedBy      string    `ch:"ValidatedBy"`
		ValidationDate   time.Time `ch:"ValidationDate"`
		Notes            string    `ch:"Notes"`
	}
	err := s.conn.Select(ctx, &rows, `
		SELECT
			metric_name       AS MetricName,
			etic_construct    AS EticConstruct,
			equivalence_level AS EquivalenceLevel,
			language          AS Language,
			validated_by      AS ValidatedBy,
			validation_date   AS ValidationDate,
			notes             AS Notes
		FROM aer_gold.metric_equivalence FINAL
	`)
	if err != nil {
		slog.Error("Failed to query metric equivalence from ClickHouse", "error", err)
		return nil, err
	}

	out := make(map[string]equivalenceEntry, len(rows))
	for _, r := range rows {
		existing, ok := out[r.MetricName]
		if !ok {
			out[r.MetricName] = equivalenceEntry{
				MetricName:     r.MetricName,
				EticConstruct:  r.EticConstruct,
				Level:          r.EquivalenceLevel,
				ValidatedBy:    r.ValidatedBy,
				ValidationDate: r.ValidationDate,
				Notes:          r.Notes,
				Languages:      []string{r.Language},
			}
			continue
		}
		existing.Languages = append(existing.Languages, r.Language)
		if equivalenceLevelRank[r.EquivalenceLevel] > equivalenceLevelRank[existing.Level] {
			existing.Level = r.EquivalenceLevel
			existing.EticConstruct = r.EticConstruct
			existing.ValidatedBy = r.ValidatedBy
			existing.ValidationDate = r.ValidationDate
			existing.Notes = r.Notes
		}
		out[r.MetricName] = existing
	}
	return out, nil
}

// CountLanguagesForSources returns the number of distinct detected
// languages observed in `aer_gold.language_detections` for the given
// source set within the requested window. Used by the Phase-115
// cross-frame equivalence gate to decide whether a normalization request
// must additionally clear the metric_equivalence check.
//
// An empty `sources` slice means "all sources"; the caller is responsible
// for passing the resolved scope.
func (s *ClickHouseStorage) CountLanguagesForSources(ctx context.Context, start, end time.Time, sources []string) (int, error) {
	query := `
		SELECT countDistinct(detected_language) AS N
		FROM aer_gold.language_detections
		WHERE rank = 1
		  AND timestamp >= $1
		  AND timestamp <= $2
	`
	args := []any{start, end}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, src)
		}
		query += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	}
	var result []struct{ N uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		slog.Error("Failed to count languages for sources", "error", err)
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return int(result[0].N), nil //nolint:gosec // bounded; distinct languages
}

// LanguagesForScope returns the distinct detected_language values
// observed in `aer_gold.language_detections` for the source set across
// the given window. Mirrors the count-only helper above; the handler
// calls this when the count says the request is cross-frame so the
// equivalence gate has the actual language list to validate against.
func (s *ClickHouseStorage) LanguagesForScope(ctx context.Context, start, end time.Time, sources []string) ([]string, error) {
	query := `
		SELECT DISTINCT detected_language AS Lang
		FROM aer_gold.language_detections
		WHERE rank = 1
		  AND timestamp >= $1
		  AND timestamp <= $2
	`
	args := []any{start, end}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, src)
		}
		query += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	}
	query += " ORDER BY Lang"
	var rows []struct{ Lang string }
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to list languages for scope", "error", err)
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Lang != "" {
			out = append(out, r.Lang)
		}
	}
	return out, nil
}

// CheckEquivalenceForLanguages returns true if the metric has at least
// one `aer_gold.metric_equivalence` row at deviation-or-absolute level
// for every language in `languages`. Phase 115: the cross-frame gate
// requires equivalence to be validated across both languages, not just
// any single language.
func (s *ClickHouseStorage) CheckEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error) {
	if len(languages) == 0 {
		return true, nil
	}
	placeholders := make([]string, len(languages))
	args := []any{metricName}
	for i, lang := range languages {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, lang)
	}
	query := fmt.Sprintf(`
		SELECT countDistinct(language) AS N
		FROM aer_gold.metric_equivalence FINAL
		WHERE metric_name = $1
		  AND equivalence_level IN ('deviation', 'absolute')
		  AND language IN (%s)
	`, strings.Join(placeholders, ", "))
	var result []struct{ N uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		slog.Error("Failed to check cross-frame equivalence", "error", err)
		return false, err
	}
	if len(result) == 0 {
		return false, nil
	}
	return int(result[0].N) >= len(languages), nil //nolint:gosec // bounded; distinct languages
}

// GetPercentileNormalizedMetrics retrieves percentile-normalized
// time-series data (Phase 115). For each (metric_name, source, language)
// group, every observation is replaced by its percentile rank within the
// group over the active query window — computed via ClickHouse window
// functions. The shape mirrors GetNormalizedMetrics so the handler can
// dispatch on the normalization mode without restructuring the response.
//
// Sharing the language-detection LEFT JOIN preserves the same
// excludedCount semantics: rows whose article has no detected language
// are dropped from the percentile computation and surfaced to the client.
func (s *ClickHouseStorage) GetPercentileNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) ([]MetricRow, int64, error) {
	cacheKey := hotQueryKey("percentile_metrics",
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
		slog.Error("Failed to count excluded percentile metrics", "error", err)
		return nil, 0, err
	}
	var excluded int64
	if len(excludedResult) > 0 {
		excluded = int64(excludedResult[0].Cnt)
	}

	// Window function ranks each value within (metric_name, source,
	// language) over the active query window. (rank-1)/(N-1) maps to
	// [0, 1]; single-row groups collapse to 0 (denominator guard).
	query := fmt.Sprintf(`
		WITH ranked AS (
			SELECT
				m.timestamp        AS ts,
				m.source           AS source,
				m.metric_name      AS metric_name,
				ld.detected_language AS language,
				rowNumberInAllBlocks() AS _row,
				row_number() OVER (
					PARTITION BY m.metric_name, m.source, ld.detected_language
					ORDER BY m.value
				) AS rnk,
				count() OVER (
					PARTITION BY m.metric_name, m.source, ld.detected_language
				) AS n
			FROM aer_gold.metrics AS m
			LEFT JOIN aer_gold.language_detections AS ld
				ON m.article_id = ld.article_id AND ld.rank = 1
			WHERE %s AND ld.detected_language IS NOT NULL
			SETTINGS join_use_nulls = 1
		)
		SELECT
			%s AS TS,
			avg(if(n > 1, (rnk - 1) / (n - 1), 0)) AS Value,
			source AS Source,
			metric_name AS MetricName,
			count() AS Count
		FROM ranked
		GROUP BY TS, Source, MetricName
		ORDER BY TS ASC
		LIMIT %d
	`, baseWhere, resolution.bucketExpr("ts"), s.rowLimit*resolution.rowLimitMultiplier())

	var results []MetricRow
	if err := s.conn.Select(ctx, &results, query, args...); err != nil {
		slog.Error("Failed to query percentile metrics from ClickHouse", "error", err)
		return nil, 0, err
	}

	s.normalizedMetricsCache.put(cacheKey, normalizedMetricsCacheEntry{rows: results, excluded: excluded})
	return results, excluded, nil
}

// GetProbeEquivalence returns per-metric Level-1 / Level-2 / Level-3
// availability for the resolved source set of one probe (Phase 115).
//
// Level-1 (temporal) is true whenever the metric has any data in the
// scope — temporal patterns are intra-culturally valid by construction.
// Level-2 (deviation) requires a deviation-or-absolute-level
// `metric_equivalence` row covering every language detected in the
// scope. Level-3 (absolute) requires an absolute-level row with the
// same language coverage.
func (s *ClickHouseStorage) GetProbeEquivalence(ctx context.Context, start, end time.Time, sources []string) ([]ProbeEquivalenceMetric, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	// 1. Distinct metric names with data in the probe scope.
	metricsQuery := `
		SELECT DISTINCT metric_name AS MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, src)
		}
		metricsQuery += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	}
	metricsQuery += " ORDER BY MetricName"
	var metricResults []struct{ MetricName string }
	if err := s.conn.Select(ctx, &metricResults, metricsQuery, args...); err != nil {
		slog.Error("Failed to list metrics for probe scope", "error", err)
		return nil, err
	}

	// 2. Languages observed in the probe scope.
	langQuery := `
		SELECT DISTINCT detected_language AS Lang
		FROM aer_gold.language_detections
		WHERE rank = 1
		  AND timestamp >= $1 AND timestamp <= $2
	`
	langArgs := []any{start, end}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			langArgs = append(langArgs, src)
		}
		langQuery += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	}
	var langResults []struct{ Lang string }
	if err := s.conn.Select(ctx, &langResults, langQuery, langArgs...); err != nil {
		slog.Error("Failed to list languages for probe scope", "error", err)
		return nil, err
	}
	languages := make([]string, 0, len(langResults))
	for _, r := range langResults {
		if r.Lang != "" {
			languages = append(languages, r.Lang)
		}
	}

	// 3. Equivalence registry once; rank reduces the per-metric loop.
	equivMap, err := s.fetchEquivalenceByMetric(ctx)
	if err != nil {
		return nil, err
	}

	out := make([]ProbeEquivalenceMetric, 0, len(metricResults))
	for _, m := range metricResults {
		row := ProbeEquivalenceMetric{
			MetricName:      m.MetricName,
			Level1Available: true,
		}
		eq, hasEq := equivMap[m.MetricName]
		if hasEq {
			levelCopy := eq.Level
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

		if hasEq && len(languages) > 0 {
			deviationOK, err := s.CheckEquivalenceForLanguages(ctx, m.MetricName, languages)
			if err != nil {
				return nil, err
			}
			row.Level2Available = deviationOK
			if deviationOK && eq.Level == "absolute" {
				absOK, err := s.checkAbsoluteEquivalenceForLanguages(ctx, m.MetricName, languages)
				if err != nil {
					return nil, err
				}
				row.Level3Available = absOK
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// ProbeEquivalenceMetric is the storage-layer view of one row in the
// /probes/{probeId}/equivalence response (Phase 115).
type ProbeEquivalenceMetric struct {
	MetricName        string
	Level1Available   bool
	Level2Available   bool
	Level3Available   bool
	EquivalenceStatus *EquivalenceStatusRow
}

// checkAbsoluteEquivalenceForLanguages mirrors CheckEquivalenceForLanguages
// but limits the gate to absolute-level rows.
func (s *ClickHouseStorage) checkAbsoluteEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error) {
	if len(languages) == 0 {
		return true, nil
	}
	placeholders := make([]string, len(languages))
	args := []any{metricName}
	for i, lang := range languages {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, lang)
	}
	query := fmt.Sprintf(`
		SELECT countDistinct(language) AS N
		FROM aer_gold.metric_equivalence FINAL
		WHERE metric_name = $1
		  AND equivalence_level = 'absolute'
		  AND language IN (%s)
	`, strings.Join(placeholders, ", "))
	var result []struct{ N uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, nil
	}
	return int(result[0].N) >= len(languages), nil //nolint:gosec
}
