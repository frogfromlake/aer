package storage

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
)

// CorrelationResult is the NxN Pearson correlation matrix for the requested
// metrics, computed from per-bucket means within the window.
//
// Matrix[i][j] is the correlation of Metrics[i] and Metrics[j]. A nil pointer
// represents an undefined cell (e.g., zero variance or no overlapping
// buckets).
type CorrelationResult struct {
	Metrics     []string
	Matrix      [][]*float64
	BucketCount int64
	Resolution  string
}

// GetMetricCorrelation computes pairwise Pearson correlation across the
// requested metric names within the given window, restricted to the source
// set. Bucketing is fixed at 5-minute (toStartOfFiveMinute), matching the
// /metrics endpoint's default resolution.
func (s *ClickHouseStorage) GetMetricCorrelation(
	ctx context.Context,
	metricNames []string,
	sources []string,
	start, end time.Time,
) (CorrelationResult, error) {
	if len(metricNames) < 2 {
		return CorrelationResult{}, fmt.Errorf("need at least 2 metrics for correlation")
	}

	args := []any{start, end}
	clauses := []string{
		"timestamp >= $1",
		"timestamp < $2",
	}

	metricPlaceholders := make([]string, len(metricNames))
	for i, m := range metricNames {
		metricPlaceholders[i] = fmt.Sprintf("$%d", len(args)+1)
		args = append(args, m)
	}
	clauses = append(clauses, fmt.Sprintf("metric_name IN (%s)", strings.Join(metricPlaceholders, ", ")))

	if len(sources) > 0 {
		srcPlaceholders := make([]string, len(sources))
		for i, src := range sources {
			srcPlaceholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, src)
		}
		clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(srcPlaceholders, ", ")))
	}

	// Pivot via groupArrayIf: for each 5-min bucket, compute the per-metric
	// mean. Then per pair compute corr() over the buckets where BOTH metrics
	// have a non-null mean.
	bucketQuery := fmt.Sprintf(`
		SELECT
			toStartOfFiveMinute(timestamp) AS Bucket,
			metric_name AS MetricName,
			avg(value) AS MeanValue
		FROM aer_gold.metrics
		WHERE %s
		GROUP BY Bucket, MetricName
		ORDER BY Bucket, MetricName
		LIMIT %d
	`, strings.Join(clauses, " AND "), s.rowLimit)

	var rows []struct {
		Bucket    time.Time
		MetricName string
		MeanValue  float64
	}
	if err := s.conn.Select(ctx, &rows, bucketQuery, args...); err != nil {
		slog.Error("Failed to query correlation buckets", "error", err)
		return CorrelationResult{}, err
	}

	// Pivot in-process: bucket -> metric -> mean.
	bucketIndex := map[time.Time]int{}
	buckets := []map[string]float64{}
	for _, r := range rows {
		idx, ok := bucketIndex[r.Bucket]
		if !ok {
			idx = len(buckets)
			bucketIndex[r.Bucket] = idx
			buckets = append(buckets, map[string]float64{})
		}
		buckets[idx][r.MetricName] = r.MeanValue
	}

	matrix := make([][]*float64, len(metricNames))
	for i := range matrix {
		matrix[i] = make([]*float64, len(metricNames))
	}

	for i, mi := range metricNames {
		for j, mj := range metricNames {
			matrix[i][j] = pearson(buckets, mi, mj)
		}
	}

	return CorrelationResult{
		Metrics:     metricNames,
		Matrix:      matrix,
		BucketCount: int64(len(buckets)),
		Resolution:  "5m",
	}, nil
}

// pearson computes the Pearson correlation between two metric series across
// shared buckets. Returns nil when fewer than two paired samples exist or
// when either side has zero variance.
func pearson(buckets []map[string]float64, a, b string) *float64 {
	xs := make([]float64, 0, len(buckets))
	ys := make([]float64, 0, len(buckets))
	for _, m := range buckets {
		xv, xOK := m[a]
		yv, yOK := m[b]
		if !xOK || !yOK {
			continue
		}
		xs = append(xs, xv)
		ys = append(ys, yv)
	}
	n := len(xs)
	if n < 2 {
		return nil
	}
	var sumX, sumY float64
	for i := range n {
		sumX += xs[i]
		sumY += ys[i]
	}
	meanX := sumX / float64(n)
	meanY := sumY / float64(n)
	var num, denX, denY float64
	for i := range n {
		dx := xs[i] - meanX
		dy := ys[i] - meanY
		num += dx * dy
		denX += dx * dx
		denY += dy * dy
	}
	if denX == 0 || denY == 0 {
		return nil
	}
	r := num / (math.Sqrt(denX) * math.Sqrt(denY))
	return &r
}
