package core

import (
	"context"
	"log/slog"

	"github.com/frogfromlake/aer/services/ingestion-api/internal/storage"
	"go.opentelemetry.io/otel"
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

	// 1. Start the Global Trace Span
	tracer := otel.Tracer("aer.core")
	ctx, span := tracer.Start(ctx, "End-To-End-Data-Pipeline")
	defer span.End()

	// 2. Create the dummy JSON payload (including a metric for our Gold layer)
	jsonData := `{"message": "Hello from AĒR Ingestion!", "status": "raw", "metric_value": 42.5}`
	objectName := "test-ingest.json"

	// 3. Upload to Bronze (ctx contains the Trace-ID)
	err := s.minio.UploadJSON(ctx, "bronze", objectName, jsonData)
	if err != nil {
		slog.Error("Failed to upload to bronze", "error", err)
		span.RecordError(err)
		return err
	}

	slog.Info("Successfully uploaded raw data to Bronze layer", "object", objectName)
	return nil
}
