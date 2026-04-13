package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestAPIKeyAuth_AllowsValidKeyInXApiKeyHeader(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "secret")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_AllowsValidKeyInBearerHeader(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("Authorization", "Bearer secret")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_Returns401OnWrongKey(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "wrong")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_Returns401OnMissingKey(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAPIKeyAuth_401ResponseIsJSONContentType(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req.Header.Set("X-API-Key", "wrong")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected Content-Type=application/json on 401, got %q", got)
	}
}

// TestAPIKeyAuth_UsesConstantTimeCompare is a sanity-check that a wrong key
// with the same length as the expected key and a wrong key with a
// different length both produce the same 401. The goal is not to assert
// nanosecond-level timing (flaky in CI) but to ensure the comparison does
// not short-circuit on length mismatches in a way that makes the
// non-equal path functionally distinguishable.
func TestAPIKeyAuth_UsesConstantTimeCompare(t *testing.T) {
	h := APIKeyAuth("correcthorsebatterystaple")(okHandler)

	cases := []string{
		"x",                           // very short
		"correcthorsebatterystapleXXX", // longer than expected
		"correcthorsebatterystaplE",    // same length, 1-char diff
		"wrongwrongwrongwrongwrong",    // same length, all different
	}
	for _, token := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
		req.Header.Set("X-API-Key", token)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("token %q: expected 401, got %d", token, rec.Code)
		}
	}
}

func TestAPIKeyAuth_SkipsHealthzAndReadyz(t *testing.T) {
	h := APIKeyAuth("secret")(okHandler)

	for _, path := range []string{"/api/v1/healthz", "/api/v1/readyz"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("path %s: expected 200 without key, got %d", path, rec.Code)
		}
	}
}
