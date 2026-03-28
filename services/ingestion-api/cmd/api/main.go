package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/frogfromlake/aer/pkg/logger"
	"github.com/frogfromlake/aer/pkg/telemetry"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/config"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
)

func main() {
	// 1. Setup Context FIRST so backoff respects interrupts
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR Ingestion API...", "environment", cfg.Environment)

	shutdown, err := telemetry.InitProvider("aer-ingestion-api")
	if err != nil {
		slog.Error("Failed to initialize OpenTelemetry", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("Error during OpenTelemetry shutdown", "error", err)
		}
	}()

	// 2. Initialize PostgreSQL Adapter (Passing Context for Backoff)
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

	// 3. Initialize MinIO Adapter (Passing Context for Backoff)
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

	svc := core.NewIngestionService(db, minioClient)

	// --- GRACEFUL SHUTDOWN LOGIC ---
	// Run core logic inside a goroutine to allow listening for cancellation
	done := make(chan struct{})
	go func() {
		if err := svc.Start(ctx); err != nil {
			slog.Error("Ingestion service encountered an error", "error", err)
		}
		close(done)
	}()

	// Wait for either the ingestion to finish or an interrupt signal
	select {
	case <-ctx.Done():
		slog.Info("Shutdown signal received. Cancelling ingestion process...")
	case <-done:
		slog.Info("Ingestion process completed successfully.")
	}

	slog.Info("Ingestion API shut down cleanly.")
}
