package handler

// Tests for the Phase-131 visual-channel surfaces: GET /metrics/scatter
// (paired-metric scatter) and the GET /metrics ?includeStddev ±1\u03c3 band.
// Split from metrics_handler_test.go for cohesion (Phase 142).

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func TestGetMetrics_IncludeStddevAttachesSpread(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		metricsSpread: []storage.MetricRow{
			{TS: ts, Value: 0.5, Source: "tagesschau", MetricName: "sentiment_score_sentiws", Count: 4, Stddev: 0.25},
		},
		// metrics (the non-spread path) deliberately left nil so a wrong
		// dispatch would surface as an empty response.
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	yes := true

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, IncludeStddev: &yes},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics200JSONResponse, got %T", resp)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 point from the spread path, got %d", len(got.Data))
	}
	if got.Data[0].Stddev == nil || *got.Data[0].Stddev != 0.25 {
		t.Errorf("expected stddev 0.25, got %v", got.Data[0].Stddev)
	}
}

func TestGetMetrics_NoStddevByDefault(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		metrics: []storage.MetricRow{
			{TS: ts, Value: 0.5, Source: "tagesschau", MetricName: "word_count", Count: 4, Stddev: 0.25},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := resp.(GetMetrics200JSONResponse)
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 point, got %d", len(got.Data))
	}
	// Even though the storage row carries a Stddev, the default path must
	// not surface it (omitempty pointer stays nil).
	if got.Data[0].Stddev != nil {
		t.Errorf("expected nil stddev on the default path, got %v", *got.Data[0].Stddev)
	}
}

func TestGetMetricScatter_MapsPointsAndChannels(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	size := 3.0
	store := &mockStore{
		scatter: storage.ScatterResult{
			Truncated: true,
			Points: []storage.ScatterPoint{
				{ArticleID: "a1", Source: "tagesschau", TS: ts, X: 100, Y: 0.5, Size: &size},
			},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	sourceIds := "tagesschau"
	sizeMetric := "entity_count"

	resp, err := s.GetMetricScatter(context.Background(), GetMetricScatterRequestObject{
		Params: GetMetricScatterParams{
			XMetric:    "word_count",
			YMetric:    "sentiment_score_sentiws",
			SizeMetric: &sizeMetric,
			SourceIds:  &sourceIds,
			Start:      &start,
			End:        &end,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricScatter200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetricScatter200JSONResponse, got %T", resp)
	}
	if got.XMetric != "word_count" || got.YMetric != "sentiment_score_sentiws" {
		t.Errorf("metric echoes wrong: x=%q y=%q", got.XMetric, got.YMetric)
	}
	if !got.Truncated {
		t.Errorf("expected truncated flag to propagate")
	}
	if len(got.Points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(got.Points))
	}
	if got.Points[0].X != 100 || got.Points[0].Y != 0.5 {
		t.Errorf("point position wrong: %+v", got.Points[0])
	}
	if got.Points[0].Size == nil || *got.Points[0].Size != 3.0 {
		t.Errorf("expected size channel 3.0, got %v", got.Points[0].Size)
	}
	if got.Points[0].ArticleID == nil || *got.Points[0].ArticleID != "a1" {
		t.Errorf("expected articleId a1, got %v", got.Points[0].ArticleID)
	}
	if store.capturedScatterX != "word_count" {
		t.Errorf("storage should receive xMetric word_count, got %q", store.capturedScatterX)
	}
}

func TestGetMetricScatter_RequiresScope(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricScatter(context.Background(), GetMetricScatterRequestObject{
		Params: GetMetricScatterParams{XMetric: "word_count", YMetric: "sentiment_score_sentiws", Start: &start, End: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetMetricScatter400JSONResponse); !ok {
		t.Fatalf("expected 400 when no scope provided, got %T", resp)
	}
}
