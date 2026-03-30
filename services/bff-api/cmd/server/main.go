package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frogfromlake/aer/pkg/logger"
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
	shutdownTracer, err := telemetry.InitProvider("aer-bff-api", cfg.OTelEndpoint)
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
	chStore, err := storage.NewClickHouseStorage(
		ctx,
		chAddr,
		cfg.ClickHouseUser,
		cfg.ClickHousePassword,
		cfg.ClickHouseDB,
	)
	if err != nil {
		slog.Error("Failed to connect to ClickHouse", "error", err)
		os.Exit(1)
	}
	slog.Info("ClickHouse connected successfully")

	// 6. Setup Handlers and Router
	serverLogic := handler.NewServer(chStore)
	strictHandler := handler.NewStrictHandler(serverLogic, nil)

	r := chi.NewRouter()

	// --- MIDDLEWARE STACK ---

	// Recovery: catches panics in handlers and returns 500 instead of crashing
	r.Use(middleware.Recoverer)

	// Request Timeout: limits each request to 30s to prevent hanging ClickHouse queries
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS: configurable allowed origins via CORS_ALLOWED_ORIGINS env var
	allowedOrigins := strings.Split(cfg.CORSOrigins, ",")
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	// OTel: wraps each request in a span and propagates the trace context
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "bff-api")
	})

	// Request Logging: structured access log for every request via slog
	r.Use(requestLogger)

	// API Key Auth: protects all routes except /healthz and /readyz
	r.Use(apiKeyAuth(cfg.APIKey))

	// --- ROUTES ---
	handler.HandlerFromMuxWithBaseURL(strictHandler, r, "/api/v1")

	// --- GRACEFUL SHUTDOWN LOGIC ---
	server := &http.Server{
		Addr:    ":" + cfg.BFFPort,
		Handler: r,
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

	// Allow up to 5 seconds for active requests to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("BFF API stopped cleanly.")
	}
}

// apiKeyAuth returns a middleware that requires a valid API key on all routes
// except /healthz and /readyz, which must remain unauthenticated for probes.
// The key is accepted via the X-API-Key header or Authorization: Bearer <key>.
func apiKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.HasSuffix(path, "/healthz") || strings.HasSuffix(path, "/readyz") {
				next.ServeHTTP(w, r)
				return
			}

			token := r.Header.Get("X-API-Key")
			if token == "" {
				if bearer := r.Header.Get("Authorization"); strings.HasPrefix(bearer, "Bearer ") {
					token = strings.TrimPrefix(bearer, "Bearer ")
				}
			}

			if token == "" || token != apiKey {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requestLogger is a structured access log middleware using slog.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"trace_id", r.Header.Get("Traceparent"),
		)
	})
}
