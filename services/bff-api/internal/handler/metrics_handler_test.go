package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// newTestRouter builds the full chi router for HTTP-level tests.
func newTestRouter(s *Server) http.Handler {
	return HandlerWithOptions(NewStrictHandler(s, nil), ChiServerOptions{})
}

// --- GetHealthz ---

func TestGetHealthz_AlwaysReturnsAlive(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)
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
	s := NewServer(&mockStore{pingErr: nil}, nil, nil, nil, nil)
	resp, err := s.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetReadyz200JSONResponse); !ok {
		t.Fatalf("expected GetReadyz200JSONResponse, got %T", resp)
	}
}

func TestGetReadyz_Returns503WhenPingFails(t *testing.T) {
	s := NewServer(&mockStore{pingErr: errors.New("connection refused")}, nil, nil, nil, nil)
	resp, err := s.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetReadyz503JSONResponse)
	if !ok {
		t.Fatalf("expected GetReadyz503JSONResponse, got %T", resp)
	}
	if got["clickhouse"] != "unavailable" {
		t.Errorf("expected opaque clickhouse=unavailable, got %q", got["clickhouse"])
	}
}

// --- GetMetrics ---

// TestGetMetrics_WindowOptional pins the unbounded-default contract: each bound
// is independently optional — omitting both returns the whole dataset (200),
// and supplying just one is a valid open-ended window (200, the other side is
// opened to the dataset extent). Time-limiting is an optional feature, not the
// default.
func TestGetMetrics_WindowOptional(t *testing.T) {
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
			req := httptest.NewRequest(http.MethodGet, "/metrics"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Errorf("expected %d, got %d: %s", tc.want, w.Code, w.Body.String())
			}
		})
	}
}

// TestGetMetrics_UnboundedResolvesToSentinel pins that an absent window
// actually reaches the storage layer as the whole-dataset sentinel
// [wholeDatasetStart, now] — not a zero time.Time (which would WHERE-filter to
// nothing) and not a 7-day lookback. This is the storage-contract half of the
// unbounded-default behaviour that the HTTP-level WindowOptional test does not
// observe.
func TestGetMetrics_UnboundedResolvesToSentinel(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	before := time.Now().UTC()
	_, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{Params: GetMetricsParams{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !store.capturedStart.Equal(wholeDatasetStart) {
		t.Errorf("unbounded start: want sentinel %v, got %v", wholeDatasetStart, store.capturedStart)
	}
	if store.capturedEnd.Before(before) {
		t.Errorf("unbounded end: want ~now (>= %v), got %v", before, store.capturedEnd)
	}
}

func TestGetMetrics_UsesProvidedDates(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	req := GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate: &start,
			EndDate:   &end,
		},
	}
	_, err := s.GetMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.capturedStart.Equal(start) {
		t.Errorf("expected start %v, got %v", start, store.capturedStart)
	}
	if !store.capturedEnd.Equal(end) {
		t.Errorf("expected end %v, got %v", end, store.capturedEnd)
	}
}

func TestGetMetrics_Returns500OnStorageError(t *testing.T) {
	store := &mockStore{metricsErr: errors.New("clickhouse timeout")}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	got, ok := resp.(GetMetrics500JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics500JSONResponse, got %T", resp)
	}
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func TestGetMetrics_ReturnsEmptySliceOnNoData(t *testing.T) {
	store := &mockStore{metrics: nil}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics200JSONResponse, got %T", resp)
	}
	if len(got.Data) != 0 {
		t.Errorf("expected empty data, got %d entries", len(got.Data))
	}
	if got.ExcludedCount != 0 {
		t.Errorf("expected excludedCount=0 for empty raw response, got %d", got.ExcludedCount)
	}
}

