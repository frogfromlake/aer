package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// ---------------------------------------------------------------------------
// /sources?silverOnly=true
// ---------------------------------------------------------------------------

type filteringLister struct {
	entries []config.SourceEntry
}

func (f *filteringLister) List(_ context.Context) ([]config.SourceEntry, error) {
	return f.entries, nil
}

func TestGetSources_SilverOnlyFiltersIneligible(t *testing.T) {
	lister := &filteringLister{entries: []config.SourceEntry{
		{Name: "tagesschau", Type: "rss", SilverEligible: true},
		{Name: "bundesregierung", Type: "rss", SilverEligible: true},
		{Name: "wikipedia", Type: "scraper", SilverEligible: false},
	}}
	server := NewServer(&mockStore{}, nil, lister, nil, testProbeRegistry())
	router := newTestRouter(server)

	// Default (silverOnly=false): all three sources.
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))
	var all []struct{ Name string }
	_ = json.Unmarshal(rec.Body.Bytes(), &all)
	if len(all) != 3 {
		t.Fatalf("expected 3 sources unfiltered, got %d", len(all))
	}

	// silverOnly=true: only the two eligible sources.
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources?silverOnly=true", nil))
	var filtered []struct{ Name string }
	_ = json.Unmarshal(rec.Body.Bytes(), &filtered)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 sources filtered, got %d: %+v", len(filtered), filtered)
	}
	for _, s := range filtered {
		if s.Name == "wikipedia" {
			t.Fatalf("ineligible source leaked through filter: %s", s.Name)
		}
	}
}

// ---------------------------------------------------------------------------
// /sources/{id}
// ---------------------------------------------------------------------------

func eligibleRow() *storage.SourceEligibilityRow {
	return &storage.SourceEligibilityRow{
		ID:                    1,
		Name:                  "tagesschau",
		Type:                  "rss",
		URL:                   sql.NullString{String: "https://tagesschau.de", Valid: true},
		SilverEligible:        true,
		SilverReviewReviewer:  sql.NullString{String: "auto-eligible (Probe 0 baseline)", Valid: true},
		SilverReviewDate:      sql.NullTime{Time: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC), Valid: true},
		SilverReviewRationale: sql.NullString{String: "institutional public data", Valid: true},
		SilverReviewReference: sql.NullString{String: "docs/arc42/09_architecture_decisions.md#adr-020", Valid: true},
	}
}

func ineligibleRow() *storage.SourceEligibilityRow {
	return &storage.SourceEligibilityRow{
		ID: 99, Name: "wikipedia", Type: "scraper", SilverEligible: false,
	}
}

func newSilverServer(dossier *fakeDossier, articles *fakeArticles, silver *fakeSilver) *Server {
	return NewServerWithOptions(
		&mockStore{}, nil, nil, nil, testProbeRegistry(),
		ServerOptions{Dossier: dossier, Articles: articles, Silver: silver},
	)
}

