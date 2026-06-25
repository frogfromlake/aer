package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// edgeKey gives a canonical key for the unordered pair (a, b). Storage
// rows already store the lexicographic canonical order so the input is
// assumed sorted; keeping a helper keeps the contract explicit.
func edgeKey(a, b string) string { return a + "\x00" + b }

// queryEdgePresence returns the distinct source set for each
// already-returned edge, keyed by “edgeKey(a, b)“. Used by the
// source-coloured-overlay overlay path (Phase 131a). Best-effort:
// failure leaves edges without presence and the dashboard falls back
// to label-based colouring.
func (s *ClickHouseStorage) queryEdgePresence(
	ctx context.Context,
	edges []CoOccurrenceEdge,
	args []any,
	clauses []string,
) (map[string][]string, error) {
	if len(edges) == 0 {
		return nil, nil //nolint:nilnil // empty input → empty result
	}
	// Build a (entity_a_text, entity_b_text) IN ((..)) tuple filter. Per
	// the table contract entity_a_text <= entity_b_text, so the same
	// canonical order applies on both sides of the IN.
	tuples := make([]string, len(edges))
	pArgs := make([]any, len(args), len(args)+len(edges)*2)
	copy(pArgs, args)
	for i, e := range edges {
		aPos := len(args) + i*2 + 1
		bPos := aPos + 1
		tuples[i] = fmt.Sprintf("($%d, $%d)", aPos, bPos)
		pArgs = append(pArgs, e.A, e.B)
	}
	windowFilter := strings.Join(clauses, " AND ")
	query := fmt.Sprintf(`
		SELECT
			entity_a_text AS A,
			entity_b_text AS B,
			groupArrayDistinct(source) AS Presence
		FROM aer_gold.entity_cooccurrences FINAL
		WHERE %s AND (entity_a_text, entity_b_text) IN (%s)
		GROUP BY A, B
	`, windowFilter, strings.Join(tuples, ", "))

	var rows []struct {
		A        string   `ch:"A"`
		B        string   `ch:"B"`
		Presence []string `ch:"Presence"`
	}
	if err := s.conn.Select(ctx, &rows, query, pArgs...); err != nil {
		slog.Warn("Failed to query edge presence (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}
	out := make(map[string][]string, len(rows))
	for _, r := range rows {
		out[edgeKey(r.A, r.B)] = r.Presence
	}
	return out, nil
}

// queryArticlesInScopeForCoOccurrence returns the count of distinct
// article_ids in “aer_gold.entities“ for the given scope+window whose
// entity count is ≥2. This is the Phase 131a pipeline-gap probe — when
// the cooccurrence query returns zero edges but this count is non-zero,
// the worker's sweep is failing to emit rows for entity-bearing
// articles. Best-effort: failure returns 0 and the dashboard falls back
// to the generic empty-graph hint.
func (s *ClickHouseStorage) queryArticlesInScopeForCoOccurrence(
	ctx context.Context,
	sources []string,
	start, end time.Time,
) (int64, error) {
	args := []any{start, end}
	clauses := []string{
		"timestamp >= $1",
		"timestamp < $2",
		"article_id IS NOT NULL",
		"entity_text != ''",
	}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, src)
		}
		clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ", ")))
	}
	// Phase 131a — count articles whose entity set contains ≥2 DISTINCT
	// (entity_text, entity_label) pairs, matching the worker extractor's
	// _pairs_for_article logic. Row-count would mislead: an article
	// mentioning "Merkel" twice and no one else has two entity rows but
	// only one unique entity and legitimately produces zero pairs.
	query := fmt.Sprintf(`
		SELECT count() AS ArticlesInScope FROM (
			SELECT article_id
			FROM aer_gold.entities FINAL
			WHERE %s
			GROUP BY article_id
			HAVING uniqExact(entity_text, entity_label) >= 2
		)
	`, strings.Join(clauses, " AND "))

	var rows []struct {
		ArticlesInScope uint64 `ch:"ArticlesInScope"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Warn("Failed to query articles-in-scope (non-fatal)", "error", err)
		return 0, err //nolint:wrapcheck // caller treats this as optional
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return int64(rows[0].ArticlesInScope), nil //nolint:gosec // bounded by aggregation
}

// queryNodePresence returns a map from entity text to the per-source distinct-
// article counts (ordered by source) where that entity appears within the
// already-computed WHERE window. Only called for multi-source scopes (Phase
// 114; Phase 148g added the per-source counts for the node tooltip).
func (s *ClickHouseStorage) queryNodePresence(
	ctx context.Context,
	acc map[string]*nodeAccumulator,
	args []any,
	clauses []string,
) (map[string][]NodeSourceCount, error) {
	texts := make([]string, 0, len(acc))
	for t := range acc {
		texts = append(texts, t)
	}

	// Build text IN (...) placeholders; args already contains time + source placeholders.
	textArgs := make([]any, len(args), len(args)+len(texts))
	copy(textArgs, args)
	textPlaceholders := make([]string, len(texts))
	for i, t := range texts {
		textPlaceholders[i] = fmt.Sprintf("$%d", len(args)+i+1)
		textArgs = append(textArgs, t)
	}
	textIN := strings.Join(textPlaceholders, ", ")
	windowFilter := strings.Join(clauses, " AND ")

	presenceQuery := fmt.Sprintf(`
		SELECT entity_text, source, uniqExact(article_id) AS Cnt
		FROM (
			SELECT entity_a_text AS entity_text, source, article_id
			FROM aer_gold.entity_cooccurrences FINAL
			WHERE %s AND entity_a_text IN (%s)
			UNION ALL
			SELECT entity_b_text AS entity_text, source, article_id
			FROM aer_gold.entity_cooccurrences FINAL
			WHERE %s AND entity_b_text IN (%s)
		)
		GROUP BY entity_text, source
		ORDER BY entity_text ASC, source ASC
	`, windowFilter, textIN, windowFilter, textIN)

	allArgs := append(textArgs, textArgs...)

	var rows []struct {
		EntityText string `ch:"entity_text"`
		Source     string `ch:"source"`
		Cnt        uint64 `ch:"Cnt"`
	}
	if err := s.conn.Select(ctx, &rows, presenceQuery, allArgs...); err != nil {
		slog.Warn("Failed to query node presence (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}

	result := make(map[string][]NodeSourceCount, len(acc))
	for _, r := range rows {
		result[r.EntityText] = append(result[r.EntityText], NodeSourceCount{
			Source: r.Source,
			Count:  int64(r.Cnt), //nolint:gosec // bounded by aggregation
		})
	}
	return result, nil
}

// queryNodeWikidataQids resolves a Wikidata QID for each node in the
// accumulator. The lookup is independent of window/source — `entity_links`
// is a per-(article_id, entity_text) table without a discourse_function or
// timestamp axis the BFF cares about for this surface — so a single
// argMax(wikidata_qid, link_confidence) GROUP BY entity_text is sufficient.
// Returns nil + nil error when the accumulator is empty.
//
// Phase 118 — best-effort: a failure here returns the unlinked graph
// rather than a 5xx (entity linking is a metadata layer over the canonical
// `aer_gold.entities` data, not a load-bearing dependency).
func (s *ClickHouseStorage) queryNodeWikidataQids(
	ctx context.Context,
	acc map[string]*nodeAccumulator,
) (map[string]string, error) {
	if len(acc) == 0 {
		return nil, nil
	}
	texts := make([]string, 0, len(acc))
	for t := range acc {
		texts = append(texts, t)
	}
	args := make([]any, 0, len(texts))
	placeholders := make([]string, len(texts))
	for i, t := range texts {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, t)
	}
	query := fmt.Sprintf(`
		SELECT
			entity_text,
			argMax(wikidata_qid, link_confidence) AS wikidata_qid
		FROM aer_gold.entity_links
		WHERE entity_text IN (%s)
		GROUP BY entity_text
	`, strings.Join(placeholders, ", "))

	var rows []struct {
		EntityText  string `ch:"entity_text"`
		WikidataQid string `ch:"wikidata_qid"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Warn("Failed to query node wikidata QIDs (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.WikidataQid != "" {
			result[r.EntityText] = r.WikidataQid
		}
	}
	return result, nil
}

