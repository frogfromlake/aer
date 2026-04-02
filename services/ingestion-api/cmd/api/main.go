package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/frogfromlake/aer/pkg/logger"
	"github.com/frogfromlake/aer/pkg/telemetry"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/config"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/handler"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
)

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

	shutdown, err := telemetry.InitProvider("aer-ingestion-api", cfg.OTelEndpoint)
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
	db, err := storage.NewPostgresDB(ctx, cfg.DBUrl)
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

	// 3. Initialize MinIO adapter
	minioClient, err := storage.NewMinioClient(
		ctx,
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioUseSSL,
	)
	if err != nil {
		slog.Error("Failed to initialize MinIO", "error", err)
		os.Exit(1)
	}
	slog.Info("MinIO client connected successfully")

	// 4. Wire service and handlers
	svc := core.NewIngestionService(db, minioClient)
	h := handler.NewHandler(svc)

	// 5. Setup chi router with OTel instrumentation
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "ingestion-api")
	})

	r.Post("/api/v1/ingest", h.Ingest)
	r.Get("/api/v1/healthz", h.Healthz)
	r.Get("/api/v1/readyz", h.Readyz)

	// 6. Start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.IngestionPort,
		Handler: r,
	}

	go func() {
		slog.Info("AĒR Ingestion API listening", "port", cfg.IngestionPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server crashed", "error", err)
			os.Exit(1)
		}
	}()

	// 7. Block until shutdown signal, then drain with 5s timeout
	<-ctx.Done()
	slog.Info("Shutdown signal received. Shutting down Ingestion API gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Forced server shutdown", "error", err)
	} else {
		slog.Info("Ingestion API stopped cleanly.")
	}
}
