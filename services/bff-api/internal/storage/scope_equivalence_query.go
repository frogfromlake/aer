package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

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

// PartialMetric is a metric present for only a subset of the scoped sources.
type PartialMetric struct {
	MetricName string
	Sources    []string
}

// DegenerateMetric is a metric present in the scope window whose value is
// CONSTANT across the whole scope (exactly one distinct value) — present, but
// carrying no signal. Disclosed (with the constant value) rather than silently
// dropped (ADR-039 DISCLOSE-NEVER-COERCE). Self-maintaining: a future source
// that introduces real variance removes the metric from this list automatically.
type DegenerateMetric struct {
	MetricName string
	Value      float64
}

// LowSignalMetric is an available metric that carries effectively no signal in
// this scope: either ≤2 distinct values, or one value dominates ≥ the threshold
// (e.g. image_count = 3 on 99.8 % of articles — a mis-read that is constant per
// article). The metric analogue of LowSignalField: the dashboard now DROPS it
// from the picker (never auto-shown) and discloses it under "no signal". The
// flag is scope-relative — a source set with real variance clears it.
type LowSignalMetric struct {
	MetricName     string
	DistinctValues int
	DominantShare  float64
	DominantValue  float64
}

// structuralNoSignalMetrics are scalar metrics that describe a document's FORMAT
// rather than its discourse content and are ALWAYS classified no-signal,
// regardless of how their values split in a scope (operator decision
// 2026-06-24). `image_count` is a mis-read structural attribute (the extractor
// returns a near-constant per-article count), not an editorial measure — the
// metric analogue of structuralNoSignalFields in article_metadata_query.go.
var structuralNoSignalMetrics = map[string]bool{
	"image_count": true,
}

// ScopeMetricAvailability splits the metrics observed in a scope's window into
// those present for every scoped source (Available) and those present for only
// some (Partial). Powers the Phase-123c cross-probe metric guard so a panel
// spanning probes with asymmetric capability never binds a metric that would
// silently yield empty cells.
//
// Degenerate (constant) and LowSignal (near-constant) are ADDITIVE advisory
// lists (Task A): Available/Partial keep their pure "has data" semantics, while
// Degenerate is dropped from the picker and LowSignal is flagged-but-offerable.
// Both are computed over the scope union; LowSignal is restricted to Available
// metrics (a partial metric is already withheld by the intersection gate).
type ScopeMetricAvailability struct {
	ScopedSources []string
	Available     []string
	Partial       []PartialMetric
	Degenerate    []DegenerateMetric
	LowSignal     []LowSignalMetric
}