func TestGetSourceById_ReturnsEligibilityMetadata(t *testing.T) {
	server := newSilverServer(&fakeDossier{eligibility: eligibleRow()}, nil, nil)
	router := newTestRouter(server)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources/tagesschau", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Name                  string `json:"name"`
		Type                  string `json:"type"`
		SilverEligible        bool   `json:"silverEligible"`
		SilverReviewReviewer  string `json:"silverReviewReviewer"`
		SilverReviewDate      string `json:"silverReviewDate"`
		SilverReviewRationale string `json:"silverReviewRationale"`
		SilverReviewReference string `json:"silverReviewReference"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "tagesschau" || !resp.SilverEligible {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.SilverReviewDate != "2026-04-25" {
		t.Fatalf("review date mismatch: %s", resp.SilverReviewDate)
	}
	if resp.SilverReviewReference == "" {
		t.Fatalf("review reference should be populated")
	}
}

func TestGetSourceById_NotEligibleStillReturnsRecord(t *testing.T) {
	// /sources/{id} is not gated — its purpose is to expose the eligibility state.
	server := newSilverServer(&fakeDossier{eligibility: ineligibleRow()}, nil, nil)
	router := newTestRouter(server)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources/wikipedia", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp struct{ SilverEligible bool `json:"silverEligible"` }
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.SilverEligible {
		t.Fatalf("ineligible source must report silverEligible=false")
	}
}

func TestGetSourceById_NotFound404(t *testing.T) {
	server := newSilverServer(&fakeDossier{eligibilityErr: storage.ErrSourceNotFound}, nil, nil)
	router := newTestRouter(server)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources/does-not-exist", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// /silver/documents — list
// ---------------------------------------------------------------------------

func TestListSilverDocuments_HappyPath(t *testing.T) {
	dossier := &fakeDossier{eligibility: eligibleRow()}
	articles := &fakeArticles{rows: []storage.ArticleAggRow{
		{
			ArticleID: "a1", Source: "tagesschau",
			Timestamp: time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC),
			Language:  "de", HasLanguage: true,
			WordCount: 250, HasWordCount: true,
		},
		{
			ArticleID: "a2", Source: "tagesschau",
			Timestamp: time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
			Language:  "de", HasLanguage: true,
			WordCount: 180, HasWordCount: true,
		},
	}}
	router := newTestRouter(newSilverServer(dossier, articles, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents?sourceId=tagesschau", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Source  string `json:"source"`
		HasMore bool   `json:"hasMore"`
		Items   []struct {
			ArticleID string `json:"articleId"`
			Language  *string `json:"language,omitempty"`
			WordCount *int    `json:"wordCount,omitempty"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Source != "tagesschau" || resp.HasMore || len(resp.Items) != 2 {
		t.Fatalf("unexpected page: %+v", resp)
	}
	if resp.Items[0].WordCount == nil || *resp.Items[0].WordCount != 250 {
		t.Fatalf("wordCount missing/wrong: %+v", resp.Items[0])
	}
}

func TestListSilverDocuments_NotEligibleReturns403WithRefusalPayload(t *testing.T) {
	dossier := &fakeDossier{eligibility: ineligibleRow()}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents?sourceId=wikipedia", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var refusal RefusalPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &refusal); err != nil {
		t.Fatalf("decode refusal: %v", err)
	}
	if refusal.Gate != SilverEligibility {
		t.Fatalf("expected gate=silver_eligibility, got %s", refusal.Gate)
	}
	if refusal.WorkingPaperAnchor == nil || *refusal.WorkingPaperAnchor != "WP-006#section-5.2" {
		t.Fatalf("expected working-paper anchor, got %v", refusal.WorkingPaperAnchor)
	}
}

