package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// SilverAggregationKind enumerates the six aggregation types backed by
// `aer_silver.documents` (Phase 103b). The handler validates the path
// parameter against this set; the storage layer dispatches the correct
// query shape (distribution / heatmap / correlation).
type SilverAggregationKind string

const (
	SilverAggCleanedTextLength         SilverAggregationKind = "cleaned_text_length"
	SilverAggWordCount                 SilverAggregationKind = "word_count"
	SilverAggRawEntityCount            SilverAggregationKind = "raw_entity_count"
	SilverAggCleanedTextLengthByHour   SilverAggregationKind = "cleaned_text_length_by_hour"
	SilverAggWordCountBySource         SilverAggregationKind = "word_count_by_source"
	SilverAggCleanedTextLengthVsWords  SilverAggregationKind = "cleaned_text_length_vs_word_count"
)

// SilverCorrelationResult is the payload for the
// `cleaned_text_length_vs_word_count` aggregation type. The matrix is 2x2
// because the projection table currently exposes exactly two numeric
// fields suitable for correlation; this is sized for that pair and
// trivially extends as more fields land.
type SilverCorrelationResult struct {
	Fields      []string
	Matrix      [][]*float64
	SampleCount int64
}

// silverFieldExpr maps a distribution-bearing aggregation type onto the
// projection column it queries. Unsupported types return ("", false).
func silverFieldExpr(kind SilverAggregationKind) (string, bool) {
	switch kind {
	case SilverAggCleanedTextLength:
		return "cleaned_text_length", true
	case SilverAggWordCount:
		return "word_count", true
	case SilverAggRawEntityCount:
		return "raw_entity_count", true
	}
	return "", false
}

// GetSilverDistribution computes histogram bins + quantile summary over a
// single projection field for one source, mirroring the Gold-side
// GetMetricDistribution shape so the BFF / frontend handle both with one
// rendering path.
func (s *ClickHouseStorage) GetSilverDistribution(
	ctx context.Context,
	field string,
	source string,
	start, end time.Time,
	bins int,
) (DistributionResult, error) {
	if bins < 1 {
		bins = 1
	}
	if bins > 200 {
		bins = 200
	}

	var summaryRows []struct {
		Count  uint64
		Min    float64
		Max    float64
		Mean   float64
		Median float64
		P05    float64
		P25    float64
		P75    float64
		P95    float64
	}

	args := []any{start, end, source}
	// Cast to Float64 in SQL — the projection columns are UInt32 and the
	// Go scanner refuses to widen UInt32 → *float64 implicitly.
	summaryQuery := fmt.Sprintf(`
		SELECT
			count() AS Count,
			toFloat64(ifNull(min(%[1]s), 0)) AS Min,
			toFloat64(ifNull(max(%[1]s), 0)) AS Max,
			ifNull(avg(%[1]s), 0) AS Mean,
			toFloat64(ifNull(quantileExact(0.5)(%[1]s), 0)) AS Median,
			toFloat64(ifNull(quantileExact(0.05)(%[1]s), 0)) AS P05,
			toFloat64(ifNull(quantileExact(0.25)(%[1]s), 0)) AS P25,
			toFloat64(ifNull(quantileExact(0.75)(%[1]s), 0)) AS P75,
			toFloat64(ifNull(quantileExact(0.95)(%[1]s), 0)) AS P95
		FROM aer_silver.documents FINAL
		WHERE timestamp >= $1 AND timestamp < $2 AND source = $3
	`, field)

	if err := s.conn.Select(ctx, &summaryRows, summaryQuery, args...); err != nil {
		slog.Error("Failed to query silver distribution summary", "error", err, "field", field)
		return DistributionResult{}, err
	}

	result := DistributionResult{}
	if len(summaryRows) == 0 || summaryRows[0].Count == 0 {
		return result, nil
	}
	r := summaryRows[0]
	result.Summary = DistributionSummary{
		Count:  int64(r.Count), //nolint:gosec // bounded by row limit
		Min:    r.Min,
		Max:    r.Max,
		Mean:   r.Mean,
		Median: r.Median,
		P05:    r.P05,
		P25:    r.P25,
		P75:    r.P75,
		P95:    r.P95,
	}

	span := r.Max - r.Min
	if span <= 0 {
		result.Bins = []DistributionBin{{
			Lower: r.Min,
			Upper: r.Min,
			Count: int64(r.Count), //nolint:gosec
		}}
		return result, nil
	}

	binWidth := span / float64(bins)
	histQuery := fmt.Sprintf(`
		SELECT
			least(toUInt32(floor((toFloat64(%[1]s) - %[2]f) / %[3]f)), toUInt32(%[4]d)) AS Bucket,
			count() AS Cnt
		FROM aer_silver.documents FINAL
		WHERE timestamp >= $1 AND timestamp < $2 AND source = $3
		GROUP BY Bucket
		ORDER BY Bucket
	`, field, r.Min, binWidth, bins-1)

	var histRows []struct {
		Bucket uint32
		Cnt    uint64
	}
	if err := s.conn.Select(ctx, &histRows, histQuery, args...); err != nil {
		slog.Error("Failed to query silver distribution histogram", "error", err, "field", field)
		return DistributionResult{}, err
	}

	binCounts := make([]int64, bins)
	for _, hr := range histRows {
		idx := max(int(hr.Bucket), 0)
		if idx >= bins {
			idx = bins - 1
		}
		binCounts[idx] += int64(hr.Cnt) //nolint:gosec
	}

	result.Bins = make([]DistributionBin, bins)
	for i := 0; i < bins; i++ {
		result.Bins[i] = DistributionBin{
			Lower: r.Min + float64(i)*binWidth,
			Upper: r.Min + float64(i+1)*binWidth,
			Count: binCounts[i],
		}
	}
	return result, nil
}

