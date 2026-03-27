package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/frogfromlake/aer/pkg/logger"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/config"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/core"
	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		panic("Failed to load configuration: " + err.Error())
	}

	// 2. Initialize Shared Logger (from pkg workspace)
	logger.Init(cfg.Environment, cfg.LogLevel)
	slog.Info("Bootstrapping AĒR Ingestion API...", "environment", cfg.Environment)

	// 3. Initialize Postgres Adapter
	db, err := storage.NewPostgresDB(cfg.DBUrl)
	if err != nil {
		slog.Error("Failed to initialize PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer db.DB.Close()
	slog.Info("PostgreSQL connected successfully")

	// 4. Initialize MinIO Adapter
	minioClient, err := storage.NewMinioClient(
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

	// 5. Dependency Injection: Wire the Core Service
	svc := core.NewIngestionService(db, minioClient)

	// 6. Execute Core Logic
	ctx := context.Background()
	if err := svc.Start(ctx); err != nil {
		slog.Error("Ingestion service encountered a fatal error", "error", err)
		os.Exit(1)
	}
}
