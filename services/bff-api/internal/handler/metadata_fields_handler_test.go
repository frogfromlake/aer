package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func TestGetMetadataFields_MapsStats(t *testing.T) {
	store := &mockStore{
		globalFieldStats: []storage.GlobalFieldStat{
			{
				Field: "article_type", TotalArticles: 300, PopulatedArticles: 300,
				PopulationRate: 1.0, SourcesObserved: 2, SourcesPopulated: 2,
				DistinctValues: 1, Constant: true, ConstantValue: "NewsArticle",
			},
			{
				Field: "comment_count", TotalArticles: 300, PopulatedArticles: 0,
				PopulationRate: 0, SourcesObserved: 2, SourcesPopulated: 0,
				DistinctValues: 0, Constant: false,
			},
		},
	}
	srv := NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier: &fakeDossier{}, Articles: &fakeArticles{}, Silver: &fakeSilver{},
	})
	router := newTestRouter(srv)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metadata-fields", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got struct {
		Fields []struct {
			Field          string  `json:"field"`
			PopulationRate float64 `json:"populationRate"`
			Constant       bool    `json:"constant"`
			ConstantValue  *string `json:"constantValue"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(got.Fields))
	}

	at := got.Fields[0]
	if at.Field != "article_type" || !at.Constant {
		t.Errorf("article_type must be constant, got %+v", at)
	}
	if at.ConstantValue == nil || *at.ConstantValue != "NewsArticle" {
		t.Errorf("article_type constantValue: want NewsArticle, got %v", at.ConstantValue)
	}
	// An empty (never-populated) field carries no constantValue and is not constant.
	cc := got.Fields[1]
	if cc.Field != "comment_count" || cc.Constant || cc.ConstantValue != nil || cc.PopulationRate != 0 {
		t.Errorf("comment_count expected empty/non-constant, got %+v", cc)
	}
}
