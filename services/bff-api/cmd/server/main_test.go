package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// --- bodyCap (SEC-015) ---

func TestBodyCap_OversizedBodyYieldsMaxBytesError(t *testing.T) {
	const limit = 16
	var readErr error
	downstream := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(r.Body)
	})
	h := bodyCap(limit)(downstream)

	body := strings.NewReader(strings.Repeat("x", limit+100))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/analyses", body))

	var maxErr *http.MaxBytesError
	if !errors.As(readErr, &maxErr) {
		t.Fatalf("expected *http.MaxBytesError reading an oversized body, got %v", readErr)
	}
}

func TestBodyCap_WithinLimitPassesThroughIntact(t *testing.T) {
	const limit = 1024
	var got string
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		w.WriteHeader(http.StatusOK)
	})
	h := bodyCap(limit)(downstream)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/analyses", strings.NewReader("hello")))
	if got != "hello" || rec.Code != http.StatusOK {
		t.Fatalf("within-limit body should pass through intact: got %q code %d", got, rec.Code)
	}
}

// --- request/response error handlers (SEC-015 + SEC-017) ---

func TestRequestBindingErrorHandler_OversizedIs413(t *testing.T) {
	rec := httptest.NewRecorder()
	requestBindingErrorHandler(rec, httptest.NewRequest(http.MethodPost, "/x", nil), &http.MaxBytesError{Limit: 16})

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for an oversized body, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestRequestBindingErrorHandler_GenericErrorIs400NoLeak(t *testing.T) {
	rec := httptest.NewRecorder()
	leak := errors.New("json: cannot unmarshal string into field Foo.Bar of type int")
	requestBindingErrorHandler(rec, httptest.NewRequest(http.MethodPost, "/x", nil), leak)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a generic binding error, got %d", rec.Code)
	}
	// SEC-017 — the raw binding error (param names, decoder internals) must not
	// reach the client.
	if b := rec.Body.String(); strings.Contains(b, "unmarshal") || strings.Contains(b, "Foo.Bar") {
		t.Fatalf("binding error leaked into the response: %s", b)
	}
}

func TestResponseErrorHandler_GenericIs500NoLeak(t *testing.T) {
	rec := httptest.NewRecorder()
	depErr := errors.New("clickhouse: connection refused on host ch:9000")
	responseErrorHandler(rec, httptest.NewRequest(http.MethodGet, "/x", nil), depErr)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	// SEC-017 — dependency/error internals must not reach the client.
	if b := rec.Body.String(); strings.Contains(b, "clickhouse") || strings.Contains(b, "ch:9000") {
		t.Fatalf("dependency error leaked into the response: %s", b)
	}
}
