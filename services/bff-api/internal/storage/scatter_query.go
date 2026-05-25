package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ScatterPoint is one per-article point in a paired-metric scatter (Phase 131).
// X and Y are the two position metrics (always present); Size and Color are the
// optional visual-channel metrics, nil when the channel is unbound or the
// article lacks that metric.
type ScatterPoint struct {
	ArticleID string
	Source    string
	TS        time.Time
	X         float64
	Y         float64
	Size      *float64
	Color     *float64
}

// ScatterResult bundles the per-article points with a truncation flag set when
// the in-window article set exceeded the requested cap.
type ScatterResult struct {
	Points    []ScatterPoint
	Truncated bool
}

// scatterRow mirrors one pivoted row from ClickHouse. The `ch:` tags pin the
// column aliases so the driver maps Nullable(Float64) onto the *float64
// channel fields.
type scatterRow struct {
	ArticleID string    `ch:"ArticleID"`
	Source    string    `ch:"Source"`
	TS        time.Time `ch:"TS"`
	X         float64   `ch:"X"`
	Y         float64   `ch:"Y"`
	Size      *float64  `ch:"Size"`
	Color     *float64  `ch:"Color"`
}

// GetMetricScatter pivots `aer_gold.metrics` by article so each article becomes
// one (x, y) point with optional size / colour channels. Only articles that
// carry both position metrics in the window contribute a point (the HAVING
// guard). The query orders by article id for determinism and over-fetches by
// one row to detect truncation against `maxPoints`.
//
// avgIf collapses any pre-merge ReplacingMergeTree duplicates (averaging
// identical values is a no-op) so the pivot is robust without a FINAL scan.
func (s *ClickHouseStorage) GetMetricScatter(
	ctx context.Context,
	xMetric, yMetric string,
	sizeMetric, colorMetric *string,
	sources []string,
	start, end time.Time,
	maxPoints int,
) (ScatterResult, error) {
	if maxPoints < 1 {
		maxPoints = 1
	}
	if maxPoints > 10000 {
		maxPoints = 10000
	}

	// Positional args: $1 start, $2 end, then one placeholder per bound
	// metric, then the source list.
	args := []any{start, end}
	xPH := fmt.Sprintf("$%d", len(args)+1)
	args = append(args, xMetric)
	yPH := fmt.Sprintf("$%d", len(args)+1)
	args = append(args, yMetric)

	inList := []string{xPH, yPH}

	sizeCol := "CAST(NULL AS Nullable(Float64)) AS Size"
	if sizeMetric != nil && strings.TrimSpace(*sizeMetric) != "" {
		ph := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, *sizeMetric)
		inList = append(inList, ph)
		sizeCol = fmt.Sprintf(
			"if(countIf(metric_name = %s) > 0, avgIf(value, metric_name = %s), CAST(NULL AS Nullable(Float64))) AS Size",
			ph, ph,
		)
	}

	colorCol := "CAST(NULL AS Nullable(Float64)) AS Color"
	if colorMetric != nil && strings.TrimSpace(*colorMetric) != "" {
		ph := fmt.Sprintf("$%d", len(args)+1)
		args = append(args, *colorMetric)
		inList = append(inList, ph)
		colorCol = fmt.Sprintf(
			"if(countIf(metric_name = %s) > 0, avgIf(value, metric_name = %s), CAST(NULL AS Nullable(Float64))) AS Color",
			ph, ph,
		)
	}

	sourceClause := ""
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			ph := fmt.Sprintf("$%d", len(args)+1)
			args = append(args, src)
			placeholders[i] = ph
		}
		sourceClause = fmt.Sprintf("AND source IN (%s)", strings.Join(placeholders, ", "))
	}

	// Over-fetch by one to detect truncation without a second count query.
	query := fmt.Sprintf(`
		SELECT
			article_id AS ArticleID,
			any(source) AS Source,
			any(timestamp) AS TS,
			avgIf(value, metric_name = %s) AS X,
			avgIf(value, metric_name = %s) AS Y,
			%s,
			%s
		FROM aer_gold.metrics
		WHERE timestamp >= $1 AND timestamp < $2
			AND article_id IS NOT NULL
			AND metric_name IN (%s)
			%s
		GROUP BY article_id
		HAVING countIf(metric_name = %s) > 0 AND countIf(metric_name = %s) > 0
		ORDER BY ArticleID ASC
		LIMIT %d
	`, xPH, yPH, sizeCol, colorCol, strings.Join(inList, ", "), sourceClause, xPH, yPH, maxPoints+1)

	var rows []scatterRow
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query metric scatter from ClickHouse", "error", err, "x", xMetric, "y", yMetric)
		return ScatterResult{}, err
	}

	result := ScatterResult{}
	if len(rows) > maxPoints {
		result.Truncated = true
		rows = rows[:maxPoints]
	}
	result.Points = make([]ScatterPoint, len(rows))
	for i, r := range rows {
		// scatterRow and ScatterPoint share an identical field layout; the
		// only difference is the `ch:` driver tags on the row type.
		result.Points[i] = ScatterPoint(r)
	}
	return result, nil
}
