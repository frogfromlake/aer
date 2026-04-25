package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// fakeDossier is a tiny in-memory DossierStore for handler tests.
type fakeDossier struct {
	rows           []storage.DossierSourceRow
	resolvedID     int64
	resolved       string
	resolveErr     error
	article        *storage.ArticleResolution
	articleErr     error
	eligibility    *storage.SourceEligibilityRow
	eligibilityErr error
}

func (f *fakeDossier) FetchSources(_ context.Context, _ []string, _, _ *time.Time) ([]storage.DossierSourceRow, error) {
	return f.rows, nil
}

func (f *fakeDossier) ResolveSource(_ context.Context, _ string) (int64, string, error) {
	return f.resolvedID, f.resolved, f.resolveErr
}

func (f *fakeDossier) ResolveArticle(_ context.Context, _ string) (*storage.ArticleResolution, error) {
	return f.article, f.articleErr
}

func (f *fakeDossier) ResolveSourceWithEligibility(_ context.Context, _ string) (*storage.SourceEligibilityRow, error) {
	return f.eligibility, f.eligibilityErr
}

type fakeArticles struct {
	rows  []storage.ArticleAggRow
	count int
}

func (f *fakeArticles) GetSourceArticles(_ context.Context, _ string, _ storage.ArticleQueryFilter) ([]storage.ArticleAggRow, error) {
	return f.rows, nil
}

func (f *fakeArticles) CountAggregationGroup(_ context.Context, _, _ string, _ time.Time) (int, error) {
	return f.count, nil
}

func (f *fakeArticles) GetArticleProvenance(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}

type fakeSilver struct {
	envelope *storage.SilverEnvelope
	err      error
}

func (f *fakeSilver) GetEnvelope(_ context.Context, _ string) (*storage.SilverEnvelope, error) {
	return f.envelope, f.err
}

func TestGetProbeDossier_ComposesSourcesAndFunctionCoverage(t *testing.T) {
	dossier := &fakeDossier{
		rows: []storage.DossierSourceRow{
			{
				Name:             "tagesschau",
				Type:             "rss",
				ArticlesTotal:    42,
				ArticlesInWindow: 12,
				PrimaryFunction:  sql.NullString{String: "epistemic_authority", Valid: true},
				SilverEligible:   true,
			},
			{
				Name:             "bundesregierung",
				Type:             "rss",
				ArticlesTotal:    9,
				ArticlesInWindow: 3,
				PrimaryFunction:  sql.NullString{String: "power_legitimation", Valid: true},
				SilverEligible:   true,
			},
		},
	}
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:             dossier,
		Articles:            &fakeArticles{},
		Silver:              &fakeSilver{},
		KAnonymityThreshold: 10,
	})
	router := newTestRouter(srv)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probes/probe-0-de-institutional-rss/dossier", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ProbeDossier
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ProbeId != "probe-0-de-institutional-rss" {
		t.Errorf("probeId mismatch: %s", got.ProbeId)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got.Sources))
	}
	if got.FunctionCoverage.Covered != 2 || got.FunctionCoverage.Total != 4 {
		t.Errorf("unexpected coverage: covered=%d total=%d", got.FunctionCoverage.Covered, got.FunctionCoverage.Total)
	}
}

func TestGetProbeDossier_404OnUnknownProbe(t *testing.T) {
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier: &fakeDossier{}, Articles: &fakeArticles{}, Silver: &fakeSilver{},
	})
	router := newTestRouter(srv)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probes/unknown/dossier", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetProbeDossier_BadWindow_400(t *testing.T) {
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier: &fakeDossier{}, Articles: &fakeArticles{}, Silver: &fakeSilver{},
	})
	router := newTestRouter(srv)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/probes/probe-0-de-institutional-rss/dossier?windowStart=2026-04-25T00:00:00Z", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetArticleDetail_KAnonymityGateRefuses(t *testing.T) {
	silver := &fakeSilver{envelope: &storage.SilverEnvelope{
		Core: storage.SilverCore{
			DocumentID:    "abc",
			Source:        "tagesschau",
			Timestamp:     "2026-04-25T12:00:00Z",
			CleanedText:   "...",
			SchemaVersion: "1.0",
			WordCount:     100,
		},
	}}
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier: &fakeDossier{
			article: &storage.ArticleResolution{BronzeObjectKey: "any.json", SourceName: "tagesschau"},
		},
		Articles:            &fakeArticles{count: 3},
		Silver:              silver,
		KAnonymityThreshold: 10,
	})
	router := newTestRouter(srv)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/articles/abc", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	var refusal RefusalPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &refusal); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if refusal.Gate != KAnonymity {
		t.Errorf("expected gate=k_anonymity, got %s", refusal.Gate)
	}
	if refusal.Threshold == nil || *refusal.Threshold != 10 {
		t.Errorf("expected threshold=10, got %+v", refusal.Threshold)
	}
	if refusal.Observed == nil || *refusal.Observed != 3 {
		t.Errorf("expected observed=3, got %+v", refusal.Observed)
	}
}

func TestGetArticleDetail_ReturnsCleanedTextWhenGatePasses(t *testing.T) {
	silver := &fakeSilver{envelope: &storage.SilverEnvelope{
		Core: storage.SilverCore{
			DocumentID:    "xyz",
			Source:        "tagesschau",
			Timestamp:     "2026-04-25T12:00:00Z",
			CleanedText:   "Berlin (dpa) — ...",
			SchemaVersion: "1.0",
			WordCount:     150,
		},
		ExtractionProvenance: map[string]string{"sentiment": "v1"},
	}}
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier: &fakeDossier{
			article: &storage.ArticleResolution{BronzeObjectKey: "any.json", SourceName: "tagesschau"},
		},
		Articles:            &fakeArticles{count: 25},
		Silver:              silver,
		KAnonymityThreshold: 10,
	})
	router := newTestRouter(srv)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/articles/xyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got ArticleDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CleanedText == "" {
		t.Error("expected cleaned text in body")
	}
	if got.ExtractionProvenance == nil || (*got.ExtractionProvenance)["sentiment"] != "v1" {
		t.Errorf("expected provenance from envelope, got %+v", got.ExtractionProvenance)
	}
}

func TestEncodeDecodeCursor_RoundTrip(t *testing.T) {
	for _, n := range []int{0, 50, 1000} {
		token := encodeCursor(n)
		got, err := decodeCursor(token)
		if err != nil {
			t.Fatalf("decode failed for offset %d: %v", n, err)
		}
		if got != n {
			t.Errorf("roundtrip mismatch: encoded %d → decoded %d", n, got)
		}
	}
	if _, err := decodeCursor("not_base64!!"); err == nil {
		t.Error("expected error for invalid cursor")
	}
}
