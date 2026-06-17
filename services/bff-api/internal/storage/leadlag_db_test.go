package storage

import (
	"testing"
	"time"
)

// llWindow brackets the seeded publication activity.
var (
	llStart = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	llEnd   = time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
)

// seedActivity inserts one metrics row (one article) at a given hour for a
// source so hourlyActivity counts a distinct article in that hour bucket.
func seedActivity(t *testing.T, ctx hasContext, s *ClickHouseStorage, source string, hour int, articleID string) {
	t.Helper()
	ts := time.Date(2026, 5, 1, hour, 0, 0, 0, time.UTC)
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{{ts, 1.0, source, "word_count", articleID}}); err != nil {
		t.Fatalf("seed activity: %v", err)
	}
}

func TestHourlyActivity_CountsDistinctArticlesPerHour(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Hour 9: two distinct articles. Hour 10: one article.
	seedActivity(t, contextWrap{ctx}, s, "tagesschau", 9, "a1")
	seedActivity(t, contextWrap{ctx}, s, "tagesschau", 9, "a2")
	seedActivity(t, contextWrap{ctx}, s, "tagesschau", 10, "a3")
	// Off-source row, excluded by scope.
	seedActivity(t, contextWrap{ctx}, s, "wikipedia", 9, "w1")

	got, err := s.hourlyActivity(ctx, []string{"tagesschau"}, llStart, llEnd)
	if err != nil {
		t.Fatalf("hourlyActivity: %v", err)
	}
	h9 := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC).Unix()
	h10 := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC).Unix()
	if got[h9] != 2 {
		t.Errorf("hour 9 activity: want 2 distinct articles, got %v", got[h9])
	}
	if got[h10] != 1 {
		t.Errorf("hour 10 activity: want 1, got %v", got[h10])
	}
}

func TestHourlyMetricSeries_MeanPerHour(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Hour 9: two sentiment values 0.2, 0.6 → mean 0.4.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC), 0.2, "tagesschau", "sentiment_score", "a1"},
			{time.Date(2026, 5, 1, 9, 30, 0, 0, time.UTC), 0.6, "tagesschau", "sentiment_score", "a2"},
			{time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC), 0.8, "tagesschau", "sentiment_score", "a3"},
		}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := s.hourlyMetricSeries(ctx, []string{"tagesschau"}, "sentiment_score", llStart, llEnd, nil)
	if err != nil {
		t.Fatalf("hourlyMetricSeries: %v", err)
	}
	h9 := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC).Unix()
	if got[h9] < 0.39 || got[h9] > 0.41 {
		t.Errorf("hour 9 mean: want ~0.4, got %v", got[h9])
	}
}

func TestGetTemporalLeadLag_KnownShiftPeaksAtPositiveLag(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Reference activity: a varying number of articles across hours 6..18.
	// Compared activity is the same shape shifted 2 hours later (lags).
	const shift = 2
	hourCounts := map[int]int{6: 1, 7: 3, 8: 5, 9: 3, 10: 1, 11: 4, 12: 6, 13: 4, 14: 2}
	id := 0
	for h, n := range hourCounts {
		for i := 0; i < n; i++ {
			id++
			seedActivity(t, contextWrap{ctx}, s, "ref", h, "r-"+itoa(id))
		}
		for i := 0; i < n; i++ {
			id++
			seedActivity(t, contextWrap{ctx}, s, "cmp", h+shift, "c-"+itoa(id))
		}
	}

	res, err := s.GetTemporalLeadLag(ctx, []string{"ref"}, []string{"cmp"}, llStart, llEnd, 6)
	if err != nil {
		t.Fatalf("GetTemporalLeadLag: %v", err)
	}
	if len(res.ReferenceSources) != 1 || res.ReferenceSources[0] != "ref" {
		t.Errorf("reference sources echoed wrong: %v", res.ReferenceSources)
	}
	if len(res.Points) != 2*6+1 {
		t.Fatalf("want %d lag points, got %d", 2*6+1, len(res.Points))
	}
	if res.PeakLagHours == nil {
		t.Fatal("expected a defined peak")
	}
	if *res.PeakLagHours != shift {
		t.Errorf("peak lag: want +%d (compared lags reference), got %d", shift, *res.PeakLagHours)
	}
}

func TestGetMetricLeadLag_TwoMetricsSameScope(t *testing.T) {
	s, ctx := setupTestStore(t)

	// A non-monotonic shape so only the lag-0 alignment yields the strongest
	// correlation (a monotonic ramp would tie at corr=1.0 across every lag).
	// metric_x == metric_y per hour, so the in-phase correlation is 1.0 and the
	// peak must land at lag 0.
	shape := []float64{1, 4, 2, 5, 3, 6, 2}
	for i, v := range shape {
		ts := time.Date(2026, 5, 1, 8+i, 0, 0, 0, time.UTC)
		if err := bulkInsert(ctx, s, "aer_gold.metrics",
			[]string{"timestamp", "value", "source", "metric_name", "article_id"},
			[][]any{
				{ts, v, "tagesschau", "metric_x", "ax" + itoa(i)},
				{ts, v, "tagesschau", "metric_y", "ay" + itoa(i)},
			}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	res, err := s.GetMetricLeadLag(ctx, []string{"tagesschau"}, "metric_x", "metric_y", llStart, llEnd, 3, nil)
	if err != nil {
		t.Fatalf("GetMetricLeadLag: %v", err)
	}
	if res.PeakLagHours == nil {
		t.Fatal("expected a defined peak for identical in-phase series")
	}
	if *res.PeakLagHours != 0 {
		t.Errorf("identical in-phase series should peak at lag 0, got %d", *res.PeakLagHours)
	}
	if res.PeakCorrelation == nil || *res.PeakCorrelation < 0.99 {
		t.Errorf("in-phase correlation should be ~1.0 at peak, got %v", res.PeakCorrelation)
	}
}

// itoa avoids pulling strconv just for the seed loops.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
