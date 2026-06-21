package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// temporalNormalizableMetrics are scalar metrics measured on a culture-
// INDEPENDENT axis (clock and calendar time). For these, z-score / percentile
// normalization expresses a temporal-pattern (rhythm) comparison, which a
// temporal Level-1 equivalence grant authorises — normalizing against a
// per-culture mean/std on a culture-independent axis asserts no cross-cultural
// intensity claim. Intensive/scaled metrics (e.g. sentiment) live on a
// culture-laden axis and still require a deviation-level (Level-2) grant.
// WP-004 §6.3 / Appendix B; mirrors the frontend isPureCountMetric split
// (Phase 124).
var temporalNormalizableMetrics = map[string]bool{
	"publication_hour":    true,
	"publication_weekday": true,
}

// normalizationEquivalenceLevels returns the equivalence_level values that
// satisfy the cross-cultural normalization gate for metricName. A temporal-
// axis metric accepts a temporal grant or stronger; every other metric
// requires deviation-or-absolute. This governs the normalization GATE only —
// the Level-2 *reporting* path (CheckEquivalenceForLanguages) stays strict so
// the Dossier never overstates the granted level.
func normalizationEquivalenceLevels(metricName string) []string {
	if temporalNormalizableMetrics[metricName] {
		return []string{"temporal", "deviation", "absolute"}
	}
	return []string{"deviation", "absolute"}
}

// CheckEquivalenceExists returns true if at least one equivalence entry at an
// admissible normalization level exists for the given metricName. The
// admissible level set is metric-class-aware (Phase 124): a temporal-axis
// metric is satisfied by a temporal Level-1 grant; every other metric requires
// deviation-or-absolute. Used by the single-frame normalization gate.
func (s *ClickHouseStorage) CheckEquivalenceExists(ctx context.Context, metricName string) (bool, error) {
	levels := normalizationEquivalenceLevels(metricName)
	placeholders := make([]string, len(levels))
	args := []any{metricName}
	for i, lv := range levels {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args = append(args, lv)
	}
	query := fmt.Sprintf(`
		SELECT count() AS Cnt
		FROM aer_gold.metric_equivalence
		WHERE metric_name = $1
		  AND equivalence_level IN (%s)
	`, strings.Join(placeholders, ", "))
	var result []struct{ Cnt uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		slog.Error("Failed to check equivalence existence", "error", err)
		return false, err
	}
	return len(result) > 0 && result[0].Cnt > 0, nil
}

// GetEquivalenceStatus returns the strongest equivalence grant on record for
// metricName, or nil when none exists. Used to populate the server-authoritative
// methodology banner of the cross-probe lead-lag cell (Phase 124).
func (s *ClickHouseStorage) GetEquivalenceStatus(ctx context.Context, metricName string) (*EquivalenceStatusRow, error) {
	equivMap, err := s.fetchEquivalenceByMetric(ctx)
	if err != nil {
		return nil, err
	}
	eq, ok := equivMap[metricName]
	if !ok {
		return nil, nil
	}
	level := eq.Level
	status := &EquivalenceStatusRow{Level: &level, Notes: eq.Notes}
	if eq.ValidatedBy != "" {
		vb := eq.ValidatedBy
		status.ValidatedBy = &vb
	}
	if !eq.ValidationDate.IsZero() {
		vd := eq.ValidationDate
		status.ValidationDate = &vd
	}
	return status, nil
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
func (s *ClickHouseStorage) GetNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) (MetricsResult, int64, error) {
	cacheKey := hotQueryKey("normalized_metrics",
		start.UnixNano(), end.UnixNano(), strings.Join(sources, ","), derefString(metricName), int(resolution))
	if cached, ok := s.normalizedMetricsCache.get(cacheKey, s.metricsCacheTTL); ok {
		return MetricsResult{Rows: cached.rows, Truncated: cached.truncated}, cached.excluded, nil
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
		return MetricsResult{}, 0, err
	}
	var excluded int64
	if len(excludedResult) > 0 {
		excluded = int64(excludedResult[0].Cnt)
	}

	// SEC-077: over-fetch one row past the cap to detect (and disclose) truncation.
	rowCap := s.rowLimit * resolution.rowLimitMultiplier()
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
	`, resolution.bucketExpr("m.timestamp"), baseWhere, rowCap+1)

	var results []MetricRow
	if err := s.conn.Select(ctx, &results, query, args...); err != nil {
		slog.Error("Failed to query normalized metrics from ClickHouse", "error", err)
		return MetricsResult{}, 0, err
	}

	rows, truncated := capMetricRows(results, rowCap)

	if excluded > 0 {
		slog.Warn("normalized metrics query excluded rows lacking language detection",
			"excluded", excluded,
			"start", start,
			"end", end,
			"sources", sources,
			"metric", metricName,
		)
	}

	s.normalizedMetricsCache.put(cacheKey, normalizedMetricsCacheEntry{rows: rows, excluded: excluded, truncated: truncated})
	return MetricsResult{Rows: rows, Truncated: truncated}, excluded, nil
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
