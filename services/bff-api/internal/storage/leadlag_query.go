package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// LeadLagPoint is the Pearson correlation of the reference and compared
// activity series at one hourly lag. Correlation is nil when too few
// overlapping buckets exist at that lag or either side has zero variance.
type LeadLagPoint struct {
	LagHours    int      `json:"lagHours"`
	Correlation *float64 `json:"correlation"`
}

// LeadLagResult is the cross-correlation of hourly publication activity
// between two probe source sets across a symmetric lag range (Phase 124).
//
// Lag convention: a point at lag τ correlates reference activity at hour h
// with compared activity at hour h+τ. A peak at τ>0 means the compared series
// LAGS the reference (the reference leads); τ<0 means the compared series
// leads the reference.
type LeadLagResult struct {
	ReferenceSources  []string       `json:"referenceSources"`
	ComparedSources   []string       `json:"comparedSources"`
	MaxLagHours       int            `json:"maxLagHours"`
	BucketCountAtZero int64          `json:"bucketCountAtZero"`
	Points            []LeadLagPoint `json:"points"`
	PeakLagHours      *int           `json:"peakLagHours"`
	PeakCorrelation   *float64       `json:"peakCorrelation"`
}

// GetTemporalLeadLag computes the lagged cross-correlation of hourly
// publication activity (distinct article count per hour) between a reference
// and a compared source set, over lags in [-maxLagHours, +maxLagHours].
//
// Publication activity is the temporal-rhythm signal authorised by the
// temporal Level-1 equivalence grant (WP-004 §6.3, Appendix B); the handler
// enforces that gate. The helper itself is pure data. Phase 125 generalises
// this to arbitrary metric series and a fully-parameterised public endpoint —
// build on this helper, do not re-implement it.
func (s *ClickHouseStorage) GetTemporalLeadLag(
	ctx context.Context,
	referenceSources, comparedSources []string,
	start, end time.Time,
	maxLagHours int,
) (LeadLagResult, error) {
	ref, err := s.hourlyActivity(ctx, referenceSources, start, end)
	if err != nil {
		return LeadLagResult{}, err
	}
	cmp, err := s.hourlyActivity(ctx, comparedSources, start, end)
	if err != nil {
		return LeadLagResult{}, err
	}

	result := computeLeadLag(ref, cmp, maxLagHours)
	result.ReferenceSources = referenceSources
	result.ComparedSources = comparedSources
	return result, nil
}

// computeLeadLag is the pure cross-correlation core: for each lag τ in
// [-maxLagHours, +maxLagHours] it correlates reference[h] with compared[h+τ]
// over the hour buckets present on both sides. Both maps are keyed by
// hour-bucket epoch seconds. Extracted from GetTemporalLeadLag so the lag
// arithmetic and peak selection are testable from a synthetic fixture without
// a live ClickHouse.
func computeLeadLag(ref, cmp map[int64]float64, maxLagHours int) LeadLagResult {
	result := LeadLagResult{
		MaxLagHours: maxLagHours,
		Points:      make([]LeadLagPoint, 0, 2*maxLagHours+1),
	}

	const secondsPerHour = int64(3600)
	for lag := -maxLagHours; lag <= maxLagHours; lag++ {
		// Align reference[h] with compared[h + lag].
		offset := int64(lag) * secondsPerHour
		xs := make([]float64, 0, len(ref))
		ys := make([]float64, 0, len(ref))
		for h, rv := range ref {
			if cv, ok := cmp[h+offset]; ok {
				xs = append(xs, rv)
				ys = append(ys, cv)
			}
		}
		corr := pearsonXY(xs, ys)
		if lag == 0 {
			result.BucketCountAtZero = int64(len(xs))
		}
		result.Points = append(result.Points, LeadLagPoint{LagHours: lag, Correlation: corr})
		// Peak = lag of maximum (most positive) correlation — the strongest
		// co-movement of the two publication rhythms.
		if corr != nil && (result.PeakCorrelation == nil || *corr > *result.PeakCorrelation) {
			lagCopy := lag
			corrCopy := *corr
			result.PeakLagHours = &lagCopy
			result.PeakCorrelation = &corrCopy
		}
	}
	return result
}

// hourlyActivity returns a map from hour-bucket epoch seconds to the distinct
// article count published in that hour for the given source set. timestamp on
// aer_gold.metrics is the article published_date, so this is genuine
// publication activity, not ingestion time.
func (s *ClickHouseStorage) hourlyActivity(ctx context.Context, sources []string, start, end time.Time) (map[int64]float64, error) {
	args := []any{start, end}
	where := "timestamp >= $1 AND timestamp < $2"
	if len(sources) > 0 {
		placeholders := make([]string, len(sources))
		for i, src := range sources {
			placeholders[i] = fmt.Sprintf("$%d", len(args)+1)
			args = append(args, src)
		}
		where += fmt.Sprintf(" AND source IN (%s)", strings.Join(placeholders, ", "))
	}
	query := fmt.Sprintf(`
		SELECT toStartOfHour(timestamp) AS Bucket, countDistinct(article_id) AS Activity
		FROM aer_gold.metrics
		WHERE %s
		GROUP BY Bucket
		ORDER BY Bucket
		LIMIT %d
	`, where, s.rowLimit)

	var rows []struct {
		Bucket   time.Time
		Activity uint64
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query hourly activity for lead-lag", "error", err)
		return nil, err
	}
	out := make(map[int64]float64, len(rows))
	for _, r := range rows {
		out[r.Bucket.Unix()] = float64(r.Activity)
	}
	return out, nil
}
