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
	normalizedMetrics    []storage.MetricRow
	normalizedMetricsErr error
	baselineExists       bool
	baselineExistsErr    error
	equivalenceExists    bool
	equivalenceExistsErr error
	entities                  []storage.EntityRow
	entitiesErr               error
	languageDetections        []storage.LanguageDetectionRow
	languageDetectionsErr     error
	availableMetrics          []storage.AvailableMetricRow
	availableMetricsErr       error
	validationStatus          string
	validationStatusErr       error
	culturalContextNotes      string
	culturalContextNotesErr   error
	// captured args
	capturedStart      time.Time
	capturedEnd        time.Time
	capturedSource     *string
	capturedMetricName *string
	capturedLabel      *string
	capturedLanguage   *string
	capturedLimit      int
	capturedResolution storage.Resolution
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockStore) GetMetrics(_ context.Context, start, end time.Time, source, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return m.metrics, m.metricsErr
}

func (m *mockStore) GetNormalizedMetrics(_ context.Context, start, end time.Time, source, metricName *string, resolution storage.Resolution) ([]storage.MetricRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSource = source
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return m.normalizedMetrics, m.normalizedMetricsErr
}

func (m *mockStore) CheckBaselineExists(_ context.Context, _ string, _ *string) (bool, error) {
	return m.baselineExists, m.baselineExistsErr
}

func (m *mockStore) CheckEquivalenceExists(_ context.Context, _ string) (bool, error) {
	return m.equivalenceExists, m.equivalenceExistsErr
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

func (m *mockStore) GetAvailableMetrics(_ context.Context, _, _ time.Time) ([]storage.AvailableMetricRow, error) {
	return m.availableMetrics, m.availableMetricsErr
}

func (m *mockStore) GetMetricValidationStatus(_ context.Context, _ string) (string, error) {
	return m.validationStatus, m.validationStatusErr
}

func (m *mockStore) GetMetricCulturalContextNotes(_ context.Context, _ string) (string, error) {
	return m.culturalContextNotes, m.culturalContextNotesErr
}

// newTestRouter builds the full chi router for HTTP-level tests.
func newTestRouter(s *Server) http.Handler {
	return HandlerWithOptions(NewStrictHandler(s, nil), ChiServerOptions{})
}

// --- GetHealthz ---

func TestGetHealthz_AlwaysReturnsAlive(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil)
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
	s := NewServer(&mockStore{pingErr: nil}, nil, nil)
	resp, err := s.GetReadyz(context.Background(), GetReadyzRequestObject{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetReadyz200JSONResponse); !ok {
		t.Fatalf("expected GetReadyz200JSONResponse, got %T", resp)
	}
}

func TestGetReadyz_Returns503WhenPingFails(t *testing.T) {
	s := NewServer(&mockStore{pingErr: errors.New("connection refused")}, nil, nil)
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

// TestGetMetrics_Returns400WhenMissingDates verifies that the generated router
// enforces startDate and endDate as required query parameters. This is an
// HTTP-level test because the requirement is enforced by the generated routing
// code before the handler is called.
func TestGetMetrics_Returns400WhenMissingDates(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil))

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
	s := NewServer(store, nil, nil)

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
	s := NewServer(store, nil, nil)

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
	if got.Message != genericInternalError {
		t.Errorf("expected generic internal error message, got %q", got.Message)
	}
}

func TestGetMetrics_ReturnsEmptySliceOnNoData(t *testing.T) {
	store := &mockStore{metrics: nil}
	s := NewServer(store, nil, nil)

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
	s := NewServer(store, nil, nil)

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
	router := newTestRouter(NewServer(&mockStore{}, nil, nil))

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
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "entity_count", ValidationStatus: "unvalidated"},
			{MetricName: "sentiment_score", ValidationStatus: "validated"},
			{MetricName: "word_count", ValidationStatus: "expired"},
		},
	}
	s := NewServer(store, nil, nil)

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
		t.Fatalf("expected 3 metrics, got %d", len(got))
	}
	if got[0].MetricName != "entity_count" {
		t.Errorf("expected first metric entity_count, got %q", got[0].MetricName)
	}
	if got[0].ValidationStatus != Unvalidated {
		t.Errorf("expected first status unvalidated, got %q", got[0].ValidationStatus)
	}
	if got[1].ValidationStatus != Validated {
		t.Errorf("expected second status validated, got %q", got[1].ValidationStatus)
	}
	if got[2].ValidationStatus != Expired {
		t.Errorf("expected third status expired, got %q", got[2].ValidationStatus)
	}
}