func TestListSilverDocuments_UnknownSource404(t *testing.T) {
	dossier := &fakeDossier{eligibilityErr: storage.ErrSourceNotFound}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents?sourceId=does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListSilverDocuments_LimitOutOfRange400(t *testing.T) {
	dossier := &fakeDossier{eligibility: eligibleRow()}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents?sourceId=tagesschau&limit=999", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListSilverDocuments_PaginationCursor(t *testing.T) {
	rows := make([]storage.ArticleAggRow, 11) // limit+1 to set hasMore
	for i := range rows {
		rows[i] = storage.ArticleAggRow{
			ArticleID: "a" + string(rune('0'+i)),
			Source:    "tagesschau",
			Timestamp: time.Date(2026, 4, 24, 10, i, 0, 0, time.UTC),
		}
	}
	dossier := &fakeDossier{eligibility: eligibleRow()}
	articles := &fakeArticles{rows: rows}
	router := newTestRouter(newSilverServer(dossier, articles, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents?sourceId=tagesschau&limit=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var resp struct {
		HasMore    bool    `json:"hasMore"`
		NextCursor *string `json:"nextCursor"`
		Items      []any   `json:"items"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.HasMore || resp.NextCursor == nil {
		t.Fatalf("expected hasMore + nextCursor: %+v", resp)
	}
	if len(resp.Items) != 10 {
		t.Fatalf("page should be capped to limit: %d", len(resp.Items))
	}
}

// ---------------------------------------------------------------------------
// /silver/documents/{id} — detail
// ---------------------------------------------------------------------------

func sampleEnvelope() *storage.SilverEnvelope {
	return &storage.SilverEnvelope{
		Core: storage.SilverCore{
			DocumentID:    "art-1",
			Source:        "tagesschau",
			SourceType:    "rss",
			RawText:       "<p>raw</p>",
			CleanedText:   "raw",
			Language:      "de",
			Timestamp:     "2026-04-24T10:00:00Z",
			URL:           "https://tagesschau.de/a/1",
			SchemaVersion: "1.0",
			WordCount:     1,
		},
		Meta:                 map[string]any{"feed_url": "https://tagesschau.de/rss"},
		ExtractionProvenance: map[string]string{"sentiment": "abc123"},
	}
}

func TestGetSilverDocumentDetail_HappyPath(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{BronzeObjectKey: "k", SourceName: "tagesschau"},
		eligibility: eligibleRow(),
	}
	silver := &fakeSilver{envelope: sampleEnvelope()}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, silver))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents/art-1", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ArticleId            string             `json:"articleId"`
		CleanedText          string             `json:"cleanedText"`
		Meta                 map[string]any     `json:"meta"`
		ExtractionProvenance map[string]string  `json:"extractionProvenance"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ArticleId != "art-1" || resp.CleanedText != "raw" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.ExtractionProvenance["sentiment"] != "abc123" {
		t.Fatalf("provenance not surfaced: %+v", resp.ExtractionProvenance)
	}
}

func TestGetSilverDocumentDetail_NotEligibleReturns403(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{BronzeObjectKey: "k", SourceName: "wikipedia"},
		eligibility: ineligibleRow(),
	}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, &fakeSilver{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents/art-2", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var refusal RefusalPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &refusal)
	if refusal.Gate != SilverEligibility {
		t.Fatalf("expected silver_eligibility gate, got %s", refusal.Gate)
	}
}

func TestGetSilverDocumentDetail_ArticleNotFound404(t *testing.T) {
	dossier := &fakeDossier{articleErr: storage.ErrSourceNotFound}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, &fakeSilver{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents/missing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetSilverDocumentDetail_SilverObjectMissing404(t *testing.T) {
	dossier := &fakeDossier{
		article:     &storage.ArticleResolution{BronzeObjectKey: "k", SourceName: "tagesschau"},
		eligibility: eligibleRow(),
	}
	silver := &fakeSilver{err: storage.ErrSilverNotFound}
	router := newTestRouter(newSilverServer(dossier, &fakeArticles{}, silver))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/silver/documents/art-1", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// /silver/aggregations/{aggregationType} — Phase 103b
// ---------------------------------------------------------------------------

func newSilverAggServer(dossier *fakeDossier, store *mockStore) *Server {
	return NewServerWithOptions(
		store, nil, nil, nil, testProbeRegistry(),
		ServerOptions{Dossier: dossier, Articles: &fakeArticles{}, Silver: &fakeSilver{}},
	)
}

func silverAggURL(kind string) string {
	return "/silver/aggregations/" + kind +
		"?sourceId=tagesschau" +
		"&start=2026-04-24T00:00:00Z" +
		"&end=2026-04-25T00:00:00Z"
}

func TestGetSilverAggregation_Distribution(t *testing.T) {
	store := &mockStore{
		silverDistribution: storage.DistributionResult{
			Bins: []storage.DistributionBin{{Lower: 0, Upper: 100, Count: 5}},
			Summary: storage.DistributionSummary{
				Count: 5, Min: 0, Max: 100, Mean: 50, Median: 50,
				P05: 5, P25: 25, P75: 75, P95: 95,
			},
		},
	}
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibility: eligibleRow()}, store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, silverAggURL("cleaned_text_length"), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if store.capturedSilverField != "cleaned_text_length" {
		t.Fatalf("field not propagated: %s", store.capturedSilverField)
	}
	var resp struct {
		AggregationType string `json:"aggregationType"`
		Source          string `json:"source"`
		Distribution    *struct {
			Bins    []any `json:"bins"`
			Summary struct{ Count int64 `json:"count"` } `json:"summary"`
		} `json:"distribution"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.AggregationType != "cleaned_text_length" || resp.Source != "tagesschau" {
		t.Fatalf("unexpected echo fields: %+v", resp)
	}
	if resp.Distribution == nil || resp.Distribution.Summary.Count != 5 {
		t.Fatalf("distribution payload missing: %+v", resp)
	}
}

func TestGetSilverAggregation_Heatmap(t *testing.T) {
	store := &mockStore{
		silverHeatmap: []storage.HeatmapCell{{X: "1", Y: "10", Value: 200, Count: 4}},
		silverHeatmapXDim: "dayOfWeek",
		silverHeatmapYDim: "hour",
	}
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibility: eligibleRow()}, store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, silverAggURL("cleaned_text_length_by_hour"), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if store.capturedSilverKind != storage.SilverAggCleanedTextLengthByHour {
		t.Fatalf("kind not propagated: %s", store.capturedSilverKind)
	}
	var resp struct {
		Heatmap *struct {
			XDimension string `json:"xDimension"`
			YDimension string `json:"yDimension"`
			Cells      []struct{ Count int64 `json:"count"` } `json:"cells"`
		} `json:"heatmap"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Heatmap == nil || resp.Heatmap.XDimension != "dayOfWeek" || len(resp.Heatmap.Cells) != 1 {
		t.Fatalf("heatmap payload missing/wrong: %+v", resp)
	}
}

func TestGetSilverAggregation_Correlation(t *testing.T) {
	c := 0.85
	store := &mockStore{
		silverCorrelation: storage.SilverCorrelationResult{
			Fields:      []string{"cleaned_text_length", "word_count"},
			Matrix:      [][]*float64{{ptrFloat(1), &c}, {&c, ptrFloat(1)}},
			SampleCount: 42,
		},
	}
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibility: eligibleRow()}, store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, silverAggURL("cleaned_text_length_vs_word_count"), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Correlation *struct {
			Fields      []string     `json:"fields"`
			Matrix      [][]*float64 `json:"matrix"`
			SampleCount int64        `json:"sampleCount"`
		} `json:"correlation"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Correlation == nil || resp.Correlation.SampleCount != 42 || len(resp.Correlation.Fields) != 2 {
		t.Fatalf("correlation payload missing/wrong: %+v", resp)
	}
	if resp.Correlation.Matrix[0][1] == nil || *resp.Correlation.Matrix[0][1] != 0.85 {
		t.Fatalf("matrix value missing: %+v", resp.Correlation.Matrix)
	}
}

func TestGetSilverAggregation_NotEligibleReturns403(t *testing.T) {
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibility: ineligibleRow()}, &mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/silver/aggregations/word_count?sourceId=wikipedia&start=2026-04-24T00:00:00Z&end=2026-04-25T00:00:00Z", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var refusal RefusalPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &refusal)
	if refusal.Gate != SilverEligibility {
		t.Fatalf("expected silver_eligibility gate, got %s", refusal.Gate)
	}
	if refusal.WorkingPaperAnchor == nil || *refusal.WorkingPaperAnchor != "WP-006#section-5.2" {
		t.Fatalf("expected anchor WP-006#section-5.2, got %v", refusal.WorkingPaperAnchor)
	}
}

func TestGetSilverAggregation_SourceNotFound404(t *testing.T) {
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibilityErr: storage.ErrSourceNotFound}, &mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/silver/aggregations/word_count?sourceId=ghost&start=2026-04-24T00:00:00Z&end=2026-04-25T00:00:00Z", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSilverAggregation_BadWindow400(t *testing.T) {
	router := newTestRouter(newSilverAggServer(&fakeDossier{eligibility: eligibleRow()}, &mockStore{}))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/silver/aggregations/word_count?sourceId=tagesschau&start=2026-04-25T00:00:00Z&end=2026-04-24T00:00:00Z", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func ptrFloat(v float64) *float64 { return &v }

