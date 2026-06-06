package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ParallelCoordRow is one article positioned on N metric axes. Values aligns
// with the requested metric order; an article contributes a row only when it
// carries ALL requested metrics (a complete polyline).
type ParallelCoordRow struct {
	ArticleID string    `ch:"ArticleID"`
	Source    string    `ch:"Source"`
	Values    []float64 `ch:"Values"`
}

// ParallelCoordResult is the per-article N-metric matrix backing the parallel-
// coordinates cell (Phase 125), with a disclosed truncation flag.
type ParallelCoordResult struct {
	Metrics   []string
	Rows      []ParallelCoordRow
	Truncated bool
}

// GetParallelCoords pivots aer_gold.metrics per article into an N-metric vector
// (one `avgIf` column per metric, in request order), keeping only articles that
// carry every requested metric. It generalises the scatter per-article pivot to
// N dimensions; the metrics read omits FINAL (avg tolerates transient skew — the
// established convention). Capped at maxPoints (+1 to detect truncation).
func (s *ClickHouseStorage) GetParallelCoords(
	ctx context.Context,
	metrics []string,
	sources []string,
	start, end time.Time,
	maxPoints int,
	metadataFilter *MetadataFilter,
) (ParallelCoordResult, error) {
	out := ParallelCoordResult{Metrics: metrics, Rows: []ParallelCoordRow{}}
	if len(metrics) < 2 || len(sources) == 0 {
		return out, nil
	}
	if maxPoints < 1 {
		maxPoints = 3000
	}
	if maxPoints > 10000 {
		maxPoints = 10000
	}

	sa := newScopeArgs()
	startP := sa.ph(start)
	endP := sa.ph(end)
	srcIn := sa.srcIn(sources)
	avgParts := make([]string, len(metrics))
	for i, m := range metrics {
		avgParts[i] = fmt.Sprintf("avgIf(value, metric_name = %s)", sa.ph(m))
	}
	inPlaceholders := make([]string, len(metrics))
	for i, m := range metrics {
		inPlaceholders[i] = sa.ph(m)
	}
	havingParts := make([]string, len(metrics))
	for i, m := range metrics {
		havingParts[i] = fmt.Sprintf("countIf(metric_name = %s) > 0", sa.ph(m))
	}
	// Faceting (Phase 125a): restrict to facet-matching articles.
	facetClause := sa.metadataFilterClause(metadataFilter, start, end, sources)

	query := fmt.Sprintf(`
		SELECT article_id AS ArticleID,
		       any(source) AS Source,
		       [%s] AS Values
		FROM aer_gold.metrics
		WHERE timestamp >= %s AND timestamp < %s
		  AND source IN (%s)
		  AND article_id IS NOT NULL AND article_id != ''
		  AND metric_name IN (%s)%s
		GROUP BY article_id
		HAVING %s
		ORDER BY ArticleID
		LIMIT %d
	`,
		strings.Join(avgParts, ", "),
		startP, endP,
		srcIn,
		strings.Join(inPlaceholders, ", "),
		facetClause,
		strings.Join(havingParts, " AND "),
		maxPoints+1,
	)

	var rows []ParallelCoordRow
	if err := s.conn.Select(ctx, &rows, query, sa.Args...); err != nil {
		slog.Error("Failed to query parallel coordinates", "error", err)
		return out, err
	}
	if len(rows) > maxPoints {
		out.Truncated = true
		rows = rows[:maxPoints]
	}
	out.Rows = rows
	return out, nil
}
