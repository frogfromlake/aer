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

// mockStore is a test double for Store.
type mockStore struct {
	pingErr    error
	metrics    []storage.MetricRow
	metricsErr error
	entities                  []storage.EntityRow
	entitiesErr               error
	languageDetections        []storage.LanguageDetectionRow
	languageDetectionsErr     error
	availableMetrics          []string
	availableMetricsErr       error
	// captured args
	capturedStart      time.Time
	capturedEnd        time.Time
	capturedSource     *string
	capturedMetricName *string
	capturedLabel      *string
	capturedLanguage   *string
	capturedLimit      int
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockStore) GetMetrics(_ context.Context, start, end time.Time, source, metricName *string) ([]storage.MetricRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedMetricName = metricName
	return m.metrics, m.metricsErr
}

func (m *mockStore) GetEntities(_ context.Context, start, end time.Time, source, label *string, limit int) ([]storage.EntityRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedLabel = label
	m.capturedLimit = limit
	return m.entities, m.entitiesErr
}

func (m *mockStore) GetLanguageDetections(_ context.Context, start, end time.Time, source, language *string, limit int) ([]storage.LanguageDetectionRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedLanguage = language
	m.capturedLimit = limit
	return m.languageDetections, m.languageDetectionsErr
}

func (m *mockStore) GetAvailableMetrics(_ context.Context, _, _ time.Time) ([]string, error) {
	return m.availableMetrics, m.availableMetricsErr
}

// newTestRouter builds the full chi router for HTTP-level tests.
func newTestRouter(s *Server) http.Handler {
	return HandlerWithOptions(NewStrictHandler(s, nil), ChiServerOptions{})
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

// --- GetMetrics ---

// TestGetMetrics_Returns400WhenMissingDates verifies that the generated router
// enforces startDate and endDate as required query parameters. This is an
// HTTP-level test because the requirement is enforced by the generated routing
// code before the handler is called.
func TestGetMetrics_Returns400WhenMissingDates(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}))

	cases := []struct {
		name  string
		query string
	}{
		{"no params", ""},
		{"only startDate", "?startDate=2025-01-01T00:00:00Z"},
		{"only endDate", "?endDate=2025-01-02T00:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestGetMetrics_UsesProvidedDates(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	req := GetMetricsRequestObject{
		Params: GetMetricsParams{
			StartDate: start,
			EndDate:   end,
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
	s := NewServer(store)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end},
	})
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

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end},
	})
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
		metrics: []storage.MetricRow{
			{TS: ts, Value: 42.5, Source: "tagesschau", MetricName: "word_count"},
		},
	}
	s := NewServer(store)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end},
	})
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
	if got[0].Source != "tagesschau" {
		t.Errorf("expected source tagesschau, got %q", got[0].Source)
	}
	if got[0].MetricName != "word_count" {
		t.Errorf("expected metricName word_count, got %q", got[0].MetricName)
	}
}

// --- GetMetricsAvailable ---

func TestGetMetricsAvailable_Returns400WhenMissingDates(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}))

	cases := []struct {
		name  string
		query string
	}{
		{"no params", ""},
		{"only startDate", "?startDate=2025-01-01T00:00:00Z"},
		{"only endDate", "?endDate=2025-01-02T00:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics/available"+tc.query, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestGetMetricsAvailable_ReturnsNames(t *testing.T) {
	store := &mockStore{
		availableMetrics: []string{"entity_count", "sentiment_score", "word_count"},
	}
	s := NewServer(store)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: start, EndDate: end},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetricsAvailable200JSONResponse)
	if !ok {
		t.Fatalf("expected GetMetricsAvailable200JSONResponse, got %T", resp)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 metric names, got %d", len(got))
	}
	if got[0] != "entity_count" {
		t.Errorf("expected first metric entity_count, got %q", got[0])
	}
}

func TestGetMetricsAvailable_Returns500OnError(t *testing.T) {
	store := &mockStore{availableMetricsErr: errors.New("db error")}
	s := NewServer(store)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	resp, err := s.GetMetricsAvailable(context.Background(), GetMetricsAvailableRequestObject{
		Params: GetMetricsAvailableParams{StartDate: start, EndDate: end},
	})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if _, ok := resp.(GetMetricsAvailable500JSONResponse); !ok {
		t.Fatalf("expected GetMetricsAvailable500JSONResponse, got %T", resp)
	}
}
