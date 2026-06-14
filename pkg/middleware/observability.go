package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/trace"
)

// TraceIDHeader is the response header carrying the active OpenTelemetry
// trace-id. It lets an operator pivot from any HTTP response — including a
// 5xx — straight to the matching trace in Tempo (Phase 154). The name is
// the de-facto convention used by tracing-aware gateways.
const TraceIDHeader = "X-Trace-Id"

// httpRequestsTotal and httpRequestDuration are the per-service HTTP server
// metrics scraped from each Go service's /metrics endpoint. They are
// registered on the default Prometheus registry at package init so a service
// process registers them exactly once; promhttp.Handler() (already mounted by
// both Go services) then exposes them.
//
// The `route` label is the chi route *pattern* (e.g. /api/v1/metrics/{metricName}/distribution),
// never the concrete path, to keep cardinality bounded — concrete path params
// (article ids, metric names) must not become distinct time series.
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_server_requests_total",
			Help: "Total HTTP requests handled, by service, method, route pattern and status code.",
		},
		[]string{"service", "method", "route", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_server_request_duration_seconds",
			Help:    "HTTP request handling duration in seconds, by service, method and route pattern.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"service", "method", "route"},
	)
)

// PrometheusMetrics returns a chi middleware that records request count and
// latency into the shared Prometheus collectors. It must run inside the chi
// route group (so the route pattern is resolved) and is the source of the
// per-service "request rate / latency / error rate" dashboard panels.
//
// The route pattern is read *after* the inner handler runs, because chi only
// populates it once the request has been matched to a route.
func PrometheusMetrics(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			route := routePattern(r)
			status := statusLabel(ww.Status())
			elapsed := time.Since(start).Seconds()

			httpRequestsTotal.WithLabelValues(service, r.Method, route, status).Inc()
			httpRequestDuration.WithLabelValues(service, r.Method, route).Observe(elapsed)
		})
	}
}

// TraceIDHeader middleware writes the active span's trace-id onto the response
// as X-Trace-Id before the inner handler runs, so the id survives every
// response path including auth rejections and 5xx errors. It is a no-op when
// no valid span is in context (e.g. the otelhttp middleware did not run first).
func TraceIDHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
			w.Header().Set(TraceIDHeader, sc.TraceID().String())
		}
		next.ServeHTTP(w, r)
	})
}

// RequestLogger returns a structured access-log middleware. Each request emits
// one slog line carrying the service name, method, path, status, duration and
// the active trace-id — the logs-to-traces correlation point (Phase 154).
//
// The trace-id is read from the active OpenTelemetry span rather than the
// inbound Traceparent header: the otelhttp middleware runs earlier and
// establishes the server-side span whose TraceID Tempo indexes, so the span's
// id is the only one that correlates an access log with its trace.
func RequestLogger(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			traceID := ""
			if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
				traceID = sc.TraceID().String()
			}
			slog.Info("http request",
				"service", service,
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"trace_id", traceID,
			)
		})
	}
}

// routePattern returns the chi route pattern for the request, falling back to
// a fixed "unmatched" label for requests that never matched a route (404s) so
// scanners and probes cannot inflate metric cardinality with arbitrary paths.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

// statusLabel maps a captured status code to its string label, defaulting to
// 200 when the handler wrote a body without an explicit WriteHeader call. The
// set of distinct HTTP status codes is naturally small, so the numeric code is
// used directly — error-rate panels aggregate by leading digit in PromQL.
func statusLabel(status int) string {
	if status == 0 {
		status = http.StatusOK
	}
	return strconv.Itoa(status)
}
