package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// MaxCoOccurrenceTopN is the hard sanity ceiling on the number of edges a single
// co-occurrence query may return. Phase 125b raised it from the previous 500 to
// support the large-scale WebGL renderer (sigma.js) in the maximized single-cell
// view; the default SVG path still requests a small topN (default 60). The
// ceiling is a backstop against a pathological request, NOT the product limit —
// `minWeight` is the real density control. 6000 edges (~3000 nodes, ≲1.5 MB JSON)
// stays smooth end-to-end (payload + parse + ForceAtlas2 + render) on moderate
// hardware; raise it empirically after measuring real corpus payloads. Going much
// higher needs a streaming/binary transport, not just a larger cap.
const MaxCoOccurrenceTopN = 6000

// CoOccurrenceEdge is one entity-pair edge aggregated over a window.
//
// Phase 131a: “Presence“ lists the source names where this edge was
// observed within the window. The dashboard uses it to render the
// source-coloured overlay on a merged multi-source graph. Populated
// only when the scope covers multiple sources (single-source scopes
// leave it nil — every edge in that case is trivially mono-source).
type CoOccurrenceEdge struct {
	A            string
	B            string
	ALabel       string
	BLabel       string
	Weight       int64
	ArticleCount int64
	Presence     []string
	// Phase 122d.2 — Negative-Space overlay: count of this edge's contributing
	// articles that have NO real publication date (`fetch_at_fallback`). 0 when
	// the overlay was not requested. A disclosure, not a filter.
	NsSupportCount int64
}

// CoOccurrenceNode is one entity vertex derived from the edge set.
type CoOccurrenceNode struct {
	Text       string
	Label      string
	Degree     int64
	TotalCount int64
	// Presence lists the source names where this entity appears within the
	// returned edge set and the query window. Populated only when the scope
	// covers multiple sources; nil for single-source requests (Phase 114).
	Presence []string
	// WikidataQid is the canonical Wikidata identifier resolved by the Phase
	// 118 entity-linking step, or "" when no link exists for the node's text.
	WikidataQid string
	// ViewerLabel is the QID's display label in the requested viewer language
	// (Phase 123b), or "" when the node has no QID, no label exists for that
	// language, or no viewer language was requested. The client relabels a
	// node only when this is non-empty; otherwise it keeps the source surface
	// form. This is the per-language rdfs:label Wikidata publishes — never a
	// machine translation.
	ViewerLabel string
	// MetricValue is the mean of a chosen per-article metric over the articles
	// where this entity appears (Phase 125 node-metric binding), or nil when no
	// node metric was requested or the entity has no article carrying it. Lets
	// the network cell size/colour nodes by a metric (e.g. mean sentiment of the
	// articles mentioning this entity) rather than only graph-intrinsic degree.
	MetricValue *float64
	// MetricValueColor is the mean of a SECOND chosen per-article metric (Phase
	// 125 / ISSUE 7), letting the colour channel bind to a different metric than
	// the size channel. nil when no separate colour metric was requested. When
	// the colour metric equals the size metric the client reuses MetricValue, so
	// this stays nil in that case.
	MetricValueColor *float64
}

// CoOccurrenceResult bundles top-N edges with the union of incident nodes.
//
// Phase 131a: “ArticlesInScope“ is the pipeline-gap diagnostic — the
// count of articles in the window whose “aer_gold.entities“ contain
// ≥2 entities for the resolved scope. The dashboard compares it against
// “len(Edges)“ to distinguish a sparse corpus (both small) from a
// missing co-occurrence sweep (entities exist, no edges).
type CoOccurrenceResult struct {
	Nodes           []CoOccurrenceNode
	Edges           []CoOccurrenceEdge
	TopN            int64
	ArticlesInScope int64
	// LinkedNodeCount is how many returned nodes carry a Wikidata QID — the
	// subset the cross-lingual relabel toggle can act on. Surfaced so the
	// client can show the linked-vs-unlinked coverage ratio (Phase 123b /
	// WP-006 no-silent-gaps), independent of whether a viewer language was
	// requested.
	LinkedNodeCount int64
	// LabeledNodeCount is how many nodes received a ViewerLabel in the
	// requested viewer language — always <= LinkedNodeCount (a linked QID may
	// lack a label in that language). Zero when no viewer language was
	// requested.
	LabeledNodeCount int64
}

