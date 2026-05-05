package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// Phase 120 — /topics/distribution handler tests. Mirror the view-mode
// handler test style: a stubbed Store so the handler logic (scope
// resolution, language filter, outlier relabelling, response shape) is
// exercised without a live ClickHouse.

func newTopicsServer(store *mockStore) *Server {
	return NewServer(store, nil, nil, nil, testProbeRegistry())
}

func TestGetTopicDistribution_ResolvesProbeAndReturnsTopics(t *testing.T) {
	store := &mockStore{
		topicDistribution: []storage.TopicDistributionRow{
			{TopicID: 0, Label: "bundestag_klima", Language: "de", ArticleCount: 42, AvgConf: 1.0, ModelHash: "abc"},
			{TopicID: 1, Label: "wirtschaft_inflation", Language: "de", ArticleCount: 30, AvgConf: 1.0, ModelHash: "abc"},
		},
	}
	router := newTestRouter(newTopicsServer(store))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?scope=probe&scopeId=probe-0-de-institutional-rss&start="+winStart+"&end="+winEnd, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if got := store.capturedTopicParams.Sources; len(got) != 2 {
		t.Fatalf("expected 2 probe sources, got %v", got)
	}

	var resp struct {
		Scope   string `json:"scope"`
		ScopeId string `json:"scopeId"`
		Topics  []struct {
			TopicId       int32  `json:"topicId"`
			Label         string `json:"label"`
			Language      string `json:"language"`
			ArticleCount  int64  `json:"articleCount"`
			AvgConfidence float32 `json:"avgConfidence"`
		} `json:"topics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Scope != "probe" || resp.ScopeId != "probe-0-de-institutional-rss" {
		t.Fatalf("scope echo mismatch: %+v", resp)
	}
	if len(resp.Topics) != 2 || resp.Topics[0].ArticleCount != 42 {
		t.Fatalf("topics decoded incorrectly: %+v", resp.Topics)
	}
}

func TestGetTopicDistribution_OutlierRelabelled(t *testing.T) {
	store := &mockStore{
		topicDistribution: []storage.TopicDistributionRow{
			{TopicID: -1, Label: "", Language: "de", ArticleCount: 5, AvgConf: 0.0},
		},
	}
	router := newTestRouter(newTopicsServer(store))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?scopeId=tagesschau&scope=source&start="+winStart+"&end="+winEnd+"&includeOutlier=true", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rec.Code, rec.Body.String())
	}
	if !store.capturedTopicParams.IncludeOutlier {
		t.Fatalf("expected IncludeOutlier=true to reach storage")
	}

	var resp struct {
		Topics []struct {
			TopicId int32  `json:"topicId"`
			Label   string `json:"label"`
		} `json:"topics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Topics) != 1 || resp.Topics[0].TopicId != -1 || resp.Topics[0].Label != "uncategorised" {
		t.Fatalf("outlier relabelling failed: %+v", resp.Topics)
	}
}

func TestGetTopicDistribution_NoScopeReturns400(t *testing.T) {
	router := newTestRouter(newTopicsServer(&mockStore{}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?start="+winStart+"&end="+winEnd, nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing scope, got %d", rec.Code)
	}
}

func TestGetTopicDistribution_UnknownProbeReturns404(t *testing.T) {
	router := newTestRouter(newTopicsServer(&mockStore{}))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?scope=probe&scopeId=does-not-exist&start="+winStart+"&end="+winEnd, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown probe, got %d", rec.Code)
	}
}

func TestGetTopicDistribution_StorageError500(t *testing.T) {
	store := &mockStore{topicDistributionErr: errors.New("boom")}
	router := newTestRouter(newTopicsServer(store))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?scope=source&scopeId=tagesschau&start="+winStart+"&end="+winEnd, nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on storage error, got %d", rec.Code)
	}
}

func TestGetTopicDistribution_DefaultWindow30Days(t *testing.T) {
	store := &mockStore{}
	router := newTestRouter(newTopicsServer(store))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/topics/distribution?scope=source&scopeId=tagesschau", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with default window, got %d %s", rec.Code, rec.Body.String())
	}
	span := store.capturedTopicParams.End.Sub(store.capturedTopicParams.Start)
	if span <= 29*24*60*60*1e9 || span >= 31*24*60*60*1e9 {
		t.Fatalf("expected ~30d default window, got %v", span)
	}
}
