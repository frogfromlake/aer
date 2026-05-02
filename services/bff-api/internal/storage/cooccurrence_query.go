package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CoOccurrenceEdge is one entity-pair edge aggregated over a window.
type CoOccurrenceEdge struct {
	A            string
	B            string
	ALabel       string
	BLabel       string
	Weight       int64
	ArticleCount int64
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
type CoOccurrenceResult struct {
	Nodes []CoOccurrenceNode
	Edges []CoOccurrenceEdge
	TopN  int64
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

	return CoOccurrenceResult{
		Nodes: nodes,
		Edges: edges,
		TopN:  int64(topN),
	}, nil
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
