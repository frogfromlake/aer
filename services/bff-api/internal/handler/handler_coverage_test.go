package handler

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// covWindow is a fixed RFC-3339 window appended to query strings.
const covWindow = "start=2025-01-01T00:00:00Z&end=2025-02-01T00:00:00Z"

// dossierServer wires a Server with a configured dossier + articles store for
// the path-param-resolved endpoints (source articles, discovery coverage).
func dossierServer(store *mockStore, dossier *fakeDossier, articles *fakeArticles) *Server {
	return NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  dossier,
		Articles: articles,
		Silver:   &fakeSilver{},
	})
}

func do(t *testing.T, router http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// --- GetCorrelationLeadLag ---

func TestGetCorrelationLeadLag_Success(t *testing.T) {
	peakLag := 3
	peakCorr := 0.71
	corr := 0.5
	store := &mockStore{leadLag: storage.LeadLagResult{
		MaxLagHours:       168,
		BucketCountAtZero: 40,
		PeakLagHours:      &peakLag,
		PeakCorrelation:   &peakCorr,
		Points:            []storage.LeadLagPoint{{LagHours: 0, Correlation: &corr}},
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/correlation/lead-lag?xMetric=word_count&yMetric=sentiment_score_sentiws&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetCorrelationLeadLag_MaxLagOutOfRange_400(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := do(t, router, "/correlation/lead-lag?xMetric=word_count&yMetric=entity_count&maxLagHours=9999&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetCorrelationLeadLag_StorageError_500(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{leadLagErr: errTest}))
	rec := do(t, router, "/correlation/lead-lag?xMetric=word_count&yMetric=entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetCorrelationLeadLag_CrossFrameRefusal_400(t *testing.T) {
	// >1 language in scope + no normalization-equivalence grant → refusal.
	store := &mockStore{
		countLanguagesForSourcesValue:            2,
		languagesForScopeRows:                    []string{"de", "fr"},
		checkNormalizationEquivForLanguagesValue: false,
	}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/correlation/lead-lag?xMetric=word_count&yMetric=entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 cross-frame refusal, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GetMetricParallelCoords ---

func TestGetMetricParallelCoords_Success(t *testing.T) {
	store := &mockStore{parallelCoords: storage.ParallelCoordResult{
		Metrics:   []string{"word_count", "entity_count"},
		Truncated: false,
		Rows:      []storage.ParallelCoordRow{{ArticleID: "a1", Source: "tagesschau", Values: []float64{300, 4}}},
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metrics/parallel?metrics=word_count,entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricParallelCoords_CapsAxesAtEight(t *testing.T) {
	store := &mockStore{parallelCoords: storage.ParallelCoordResult{Metrics: []string{"a"}, Rows: nil}}
	router := newTestRouter(testServer(store))
	// 10 metrics requested; the handler caps the axis list at 8 before querying.
	rec := do(t, router, "/metrics/parallel?metrics=m1,m2,m3,m4,m5,m6,m7,m8,m9,m10&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// The cap must reach storage: exactly the first 8 metrics, never all 10.
	if len(store.capturedMetrics) != 8 {
		t.Fatalf("expected the axis list capped to 8 at the storage boundary, got %d: %v", len(store.capturedMetrics), store.capturedMetrics)
	}
	if store.capturedMetrics[7] != "m8" {
		t.Errorf("expected the cap to keep the first 8 in order (m8 last), got %v", store.capturedMetrics)
	}
}

func TestGetMetricCorrelation_Success(t *testing.T) {
	store := &mockStore{correlation: storage.CorrelationResult{
		Metrics:     []string{"word_count", "entity_count"},
		BucketCount: 24,
		Resolution:  "hourly",
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metrics/correlation?metrics=word_count,entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricParallelCoords_TooFewMetrics_400(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := do(t, router, "/metrics/parallel?metrics=word_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetMetricParallelCoords_StorageError_500(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{parallelCoordsErr: errTest}))
	rec := do(t, router, "/metrics/parallel?metrics=word_count,entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- GetMetadataCrossTab ---

func TestGetMetadataCrossTab_Success(t *testing.T) {
	store := &mockStore{crossTab: storage.CrossTabResult{
		DistinctValues: 3,
		Buckets:        []storage.CrossTabBucket{{Value: "Inland", Articles: 50, Mean: 320, Std: 40}},
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metadata/section/by-metric/word_count?scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetadataCrossTab_StorageError_500(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{crossTabErr: errTest}))
	rec := do(t, router, "/metadata/section/by-metric/word_count?scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetMetadataCrossTab_CrossFrameRefusal_400(t *testing.T) {
	// Cross-tabbing a metric across languages requires its equivalence grant.
	store := &mockStore{
		countLanguagesForSourcesValue:            2,
		languagesForScopeRows:                    []string{"de", "fr"},
		checkNormalizationEquivForLanguagesValue: false,
	}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metadata/section/by-metric/word_count?scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 cross-frame refusal, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- GetMetadataSankey ---

func TestGetMetadataSankey_Success(t *testing.T) {
	store := &mockStore{sankey: storage.SankeyResult{
		Fields: []string{"section", "author"},
		Nodes:  []storage.SankeyNode{{ID: "s:Inland", Field: "section", Value: "Inland", Layer: 0}},
		Links:  []storage.SankeyLink{{Source: "s:Inland", Target: "a:Doe", Value: 12}},
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metadata/sankey?fields=section,author&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetadataSankey_TooFewFields_400(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := do(t, router, "/metadata/sankey?fields=section&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetMetadataSankey_StorageError_500(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{sankeyErr: errTest}))
	rec := do(t, router, "/metadata/sankey?fields=section,author&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- GetScopeAvailableMetrics ---

func TestGetScopeAvailableMetrics_Success(t *testing.T) {
	store := &mockStore{}
	store.scopeAvailableMetricsResult = storage.ScopeMetricAvailability{
		ScopedSources: []string{"tagesschau", "bundesregierung"},
		Available:     []string{"word_count"},
		Partial:       []storage.PartialMetric{{MetricName: "sentiment_score_sentiws", Sources: []string{"tagesschau"}}},
	}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/scope/available-metrics?scope=source&sourceIds=tagesschau,bundesregierung&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetScopeAvailableMetrics_StorageError_500(t *testing.T) {
	store := &mockStore{scopeAvailableMetricsErr: errTest}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/scope/available-metrics?scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- GetSourceDiscoveryCoverage ---

func TestGetSourceDiscoveryCoverage_Success(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau", discovery: &storage.DiscoveryCoverageSummary{
		WindowDays:              30,
		TotalDiscoveredLastRun:  120,
		UniqueAfterDedupLastRun: 100,
		UnderflowAlertActive:    true,
		ExpectedFloorPerRun:     sql.NullInt64{Int64: 80, Valid: true},
		PerChannel: []storage.DiscoveryCoverageRow{
			{Channel: "sitemap", LastRunDiscovered: 90, LastRunAfterDedup: 75, AverageDiscoveredPerRun: 88.5},
		},
	}}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/discovery-coverage?windowDays=30")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSourceDiscoveryCoverage_SourceNotFound_404(t *testing.T) {
	dossier := &fakeDossier{resolveErr: storage.ErrSourceNotFound}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/ghost/discovery-coverage")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetSourceDiscoveryCoverage_StorageError_500(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau", discoveryErr: errTest}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/discovery-coverage")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

// --- GetSourceArticles ---

func TestGetSourceArticles_MapsAllOptionalFieldsAndPaginates(t *testing.T) {
	rev := storage.ArticleAggRow{
		ArticleID: "a1", Source: "tagesschau",
		Timestamp: timeAt("2025-01-05T08:00:00Z"),
		Language:  "de", HasLanguage: true,
		WordCount: 320, HasWordCount: true,
		SentimentScore: 0.2, HasSentiment: true,
		TimestampSource:      "fetch_at_fallback",
		HasRevisions:         true,
		ChainLength:          3,
		EditorialChangeCount: 2,
		HasHeadlineChange:    true,
		LatestRevisionAt:     timeAt("2025-01-06T09:00:00Z"),
	}
	bare := storage.ArticleAggRow{ArticleID: "a2", Source: "tagesschau", Timestamp: timeAt("2025-01-05T09:00:00Z")}
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	articles := &fakeArticles{rows: []storage.ArticleAggRow{rev, bare}}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, articles))
	rec := do(t, router, "/sources/tagesschau/articles?limit=1&includeRevisions=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSourceArticles_SourceNotFound_404(t *testing.T) {
	dossier := &fakeDossier{resolveErr: storage.ErrSourceNotFound}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/ghost/articles")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetSourceArticles_BadLimit_400(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/articles?limit=9999")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetSourceArticles_BadCursor_400(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/articles?cursor=%21%21notbase64")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetSourceArticles_InvalidSentimentBand_400(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/articles?sentimentBand=purple")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- metadata-coverage ---

func TestGetSourceMetadataCoverage_Success(t *testing.T) {
	store := &mockStore{metadataCoverage: []storage.MetadataCoverageCell{
		{Source: "tagesschau", Field: "section", Method: "json_ld", Articles: 40, LastSeen: timeAt("2025-01-10T00:00:00Z")},
		{Source: "tagesschau", Field: "author", Method: "byline", Articles: 12, LastSeen: timeAt("2025-01-09T00:00:00Z")},
	}}
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	router := newTestRouter(dossierServer(store, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/metadata-coverage")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSourceMetadataCoverage_SourceNotFound_404(t *testing.T) {
	dossier := &fakeDossier{resolveErr: storage.ErrSourceNotFound}
	router := newTestRouter(dossierServer(&mockStore{}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/ghost/metadata-coverage")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetSourceMetadataCoverage_StorageError_500(t *testing.T) {
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}
	router := newTestRouter(dossierServer(&mockStore{metadataCoverageErr: errTest}, dossier, &fakeArticles{}))
	rec := do(t, router, "/sources/tagesschau/metadata-coverage")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestGetProbeMetadataCoverage_Success(t *testing.T) {
	store := &mockStore{metadataCoverage: []storage.MetadataCoverageCell{
		{Source: "tagesschau", Field: "section", Method: "json_ld", Articles: 40, LastSeen: timeAt("2025-01-10T00:00:00Z")},
	}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/probes/probe-0-de-institutional-web/metadata-coverage")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProbeMetadataCoverage_UnknownProbe_404(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := do(t, router, "/probes/ghost/metadata-coverage")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// --- GetMetricHeatmap segmentBy streams ---

// heatmapStreams decodes just the segment-stream envelope from a heatmap body.
func heatmapStreams(t *testing.T, rec *httptest.ResponseRecorder) []struct {
	ID        string `json:"id"`
	ScopeKind string `json:"scopeKind"`
} {
	t.Helper()
	var body struct {
		Streams []struct {
			ID        string `json:"id"`
			ScopeKind string `json:"scopeKind"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode heatmap body: %v", err)
	}
	return body.Streams
}

func TestGetMetricHeatmap_SegmentBySource_BuildsStreams(t *testing.T) {
	store := &mockStore{heatmap: []storage.HeatmapCell{{X: "Mon", Y: "8", Value: 1, Count: 2}}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metrics/word_count/heatmap?sourceIds=tagesschau,bundesregierung&segmentBy=source&xDimension=dayOfWeek&yDimension=hour&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	// segmentBy=source must yield one stream per resolved source, each tagged source.
	streams := heatmapStreams(t, rec)
	if len(streams) != 2 {
		t.Fatalf("expected 2 source streams, got %d: %s", len(streams), rec.Body.String())
	}
	got := map[string]bool{streams[0].ID: true, streams[1].ID: true}
	if !got["tagesschau"] || !got["bundesregierung"] {
		t.Errorf("stream ids = %v, want tagesschau + bundesregierung", got)
	}
	if streams[0].ScopeKind != "source" {
		t.Errorf("scopeKind = %q, want source", streams[0].ScopeKind)
	}
}

func TestGetMetricHeatmap_SegmentByProbe_BuildsStreams(t *testing.T) {
	store := &mockStore{heatmap: []storage.HeatmapCell{{X: "Mon", Y: "8", Value: 1, Count: 2}}}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metrics/word_count/heatmap?probeIds=probe-0-de-institutional-web&segmentBy=probe&xDimension=dayOfWeek&yDimension=hour&"+covWindow)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	streams := heatmapStreams(t, rec)
	if len(streams) != 1 || streams[0].ID != "probe-0-de-institutional-web" || streams[0].ScopeKind != "probe" {
		t.Fatalf("expected one probe stream for probe-0, got %+v", streams)
	}
}

func TestGetMetricHeatmap_SegmentByProbeWithoutProbeIds_400(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := do(t, router, "/metrics/word_count/heatmap?sourceIds=tagesschau&segmentBy=probe&xDimension=dayOfWeek&yDimension=hour&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// --- pure handler helpers ---

func sp(s string) *string { return &s }

func TestParseMetadataFilter(t *testing.T) {
	if parseMetadataFilter(nil, sp("x")) != nil {
		t.Error("nil field must yield no filter")
	}
	if parseMetadataFilter(sp("section"), nil) != nil {
		t.Error("nil value must yield no filter")
	}
	if parseMetadataFilter(sp("  "), sp("Inland")) != nil {
		t.Error("blank field must yield no filter")
	}
	if parseMetadataFilter(sp("section"), sp("   ")) != nil {
		t.Error("blank value must yield no filter")
	}
	mf := parseMetadataFilter(sp(" section "), sp(" Inland "))
	if mf == nil || mf.Field != "section" || mf.Value != "Inland" {
		t.Errorf("expected trimmed {section, Inland}, got %+v", mf)
	}
}

func TestResolutionFromParam(t *testing.T) {
	hourly := GetMetricsParamsResolutionHourly
	daily := GetMetricsParamsResolutionDaily
	weekly := GetMetricsParamsResolutionWeekly
	monthly := GetMetricsParamsResolutionMonthly
	cases := []struct {
		name string
		in   *GetMetricsParamsResolution
		want storage.Resolution
	}{
		{"nil → 5-minute", nil, storage.ResolutionFiveMinute},
		{"hourly", &hourly, storage.ResolutionHourly},
		{"daily", &daily, storage.ResolutionDaily},
		{"weekly", &weekly, storage.ResolutionWeekly},
		{"monthly", &monthly, storage.ResolutionMonthly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolutionFromParam(tc.in); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUnionSourceParams(t *testing.T) {
	if got := unionSourceParams(nil, nil); got != nil {
		t.Errorf("no params → nil, got %v", got)
	}
	got := unionSourceParams(sp("tagesschau"), sp("tagesschau, bundesregierung ,"))
	// Dedupe (tagesschau appears twice), trim, and drop the empty trailing token.
	if len(got) != 2 || got[0] != "tagesschau" || got[1] != "bundesregierung" {
		t.Errorf("union = %v, want [tagesschau bundesregierung]", got)
	}
}

func TestLoginThrottleAccessor(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, nil, nil)
	if s.LoginThrottle() == nil {
		t.Error("LoginThrottle() must expose the non-nil throttle for the cleanup tick")
	}
}

func TestSafeFloat(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"NaN → 0", math.NaN(), 0},
		{"+Inf → 0", math.Inf(1), 0},
		{"-Inf → 0", math.Inf(-1), 0},
		{"finite passthrough", 3.14, 3.14},
		{"zero", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeFloat(tc.in); got != tc.want {
				t.Errorf("safeFloat(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormaliseWindow(t *testing.T) {
	s, e, err := normaliseWindow(nil, nil)
	if err != nil || s != nil || e != nil {
		t.Errorf("both nil → (nil,nil,nil), got (%v,%v,%v)", s, e, err)
	}

	end := timeAt("2025-02-01T00:00:00Z")
	s, e, err = normaliseWindow(nil, &end)
	if err != nil || s == nil || !s.Equal(wholeDatasetStart) || !e.Equal(end) {
		t.Errorf("nil start opens to the dataset floor, got (%v,%v,%v)", s, e, err)
	}

	start := timeAt("2025-01-01T00:00:00Z")
	s, e, err = normaliseWindow(&start, nil)
	if err != nil || !s.Equal(start) || e == nil {
		t.Errorf("nil end opens to ~now, got (%v,%v,%v)", s, e, err)
	}

	bad := timeAt("2024-01-01T00:00:00Z") // end before start
	if _, _, err := normaliseWindow(&start, &bad); err == nil {
		t.Error("end before start must error")
	}
}

func TestGetMetricCorrelation_StorageError_500(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{correlationErr: errTest}))
	rec := do(t, router, "/metrics/correlation?metrics=word_count,entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetMetricCorrelation_CrossFrameRefusal_400(t *testing.T) {
	store := &mockStore{
		countLanguagesForSourcesValue:            2,
		languagesForScopeRows:                    []string{"de", "fr"},
		checkNormalizationEquivForLanguagesValue: false,
	}
	router := newTestRouter(testServer(store))
	rec := do(t, router, "/metrics/correlation?metrics=word_count,entity_count&scope=source&sourceIds=tagesschau&"+covWindow)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 cross-frame refusal, got %d: %s", rec.Code, rec.Body.String())
	}
}
