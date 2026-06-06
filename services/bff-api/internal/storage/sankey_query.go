package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SankeyNode is one category value at one layer (a field position in the chain).
// ID is layer-namespaced (`<layer>::<value>`) so identical values in different
// fields never collide.
type SankeyNode struct {
	ID    string `json:"id"`
	Field string `json:"field"`
	Value string `json:"value"`
	Layer int    `json:"layer"`
}

// SankeyLink is a flow of distinct articles between a node in layer i and a node
// in layer i+1.
type SankeyLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

// SankeyResult is the alluvial flow across an ordered chain of categorical
// fields (Phase 125).
type SankeyResult struct {
	Fields []string
	Nodes  []SankeyNode
	Links  []SankeyLink
}

type sankeyPairRow struct {
	Src string `ch:"Src"`
	Dst string `ch:"Dst"`
	N   uint64 `ch:"N"`
}

// GetSankey builds the alluvial flow between an ordered list of categorical
// metadata fields: for each consecutive pair it joins aer_gold.article_metadata
// on article_id and counts distinct articles per (value, value) pair. FINAL on
// both sides (exact distinct-article counts; the documented metadata exception).
// topN caps the edges PER pair so a long tail does not explode the graph.
//
// Caveat (disclosed in the cell): for a list-valued field an article may carry
// several values, so it can contribute to several flows — the count is distinct
// articles per (src,dst) pair, a value-occurrence weight across the chain.
func (s *ClickHouseStorage) GetSankey(
	ctx context.Context,
	fields []string,
	sources []string,
	start, end time.Time,
	topN int,
) (SankeyResult, error) {
	out := SankeyResult{Fields: fields, Nodes: []SankeyNode{}, Links: []SankeyLink{}}
	if len(fields) < 2 || len(sources) == 0 {
		return out, nil
	}
	if topN < 1 {
		topN = 50
	}
	if topN > 200 {
		topN = 200
	}

	nodeIndex := make(map[string]int) // id → index in out.Nodes
	addNode := func(layer int, field, value string) string {
		id := fmt.Sprintf("%d::%s", layer, value)
		if _, ok := nodeIndex[id]; !ok {
			nodeIndex[id] = len(out.Nodes)
			out.Nodes = append(out.Nodes, SankeyNode{ID: id, Field: field, Value: value, Layer: layer})
		}
		return id
	}

	for i := 0; i+1 < len(fields); i++ {
		f1 := fields[i]
		f2 := fields[i+1]

		sa := newScopeArgs()
		aWhere := fmt.Sprintf("field = %s AND timestamp >= %s AND timestamp < %s AND source IN (%s)",
			sa.ph(f1), sa.ph(start), sa.ph(end), sa.srcIn(sources))
		bWhere := fmt.Sprintf("field = %s AND timestamp >= %s AND timestamp < %s AND source IN (%s)",
			sa.ph(f2), sa.ph(start), sa.ph(end), sa.srcIn(sources))

		query := fmt.Sprintf(`
			SELECT a.v AS Src, b.v AS Dst, uniqExact(a.article_id) AS N
			FROM (
				SELECT article_id, arrayJoin(value) AS v
				FROM aer_gold.article_metadata FINAL WHERE %s
			) a
			INNER JOIN (
				SELECT article_id, arrayJoin(value) AS v
				FROM aer_gold.article_metadata FINAL WHERE %s
			) b ON a.article_id = b.article_id
			GROUP BY Src, Dst
			ORDER BY N DESC, Src ASC, Dst ASC
			LIMIT %d
		`, aWhere, bWhere, topN)

		var rows []sankeyPairRow
		if err := s.conn.Select(ctx, &rows, query, sa.Args...); err != nil {
			slog.Error("Failed to query sankey pair", "error", err, "from", f1, "to", f2)
			return out, err
		}
		for _, r := range rows {
			srcID := addNode(i, f1, r.Src)
			dstID := addNode(i+1, f2, r.Dst)
			out.Links = append(out.Links, SankeyLink{Source: srcID, Target: dstID, Value: int64(r.N)}) //nolint:gosec // bounded by 365-day TTL
		}
	}
	return out, nil
}
