package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCacheControlForPaths_AppliesOn2xxGet(t *testing.T) {
	mw := CacheControlForPaths(24*time.Hour, "/api/v1/content/")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/metric/sentiment_score_sentiws", nil)
	h.ServeHTTP(rec, req)

	want := "public, max-age=86400, must-revalidate"
	if got := rec.Header().Get("Cache-Control"); got != want {
		t.Fatalf("Cache-Control mismatch: got %q, want %q", got, want)
	}
}

func TestCacheControlForPaths_SkipsNon2xx(t *testing.T) {
	mw := CacheControlForPaths(24*time.Hour, "/api/v1/content/")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/content/metric/missing", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("expected no Cache-Control on 404, got %q", got)
	}
}

func TestCacheControlForPaths_SkipsNonMatchingPath(t *testing.T) {
	mw := CacheControlForPaths(24*time.Hour, "/api/v1/content/")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("expected no Cache-Control on /metrics, got %q", got)
	}
}

func TestCacheControlForPaths_SkipsNonGet(t *testing.T) {
	mw := CacheControlForPaths(24*time.Hour, "/api/v1/content/")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/content/metric/x", nil)
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("expected no Cache-Control on POST, got %q", got)
	}
}
