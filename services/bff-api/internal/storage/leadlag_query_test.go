package storage

import (
	"math"
	"testing"
)

// TestComputeLeadLag_DetectsKnownPhaseShift is the Probe-0×Probe-1 fixture the
// Phase-124 lead-lag query path is exercised against. The compared series is
// the reference publication rhythm shifted later by a known 3-hour offset, so
// the cross-correlation peak must land at lag +3 (compared LAGS reference).
func TestComputeLeadLag_DetectsKnownPhaseShift(t *testing.T) {
	const shift = 3
	sig := func(h int) float64 { return math.Sin(2*math.Pi*float64(h)/24) + 2 }

	ref := map[int64]float64{}
	cmp := map[int64]float64{}
	for h := 0; h < 240; h++ { // 10 days of hourly buckets
		ref[int64(h)*3600] = sig(h)
		cmp[int64(h)*3600] = sig(h - shift) // compared lags reference by `shift` hours
	}

	res := computeLeadLag(ref, cmp, 12)

	if len(res.Points) != 2*12+1 {
		t.Fatalf("expected %d lag points, got %d", 2*12+1, len(res.Points))
	}
	if res.BucketCountAtZero == 0 {
		t.Fatal("expected non-zero overlapping buckets at lag 0")
	}
	if res.PeakLagHours == nil || res.PeakCorrelation == nil {
		t.Fatal("expected a defined peak for a strongly correlated fixture")
	}
	if *res.PeakLagHours != shift {
		t.Errorf("expected peak lag at +%d (compared lags reference), got %d", shift, *res.PeakLagHours)
	}
	if *res.PeakCorrelation < 0.99 {
		t.Errorf("expected near-perfect peak correlation for a pure phase shift, got %f", *res.PeakCorrelation)
	}
}

// TestComputeLeadLag_NoOverlapYieldsNilCorrelations guards the degenerate case:
// disjoint hour buckets must produce nil correlations and no peak, never a
// panic or a spurious peak.
func TestComputeLeadLag_NoOverlapYieldsNilCorrelations(t *testing.T) {
	ref := map[int64]float64{0: 1, 3600: 2, 7200: 3}
	cmp := map[int64]float64{1_000_000: 1, 1_003_600: 2} // far outside ±maxLag of ref

	res := computeLeadLag(ref, cmp, 6)

	if res.PeakLagHours != nil || res.PeakCorrelation != nil {
		t.Errorf("expected no peak for disjoint series, got lag=%v corr=%v", res.PeakLagHours, res.PeakCorrelation)
	}
	for _, p := range res.Points {
		if p.Correlation != nil {
			t.Errorf("expected nil correlation at lag %d for disjoint series, got %f", p.LagHours, *p.Correlation)
		}
	}
}