// --- GetMetrics: normalization=zscore ---

func TestGetMetrics_ZscoreRequiresMetricName(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end, Normalization: &norm},
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
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end, Normalization: &norm, MetricName: &metric},
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
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end, Normalization: &norm, MetricName: &metric},
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
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	norm := Zscore
	metric := "sentiment_score"

	resp, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end, Normalization: &norm, MetricName: &metric},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetMetrics200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 when gate passes, got %T", resp)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if got[0].Value != 1.5 {
		t.Errorf("expected zscore value 1.5, got %v", got[0].Value)
	}
}

func TestGetMetrics_ResolutionParamPropagatesToStore(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	hourly := GetMetricsParamsResolutionHourly

	if _, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end, Resolution: &hourly},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedResolution != storage.ResolutionHourly {
		t.Errorf("expected ResolutionHourly forwarded to store, got %v", store.capturedResolution)
	}
}

func TestGetMetrics_DefaultResolutionIsFiveMinute(t *testing.T) {
	store := &mockStore{}
	s := NewServer(store, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)

	if _, err := s.GetMetrics(context.Background(), GetMetricsRequestObject{
		Params: GetMetricsParams{StartDate: start, EndDate: end},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.capturedResolution != storage.ResolutionFiveMinute {
		t.Errorf("expected ResolutionFiveMinute by default, got %v", store.capturedResolution)
	}
}

func TestGetMetricsAvailable_IncludesMinMeaningfulResolution(t *testing.T) {
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "word_count", ValidationStatus: "unvalidated"},
			{MetricName: "unmapped_metric", ValidationStatus: "unvalidated"},
		},
	}
	s := NewServer(store, nil, nil)

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
		t.Fatalf("expected 200, got %T", resp)
	}
	if got[0].MinMeaningfulResolution == nil || *got[0].MinMeaningfulResolution != ResolutionHourly {
		t.Errorf("expected word_count minMeaningfulResolution=hourly, got %v", got[0].MinMeaningfulResolution)
	}
	if got[1].MinMeaningfulResolution != nil {
		t.Errorf("expected unmapped_metric minMeaningfulResolution=nil, got %v", got[1].MinMeaningfulResolution)
	}
}

func TestGetMetrics_RawNormalizationIsDefault(t *testing.T) {
	store := &mockStore{
		metrics: []storage.MetricRow{
			{TS: time.Now(), Value: 42.0, Source: "test", MetricName: "word_count"},
		},
	}
	s := NewServer(store, nil, nil)

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
		t.Fatalf("expected 200, got %T", resp)
	}
	if len(got) != 1 || got[0].Value != 42.0 {
		t.Errorf("expected raw value 42.0, got %v", got[0].Value)
	}
}

// --- GetMetricsAvailable: equivalence metadata ---

func TestGetMetricsAvailable_IncludesEquivalenceMetadata(t *testing.T) {
	etic := "evaluative_polarity"
	equivLevel := "deviation"
	store := &mockStore{
		availableMetrics: []storage.AvailableMetricRow{
			{MetricName: "sentiment_score", ValidationStatus: "unvalidated", EticConstruct: &etic, EquivalenceLevel: &equivLevel},
			{MetricName: "word_count", ValidationStatus: "unvalidated"},
		},
	}
	s := NewServer(store, nil, nil)

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

func TestGetMetricsAvailable_Returns500OnError(t *testing.T) {
	store := &mockStore{availableMetricsErr: errors.New("db error")}
	s := NewServer(store, nil, nil)

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