// GetScopeAvailableMetrics returns, for the given sources and window, which
// metric names have Gold data for every source (the intersection) versus only
// some. The intersection is the only set a panel spanning the whole scope may
// safely bind. Source lists in Partial are returned in scope order.
func (s *ClickHouseStorage) GetScopeAvailableMetrics(ctx context.Context, start, end time.Time, sources []string) (ScopeMetricAvailability, error) {
	out := ScopeMetricAvailability{ScopedSources: sources, Available: []string{}, Partial: []PartialMetric{}}
	if len(sources) == 0 {
		return out, nil
	}

	query := `
		SELECT DISTINCT source AS Source, metric_name AS MetricName
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	placeholders := make([]string, len(sources))
	for i, src := range sources {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, src)
	}
	query += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	query += " ORDER BY MetricName, Source"

	var rows []struct {
		Source     string
		MetricName string
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query scope available metrics", "error", err)
		return out, err
	}

	// Group scoped sources per metric, preserving first-seen metric order.
	bySrc := map[string]map[string]bool{}
	order := []string{}
	for _, r := range rows {
		if _, ok := bySrc[r.MetricName]; !ok {
			bySrc[r.MetricName] = map[string]bool{}
			order = append(order, r.MetricName)
		}
		bySrc[r.MetricName][r.Source] = true
	}

	total := len(sources)
	for _, metric := range order {
		srcSet := bySrc[metric]
		if len(srcSet) == total {
			out.Available = append(out.Available, metric)
			continue
		}
		present := make([]string, 0, len(srcSet))
		for _, src := range sources {
			if srcSet[src] {
				present = append(present, src)
			}
		}
		out.Partial = append(out.Partial, PartialMetric{MetricName: metric, Sources: present})
	}

	// Task A: per-metric value concentration over the scope union — yields BOTH
	// degenerate (distinct == 1, dropped) and low-signal (≤2 distinct values, or
	// a single value dominating ≥ threshold, e.g. image_count = 3 on 99.8 % of
	// articles — also dropped from the picker, disclosed under "no signal"). The
	// nested (metric, value) rollup is bounded by each metric's distinct-value
	// cardinality, so a continuous metric like sentiment_score has a near-uniform
	// distribution (dominantShare ≈ 1/N, many distinct) and is never flagged —
	// only genuinely discrete, near-constant metrics surface.
	availableSet := map[string]bool{}
	for _, mn := range out.Available {
		availableSet[mn] = true
	}
	concQuery := `
		SELECT
			Metric,
			uniqExact(Value)  AS Distinct,
			max(Cnt)          AS Dominant,
			sum(Cnt)          AS Total,
			argMax(Value, Cnt) AS DominantValue
		FROM (
			SELECT metric_name AS Metric, value AS Value, count() AS Cnt
			FROM aer_gold.metrics
			WHERE timestamp >= $1 AND timestamp <= $2
	`
	concQuery += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	concQuery += `
			GROUP BY Metric, Value
		)
		GROUP BY Metric
		ORDER BY Metric
	`
	var concRows []struct {
		Metric        string  `ch:"Metric"`
		Distinct      uint64  `ch:"Distinct"`
		Dominant      uint64  `ch:"Dominant"`
		Total         uint64  `ch:"Total"`
		DominantValue float64 `ch:"DominantValue"`
	}
	if err := s.conn.Select(ctx, &concRows, concQuery, args...); err != nil {
		slog.Error("Failed to query scope metric concentration", "error", err)
		return out, err
	}
	for _, r := range concRows {
		structural := structuralNoSignalMetrics[r.Metric]
		// Phase 148g — no-signal classification (degenerate AND low-signal) applies
		// to OFFERABLE (available) metrics + STRUCTURAL metrics only. A PARTIAL
		// metric is already surfaced as "withheld" by the intersection gate;
		// classifying it no-signal here — and pruning it from Partial below — would
		// wrongly empty the withheld list when a partial metric is constant over the
		// scope union in the window. A STRUCTURAL metric (format, e.g. image_count)
		// is no-signal on ANY source set, so it IS classified even when partial,
		// otherwise "show anyway" would resurrect it.
		if (!structural && !availableSet[r.Metric]) || r.Total == 0 {
			continue
		}
		if r.Distinct <= 1 {
			out.Degenerate = append(out.Degenerate, DegenerateMetric{MetricName: r.Metric, Value: r.DominantValue})
			continue
		}
		share := float64(r.Dominant) / float64(r.Total)
		if structural || r.Distinct <= lowSignalMaxDistinct || share >= lowSignalDominanceThreshold {
			out.LowSignal = append(out.LowSignal, LowSignalMetric{
				MetricName:     r.Metric,
				DistinctValues: int(r.Distinct), //nolint:gosec // bounded by metric value cardinality
				DominantShare:  share,
				DominantValue:  r.DominantValue,
			})
		}
	}

	// A no-signal metric (constant / near-constant / structural) belongs ONLY
	// under the "no signal" disclosure — never the withheld (Partial) list, where
	// "show anyway" would resurrect it. Prune it from Partial.
	noSignal := make(map[string]bool, len(out.Degenerate)+len(out.LowSignal))
	for _, d := range out.Degenerate {
		noSignal[d.MetricName] = true
	}
	for _, l := range out.LowSignal {
		noSignal[l.MetricName] = true
	}
	if len(noSignal) > 0 {
		kept := out.Partial[:0]
		for _, p := range out.Partial {
			if !noSignal[p.MetricName] {
				kept = append(kept, p)
			}
		}
		out.Partial = kept
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

// CheckNormalizationEquivalenceForLanguages is the cross-frame normalization
// gate (Phase 124). It returns true when metricName has a granted
// metric_equivalence row at an admissible normalization level for EVERY
// language in `languages`. The admissible level set is metric-class-aware
// (normalizationEquivalenceLevels): a temporal-axis metric is satisfied by a
// temporal Level-1 grant; every other metric requires deviation-or-absolute.
//
// This deliberately differs from CheckEquivalenceForLanguages, which stays
// strict (deviation/absolute) for the Level-2 *reporting* path so the Dossier
// equivalence matrix never reports a metric as deviation-comparable on the
// strength of a temporal-only grant.
func (s *ClickHouseStorage) CheckNormalizationEquivalenceForLanguages(ctx context.Context, metricName string, languages []string) (bool, error) {
	if len(languages) == 0 {
		return true, nil
	}
	levels := normalizationEquivalenceLevels(metricName)
	args := []any{metricName}
	levelPlaceholders := make([]string, len(levels))
	for i, lv := range levels {
		levelPlaceholders[i] = fmt.Sprintf("$%d", len(args)+1)
		args = append(args, lv)
	}
	langPlaceholders := make([]string, len(languages))
	for i, lang := range languages {
		langPlaceholders[i] = fmt.Sprintf("$%d", len(args)+1)
		args = append(args, lang)
	}
	query := fmt.Sprintf(`
		SELECT countDistinct(language) AS N
		FROM aer_gold.metric_equivalence FINAL
		WHERE metric_name = $1
		  AND equivalence_level IN (%s)
		  AND language IN (%s)
	`, strings.Join(levelPlaceholders, ", "), strings.Join(langPlaceholders, ", "))
	var result []struct{ N uint64 }
	if err := s.conn.Select(ctx, &result, query, args...); err != nil {
		slog.Error("Failed to check cross-frame normalization equivalence", "error", err)
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
func (s *ClickHouseStorage) GetPercentileNormalizedMetrics(ctx context.Context, start, end time.Time, sources []string, metricName *string, resolution Resolution) (MetricsResult, int64, error) {
	cacheKey := hotQueryKey("percentile_metrics",
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
		return MetricsResult{}, 0, err
	}
	var excluded int64
	if len(excludedResult) > 0 {
		excluded = int64(excludedResult[0].Cnt)
	}

	// SEC-077: over-fetch one row past the cap to detect (and disclose) truncation.
	rowCap := s.rowLimit * resolution.rowLimitMultiplier()
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
	`, baseWhere, resolution.bucketExpr("ts"), rowCap+1)

	var results []MetricRow
	if err := s.conn.Select(ctx, &results, query, args...); err != nil {
		slog.Error("Failed to query percentile metrics from ClickHouse", "error", err)
		return MetricsResult{}, 0, err
	}

	rows, truncated := capMetricRows(results, rowCap)
	s.normalizedMetricsCache.put(cacheKey, normalizedMetricsCacheEntry{rows: rows, excluded: excluded, truncated: truncated})
	return MetricsResult{Rows: rows, Truncated: truncated}, excluded, nil
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
