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
}

// CoOccurrenceResult bundles top-N edges with the union of incident nodes.
type CoOccurrenceResult struct {
	Nodes []CoOccurrenceNode
	Edges []CoOccurrenceEdge
	TopN  int64
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
	type nodeAccumulator struct {
		label    string
		degree   int64
		total    int64
	}
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

	nodes := make([]CoOccurrenceNode, 0, len(acc))
	for text, a := range acc {
		nodes = append(nodes, CoOccurrenceNode{
			Text:       text,
			Label:      a.label,
			Degree:     a.degree,
			TotalCount: a.total,
		})
	}

	return CoOccurrenceResult{
		Nodes: nodes,
		Edges: edges,
		TopN:  int64(topN),
	}, nil
}
