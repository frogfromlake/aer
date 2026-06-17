package handler

// Tests for the entity co-occurrence surfaces: GET /entities/cooccurrence
// and POST /entities/cooccurrence/query (multi-scope group union, cross-
// language refusal, scope-limit + topN clamps). Split from
// view_mode_handlers_test.go for cohesion (Phase 142 test-code health).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

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
		"/entities/cooccurrence?scope=probe&scopeId=probe-0-de-institutional-web&start="+winStart+"&end="+winEnd, nil))

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

func TestGetEntityCoOccurrence_NodePresencePopulated(t *testing.T) {
	store := &mockStore{
		cooccurrence: storage.CoOccurrenceResult{
			Edges: []storage.CoOccurrenceEdge{
				{A: "Berlin", B: "Scholz", ALabel: "LOC", BLabel: "PER", Weight: 3, ArticleCount: 2},
			},
			Nodes: []storage.CoOccurrenceNode{
				{Text: "Berlin", Label: "LOC", Degree: 1, TotalCount: 3, Presence: []string{"tagesschau", "bundesregierung"}},
				{Text: "Scholz", Label: "PER", Degree: 1, TotalCount: 3},
			},
			TopN: 50,
		},
	}
	router := newTestRouter(newViewModeServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/entities/cooccurrence?sourceIds=tagesschau,bundesregierung&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Nodes []struct {
			Text     string   `json:"text"`
			Presence []string `json:"presence"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, n := range resp.Nodes {
		if n.Text == "Berlin" {
			if len(n.Presence) != 2 {
				t.Fatalf("expected 2 presence entries for Berlin, got %d", len(n.Presence))
			}
			return
		}
	}
	t.Fatalf("Berlin node not found in response")
}

// helpers shared with this file only
func ptrF(v float64) *float64 { return &v }

// ---------------------------------------------------------------------------
// Phase 122i / ADR-034 — POST /entities/cooccurrence/query
// ---------------------------------------------------------------------------

// testProbeRegistryMultiLang returns two probes with distinct languages so
// the cross-language refusal path can be exercised without touching the
// single-probe fixture other tests rely on.
func testProbeRegistryMultiLang() config.ProbeRegistry {
	base := testProbeRegistry()
	base["probe-1-en-public-web"] = config.ProbeEntry{
		ProbeID:  "probe-1-en-public-web",
		Language: "en",
		Sources:  []string{"bbc", "guardian"},
		EmissionPoints: []config.EmissionPoint{
			{Latitude: 51.5074, Longitude: -0.1278, Label: "London"},
		},
	}
	return base
}

func TestPostEntityCoOccurrenceQuery_RoundTripsSingleScope(t *testing.T) {
	store := &mockStore{
		cooccurrence: storage.CoOccurrenceResult{
			Edges: []storage.CoOccurrenceEdge{
				{A: "Berlin", B: "Merkel", ALabel: "LOC", BLabel: "PER", Weight: 9, ArticleCount: 4},
			},
			Nodes: []storage.CoOccurrenceNode{
				{Text: "Berlin", Label: "LOC", Degree: 1, TotalCount: 9},
				{Text: "Merkel", Label: "PER", Degree: 1, TotalCount: 9},
			},
			TopN: 50,
		},
	}
	router := newTestRouter(newViewModeServer(store))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": []}],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.capturedSources; len(got) != 2 || got[0] != "tagesschau" || got[1] != "bundesregierung" {
		t.Fatalf("expected both probe-0 sources to reach storage, got %v", got)
	}
	if store.capturedTopN != 50 {
		t.Fatalf("default topN should be 50, got %d", store.capturedTopN)
	}
}

func TestPostEntityCoOccurrenceQuery_FiltersBySourceIdsInGroup(t *testing.T) {
	store := &mockStore{
		cooccurrence: storage.CoOccurrenceResult{TopN: 50},
	}
	router := newTestRouter(newViewModeServer(store))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": ["tagesschau"]}],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.capturedSources; len(got) != 1 || got[0] != "tagesschau" {
		t.Fatalf("expected sourceIds filter to narrow to tagesschau, got %v", got)
	}
}

func TestPostEntityCoOccurrenceQuery_UnionsAcrossGroups(t *testing.T) {
	store := &mockStore{cooccurrence: storage.CoOccurrenceResult{TopN: 50}}
	router := newTestRouter(newViewModeServer(store))

	body := strings.NewReader(`{
		"scopes": [
			{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": ["tagesschau"]},
			{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": ["bundesregierung"]}
		],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.capturedSources; len(got) != 2 || got[0] != "tagesschau" || got[1] != "bundesregierung" {
		t.Fatalf("expected union across groups, got %v", got)
	}
}

func TestPostEntityCoOccurrenceQuery_UnknownProbe404(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-does-not-exist"], "sourceIds": []}],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPostEntityCoOccurrenceQuery_EmptyScopes400(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	body := strings.NewReader(`{"scopes": [], "windowStart": "` + winStart + `", "windowEnd": "` + winEnd + `"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPostEntityCoOccurrenceQuery_InvalidWindow400(t *testing.T) {
	router := newTestRouter(newViewModeServer(&mockStore{}))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": []}],
		"windowStart": "` + winEnd + `",
		"windowEnd": "` + winStart + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for inverted window, got %d", rec.Code)
	}
}

func TestPostEntityCoOccurrenceQuery_CrossLanguageRefused422(t *testing.T) {
	router := newTestRouter(
		NewServer(&mockStore{cooccurrence: storage.CoOccurrenceResult{TopN: 50}}, nil, nil, nil, testProbeRegistryMultiLang()),
	)

	body := strings.NewReader(`{
		"scopes": [
			{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": []},
			{"probeIds": ["probe-1-en-public-web"], "sourceIds": []}
		],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for cross-language scope, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Message string  `json:"message"`
		Gate    *string `json:"gate"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Gate == nil || *resp.Gate != "cross_language_merge_unsupported" {
		t.Fatalf("expected gate=cross_language_merge_unsupported, got %+v", resp)
	}
}

func TestPostEntityCoOccurrenceQuery_ScopeLimitExceeded413(t *testing.T) {
	// Build a probe with 101 fake sources so the cap fires from a single group.
	registry := testProbeRegistry()
	sources := make([]string, 0, 101)
	for i := 0; i < 101; i++ {
		sources = append(sources, fmt.Sprintf("src-%d", i))
	}
	entry := registry["probe-0-de-institutional-web"]
	entry.Sources = sources
	registry["probe-0-de-institutional-web"] = entry

	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil, registry))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": []}],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for source cap, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Gate *string `json:"gate"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Gate == nil || *resp.Gate != "scope_limit_exceeded" {
		t.Fatalf("expected gate=scope_limit_exceeded, got %+v", resp)
	}
}

func TestPostEntityCoOccurrenceQuery_ClampsTopN(t *testing.T) {
	store := &mockStore{cooccurrence: storage.CoOccurrenceResult{TopN: 500}}
	router := newTestRouter(newViewModeServer(store))

	body := strings.NewReader(`{
		"scopes": [{"probeIds": ["probe-0-de-institutional-web"], "sourceIds": []}],
		"windowStart": "` + winStart + `",
		"windowEnd": "` + winEnd + `",
		"topN": 9999
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/entities/cooccurrence/query", body)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if store.capturedTopN != 500 {
		t.Fatalf("topN should clamp to 500, got %d", store.capturedTopN)
	}
}
