package handler

// Phase 95 — GET /content/{entityType}/{entityId} handler coverage.
//
// Tests cover:
//   (a) 200 successful fetch for each entity type (metric, probe, discourse_function, refusal)
//   (b) 404 on missing entity
//   (c) 404 on missing locale
//   (d) invalid entity type → 400 via HTTP router (enum gate)
//   (e) locale defaulting to "en" when param is absent
//   (f) German locale returned correctly

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

// testCatalog builds a minimal content catalog for use in handler tests.
// Files live under a test-only configs path; we construct the catalog in-process
// to avoid filesystem dependency in unit tests.
func testCatalog() config.ContentCatalog {
	makeRecord := func(locale, entityType, entityID string) config.ContentRecord {
		return config.ContentRecord{
			EntityID:         entityID,
			EntityType:       entityType,
			Locale:           locale,
			ContentVersion:   "v2026-04-a",
			LastReviewedBy:   "Engineering Team",
			LastReviewedDate: "2026-04-19",
			Registers: config.ContentRegisters{
				Semantic: config.ContentRegister{
					Short: "Test semantic short for " + entityID,
					Long:  "Test semantic long for " + entityID + ".",
				},
				Methodological: config.ContentRegister{
					Short: "Test methodological short for " + entityID,
					Long:  "Test methodological long for " + entityID + ".",
				},
			},
			WorkingPaperAnchors: []string{"WP-002 §3"},
		}
	}

	catalog := config.ContentCatalog{}

	entities := []struct{ locale, entityType, entityID string }{
		{"en", "metric", "sentiment_score"},
		{"en", "probe", "probe-0-de-institutional-rss"},
		{"en", "discourse_function", "epistemic_authority"},
		{"en", "refusal", "normalization_equivalence_missing"},
		{"de", "metric", "sentiment_score"},
	}
	for _, e := range entities {
		r := makeRecord(e.locale, e.entityType, e.entityID)
		catalog[config.CatalogKey(e.locale, e.entityType, e.entityID)] = r
	}
	return catalog
}

// TestGetContent_MetricReturns200 verifies a successful fetch for entity type "metric".
func TestGetContent_MetricReturns200(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeMetric,
		EntityId:   "sentiment_score",
		Params:     GetContentParams{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetContent200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.EntityId != "sentiment_score" {
		t.Errorf("entityId: want sentiment_score, got %s", got.EntityId)
	}
	if got.EntityType != ContentResponseEntityTypeMetric {
		t.Errorf("entityType: want metric, got %s", got.EntityType)
	}
	if got.Locale != ContentResponseLocaleEn {
		t.Errorf("locale: want en, got %s", got.Locale)
	}
	if got.Registers.Semantic.Short == "" {
		t.Error("semantic.short must not be empty")
	}
	if got.Registers.Methodological.Long == "" {
		t.Error("methodological.long must not be empty")
	}
	wantDate, _ := time.Parse("2006-01-02", "2026-04-19")
	if !got.LastReviewedDate.Equal(wantDate) {
		t.Errorf("lastReviewedDate: want 2026-04-19, got %v", got.LastReviewedDate)
	}
	if got.WorkingPaperAnchors == nil || len(*got.WorkingPaperAnchors) == 0 {
		t.Error("workingPaperAnchors must be populated")
	}
}

// TestGetContent_ProbeReturns200 verifies a successful fetch for entity type "probe".
func TestGetContent_ProbeReturns200(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeProbe,
		EntityId:   "probe-0-de-institutional-rss",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetContent200JSONResponse); !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
}

// TestGetContent_DiscourseFunction verifies a successful fetch for entity type "discourse_function".
func TestGetContent_DiscourseFunction(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeDiscourseFunction,
		EntityId:   "epistemic_authority",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetContent200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.EntityType != ContentResponseEntityTypeDiscourseFunction {
		t.Errorf("entityType: want discourse_function, got %s", got.EntityType)
	}
}

// TestGetContent_Refusal verifies a successful fetch for entity type "refusal".
func TestGetContent_Refusal(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeRefusal,
		EntityId:   "normalization_equivalence_missing",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetContent200JSONResponse); !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
}

// TestGetContent_MissingEntityReturns404 verifies 404 when the entity does not exist.
func TestGetContent_MissingEntityReturns404(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeMetric,
		EntityId:   "nonexistent_metric",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetContent404JSONResponse); !ok {
		t.Fatalf("expected 404 response, got %T", resp)
	}
}

// TestGetContent_MissingLocaleReturns404 verifies 404 when the entity exists in "en" but
// the caller requests "de" and no DE record has been authored.
func TestGetContent_MissingLocaleReturns404(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	locDE := GetContentParamsLocaleDe
	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeProbe,
		EntityId:   "probe-0-de-institutional-rss",
		Params:     GetContentParams{Locale: &locDE},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.(GetContent404JSONResponse); !ok {
		t.Fatalf("expected 404 (locale not found), got %T", resp)
	}
}

// TestGetContent_LocaleDefaultsToEN verifies that when no locale param is provided,
// the "en" catalog is used.
func TestGetContent_LocaleDefaultsToEN(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeMetric,
		EntityId:   "sentiment_score",
		Params:     GetContentParams{Locale: nil},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetContent200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.Locale != ContentResponseLocaleEn {
		t.Errorf("locale: want en, got %s", got.Locale)
	}
}

// TestGetContent_GermanLocaleReturns200 verifies that the DE locale is served correctly.
func TestGetContent_GermanLocaleReturns200(t *testing.T) {
	s := NewServer(&mockStore{}, nil, nil, testCatalog())

	locDE := GetContentParamsLocaleDe
	resp, err := s.GetContent(context.Background(), GetContentRequestObject{
		EntityType: GetContentParamsEntityTypeMetric,
		EntityId:   "sentiment_score",
		Params:     GetContentParams{Locale: &locDE},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := resp.(GetContent200JSONResponse)
	if !ok {
		t.Fatalf("expected 200 response, got %T", resp)
	}
	if got.Locale != ContentResponseLocaleDe {
		t.Errorf("locale: want de, got %s", got.Locale)
	}
}

// TestGetContent_InvalidEntityTypeHTTP verifies that the router returns 400 for an invalid
// entityType path param (enum gate enforced by the generated chi router, not the handler).
func TestGetContent_InvalidEntityTypeHTTP(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, testCatalog()))

	req := httptest.NewRequest(http.MethodGet, "/content/invalid_type/word_count", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestGetContent_HTTPPathReturns200 verifies the full HTTP path returns a well-formed
// JSON body with the correct Content-Type header.
func TestGetContent_HTTPPathReturns200(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, testCatalog()))

	req := httptest.NewRequest(http.MethodGet, "/content/metric/sentiment_score?locale=en", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d — body: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct == "" {
		t.Error("Content-Type header missing")
	}

	var body ContentResponse
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if body.EntityId != "sentiment_score" {
		t.Errorf("entityId: want sentiment_score, got %s", body.EntityId)
	}
	if body.Registers.Semantic.Short == "" {
		t.Error("semantic.short must not be empty")
	}
}
