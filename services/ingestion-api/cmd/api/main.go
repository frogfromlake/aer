package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frogfromlake/aer/pkg/logger"
	mw "github.com/frogfromlake/aer/pkg/middleware"
	"github.com/frogfromlake/aer/pkg/telemetry"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/config"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/handler"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
)

// startRetentionCleanup runs a daily PostgreSQL retention sweep in the background.
// Documents older than 90 days are deleted first, then orphaned completed/failed
// ingestion jobs older than 90 days. The 90-day window matches the MinIO bronze
// ILM policy, preventing accumulation of orphan metadata records.
func startRetentionCleanup(ctx context.Context, db *storage.PostgresDB) {
	const retentionDays = 90
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	runCleanup := func() {
		cutoff := time.Now().AddDate(0, 0, -retentionDays)

		docsDeleted, err := db.DeleteOldDocuments(ctx, cutoff)
		if err != nil {
			slog.Error("PostgreSQL document retention cleanup failed", "error", err)
		} else if docsDeleted > 0 {
			slog.Info("PostgreSQL retention: deleted old documents", "count", docsDeleted, "cutoff", cutoff.Format(time.RFC3339))
		}

		jobsDeleted, err := db.DeleteOldIngestionJobs(ctx, cutoff)
		if err != nil {
			slog.Error("PostgreSQL job retention cleanup failed", "error", err)
		} else if jobsDeleted > 0 {
			slog.Info("PostgreSQL retention: deleted old ingestion jobs", "count", jobsDeleted, "cutoff", cutoff.Format(time.RFC3339))
		}
	}

	runCleanup() // run once immediately on startup

	for {
		select {
		case <-ctx.Done():
			slog.Info("PostgreSQL retention cleanup goroutine stopped.")
			return
		case <-ticker.C:
			runCleanup()
		}
	}
}

func main() {
	// 1. Setup context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR Ingestion API...", "environment", cfg.Environment)

	shutdown, err := telemetry.InitProvider("aer-ingestion-api", cfg.OTelEndpoint, cfg.OTelSampleRate)
	if err != nil {
		slog.Error("Failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("Error during OpenTelemetry shutdown", "error", err)
		}
	}()

	// 2. Initialize PostgreSQL adapter
	db, err := storage.NewPostgresDB(ctx, cfg.DBUrl, storage.PoolConfig{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.DBConnMaxLifeMin) * time.Minute,
	})
	if err != nil {
		slog.Error("Failed to initialize PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.DB.Close(); err != nil {
			slog.Error("Error closing PostgreSQL connection", "error", err)
		}
	}()
	slog.Info("PostgreSQL connected successfully")

	// 2b. Run database migrations
	if err := storage.RunMigrations(cfg.DBUrl, cfg.MigrationsPath); err != nil {
		slog.Error("Failed to run database migrations", "error", err)
		os.Exit(1)
	}
	slog.Info("Database migrations applied successfully")

	// 3. Initialize MinIO adapter
	minioClient, err := storage.NewMinioClient(
		ctx,
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioUseSSL,
		cfg.BronzeBucket,
	)
	if err != nil {
		slog.Error("Failed to initialize MinIO", "error", err)
		os.Exit(1)
	}
	slog.Info("MinIO client connected successfully")

	// 4. Wire service and handlers
	svc := core.NewIngestionService(db, minioClient, cfg.BronzeBucket, cfg.MinioUploadConcurrency)
	srv := handler.NewServer(svc, cfg.MaxBodyBytes)
	strictH := handler.NewStrictHandlerWithOptions(srv, nil, handler.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: handler.RequestErrorHandler,
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			slog.Error("response encoding failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
		},
	})

	// 5. Setup chi router with OTel instrumentation
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "ingestion-api")
	})

	// Prometheus scrape endpoint sits outside the API-key middleware: the
	// backend network is zero-trust-internal only, and scrape targets
	// cannot carry per-caller secrets.
	r.Handle("/metrics", promhttp.Handler())

	// API Key Auth: protects all routes except /healthz and /readyz.
	// Scoped via Group so /metrics above stays unauthenticated.
	// BodyLimitMiddleware must run before the strict handler decodes the body.
	r.Group(func(r chi.Router) {
		r.Use(mw.NewCORSHandler(strings.Split(cfg.CORSOrigins, ","), []string{"GET", "POST", "OPTIONS"}))
		r.Use(mw.APIKeyAuth(cfg.APIKey))
		r.Use(srv.BodyLimitMiddleware)
		handler.HandlerFromMuxWithBaseURL(strictH, r, "/api/v1")
	})

	// 6. Start background PostgreSQL retention cleanup (runs every 24h)
	go startRetentionCleanup(ctx, db)

	// 7. Start HTTP server.
	//
	// Timeouts are hard defaults chosen to survive slowloris-style attacks
	// and keep idle sockets from accumulating. WriteTimeout must be the
	// largest per-request budget; ShutdownTimeout must exceed it so a mid-
	// flight batch can drain. MaxHeaderBytes caps the request-line + header
	// block and is independent of the JSON body limit, which the Ingest
	// handler enforces via http.MaxBytesReader.
	server := &http.Server{
		Addr:              ":" + cfg.IngestionPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	go func() {
		slog.Info("AĒR Ingestion API listening", "port", cfg.IngestionPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	// 8. Block until shutdown signal, then drain with configurable grace period.
	<-ctx.Done()
	slog.Info("Shutdown signal received. Shutting down Ingestion API gracefully...")

	shutdownTimeout := time.Duration(cfg.ShutdownTimeoutSeconds) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("Ingestion API stopped cleanly.")
	}
}
