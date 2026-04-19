package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/frogfromlake/aer/services/bff-api/internal/config"
)

// fakeSourceLister drives the SourceStore-facing path without spinning
// up a Postgres container. Every handler test here exercises the
// contract the handler actually relies on: a List(ctx) call that either
// returns a slice or an error.
type fakeSourceLister struct {
	entries []config.SourceEntry
	err     error
}

func (f *fakeSourceLister) List(_ context.Context) ([]config.SourceEntry, error) {
	return f.entries, f.err
}

func sourceStrPtr(s string) *string { return &s }

func TestGetSources_ReturnsRowsFromLister(t *testing.T) {
	lister := &fakeSourceLister{entries: []config.SourceEntry{
		{Name: "tagesschau", Type: "rss", URL: sourceStrPtr("https://tagesschau.de/rss"), DocumentationURL: sourceStrPtr("docs/probes/probe-0-de-institutional-rss/")},
		{Name: "wikipedia", Type: "scraper", URL: sourceStrPtr("https://en.wikipedia.org/")},
	}}
	router := newTestRouter(NewServer(&mockStore{}, nil, lister, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "tagesschau") || !strings.Contains(body, "wikipedia") {
		t.Errorf("expected both sources in response, got %s", body)
	}
}

func TestGetSources_Returns500OnListerError(t *testing.T) {
	lister := &fakeSourceLister{err: errors.New("postgres exploded")}
	router := newTestRouter(NewServer(&mockStore{}, nil, lister, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on lister failure, got %d", rec.Code)
	}
}

func TestGetSources_Returns500WhenListerNil(t *testing.T) {
	router := newTestRouter(NewServer(&mockStore{}, nil, nil, nil))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sources", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when no lister is wired up, got %d", rec.Code)
	}
}