// GetSilverHeatmap returns a 2D aggregation. The two supported aggregation
// types fix the (xDim, yDim, valueField) triple, so the storage layer takes
// the kind directly rather than four separate dimension parameters.
func (s *ClickHouseStorage) GetSilverHeatmap(
	ctx context.Context,
	kind SilverAggregationKind,
	source string,
	start, end time.Time,
) ([]HeatmapCell, string, string, error) {
	var xExpr, yExpr, valueField, xDim, yDim string
	switch kind {
	case SilverAggCleanedTextLengthByHour:
		// dayOfWeek x hour heatmap of cleaned-text length, mirroring the
		// Gold heatmap dimensions so the frontend reuses the same axis
		// renderer.
		xExpr = "toString(toDayOfWeek(timestamp))"
		yExpr = "toString(toHour(timestamp))"
		valueField = "cleaned_text_length"
		xDim, yDim = "dayOfWeek", "hour"
	case SilverAggWordCountBySource:
		// source x dayOfWeek heatmap. Filtering to a single source reduces
		// this to one column; whole-probe queries (Phase 111 source toggle)
		// will yield meaningful x-axis variation.
		xExpr = "source"
		yExpr = "toString(toDayOfWeek(timestamp))"
		valueField = "word_count"
		xDim, yDim = "source", "dayOfWeek"
	default:
		return nil, "", "", fmt.Errorf("unsupported silver heatmap kind: %s", kind)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS X,
			%s AS Y,
			toFloat64(avg(%s)) AS Value,
			count() AS Count
		FROM aer_silver.documents FINAL
		WHERE timestamp >= $1 AND timestamp < $2 AND source = $3
		GROUP BY X, Y
		ORDER BY X, Y
		LIMIT %d
	`, xExpr, yExpr, valueField, s.rowLimit)

	var rows []struct {
		X     string
		Y     string
		Value float64
		Count uint64
	}
	if err := s.conn.Select(ctx, &rows, query, start, end, source); err != nil {
		slog.Error("Failed to query silver heatmap", "error", err, "kind", kind)
		return nil, "", "", err
	}

	cells := make([]HeatmapCell, len(rows))
	for i, r := range rows {
		cells[i] = HeatmapCell{
			X:     r.X,
			Y:     r.Y,
			Value: r.Value,
			Count: int64(r.Count), //nolint:gosec
		}
	}
	return cells, xDim, yDim, nil
}

// GetSilverCorrelation computes Pearson correlation across the two
// projection fields named by the cleaned_text_length_vs_word_count
// aggregation type. ClickHouse's `corr()` is exact; on insufficient
// samples (n<2 or zero variance) the cell is returned as null so the
// frontend can render a "n/a" state without inventing a number.
func (s *ClickHouseStorage) GetSilverCorrelation(
	ctx context.Context,
	source string,
	start, end time.Time,
) (SilverCorrelationResult, error) {
	var rows []struct {
		Sample uint64
		Corr   float64
		HasCorr uint8
	}
	query := `
		SELECT
			count() AS Sample,
			ifNull(corr(cleaned_text_length, word_count), 0) AS Corr,
			toUInt8(corr(cleaned_text_length, word_count) IS NOT NULL) AS HasCorr
		FROM aer_silver.documents FINAL
		WHERE timestamp >= $1 AND timestamp < $2 AND source = $3
	`
	if err := s.conn.Select(ctx, &rows, query, start, end, source); err != nil {
		slog.Error("Failed to query silver correlation", "error", err)
		return SilverCorrelationResult{}, err
	}

	fields := []string{"cleaned_text_length", "word_count"}
	res := SilverCorrelationResult{Fields: fields}

	one := 1.0
	res.Matrix = [][]*float64{
		{&one, nil},
		{nil, &one},
	}

	if len(rows) == 0 || rows[0].Sample < 2 || rows[0].HasCorr == 0 {
		// Diagonals stay 1.0; off-diagonals stay nil.
		return res, nil
	}
	c := rows[0].Corr
	res.Matrix[0][1] = &c
	res.Matrix[1][0] = &c
	res.SampleCount = int64(rows[0].Sample) //nolint:gosec
	return res, nil
}

// IsSilverDistributionKind reports whether the kind queries a single
// projection field with a histogram + quantile shape.
func IsSilverDistributionKind(kind SilverAggregationKind) bool {
	_, ok := silverFieldExpr(kind)
	return ok
}

// IsSilverHeatmapKind reports whether the kind maps to a 2D bucket query.
func IsSilverHeatmapKind(kind SilverAggregationKind) bool {
	return kind == SilverAggCleanedTextLengthByHour || kind == SilverAggWordCountBySource
}

// IsSilverCorrelationKind reports whether the kind requests a pairwise
// correlation across projection fields.
func IsSilverCorrelationKind(kind SilverAggregationKind) bool {
	return kind == SilverAggCleanedTextLengthVsWords
}
