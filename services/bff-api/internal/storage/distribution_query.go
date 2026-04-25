package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// DistributionBin is one histogram bucket: [Lower, Upper) (inclusive on the
// last bin). Count is the number of source rows whose value fell in the range.
type DistributionBin struct {
	Lower float64
	Upper float64
	Count int64
}

// DistributionSummary holds the quantile / extrema summary computed alongside
// the histogram so the frontend can render histogram, density, ridgeline, or
// violin without a second round-trip.
type DistributionSummary struct {
	Count  int64
	Min    float64
	Max    float64
	Mean   float64
	Median float64
	P05    float64
	P25    float64
	P75    float64
	P95    float64
}

// DistributionResult bundles the histogram + summary returned by
// GetMetricDistribution.
type DistributionResult struct {
	Bins    []DistributionBin
	Summary DistributionSummary
}

// GetMetricDistribution computes a histogram and quantile summary of a
// metric over a time window restricted to the provided source set. An empty
// source set returns an empty distribution (zero rows, zero bins) — the
// handler is responsible for resolving probe / source scope into sources.
//
// Histogram bin edges are derived from the observed (min, max) of the
// in-window sample, which keeps bins stable for skewed distributions
// (e.g. sentiment_score in [-1, 1]) without assuming a fixed range.
func (s *ClickHouseStorage) GetMetricDistribution(
	ctx context.Context,
	metricName string,
	sources []string,
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

	baseWhere, args := buildScopeWhere(metricName, sources, start, end)
	// Aggregates over an empty set: count()==0 and the floats default to 0.
	// We short-circuit on Count==0 below, so the zero summary is never
	// surfaced as a real bin set.
	summaryQuery := fmt.Sprintf(`
		SELECT
			count() AS Count,
			ifNull(min(value), 0) AS Min,
			ifNull(max(value), 0) AS Max,
			ifNull(avg(value), 0) AS Mean,
			ifNull(quantileExact(0.5)(value), 0) AS Median,
			ifNull(quantileExact(0.05)(value), 0) AS P05,
			ifNull(quantileExact(0.25)(value), 0) AS P25,
			ifNull(quantileExact(0.75)(value), 0) AS P75,
			ifNull(quantileExact(0.95)(value), 0) AS P95
		FROM aer_gold.metrics
		WHERE %s
	`, baseWhere)

	if err := s.conn.Select(ctx, &summaryRows, summaryQuery, args...); err != nil {
		slog.Error("Failed to query metric distribution summary", "error", err, "metric", metricName)
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
		// Degenerate case: all values identical. Emit a single bin centered
		// on the value so the frontend can still render a flat histogram.
		result.Bins = []DistributionBin{{
			Lower: r.Min,
			Upper: r.Min,
			Count: int64(r.Count), //nolint:gosec // bounded by row limit
		}}
		return result, nil
	}

	binWidth := span / float64(bins)
	histQuery := fmt.Sprintf(`
		SELECT
			least(toUInt32(floor((value - %f) / %f)), toUInt32(%d)) AS Bucket,
			count() AS Cnt
		FROM aer_gold.metrics
		WHERE %s
		GROUP BY Bucket
		ORDER BY Bucket
	`, r.Min, binWidth, bins-1, baseWhere)

	var histRows []struct {
		Bucket uint32
		Cnt    uint64
	}
	if err := s.conn.Select(ctx, &histRows, histQuery, args...); err != nil {
		slog.Error("Failed to query metric distribution histogram", "error", err, "metric", metricName)
		return DistributionResult{}, err
	}

	binCounts := make([]int64, bins)
	for _, hr := range histRows {
		idx := int(hr.Bucket)
		idx = max(idx, 0)
		if idx >= bins {
			idx = bins - 1
		}
		binCounts[idx] += int64(hr.Cnt) //nolint:gosec // bounded by row limit
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

// buildScopeWhere assembles the base WHERE clause shared across distribution,
// heatmap, and correlation queries: time window, metric name, and an optional
// source IN (...) predicate. Caller-supplied source slice is the resolved
// scope (probe sources or single source).
func buildScopeWhere(metricName string, sources []string, start, end time.Time) (string, []any) {
	args := []any{start, end, metricName}
	clauses := []string{
		"timestamp >= $1",
		"timestamp < $2",
		"metric_name = $3",
	}
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", i+4)
			args = append(args, src)
		}
		clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ", ")))
	}
	return strings.Join(clauses, " AND "), args
}
