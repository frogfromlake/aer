package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func testServer(store *mockStore) *Server {
	return NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  &fakeDossier{},
		Articles: &fakeArticles{},
		Silver:   &fakeSilver{},
	})
}

func TestGetMetadataDistribution_RoundTrip(t *testing.T) {
	store := &mockStore{
		categoricalDistribution: storage.CategoricalDistributionResult{
			Categories: []storage.CategoryCount{
				{Value: "Inland", Articles: 120},
				{Value: "Ausland", Articles: 80},
			},
			TotalArticles:  200,
			DistinctValues: 5,
			OtherArticles:  30,
		},
	}
	router := newTestRouter(testServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/metadata/section/distribution?scope=source&sourceIds=tagesschau", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Field          string `json:"field"`
		TotalArticles  int    `json:"totalArticles"`
		DistinctValues int    `json:"distinctValues"`
		OtherArticles  int    `json:"otherArticles"`
		Categories     []struct {
			Value    string `json:"value"`
			Articles int    `json:"articles"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Field != "section" {
		t.Errorf("field mismatch: %q", got.Field)
	}
	if got.TotalArticles != 200 || got.DistinctValues != 5 || got.OtherArticles != 30 {
		t.Errorf("totals mismatch: %+v", got)
	}
	if len(got.Categories) != 2 || got.Categories[0].Value != "Inland" || got.Categories[0].Articles != 120 {
		t.Errorf("categories mismatch: %+v", got.Categories)
	}
	// the resolved source must reach the store
	if len(store.capturedSources) != 1 || store.capturedSources[0] != "tagesschau" {
		t.Errorf("expected sources=[tagesschau], got %v", store.capturedSources)
	}
}

func TestGetMetadataDistribution_400OnNoScope(t *testing.T) {
	router := newTestRouter(testServer(&mockStore{}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metadata/section/distribution", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (no scope), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetScopeAvailableMetadata_IntersectionAndPartial(t *testing.T) {
	store := &mockStore{
		scopeAvailableMetadata: storage.ScopeMetadataAvailability{
			ScopedSources: []string{"tagesschau", "bundesregierung"},
			Available:     []string{"article_type"},
			Partial: []storage.PartialMetadataField{
				{Field: "section", Sources: []string{"tagesschau"}},
				{Field: "author", Sources: []string{"tagesschau"}},
			},
		},
	}
	router := newTestRouter(testServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/scope/available-metadata?scope=source&sourceIds=tagesschau,bundesregierung", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		ScopedSources []string `json:"scopedSources"`
		Available     []string `json:"available"`
		Partial       []struct {
			Field   string   `json:"field"`
			Sources []string `json:"sources"`
		} `json:"partial"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Available) != 1 || got.Available[0] != "article_type" {
		t.Errorf("expected available=[article_type], got %v", got.Available)
	}
	if len(got.Partial) != 2 || got.Partial[0].Field != "section" {
		t.Errorf("partial mismatch: %+v", got.Partial)
	}
}
