package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/time/rate"
)

// okHandler is a trivial downstream handler that always returns 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// --- rateLimiter ---

func TestRateLimiter_AllowsRequestsWithinBurst(t *testing.T) {
	// burst=3: the first three requests must all pass through
	limiter := rate.NewLimiter(rate.Limit(1), 3)
	h := rateLimiter(limiter)(okHandler)

	for i := range 3 {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimiter_Returns429WhenBurstExhausted(t *testing.T) {
	// burst=1, rate=0: the second request must be rejected
	limiter := rate.NewLimiter(rate.Limit(0), 1)
	h := rateLimiter(limiter)(okHandler)

	// first request consumes the single token
	first := httptest.NewRecorder()
	h.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
	if first.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", first.Code)
	}

	// second request must be rate-limited
	second := httptest.NewRecorder()
	h.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("second request: expected 429, got %d", second.Code)
	}
}

func TestRateLimiter_429HasJSONBodyAndContentType(t *testing.T) {
	limiter := rate.NewLimiter(rate.Limit(0), 0) // no tokens ever
	h := rateLimiter(limiter)(okHandler)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["error"] == "" {
		t.Error("expected non-empty error field in JSON body")
	}
}

func TestRateLimiter_PassesThroughToNextHandler(t *testing.T) {
	// verify the downstream handler is actually called when tokens are available
	called := false
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	limiter := rate.NewLimiter(rate.Limit(100), 10)
	h := rateLimiter(limiter)(downstream)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("downstream handler was not called")
	}
}
