package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// HeatmapDimension is the enum of supported axis dimensions for the
// /metrics/{metric}/heatmap endpoint. Mirrors the OpenAPI enum.
type HeatmapDimension string

const (
	HeatmapDimDayOfWeek   HeatmapDimension = "dayOfWeek"
	HeatmapDimHour        HeatmapDimension = "hour"
	HeatmapDimSource      HeatmapDimension = "source"
	HeatmapDimEntityLabel HeatmapDimension = "entityLabel"
	HeatmapDimLanguage    HeatmapDimension = "language"
)

// HeatmapCell is one (x, y) bucket of a 2D metric aggregation. X and Y are
// string-encoded so int (hour, dayOfWeek) and category (source, label,
// language) share a single response shape.
type HeatmapCell struct {
	X     string
	Y     string
	Value float64
	Count int64
}

// dimensionExpr returns the ClickHouse SELECT expression for a dimension and
// reports whether the dimension requires a JOIN against another Gold table.
// `metricsAlias` is the alias of the aer_gold.metrics row in the assembled
// query (typically "m").
func dimensionExpr(dim HeatmapDimension, metricsAlias string) (selectExpr, joinKind string, ok bool) {
	switch dim {
	case HeatmapDimDayOfWeek:
		return fmt.Sprintf("toString(toDayOfWeek(%s.timestamp))", metricsAlias), "", true
	case HeatmapDimHour:
		return fmt.Sprintf("toString(toHour(%s.timestamp))", metricsAlias), "", true
	case HeatmapDimSource:
		return fmt.Sprintf("%s.source", metricsAlias), "", true
	case HeatmapDimEntityLabel:
		return "e.entity_label", "entities", true
	case HeatmapDimLanguage:
		return "ld.detected_language", "languages", true
	}
	return "", "", false
}

// GetMetricHeatmap computes a 2D aggregation of a metric over a time window,
// bucketed by xDim and yDim. dayOfWeek/hour bin on the metric timestamp;
// source groups by source name; entityLabel and language join against
// aer_gold.entities / aer_gold.language_detections on (article_id, source).
func (s *ClickHouseStorage) GetMetricHeatmap(
	ctx context.Context,
	metricName string,
	sources []string,
	xDim, yDim HeatmapDimension,
	start, end time.Time,
) ([]HeatmapCell, error) {
	xExpr, xJoin, xOK := dimensionExpr(xDim, "m")
	yExpr, yJoin, yOK := dimensionExpr(yDim, "m")
	if !xOK || !yOK {
		return nil, fmt.Errorf("unsupported heatmap dimension")
	}

	// Build the JOIN clauses on demand; deduplicate so requesting
	// (entityLabel, entityLabel) doesn't emit two e-aliases.
	joins := []string{}
	joinedTables := map[string]bool{}
	for _, kind := range []string{xJoin, yJoin} {
		if kind == "" || joinedTables[kind] {
			continue
		}
		switch kind {
		case "entities":
			joins = append(joins, "INNER JOIN aer_gold.entities AS e ON e.article_id = m.article_id AND e.source = m.source")
		case "languages":
			joins = append(joins, "INNER JOIN aer_gold.language_detections AS ld ON ld.article_id = m.article_id AND ld.source = m.source AND ld.rank = 1")
		}
		joinedTables[kind] = true
	}

	args := []any{start, end, metricName}
	clauses := []string{
		"m.timestamp >= $1",
		"m.timestamp < $2",
		"m.metric_name = $3",
	}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+4)
			args = append(args, src)
		}
		clauses = append(clauses, fmt.Sprintf("m.source IN (%s)", strings.Join(placeholders, ", ")))
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS X,
			%s AS Y,
			avg(m.value) AS Value,
			count() AS Count
		FROM aer_gold.metrics AS m
		%s
		WHERE %s
		GROUP BY X, Y
		ORDER BY X, Y
		LIMIT %d
	`,
		xExpr, yExpr,
		strings.Join(joins, "\n"),
		strings.Join(clauses, " AND "),
		s.rowLimit,
	)

	var rows []struct {
		X     string
		Y     string
		Value float64
		Count uint64
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query metric heatmap", "error", err, "metric", metricName, "x", xDim, "y", yDim)
		return nil, err
	}

	cells := make([]HeatmapCell, len(rows))
	for i, r := range rows {
		cells[i] = HeatmapCell{
			X:     r.X,
			Y:     r.Y,
			Value: r.Value,
			Count: int64(r.Count), //nolint:gosec // bounded by row limit
		}
	}
	return cells, nil
}
