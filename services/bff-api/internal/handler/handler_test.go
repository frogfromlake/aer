package handler

import (
	"context"
	"errors"
	"testing"
	"time"
)

// mockStore is a test double for MetricsStore.
type mockStore struct {
	pingErr    error
	metrics    []struct {
		TS    time.Time
		Value float64
	}
	metricsErr error
	// captured args
	capturedStart      time.Time
	capturedEnd        time.Time
	capturedSource     *string
	capturedMetricName *string
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockStore) GetMetrics(_ context.Context, start, end time.Time, source, metricName *string) ([]struct {
	TS    time.Time
	Value float64
}, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedMetricName = metricName
	return m.metrics, m.metricsErr
}

// --- GetHealthz ---

func TestGetHealthz_AlwaysReturnsAlive(t *testing.T) {
	s := NewServer(&mockStore{})
	resp, err := s.GetHealthz(context.Background(), GetHealthzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetHealthz200JSONResponse)
	if !ok {
		t.Fatalf("expected GetHealthz200JSONResponse, got %T", resp)
	}
	if got["status"] != "alive" {
		t.Errorf("expected status=alive, got %q", got["status"])
	}
}

// --- GetReadyz ---

func TestGetReadyz_ReturnsOKWhenPingSucceeds(t *testing.T) {
	s := NewServer(&mockStore{pingErr: nil})
	resp, err := s.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetReadyz200JSONResponse); !ok {
		t.Fatalf("expected GetReadyz200JSONResponse, got %T", resp)
	}
}

func TestGetReadyz_Returns503WhenPingFails(t *testing.T) {
	s := NewServer(&mockStore{pingErr: errors.New("connection refused")})
	resp, err := s.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetReadyz503JSONResponse)
	if !ok {
		t.Fatalf("expected GetReadyz503JSONResponse, got %T", resp)
	}
	if got["clickhouse"] == "" {
		t.Error("expected non-empty clickhouse error message in 503 response")
	}
}

// --- GetMetrics: fallback logic ---

func TestGetMetrics_FallbackRange_WhenNoParamsProvided(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store)

	before := time.Now()
	_, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{})
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// end must be within the test window
	if store.capturedEnd.Before(before) || store.capturedEnd.After(after) {
		t.Errorf("fallback end time %v is outside expected window [%v, %v]", store.capturedEnd, before, after)
	}

	// start must be ~24 hours before end
	expectedStart := store.capturedEnd.Add(-24 * time.Hour)
	diff := store.capturedStart.Sub(expectedStart)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("fallback start time %v deviates too much from expected %v", store.capturedStart, expectedStart)
	}
}

func TestGetMetrics_ExplicitParams_OverrideFallback(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store)

	explicitStart := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	explicitEnd := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	req := GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate: &explicitStart,
			EndDate:   &explicitEnd,
		},
	}
	_, err := s.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.capturedStart.Equal(explicitStart) {
		t.Errorf("expected start %v, got %v", explicitStart, store.capturedStart)
	}
	if !store.capturedEnd.Equal(explicitEnd) {
		t.Errorf("expected end %v, got %v", explicitEnd, store.capturedEnd)
	}
}

func TestGetMetrics_OnlyStartDate_EndFallsBackToNow(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store)

	explicitStart := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	req := GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &explicitStart},
	}

	before := time.Now()
	_, err := s.GetMetrics(context.Background(), req)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.capturedStart.Equal(explicitStart) {
		t.Errorf("expected start %v, got %v", explicitStart, store.capturedStart)
	}
	if store.capturedEnd.Before(before) || store.capturedEnd.After(after) {
		t.Errorf("fallback end %v is outside expected window [%v, %v]", store.capturedEnd, before, after)
	}
}

// --- GetMetrics: error handling ---

func TestGetMetrics_Returns500OnStorageError(t *testing.T) {
	store := &mockStore{metricsErr: errors.New("clickhouse timeout")}
	s := NewServer(store)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetMetrics500JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics500JSONResponse, got %T", resp)
	}
	if got.Message == "" {
		t.Error("expected non-empty error message in 500 response")
	}
}

func TestGetMetrics_ReturnsEmptySliceOnNoData(t *testing.T) {
	store := &mockStore{metrics: nil}
	s := NewServer(store)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics200JSONResponse, got %T", resp)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(got))
	}
}

func TestGetMetrics_MapsStorageRowsToResponse(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		metrics: []struct {
			TS    time.Time
			Value float64
		}{
			{TS: ts, Value: 42.5},
		},
	}
	s := NewServer(store)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics200JSONResponse, got %T", resp)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if !got[0].Timestamp.Equal(ts) {
		t.Errorf("expected timestamp %v, got %v", ts, got[0].Timestamp)
	}
	if got[0].Value != 42.5 {
		t.Errorf("expected value 42.5, got %v", got[0].Value)
	}
}
