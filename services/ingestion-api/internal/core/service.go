package core

import (
	"context"
	"log/slog"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
)

// IngestionService orchestrates the data collection and storage processes.
type IngestionService struct {
	db    *storage.PostgresDB
	minio *storage.MinioClient
}

// NewIngestionService creates a new core service via Dependency Injection.
func NewIngestionService(db *storage.PostgresDB, minio *storage.MinioClient) *IngestionService {
	return &IngestionService{
		db:    db,
		minio: minio,
	}
}

// Start executes the initial bootstrapping of the core service.
func (s *IngestionService) Start(ctx context.Context) error {
	slog.Info("Starting AĒR Ingestion Core Logic...")
	slog.Info("Ingestion service is ready and waiting for tasks.")

	// Add crawling/ingestion logic here in the future.

	return nil
}
