package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func metadataCoverageFixture(now time.Time) []storage.MetadataCoverageCell {
	return []storage.MetadataCoverageCell{
		// bundesregierung — author structurally absent.
		{Source: "bundesregierung", Field: "author", Method: "null", Articles: 60, LastSeen: now},
		// bundesregierung — published_date partially populated.
		{Source: "bundesregierung", Field: "published_date", Method: "html_meta", Articles: 49, LastSeen: now},
		{Source: "bundesregierung", Field: "published_date", Method: "null", Articles: 71, LastSeen: now},
		// tagesschau — author always populated.
		{Source: "tagesschau", Field: "author", Method: "json_ld", Articles: 200, LastSeen: now},
	}
}

func TestGetProbeMetadataCoverage_RoundTripWithFlags(t *testing.T) {
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	store := &mockStore{metadataCoverage: metadataCoverageFixture(now)}

	srv := NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  &fakeDossier{},
		Articles: &fakeArticles{},
		Silver:   &fakeSilver{},
	})
	router := newTestRouter(srv)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/probes/probe-0-de-institutional-web/metadata-coverage", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var got MetadataCoverageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Scope != "probe-0-de-institutional-web" {
		t.Errorf("scope mismatch: %s", got.Scope)
	}
	if len(got.Sources) != 2 {
		t.Fatalf("expected 2 sources (probe registry order), got %d", len(got.Sources))
	}
	// Probe registry order: tagesschau, bundesregierung — preserved.
	if got.Sources[0].Name != "tagesschau" || got.Sources[1].Name != "bundesregierung" {
		t.Errorf("unexpected source order: %s, %s", got.Sources[0].Name, got.Sources[1].Name)
	}

	bundes := got.Sources[1]
	var foundAbsent, foundPub bool
	for _, f := range bundes.Fields {
		if f.Field == "author" {
			foundAbsent = true
			if !f.StructurallyAbsent {
				t.Errorf("expected bundesregierung.author structurallyAbsent=true")
			}
			if f.PopulationRate != 0 {
				t.Errorf("expected populationRate=0 for absent author, got %v", f.PopulationRate)
			}
		}
		if f.Field == "published_date" {
			foundPub = true
			if f.StructurallyAbsent {
				t.Errorf("expected published_date structurallyAbsent=false")
			}
			if f.TotalArticles != 120 {
				t.Errorf("expected totalArticles=120, got %d", f.TotalArticles)
			}
		}
	}
	if !foundAbsent || !foundPub {
		t.Errorf("expected both author and published_date in bundesregierung result")
	}
}

func TestGetProbeMetadataCoverage_404OnUnknownProbe(t *testing.T) {
	srv := NewServerWithOptions(&mockStore{}, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  &fakeDossier{},
		Articles: &fakeArticles{},
		Silver:   &fakeSilver{},
	})
	router := newTestRouter(srv)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probes/unknown/metadata-coverage", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetSourceMetadataCoverage_ResolvesSourceAndReturnsMatrix(t *testing.T) {
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	store := &mockStore{metadataCoverage: metadataCoverageFixture(now)}
	dossier := &fakeDossier{resolvedID: 1, resolved: "tagesschau"}

	srv := NewServerWithOptions(store, nil, nil, nil, testProbeRegistry(), ServerOptions{
		Dossier:  dossier,
		Articles: &fakeArticles{},
		Silver:   &fakeSilver{},
	})
	router := newTestRouter(srv)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources/tagesschau/metadata-coverage", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var got MetadataCoverageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Scope != "tagesschau" {
		t.Errorf("scope mismatch: %s", got.Scope)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(got.Sources))
	}
	if got.Sources[0].Name != "tagesschau" {
		t.Errorf("expected tagesschau, got %s", got.Sources[0].Name)
	}
}
