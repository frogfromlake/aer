package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/time/rate"

	"github.com/frogfromlake/aer/pkg/logger"
	mw "github.com/frogfromlake/aer/pkg/middleware"
	"github.com/frogfromlake/aer/pkg/telemetry"
	"github.com/frogfromlake/aer/services/bff-api/internal/config"
	"github.com/frogfromlake/aer/services/bff-api/internal/handler"
	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

func main() {
	// 1. Setup Context FIRST so backoff respects interrupts
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 2. Load configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// 3. Initialize Logger
	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR BFF API...", "environment", cfg.Environment)

	// 4. Initialize OpenTelemetry
	shutdownTracer, err := telemetry.InitProvider("aer-bff-api", cfg.OTelEndpoint, cfg.OTelSampleRate)
	if err != nil {
		slog.Error("Failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracer(context.Background()); err != nil {
			slog.Error("Error during OpenTelemetry shutdown", "error", err)
		}
	}()

	// 5. Initialize Storage (Passing Context for Backoff)
	chAddr := cfg.ClickHouseHost + ":" + cfg.ClickHousePort
	cacheTTL := time.Duration(cfg.MetricsCacheTTLSecs) * time.Second
	chStore, err := storage.NewClickHouseStorage(
		ctx,
		chAddr,
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
		cfg.QueryRowLimit,
		cacheTTL,
	)
	if err != nil {
		slog.Error("Failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	slog.Info("ClickHouse connected successfully")

	// 6. Load static BFF config (metric provenance). Source metadata now
	// lives in Postgres (Phase 87 — no more YAML mirror).
	provenance, err := config.LoadMetricProvenance("configs/metric_provenance.yaml")
	if err != nil {
		slog.Error("Failed to load metric_provenance.yaml", "error", err)
		os.Exit(1)
	}

	// 6b. Open the read-only PostgreSQL pool that backs /api/v1/sources.
	// The pool is intentionally tiny: /sources is served from a TTL cache
	// so we only need enough capacity to refresh every BFF_SOURCES_CACHE_TTL
	// and absorb the occasional startup probe.
	pgDB, err := openSourcesPool(ctx, cfg)
	if err != nil {
		slog.Error("Failed to connect to Postgres for /sources", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := pgDB.Close(); err != nil {
			slog.Error("Error closing Postgres pool", "error", err)
		}
	}()
	sourcesTTL := time.Duration(cfg.SourcesCacheTTLSecs) * time.Second
	sourceStore := storage.NewSourceStore(pgDB, sourcesTTL)

	// 7. Setup Handlers and Router
	serverLogic := handler.NewServer(chStore, provenance, sourceStore)
	strictHandler := handler.NewStrictHandler(serverLogic, nil)

	r := chi.NewRouter()

	// Recovery runs on every request (including /metrics) so a panic in any
	// handler becomes a 500 instead of crashing the server.
	r.Use(middleware.Recoverer)

	// Prometheus scrape endpoint sits outside the API-key middleware: the
	// backend network is zero-trust-internal only, and scrape targets
	// cannot carry per-caller secrets.
	r.Handle("/metrics", promhttp.Handler())

	// --- API ROUTE GROUP (authenticated, rate-limited, traced) ---
	r.Group(func(r chi.Router) {
		// Request Timeout: limits each request to 30s to prevent hanging ClickHouse queries
		r.Use(middleware.Timeout(30 * time.Second))

		// CORS: configurable allowed origins via CORS_ALLOWED_ORIGINS env var
		allowedOrigins := strings.Split(cfg.CORSOrigins, ",")
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   allowedOrigins,
			AllowedMethods:   []string{"GET", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type", "X-API-Key"},
			AllowCredentials: false,
			MaxAge:           300,
		}))

		// OTel: wraps each request in a span and propagates the trace context
		r.Use(func(next http.Handler) http.Handler {
			return otelhttp.NewHandler(next, "bff-api")
		})

		// Request Logging: structured access log for every request via slog
		r.Use(requestLogger)

		// Rate Limiting: token-bucket limiter; rejects excess requests with 429
		limiter := rate.NewLimiter(rate.Limit(cfg.RateLimitRPS), cfg.RateLimitBurst)
		r.Use(rateLimiter(limiter))

		// API Key Auth: protects all routes except /healthz and /readyz
		r.Use(mw.APIKeyAuth(cfg.APIKey))

		handler.HandlerFromMuxWithBaseURL(strictHandler, r, "/api/v1")
	})

	// --- GRACEFUL SHUTDOWN LOGIC ---
	//
	// HTTP server timeouts are hard defaults: ReadHeaderTimeout guards
	// against slowloris-style header stalls, WriteTimeout must exceed the
	// chi request timeout (30s) so a slow ClickHouse query still has time
	// to serialize its response, and ShutdownTimeout in turn must exceed
	// WriteTimeout so an in-flight request can drain on SIGTERM.
	server := &http.Server{
		Addr:              ":" + cfg.BFFPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	// Start server in a separate goroutine
	go func() {
		slog.Info("AĒR BFF API listening", "port", cfg.BFFPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	// Block main thread until a signal is received
	<-ctx.Done()
	slog.Info("Shutdown signal received. Shutting down BFF API gracefully...")

	// Grace period for active requests to finish before forced shutdown.
	shutdownTimeout := time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("BFF API stopped cleanly.")
	}
}

// openSourcesPool opens a small, read-only PostgreSQL pool dedicated to
// the /sources endpoint. The BFF connects as a role that only holds
// SELECT on the `sources` table — any future schema escalation is caught
// at the database layer, not in Go. A short connection lifetime makes
// the pool resilient to long-running deployments where the underlying
// Postgres server restarts.
func openSourcesPool(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		url.QueryEscape(cfg.BFFDBUser),
		url.QueryEscape(cfg.BFFDBPassword),
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDB,
	)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open pgx pool: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

// rateLimiter returns a middleware that enforces a token-bucket rate limit.
// Requests exceeding the limit are rejected immediately with 429 Too Many Requests.
func rateLimiter(limiter *rate.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger is a structured access log middleware using slog.
//
// The trace_id is read from the active OpenTelemetry span rather than the
// incoming Traceparent header. The otelhttp middleware runs earlier in the
// stack and establishes the server-side span; its TraceID is what Tempo
// indexes, so using the span's ID is the only way to make access logs and
// Tempo traces correlatable.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		traceID := ""
		if sc := trace.SpanFromContext(r.Context()).SpanContext(); sc.IsValid() {
			traceID = sc.TraceID().String()
		}
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"trace_id", traceID,
		)
	})
}
