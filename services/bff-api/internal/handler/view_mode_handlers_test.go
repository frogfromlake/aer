package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

const (
	winStart = "2026-04-24T00:00:00Z"
	winEnd   = "2026-04-25T00:00:00Z"
)

func mustTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return parsed
}

func newViewModeServer(store *mockStore) *Server {
	return NewServer(store, nil, nil, nil, testProbeRegistry())
}

// ---------------------------------------------------------------------------
// /metrics/{name}/distribution
// ---------------------------------------------------------------------------

func TestGetMetricDistribution_ResolvesProbeAndReturnsBins(t *testing.T) {
	store := &mockStore{
		distribution: storage.DistributionResult{
			Bins: []storage.DistributionBin{
				{Lower: -1.0, Upper: 0.0, Count: 7},
				{Lower: 0.0, Upper: 1.0, Count: 13},
			},
			Summary: storage.DistributionSummary{
				Count: 20, Min: -0.9, Max: 0.95, Mean: 0.1, Median: 0.05,
				P05: -0.8, P25: -0.2, P75: 0.4, P95: 0.85,
			},
		},
	}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/distribution?scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd+"&bins=2", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}

	if got := store.capturedSources; len(got) != 2 || got[0] != "tagesschau" || got[1] != "bundesregierung" {
		t.Fatalf("expected probe sources to be passed to storage, got %v", got)
	}
	if store.capturedBins != 2 {
		t.Fatalf("bins mismatch: %d", store.capturedBins)
	}
	if !store.capturedStart.Equal(mustTime(t, winStart)) || !store.capturedEnd.Equal(mustTime(t, winEnd)) {
		t.Fatalf("window mismatch: %v / %v", store.capturedStart, store.capturedEnd)
	}

	var resp struct {
		MetricName string `json:"metricName"`
		Scope      string `json:"scope"`
		ScopeId    string `json:"scopeId"`
		Bins       []struct {
			Lower float64 `json:"lower"`
			Upper float64 `json:"upper"`
			Count int64   `json:"count"`
		} `json:"bins"`
		Summary struct {
			Count int64 `json:"count"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.MetricName != "sentiment_score" || resp.Scope != "probe" || resp.ScopeId != "probe-0-de-institutional-rss" {
		t.Fatalf("response echo mismatch: %+v", resp)
	}
	if len(resp.Bins) != 2 || resp.Bins[1].Count != 13 {
		t.Fatalf("bins decoded incorrectly: %+v", resp.Bins)
	}
	if resp.Summary.Count != 20 {
		t.Fatalf("summary count: %d", resp.Summary.Count)
	}
}

func TestGetMetricDistribution_SourceScopeFiltersToOneSource(t *testing.T) {
	store := &mockStore{}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/word_count/distribution?scope=source&scopeId=tagesschau&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	if len(store.capturedSources) != 1 || store.capturedSources[0] != "tagesschau" {
		t.Fatalf("expected single source, got %v", store.capturedSources)
	}
	if store.capturedBins != 30 {
		t.Fatalf("default bins should be 30, got %d", store.capturedBins)
	}
}

func TestGetMetricDistribution_UnknownProbeReturns404(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/distribution?scope=probe&scopeId=does-not-exist&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricDistribution_BadWindowReturns400(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/distribution?scope=probe&scopeId=probe-0-de-institutional-rss&start="+winEnd+"&end="+winStart, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricDistribution_StorageError500(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{distributionErr: errors.New("ch down")}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/distribution?scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !contains(rec.Body.String(), genericInternalError) {
		t.Fatalf("response should not leak internals: %s", rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// /metrics/{name}/heatmap
// ---------------------------------------------------------------------------

func TestGetMetricHeatmap_RoundTrip(t *testing.T) {
	store := &mockStore{
		heatmap: []storage.HeatmapCell{
			{X: "1", Y: "9", Value: 0.4, Count: 3},
			{X: "1", Y: "10", Value: 0.3, Count: 2},
		},
	}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/heatmap?scope=probe&scopeId=probe-0-de-institutional-rss&xDimension=dayOfWeek&yDimension=hour&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if store.capturedXDim != storage.HeatmapDimDayOfWeek || store.capturedYDim != storage.HeatmapDimHour {
		t.Fatalf("dims mismatch: %s / %s", store.capturedXDim, store.capturedYDim)
	}

	var resp struct {
		Cells []struct {
			X, Y  string
			Value float64
			Count int64
		} `json:"cells"`
		XDimension string `json:"xDimension"`
		YDimension string `json:"yDimension"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.XDimension != "dayOfWeek" || resp.YDimension != "hour" {
		t.Fatalf("echo mismatch: %+v", resp)
	}
	if len(resp.Cells) != 2 {
		t.Fatalf("cells: %v", resp.Cells)
	}
}

func TestGetMetricHeatmap_InvalidDimensionRejectedByRouter(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/sentiment_score/heatmap?scope=probe&scopeId=probe-0-de-institutional-rss&xDimension=banana&yDimension=hour&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// /metrics/correlation
// ---------------------------------------------------------------------------

func TestGetMetricCorrelation_RoundTrip(t *testing.T) {
	v := 0.42
	store := &mockStore{
		correlation: storage.CorrelationResult{
			Metrics: []string{"sentiment_score", "word_count"},
			Matrix: [][]*float64{
				{ptrF(1.0), &v},
				{&v, ptrF(1.0)},
			},
			BucketCount: 12,
			Resolution:  "5m",
		},
	}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/correlation?metrics=sentiment_score,word_count&scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.capturedMetrics; len(got) != 2 || got[0] != "sentiment_score" || got[1] != "word_count" {
		t.Fatalf("metrics passed to store: %v", got)
	}

	var resp struct {
		Metrics     []string     `json:"metrics"`
		Matrix      [][]*float64 `json:"matrix"`
		BucketCount int64        `json:"bucketCount"`
		Resolution  string       `json:"resolution"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.BucketCount != 12 || resp.Resolution != "5m" {
		t.Fatalf("envelope mismatch: %+v", resp)
	}
	if len(resp.Matrix) != 2 || len(resp.Matrix[0]) != 2 {
		t.Fatalf("matrix shape: %v", resp.Matrix)
	}
}

func TestGetMetricCorrelation_TooFewMetrics400(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metrics/correlation?metrics=sentiment_score&scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// /entities/cooccurrence
// ---------------------------------------------------------------------------

func TestGetEntityCoOccurrence_RoundTripAndClampsTopN(t *testing.T) {
	store := &mockStore{
		cooccurrence: storage.CoOccurrenceResult{
			Edges: []storage.CoOccurrenceEdge{
				{A: "Berlin", B: "Merkel", ALabel: "LOC", BLabel: "PER", Weight: 9, ArticleCount: 4},
				{A: "Berlin", B: "Scholz", ALabel: "LOC", BLabel: "PER", Weight: 5, ArticleCount: 3},
			},
			Nodes: []storage.CoOccurrenceNode{
				{Text: "Berlin", Label: "LOC", Degree: 2, TotalCount: 14},
				{Text: "Merkel", Label: "PER", Degree: 1, TotalCount: 9},
				{Text: "Scholz", Label: "PER", Degree: 1, TotalCount: 5},
			},
			TopN: 50,
		},
	}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/entities/cooccurrence?scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if store.capturedTopN != 50 {
		t.Fatalf("default topN should be 50, got %d", store.capturedTopN)
	}

	var resp struct {
		Edges []struct {
			A, B   string
			Weight int64 `json:"weight"`
		} `json:"edges"`
		Nodes []struct {
			Text   string
			Label  string
			Degree int64 `json:"degree"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Edges) != 2 || resp.Edges[0].Weight != 9 {
		t.Fatalf("edges decoded incorrectly: %+v", resp.Edges)
	}
	names := make([]string, 0, len(resp.Nodes))
	for _, n := range resp.Nodes {
		names = append(names, n.Text)
	}
	sort.Strings(names)
	if names[0] != "Berlin" || names[1] != "Merkel" || names[2] != "Scholz" {
		t.Fatalf("nodes mismatch: %v", names)
	}
}

func TestGetEntityCoOccurrence_MissingScopeId400(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/entities/cooccurrence?scope=probe&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// helpers shared with this file only
func ptrF(v float64) *float64 { return &v }
