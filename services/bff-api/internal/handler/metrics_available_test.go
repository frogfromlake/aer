package handler

// Tests for GET /metrics/available — the metric-catalogue endpoint
// (names, validation status, min-meaningful-resolution, equivalence
// metadata, alias filtering). Split from metrics_handler_test.go for
// cohesion (Phase 142 test-code health).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func TestGetMetricsAvailable_WindowOptional(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"no params → whole dataset", "", http.StatusOK},
		{"only startDate → open-ended", "?startDate=2025-01-01T00:00:00Z", http.StatusOK},
		{"only endDate → open-ended", "?endDate=2025-01-02T00:00:00Z", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics/available"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

func TestGetMetricsAvailable_ReturnsNames(t *testing.T) {
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "entity_count", ValidationStatus: "unvalidated"},
			{MetricName: "sentiment_score_sentiws", ValidationStatus: "validated"},
			{MetricName: "word_count", ValidationStatus: "expired"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetricsAvailable200JSONResponse, got %T", resp)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(got))
	}
	if got[0].MetricName != "entity_count" {
		t.Errorf("expected first metric entity_count, got %q", got[0].MetricName)
	}
	if got[0].ValidationStatus != AvailableMetricValidationStatusUnvalidated {
		t.Errorf("expected first status unvalidated, got %q", got[0].ValidationStatus)
	}
	if got[1].ValidationStatus != AvailableMetricValidationStatusValidated {
		t.Errorf("expected second status validated, got %q", got[1].ValidationStatus)
	}
	if got[2].ValidationStatus != AvailableMetricValidationStatusExpired {
		t.Errorf("expected third status expired, got %q", got[2].ValidationStatus)
	}
}

func TestGetMetricsAvailable_IncludesMinMeaningfulResolution(t *testing.T) {
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "word_count", ValidationStatus: "unvalidated"},
			{MetricName: "unmapped_metric", ValidationStatus: "unvalidated"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if got[0].MinMeaningfulResolution == nil || *got[0].MinMeaningfulResolution != AvailableMetricMinMeaningfulResolutionHourly {
		t.Errorf("expected word_count minMeaningfulResolution=hourly, got %v", got[0].MinMeaningfulResolution)
	}
	if got[1].MinMeaningfulResolution != nil {
		t.Errorf("expected unmapped_metric minMeaningfulResolution=nil, got %v", got[1].MinMeaningfulResolution)
	}
}

func TestGetMetricsAvailable_IncludesEquivalenceMetadata(t *testing.T) {
	etic := "evaluative_polarity"
	equivLevel := "deviation"
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "sentiment_score_sentiws", ValidationStatus: "unvalidated", EticConstruct: &etic, EquivalenceLevel: &equivLevel},
			{MetricName: "word_count", ValidationStatus: "unvalidated"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(got))
	}
	if got[0].EticConstruct == nil || *got[0].EticConstruct != "evaluative_polarity" {
		t.Errorf("expected eticConstruct=evaluative_polarity for first metric")
	}
	if got[0].EquivalenceLevel == nil || *got[0].EquivalenceLevel != Deviation {
		t.Errorf("expected equivalenceLevel=deviation for first metric")
	}
	if got[1].EticConstruct != nil {
		t.Errorf("expected nil eticConstruct for word_count, got %v", *got[1].EticConstruct)
	}
	if got[1].EquivalenceLevel != nil {
		t.Errorf("expected nil equivalenceLevel for word_count, got %v", *got[1].EquivalenceLevel)
	}
}

// TestGetMetricsAvailable_FiltersAliasKeys verifies the Phase 121b
// forward-looking guard: rows whose metric_name is a key of
// metric_aliases.go::metricNameAliases (currently `sentiment_score`) are
// dropped from the /metrics/available response, since the canonical name
// (`sentiment_score_sentiws`) already appears in the same response and the
// legacy entry would only ever surface as a duplicate in MetricSwitcher.
func TestGetMetricsAvailable_FiltersAliasKeys(t *testing.T) {
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "sentiment_score", ValidationStatus: "unvalidated"},
			{MetricName: "sentiment_score_sentiws", ValidationStatus: "unvalidated"},
			{MetricName: "word_count", ValidationStatus: "unvalidated"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 metrics after alias filter (sentiment_score dropped), got %d", len(got))
	}
	for _, m := range got {
		if m.MetricName == "sentiment_score" {
			t.Errorf("alias key sentiment_score must be filtered out, but appeared in response")
		}
	}
	// Canonical name must still be present.
	var sawCanonical bool
	for _, m := range got {
		if m.MetricName == "sentiment_score_sentiws" {
			sawCanonical = true
		}
	}
	if !sawCanonical {
		t.Errorf("expected canonical sentiment_score_sentiws in response, got %v", got)
	}
}

func TestGetMetricsAvailable_Returns500OnError(t *testing.T) {
	store := &mockStore{availableMetricsErr: errors.New("db error")}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if _, ok := resp.(GetMetricsAvailable500JSONResponse); !ok {
		t.Fatalf("expected GetMetricsAvailable500JSONResponse, got %T", resp)
	}
}
