package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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
	slog.Info("Starting AĒR Ingestion Core Logic (Metadata Index Test)...")

	// 1. Create a tracking job in PostgreSQL (Assuming source_id 1 is our Dummy Source)
	jobID, err := s.db.CreateIngestionJob(ctx, 1)
	if err != nil {
		return fmt.Errorf("could not initialize ingestion job: %w", err)
	}
	slog.Info("Created new ingestion job", "job_id", jobID)

	tracer := otel.Tracer("aer.core")

	testCases := []struct {
		name     string
		filename string
		payload  string
	}{
		{
			name:     "Happy Path",
			filename: "test-happy-path.json",
			payload:  `{"message": "Hello from AĒR Ingestion!", "status": "raw", "metric_value": 42.5}`,
		},
		{
			name:     "Corrupt Data",
			filename: "test-corrupt-data.json",
			payload:  `{"status": "raw", "metric_value": 99.9}`,
		},
		{
			name:     "Duplicate Data",
			filename: "test-happy-path.json",
			payload:  `{"message": "Hello from AĒR Ingestion!", "status": "raw", "metric_value": 42.5}`,
		},
	}

	for _, tc := range testCases {
		ctxSpan, span := tracer.Start(ctx, fmt.Sprintf("Ingest-%s", tc.name))
		traceID := span.SpanContext().TraceID().String()

		// 1. Write to DB FIRST (Status: pending)
		logErr := s.db.LogDocument(ctxSpan, jobID, tc.filename, traceID)
		if logErr != nil {
			slog.Error("Failed to log pending document metadata. Skipping upload to prevent Dark Data.", "case", tc.name, "error", logErr)
			span.RecordError(logErr)
			span.End()
			continue
		}

		// 2. Upload to MinIO (Bronze Layer)
		uploadErr := s.minio.UploadJSON(ctxSpan, "bronze", tc.filename, tc.payload)

		if uploadErr != nil {
			slog.Error("Failed to upload to bronze", "case", tc.name, "error", uploadErr)
			span.RecordError(uploadErr)

			// Mark as failed in DB so we can audit/retry later
			_ = s.db.UpdateDocumentStatus(ctxSpan, tc.filename, "failed")
		} else {
			// 3. Commit Success: Update DB Status to 'uploaded'
			updateErr := s.db.UpdateDocumentStatus(ctxSpan, tc.filename, "uploaded")
			if updateErr != nil {
				slog.Error("Failed to update document status to uploaded", "error", updateErr)
			} else {
				slog.Info("Successfully ingested and indexed data", "case", tc.name, "object", tc.filename, "trace_id", traceID)
			}
		}

		span.End()
		time.Sleep(2 * time.Second)
	}

	// 4. Mark job as completed
	err = s.db.UpdateJobStatus(ctx, jobID, "completed")
	if err != nil {
		slog.Error("Failed to complete job", "error", err)
	} else {
		slog.Info("Ingestion job marked as completed", "job_id", jobID)
	}

	return nil
}
