package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// Phase 115 — BFF coverage for percentile normalization, the cross-frame
// equivalence gate, the structured equivalenceStatus on /metrics/available,
// and the new /probes/{probeId}/equivalence endpoint.

// --- ?normalization=percentile ---

func TestGetMetrics_PercentileDispatchesToPercentileQuery(t *testing.T) {
	ts := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		baselineExists:    true,
		equivalenceExists: true,
		percentileMetrics: []storage.MetricRow{
			{TS: ts, Value: 0.42, Source: "tagesschau", MetricName: "word_count"},
		},
		// Within-frame: only one language → cross-frame gate is skipped.
		countLanguagesForSourcesValue: 1,
	}
	s := NewServer(store, nil, nil, nil, nil)

	mode := Percentile
	metric := "word_count"
	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			Normalization: &mode,
			MetricName:    &metric,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got.Data) != 1 || got.Data[0].Value != 0.42 {
		t.Errorf("expected the percentile row to flow through, got %+v", got.Data)
	}
}

func TestGetMetrics_RejectsUnknownNormalization(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/metrics?startDate=2026-04-01T00:00:00Z&endDate=2026-04-02T00:00:00Z&normalization=garbage&metricName=word_count",
		nil,
	))
	// The OpenAPI router enforces the enum, returning 400 before the handler.
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown normalization, got %d (%s)", rec.Code, rec.Body.String())
	}
}

// --- Cross-frame equivalence gate ---

func TestGetMetrics_CrossFrameRefusalReturnsStructured400(t *testing.T) {
	store := &mockStore{
		baselineExists:                    true,
		equivalenceExists:                 true,
		countLanguagesForSourcesValue:     2,
		languagesForScopeRows:             []string{"de", "en"},
		checkEquivalenceForLanguagesValue: false, // gate fires
	}
	s := NewServer(store, nil, nil, nil, nil)

	mode := Zscore
	metric := "sentiment_score_sentiws"
	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			Normalization: &mode,
			MetricName:    &metric,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics400JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics400JSONResponse, got %T", resp)
	}
	if got.Gate == nil || *got.Gate != crossFrameGateID {
		t.Errorf("expected gate=%q, got %+v", crossFrameGateID, got.Gate)
	}
	if got.WorkingPaperAnchor == nil || *got.WorkingPaperAnchor != crossFrameAnchor {
		t.Errorf("expected anchor=%q, got %+v", crossFrameAnchor, got.WorkingPaperAnchor)
	}
	if got.Alternatives == nil || len(*got.Alternatives) != 3 {
		t.Errorf("expected 3 alternatives, got %+v", got.Alternatives)
	}
}

func TestGetMetrics_CrossFrameWithEquivalenceLetsRequestThrough(t *testing.T) {
	ts := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		baselineExists:                    true,
		equivalenceExists:                 true,
		countLanguagesForSourcesValue:     2,
		languagesForScopeRows:             []string{"de", "en"},
		checkEquivalenceForLanguagesValue: true, // grant exists
		normalizedMetrics: []storage.MetricRow{
			{TS: ts, Value: -0.42, Source: "tagesschau", MetricName: "sentiment"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	mode := Zscore
	metric := "sentiment"
	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:       time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
			Normalization: &mode,
			MetricName:    &metric,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetMetrics200JSONResponse); !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
}

// --- structured equivalenceStatus on /metrics/available ---

func TestGetMetricsAvailable_IncludesEquivalenceStatusWithNotes(t *testing.T) {
	level := "deviation"
	validatedBy := "Dr. Reviewer"
	validationDate := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	notes := "temporal-level grant per WP-004 §6.3"
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{
				MetricName:       "word_count",
				ValidationStatus: "validated",
				EquivalenceLevel: &level,
				EquivalenceStatus: &storage.EquivalenceStatusRow{
					Level:          &level,
					ValidatedBy:    &validatedBy,
					ValidationDate: &validationDate,
					Notes:          notes,
				},
			},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)
	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{
			StartDate: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	m := got[0]
	if m.EquivalenceLevel == nil || string(*m.EquivalenceLevel) != "deviation" {
		t.Errorf("deprecated equivalenceLevel mismatch: %+v", m.EquivalenceLevel)
	}
	if m.EquivalenceStatus == nil {
		t.Fatalf("expected equivalenceStatus to be present")
	}
	if m.EquivalenceStatus.Notes != notes {
		t.Errorf("notes mismatch: got %q, want %q", m.EquivalenceStatus.Notes, notes)
	}
	if m.EquivalenceStatus.ValidatedBy == nil || *m.EquivalenceStatus.ValidatedBy != validatedBy {
		t.Errorf("validatedBy mismatch: %+v", m.EquivalenceStatus.ValidatedBy)
	}
	if m.EquivalenceStatus.ValidationDate == nil || !m.EquivalenceStatus.ValidationDate.Equal(validationDate) {
		t.Errorf("validationDate mismatch: %+v", m.EquivalenceStatus.ValidationDate)
	}
	if m.EquivalenceStatus.Level == nil || *m.EquivalenceStatus.Level != "deviation" {
		t.Errorf("structured level mismatch: %+v", m.EquivalenceStatus.Level)
	}
}

// --- /probes/{probeId}/equivalence ---

func TestGetProbeEquivalence_404OnUnknownProbe(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, config.ProbeRegistry{})
	resp, err := s.GetProbeEquivalence(context.Background(), GetProbeEquivalenceRequestObject{
		ProbeId: "no-such-probe",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetProbeEquivalence404JSONResponse); !ok {
		t.Fatalf("expected 404, got %T", resp)
	}
}

func TestGetProbeEquivalence_ReturnsLevel1OnlyForEmptyRegistry(t *testing.T) {
	registry := config.ProbeRegistry{
		"probe-0-de-institutional-rss": config.ProbeEntry{
			ProbeID:  "probe-0-de-institutional-rss",
			Language: "de",
			Sources:  []string{"tagesschau", "bundesregierung"},
		},
	}
	store := &mockStore{
		probeEquivalenceRows: []storage.ProbeEquivalenceMetric{
			{MetricName: "word_count", Level1Available: true},
			{MetricName: "sentiment_score_sentiws", Level1Available: true},
		},
	}
	s := NewServer(store, nil, nil, nil, registry)
	router := newTestRouter(s)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(
		http.MethodGet,
		"/probes/probe-0-de-institutional-rss/equivalence",
		nil,
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		ProbeId string `json:"probeId"`
		Sources []string `json:"sources"`
		Metrics []struct {
			MetricName      string `json:"metricName"`
			Level1Available bool   `json:"level1Available"`
			Level2Available bool   `json:"level2Available"`
			Level3Available bool   `json:"level3Available"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.ProbeId != "probe-0-de-institutional-rss" {
		t.Errorf("probeId mismatch: %q", body.ProbeId)
	}
	if len(body.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(body.Sources))
	}
	if len(body.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(body.Metrics))
	}
	for _, m := range body.Metrics {
		if !m.Level1Available {
			t.Errorf("metric %q should have Level1=true", m.MetricName)
		}
		if m.Level2Available || m.Level3Available {
			t.Errorf("metric %q should have Level2/3=false on empty registry", m.MetricName)
		}
	}
}