// nodeAccumulator tracks per-entity degree and total weight while building
// the incident node set from the returned edge list.
type nodeAccumulator struct {
	label  string
	degree int64
	total  int64
}

// GetEntityCoOccurrence aggregates aer_gold.entity_cooccurrences over a
// window restricted to the provided source set, returns the top-N edges by
// summed cooccurrence_count, and derives the incident node set with degrees
// and total weights.
// viewerLanguage (Phase 123b) is the optional viewer-language code (e.g. "de")
// for the cross-lingual relabel toggle. When non-empty, each node's resolved
// QID is looked up in aer_gold.wikidata_labels and the display label in that
// language is attached as ViewerLabel. Empty disables relabelling (default —
// nothing changes silently).
func (s *ClickHouseStorage) GetEntityCoOccurrence(
	ctx context.Context,
	sources []string,
	start, end time.Time,
	topN int,
	viewerLanguage string,
	nodeMetric string,
	minWeight int,
	nsOverlay bool,
	colorMetric string,
) (CoOccurrenceResult, error) {
	if topN < 1 {
		topN = 1
	}
	if topN > MaxCoOccurrenceTopN {
		topN = MaxCoOccurrenceTopN
	}
	if minWeight < 0 {
		minWeight = 0
	}

	args := []any{start, end}
	clauses := []string{
		"window_start >= $1",
		"window_start < $2",
	}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+3)
			args = append(args, src)
		}
		clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Phase 125b — min-weight edge threshold (the primary defence against the
	// quadratic edge "hairball" at scale): keep only pairs whose summed
	// co-occurrence count meets the floor. Applied as HAVING over the aggregate;
	// omitted (no-op) when minWeight <= 0.
	havingClause := ""
	if minWeight > 0 {
		havingClause = fmt.Sprintf("HAVING sum(cooccurrence_count) >= $%d", len(args)+1)
		args = append(args, uint64(minWeight))
	}

	// Phase 122d.2 — Negative-Space overlay: per edge, how many of its
	// contributing articles have NO real publication date (`fetch_at_fallback`).
	// A reflexive disclosure ("this connection leans on undated articles", WP-005
	// §3.1), NOT a filter — the edge stays in the graph. Computed only when the
	// overlay is requested (a membership subquery against aer_gold.metrics, which
	// carries timestamp_source). When off, a constant 0 keeps the column shape.
	nsCol := "toUInt64(0) AS NsArticleCount"
	if nsOverlay {
		startP := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, start)
		endP := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, end)
		nsSrcClause := ""
		if len(sources) > 0 {
			ph := make([]string, len(sources))
			for i, src := range sources {
				ph[i] = fmt.Sprintf("$%d", len(args)+1)
				args = append(args, src)
			}
			nsSrcClause = fmt.Sprintf(" AND source IN (%s)", strings.Join(ph, ", "))
		}
		nsCol = fmt.Sprintf(
			"uniqExactIf(article_id, article_id IN (SELECT article_id FROM aer_gold.metrics WHERE timestamp_source = 'fetch_at_fallback' AND timestamp >= %s AND timestamp < %s%s)) AS NsArticleCount",
			startP, endP, nsSrcClause,
		)
	}

	query := fmt.Sprintf(`
		SELECT
			entity_a_text  AS A,
			entity_b_text  AS B,
			any(entity_a_label) AS ALabel,
			any(entity_b_label) AS BLabel,
			sum(cooccurrence_count) AS Weight,
			uniqExact(article_id) AS ArticleCount,
			%s
		FROM aer_gold.entity_cooccurrences FINAL
		WHERE %s
		GROUP BY A, B
		%s
		ORDER BY Weight DESC, A ASC, B ASC
		LIMIT %d
	`, nsCol, strings.Join(clauses, " AND "), havingClause, topN)

	var rows []struct {
		A              string
		B              string
		ALabel         string
		BLabel         string
		Weight         uint64
		ArticleCount   uint64
		NsArticleCount uint64
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query entity co-occurrence", "error", err)
		return CoOccurrenceResult{}, err
	}

	edges := make([]CoOccurrenceEdge, len(rows))
	for i, r := range rows {
		edges[i] = CoOccurrenceEdge{
			A:              r.A,
			B:              r.B,
			ALabel:         r.ALabel,
			BLabel:         r.BLabel,
			Weight:         int64(r.Weight),         //nolint:gosec // bounded by aggregation
			ArticleCount:   int64(r.ArticleCount),   //nolint:gosec // bounded by aggregation
			NsSupportCount: int64(r.NsArticleCount), //nolint:gosec // bounded by aggregation
		}
	}

	// Derive incident nodes from the edge set so degree / totalCount are
	// consistent with the topN truncation. Otherwise a node would appear
	// here with weight totals that include edges the client never sees.
	acc := map[string]*nodeAccumulator{}
	for _, e := range edges {
		if _, ok := acc[e.A]; !ok {
			acc[e.A] = &nodeAccumulator{label: e.ALabel}
		}
		if _, ok := acc[e.B]; !ok {
			acc[e.B] = &nodeAccumulator{label: e.BLabel}
		}
		acc[e.A].degree++
		acc[e.A].total += e.Weight
		acc[e.B].degree++
		acc[e.B].total += e.Weight
	}

	// Derive per-node source presence when multiple sources are in scope.
	// A second query collects the distinct source names each entity appears in
	// within the window — this lets the frontend shade nodes by source without
	// an additional round-trip (Phase 114).
	var presenceMap map[string][]string
	if len(sources) > 1 && len(acc) > 0 {
		presenceMap, _ = s.queryNodePresence(ctx, acc, args, clauses)
	}

	// Phase 131a — per-edge source presence powers the source-coloured
	// overlay on a merged multi-source graph. Single-source scopes get
	// a trivial mono-source presence, so we skip the round-trip there.
	if len(sources) > 1 && len(edges) > 0 {
		if edgePresence, err := s.queryEdgePresence(ctx, edges, args, clauses); err == nil {
			for i := range edges {
				key := edgeKey(edges[i].A, edges[i].B)
				if p, ok := edgePresence[key]; ok {
					edges[i].Presence = p
				}
			}
		}
	}

	// Phase 118: resolve a Wikidata QID per node. The lookup is best-effort
	// — failure does not block the graph response, just leaves QIDs empty.
	var qidMap map[string]string
	if len(acc) > 0 {
		qidMap, _ = s.queryNodeWikidataQids(ctx, acc)
	}

	// Phase 123b: resolve the per-language display label for each linked QID
	// when a viewer language is requested. Best-effort, mirroring the QID
	// lookup — a failure leaves nodes without a ViewerLabel (every node then
	// keeps its source surface form), never a 5xx. labelMap is keyed by QID.
	var labelMap map[string]string
	if viewerLanguage != "" && qidMap != nil && len(qidMap) > 0 {
		labelMap, _ = s.queryNodeLabels(ctx, qidMap, viewerLanguage)
	}

	// Phase 125: per-node metric aggregation (mean of `nodeMetric` over the
	// articles where each entity appears). Best-effort, mirroring the QID/label
	// lookups — a failure leaves nodes without a MetricValue, never a 5xx.
	var metricMap map[string]float64
	if nodeMetric != "" && len(acc) > 0 {
		metricMap, _ = s.queryNodeMetric(ctx, acc, nodeMetric, sources, start, end)
	}
	// Phase 125 / ISSUE 7: optional SECOND metric for the colour channel, so
	// size and colour can bind to different metrics. Only queried when it is set
	// AND differs from the size metric (equal → the client reuses MetricValue).
	var colorMetricMap map[string]float64
	if colorMetric != "" && colorMetric != nodeMetric && len(acc) > 0 {
		colorMetricMap, _ = s.queryNodeMetric(ctx, acc, colorMetric, sources, start, end)
	}

	var linkedNodeCount, labeledNodeCount int64
	nodes := make([]CoOccurrenceNode, 0, len(acc))
	for text, a := range acc {
		n := CoOccurrenceNode{
			Text:       text,
			Label:      a.label,
			Degree:     a.degree,
			TotalCount: a.total,
		}
		if presenceMap != nil {
			n.Presence = presenceMap[text]
		}
		if metricMap != nil {
			if v, ok := metricMap[text]; ok {
				vc := v
				n.MetricValue = &vc
			}
		}
		if colorMetricMap != nil {
			if v, ok := colorMetricMap[text]; ok {
				vc := v
				n.MetricValueColor = &vc
			}
		}
		if qidMap != nil {
			n.WikidataQid = qidMap[text]
		}
		if n.WikidataQid != "" {
			linkedNodeCount++
			if labelMap != nil {
				if lbl, ok := labelMap[n.WikidataQid]; ok && lbl != "" {
					n.ViewerLabel = lbl
					labeledNodeCount++
				}
			}
		}
		nodes = append(nodes, n)
	}

	// Phase 131a pipeline-gap diagnostic: count articles with ≥2 entities
	// in the window/scope. Lets the dashboard distinguish "sparse corpus"
	// from "co-occurrence sweep missing this article". Best-effort — a
	// failure leaves the count at 0 and the dashboard falls back to the
	// generic empty-graph hint rather than returning 5xx.
	articlesInScope, _ := s.queryArticlesInScopeForCoOccurrence(ctx, sources, start, end)

	return CoOccurrenceResult{
		Nodes:            nodes,
		Edges:            edges,
		TopN:             int64(topN),
		ArticlesInScope:  articlesInScope,
		LinkedNodeCount:  linkedNodeCount,
		LabeledNodeCount: labeledNodeCount,
	}, nil
}

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