func TestGetMetrics_MapsStorageRowsToResponse(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		metrics: []storage.MetricRow{
			{TS: ts, Value: 42.5, Source: "tagesschau", MetricName: "word_count"},
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
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetrics200JSONResponse, got %T", resp)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got.Data))
	}
	if !got.Data[0].Timestamp.Equal(ts) {
		t.Errorf("expected timestamp %v, got %v", ts, got.Data[0].Timestamp)
	}
	if got.Data[0].Value != 42.5 {
		t.Errorf("expected value 42.5, got %v", got.Data[0].Value)
	}
	if got.Data[0].Source != "tagesschau" {
		t.Errorf("expected source tagesschau, got %q", got.Data[0].Source)
	}
	if got.Data[0].MetricName != "word_count" {
		t.Errorf("expected metricName word_count, got %q", got.Data[0].MetricName)
	}
}

// --- GetMetrics: normalization=zscore ---

func TestGetMetrics_ZscoreRequiresMetricName(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, Normalization: &norm},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetMetrics400JSONResponse); !ok {
		t.Fatalf("expected 400 when metricName is missing for zscore, got %T", resp)
	}
}

func TestGetMetrics_ZscoreReturns400WhenNoBaseline(t *testing.T) {
	store := &mockStore{baselineExists: false, equivalenceExists: true}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, Normalization: &norm, MetricName: &metric},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics400JSONResponse)
	if !ok {
		t.Fatalf("expected 400 when no baseline exists, got %T", resp)
	}
	if got.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGetMetrics_ZscoreReturns400WhenNoEquivalence(t *testing.T) {
	store := &mockStore{baselineExists: true, equivalenceExists: false}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, Normalization: &norm, MetricName: &metric},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics400JSONResponse)
	if !ok {
		t.Fatalf("expected 400 when no equivalence exists, got %T", resp)
	}
	if got.Message == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGetMetrics_ZscoreReturnsDataWhenGatePasses(t *testing.T) {
	ts := time.Date(2025, 3, 15, 12, 0, 0, 0, time.UTC)
	store := &mockStore{
		baselineExists:    true,
		equivalenceExists: true,
		normalizedMetrics: []storage.MetricRow{
			{TS: ts, Value: 1.5, Source: "tagesschau", MetricName: "sentiment_score"},
		},
	}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, Normalization: &norm, MetricName: &metric},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 when gate passes, got %T", resp)
	}
	if len(got.Data) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got.Data))
	}
	if got.Data[0].Value != 1.5 {
		t.Errorf("expected zscore value 1.5, got %v", got.Data[0].Value)
	}
}

func TestGetMetrics_ResolutionParamPropagatesToStore(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	hourly := GetMetricsParamsResolutionHourly

	if _, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end, Resolution: &hourly},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedResolution != storage.ResolutionHourly {
		t.Errorf("expected ResolutionHourly forwarded to store, got %v", store.capturedResolution)
	}
}

func TestGetMetrics_Returns400OnInvalidNormalization(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics?startDate=2025-01-01T00:00:00Z&endDate=2025-01-02T00:00:00Z&normalization=zcore", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid normalization, got %d", w.Code)
	}
}

func TestGetMetrics_Returns400OnInvalidResolution(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, nil))

	req := httptest.NewRequest(http.MethodGet, "/metrics?startDate=2025-01-01T00:00:00Z&endDate=2025-01-02T00:00:00Z&resolution=minutely", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid resolution, got %d", w.Code)
	}
}

func TestGetMetrics_DefaultResolutionIsFiveMinute(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	if _, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: &start, EndDate: &end},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedResolution != storage.ResolutionFiveMinute {
		t.Errorf("expected ResolutionFiveMinute by default, got %v", store.capturedResolution)
	}
}

func TestGetMetrics_RawNormalizationIsDefault(t *testing.T) {
	store := &mockStore{
		metrics: []storage.MetricRow{
			{TS: time.Now(), Value: 42.0, Source: "test", MetricName: "word_count"},
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
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got.Data) != 1 || got.Data[0].Value != 42.0 {
		t.Errorf("expected raw value 42.0, got %v", got)
	}
}