// queryNodeMetric returns, for the node entity-text set, the mean of `metric`
// over the articles where each entity appears (Phase 125). Joins
// aer_gold.entities (entity → article_id) with a per-article metric pivot on
// article_id, within the window + sources, grouped by entity. Best-effort: a
// failure returns nil and the caller leaves nodes without a MetricValue.
func (s *ClickHouseStorage) queryNodeMetric(
	ctx context.Context,
	acc map[string]*nodeAccumulator,
	metric string,
	sources []string,
	start, end time.Time,
) (map[string]float64, error) {
	if len(acc) == 0 || metric == "" {
		return nil, nil
	}
	args := make([]any, 0, 6+len(acc)+2*len(sources))
	ph := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	srcIn := func() string {
		if len(sources) == 0 {
			return ""
		}
		sp := make([]string, len(sources))
		for i, src := range sources {
			sp[i] = ph(src)
		}
		return fmt.Sprintf(" AND source IN (%s)", strings.Join(sp, ", "))
	}
	textPh := make([]string, 0, len(acc))
	for t := range acc {
		textPh = append(textPh, ph(t))
	}
	eStart := ph(start)
	eEnd := ph(end)
	eSrc := srcIn()
	metricP := ph(metric)
	mStart := ph(start)
	mEnd := ph(end)
	mSrc := srcIn()

	query := fmt.Sprintf(`
		SELECT e.entity_text AS EntityText, avg(m.MetricValue) AS Mean
		FROM (
			SELECT DISTINCT entity_text, article_id
			FROM aer_gold.entities FINAL
			WHERE entity_text IN (%s) AND timestamp >= %s AND timestamp < %s AND article_id IS NOT NULL%s
		) e
		INNER JOIN (
			SELECT article_id, avg(value) AS MetricValue
			FROM aer_gold.metrics
			WHERE metric_name = %s AND timestamp >= %s AND timestamp < %s AND article_id IS NOT NULL%s
			GROUP BY article_id
		) m ON e.article_id = m.article_id
		GROUP BY EntityText
	`, strings.Join(textPh, ", "), eStart, eEnd, eSrc, metricP, mStart, mEnd, mSrc)

	var rows []struct {
		EntityText string  `ch:"EntityText"`
		Mean       float64 `ch:"Mean"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Warn("Failed to query node metric (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}
	result := make(map[string]float64, len(rows))
	for _, r := range rows {
		result[r.EntityText] = r.Mean
	}
	return result, nil
}

// queryNodeLabels resolves the display label per QID in the given viewer
// language from aer_gold.wikidata_labels (Phase 123b). qidMap is the
// entity-text → QID map already resolved by queryNodeWikidataQids; the lookup
// is keyed and returned by QID. FINAL collapses the ReplacingMergeTree so a
// reloaded reference table never yields duplicate rows. Best-effort: a failure
// returns nil and the caller leaves nodes on their source surface form.
func (s *ClickHouseStorage) queryNodeLabels(
	ctx context.Context,
	qidMap map[string]string,
	language string,
) (map[string]string, error) {
	// Distinct QIDs (several entity texts can map to the same QID).
	qidSet := make(map[string]struct{}, len(qidMap))
	for _, qid := range qidMap {
		if qid != "" {
			qidSet[qid] = struct{}{}
		}
	}
	if len(qidSet) == 0 {
		return nil, nil //nolint:nilnil // empty input → empty result
	}
	args := make([]any, 0, len(qidSet)+1)
	args = append(args, language)
	placeholders := make([]string, 0, len(qidSet))
	i := 0
	for qid := range qidSet {
		placeholders = append(placeholders, fmt.Sprintf("$%d", i+2))
		args = append(args, qid)
		i++
	}
	query := fmt.Sprintf(`
		SELECT wikidata_qid, label
		FROM aer_gold.wikidata_labels FINAL
		WHERE language = $1 AND wikidata_qid IN (%s)
	`, strings.Join(placeholders, ", "))

	var rows []struct {
		WikidataQid string `ch:"wikidata_qid"`
		Label       string `ch:"label"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Warn("Failed to query node display labels (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}
	result := make(map[string]string, len(rows))
	for _, r := range rows {
		if r.Label != "" {
			result[r.WikidataQid] = r.Label
		}
	}
	return result, nil
}