// queryNodePresence returns a map from entity text to the sorted list of
// distinct sources where that entity appears within the already-computed
// WHERE window. Only called for multi-source scopes (Phase 114).
func (s *ClickHouseStorage) queryNodePresence(
	ctx context.Context,
	acc map[string]*nodeAccumulator,
	args []any,
	clauses []string,
) (map[string][]string, error) {
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
		SELECT entity_text, groupArrayDistinct(source) AS Presence
		FROM (
			SELECT entity_a_text AS entity_text, source
			FROM aer_gold.entity_cooccurrences FINAL
			WHERE %s AND entity_a_text IN (%s)
			UNION ALL
			SELECT entity_b_text AS entity_text, source
			FROM aer_gold.entity_cooccurrences FINAL
			WHERE %s AND entity_b_text IN (%s)
		)
		GROUP BY entity_text
	`, windowFilter, textIN, windowFilter, textIN)

	allArgs := append(textArgs, textArgs...)

	var rows []struct {
		EntityText string   `ch:"entity_text"`
		Presence   []string `ch:"Presence"`
	}
	if err := s.conn.Select(ctx, &rows, presenceQuery, allArgs...); err != nil {
		slog.Warn("Failed to query node presence (non-fatal)", "error", err)
		return nil, err //nolint:wrapcheck // caller treats this as optional
	}

	result := make(map[string][]string, len(rows))
	for _, r := range rows {
		result[r.EntityText] = r.Presence
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
