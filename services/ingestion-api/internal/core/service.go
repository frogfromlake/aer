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
	slog.Info("Starting AĒR Ingestion Core Logic (Poison Pill Test)...")

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
			// Dieses JSON hat kein "message" Feld und wird die Pydantic/Python-Validierung nicht bestehen!
			payload:  `{"status": "raw", "metric_value": 99.9}`,
		},
		{
			name:     "Duplicate Data",
			filename: "test-happy-path.json", // Gleicher Dateiname wie im Happy Path
			payload:  `{"message": "Hello from AĒR Ingestion!", "status": "raw", "metric_value": 42.5}`,
		},
	}

	for _, tc := range testCases {
		// Für jeden Upload starten wir einen eigenen kleinen Trace-Span
		ctxSpan, span := tracer.Start(ctx, fmt.Sprintf("Ingest-%s", tc.name))

		err := s.minio.UploadJSON(ctxSpan, "bronze", tc.filename, tc.payload)
		if err != nil {
			slog.Error("Failed to upload to bronze", "case", tc.name, "error", err)
			span.RecordError(err)
		} else {
			slog.Info("Successfully uploaded data to Bronze layer", "case", tc.name, "object", tc.filename)
		}

		span.End()
		
		// Kurze Pause, damit NATS die Events sauber nacheinander an den Python-Worker schickt
		time.Sleep(2 * time.Second)
	}

	slog.Info("All test cases uploaded.")
	return nil
}