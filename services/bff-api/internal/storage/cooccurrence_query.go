package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CoOccurrenceEdge is one entity-pair edge aggregated over a window.
//
// Phase 131a: ``Presence`` lists the source names where this edge was
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
}

// CoOccurrenceResult bundles top-N edges with the union of incident nodes.
//
// Phase 131a: ``ArticlesInScope`` is the pipeline-gap diagnostic — the
// count of articles in the window whose ``aer_gold.entities`` contain
// ≥2 entities for the resolved scope. The dashboard compares it against
// ``len(Edges)`` to distinguish a sparse corpus (both small) from a
// missing co-occurrence sweep (entities exist, no edges).
type CoOccurrenceResult struct {
	Nodes           []CoOccurrenceNode
	Edges           []CoOccurrenceEdge
	TopN            int64
	ArticlesInScope int64
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
func (s *ClickHouseStorage) GetEntityCoOccurrence(
	ctx context.Context,
	sources []string,
	start, end time.Time,
	topN int,
) (CoOccurrenceResult, error) {
	if topN < 1 {
		topN = 1
	}
	if topN > 500 {
		topN = 500
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

	query := fmt.Sprintf(`
		SELECT
			entity_a_text  AS A,
			entity_b_text  AS B,
			any(entity_a_label) AS ALabel,
			any(entity_b_label) AS BLabel,
			sum(cooccurrence_count) AS Weight,
			uniqExact(article_id) AS ArticleCount
		FROM aer_gold.entity_cooccurrences FINAL
		WHERE %s
		GROUP BY A, B
		ORDER BY Weight DESC, A ASC, B ASC
		LIMIT %d
	`, strings.Join(clauses, " AND "), topN)

	var rows []struct {
		A            string
		B            string
		ALabel       string
		BLabel       string
		Weight       uint64
		ArticleCount uint64
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query entity co-occurrence", "error", err)
		return CoOccurrenceResult{}, err
	}

	edges := make([]CoOccurrenceEdge, len(rows))
	for i, r := range rows {
		edges[i] = CoOccurrenceEdge{
			A:            r.A,
			B:            r.B,
			ALabel:       r.ALabel,
			BLabel:       r.BLabel,
			Weight:       int64(r.Weight),       //nolint:gosec // bounded by aggregation
			ArticleCount: int64(r.ArticleCount), //nolint:gosec // bounded by aggregation
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
		if qidMap != nil {
			n.WikidataQid = qidMap[text]
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
		Nodes:           nodes,
		Edges:           edges,
		TopN:            int64(topN),
		ArticlesInScope: articlesInScope,
	}, nil
}

// edgeKey gives a canonical key for the unordered pair (a, b). Storage
// rows already store the lexicographic canonical order so the input is
// assumed sorted; keeping a helper keeps the contract explicit.
func edgeKey(a, b string) string { return a + "\x00" + b }

// queryEdgePresence returns the distinct source set for each
// already-returned edge, keyed by ``edgeKey(a, b)``. Used by the
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
// article_ids in ``aer_gold.entities`` for the given scope+window whose
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
