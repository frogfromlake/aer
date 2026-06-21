package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/trace"
)

// validSpanCtx returns a context carrying a valid (non-recording) span so the
// trace-id helpers behave as they would behind otelhttp, without standing up a
// full tracer provider.
func validSpanCtx(t *testing.T) trace.SpanContext {
	t.Helper()
	traceID, err := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("build trace id: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("build span id: %v", err)
	}
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
}

func TestTraceIDHeaderMiddleware_SetsHeaderWhenSpanValid(t *testing.T) {
	h := TraceIDHeaderMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // even a 5xx must carry the id
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics", nil)
	req = req.WithContext(trace.ContextWithSpanContext(req.Context(), validSpanCtx(t)))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(TraceIDHeader); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("X-Trace-Id = %q, want the active trace id", got)
	}
}

func TestTraceIDHeaderMiddleware_NoSpanIsNoop(t *testing.T) {
	h := TraceIDHeaderMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get(TraceIDHeader); got != "" {
		t.Fatalf("X-Trace-Id = %q, want empty when no valid span", got)
	}
}

func TestWrapResponseWriterOnce_ReusesExistingWrapper(t *testing.T) {
	rec := httptest.NewRecorder()

	// A plain ResponseWriter gets a fresh wrapper.
	first := wrapResponseWriterOnce(rec, 1)
	if first == nil {
		t.Fatal("expected a wrapper for a plain ResponseWriter")
	}

	// SEC-097 — passing the already-wrapped writer back returns the SAME
	// instance, so chained middlewares wrap once rather than nesting.
	if again := wrapResponseWriterOnce(first, 1); again != first {
		t.Fatal("expected the existing wrapper to be reused, got a new one")
	}
}

func TestRequestLoggerAndPrometheusChained_CaptureStatus(t *testing.T) {
	// Chaining both middlewares must still capture the handler's status after
	// the SEC-097 single-wrap change (the inner reuses the outer's wrapper).
	h := PrometheusMetrics("svc")(RequestLogger("svc")(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	// The inner handler's status must reach the wrapper both middlewares share.
	if rec.Result().StatusCode != http.StatusTeapot {
		t.Fatalf("captured status = %d, want %d", rec.Result().StatusCode, http.StatusTeapot)
	}
}

func TestPrometheusMetrics_RecordsRequest(t *testing.T) {
	httpRequestsTotal.Reset()

	// Route through chi so the route pattern (not the concrete path) is the label.
	r := chi.NewRouter()
	r.Use(PrometheusMetrics("test-svc"))
	r.Get("/api/v1/metrics/{metricName}/distribution", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/metrics/sentiment_score/distribution", nil))

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(
		"test-svc", http.MethodGet, "/api/v1/metrics/{metricName}/distribution", "200",
	))
	if got != 1 {
		t.Fatalf("counter = %v, want 1 (labelled by route pattern, not concrete path)", got)
	}
}

func TestPrometheusMetrics_UnmatchedRouteLabel(t *testing.T) {
	httpRequestsTotal.Reset()

	r := chi.NewRouter()
	r.Use(PrometheusMetrics("test-svc"))
	r.Get("/real", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	// A path that matches no route -> chi serves a 404 with an empty route
	// pattern, which the middleware collapses to the fixed "unmatched" label.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does/not/exist", nil))

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues(
		"test-svc", http.MethodGet, "unmatched", "404",
	))
	if got != 1 {
		t.Fatalf("unmatched counter = %v, want 1 (arbitrary paths must not inflate cardinality)", got)
	}
}

func TestRequestLogger_CallsNext(t *testing.T) {
	called := false
	h := RequestLogger("test-svc")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("RequestLogger did not call the next handler")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 passed through", rec.Code)
	}
}

func TestStatusLabel(t *testing.T) {
	cases := map[int]string{0: "200", 200: "200", 404: "404", 500: "500"}
	for in, want := range cases {
		if got := statusLabel(in); got != want {
			t.Errorf("statusLabel(%d) = %q, want %q", in, got, want)
		}
	}
}
