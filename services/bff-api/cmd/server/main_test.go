package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// okHandler is a trivial downstream handler that always returns 200.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// testClientLimiter builds a per-client limiter whose general and strict budgets
// are identical, so the single-budget tests below (which hit non-/auth/ paths
// from one peer IP) exercise it exactly like the old shared limiter did.
func testClientLimiter(rps rate.Limit, burst int) *clientRateLimiter {
	return newClientRateLimiter(
		rateBudget{rps: rps, burst: burst},
		rateBudget{rps: rps, burst: burst},
		time.Minute, time.Minute,
	)
}

// --- rateLimiter ---

func TestRateLimiter_AllowsRequestsWithinBurst(t *testing.T) {
	// burst=3: the first three requests must all pass through
	h := rateLimiter(testClientLimiter(rate.Limit(1), 3))(okHandler)

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
	h := rateLimiter(testClientLimiter(rate.Limit(0), 1))(okHandler)

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
	h := rateLimiter(testClientLimiter(rate.Limit(0), 0))(okHandler) // no tokens ever

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

	h := rateLimiter(testClientLimiter(rate.Limit(100), 10))(downstream)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Error("downstream handler was not called")
	}
}

func TestRateLimiter_PerClientIsolation(t *testing.T) {
	// SEC-014: one client exhausting its budget must NOT rate-limit a different
	// client — the whole point of keying the limiter by IP.
	h := rateLimiter(testClientLimiter(rate.Limit(0), 1))(okHandler)

	reqFrom := func(ip string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":1234"
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := reqFrom("10.0.0.1"); code != http.StatusOK {
		t.Fatalf("client A first request: expected 200, got %d", code)
	}
	if code := reqFrom("10.0.0.1"); code != http.StatusTooManyRequests {
		t.Fatalf("client A second request: expected 429, got %d", code)
	}
	// client B has its own bucket and must still pass
	if code := reqFrom("10.0.0.2"); code != http.StatusOK {
		t.Errorf("client B: expected 200 (separate per-IP budget), got %d", code)
	}
}

func TestRateLimiter_PreAuthUsesStrictBudget(t *testing.T) {
	// SEC-014: /auth/* draws on the tighter strict bucket, exhausted independently
	// of the generous general budget.
	limiter := newClientRateLimiter(
		rateBudget{rps: rate.Limit(100), burst: 100}, // generous general
		rateBudget{rps: rate.Limit(0), burst: 1},     // strict: a single token
		time.Minute, time.Minute,
	)
	h := rateLimiter(limiter)(okHandler)

	authReq := func() int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.9:1234"
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := authReq(); code != http.StatusOK {
		t.Fatalf("first /auth/ request: expected 200, got %d", code)
	}
	if code := authReq(); code != http.StatusTooManyRequests {
		t.Errorf("second /auth/ request: expected 429 (strict budget), got %d", code)
	}

	// a non-/auth/ path from the same IP draws on the still-full general bucket
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/probes", nil)
	req.RemoteAddr = "10.0.0.9:1234"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("general path: expected 200 (budget separate from strict), got %d", rec.Code)
	}
}

func TestClientRateLimiter_EvictsIdleBuckets(t *testing.T) {
	// SEC-014: the per-IP map must not grow without bound — idle buckets are
	// evicted once a client has been silent beyond idleTTL.
	c := newClientRateLimiter(
		rateBudget{rps: rate.Limit(1), burst: 1},
		rateBudget{rps: rate.Limit(1), burst: 1},
		10*time.Minute, time.Hour, // long sweep so the background loop never fires mid-test
	)
	c.allow("10.0.0.1", false) // first request creates the bucket
	if got := len(c.clients); got != 1 {
		t.Fatalf("expected 1 tracked client after first request, got %d", got)
	}
	// not yet idle past the TTL → retained
	c.evictIdle(time.Now())
	if got := len(c.clients); got != 1 {
		t.Fatalf("expected the active bucket retained, got %d", got)
	}
	// advance the clock past idleTTL → evicted
	c.evictIdle(time.Now().Add(11 * time.Minute))
	if got := len(c.clients); got != 0 {
		t.Errorf("expected idle bucket evicted, got %d remaining", got)
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
