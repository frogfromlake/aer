package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
)

// MetadataStore abstracts the PostgreSQL operations needed by IngestionService.
type MetadataStore interface {
	CreateIngestionJob(ctx context.Context, sourceID int) (int, error)
	UpdateJobStatus(ctx context.Context, jobID int, status string) error
	LogDocument(ctx context.Context, jobID int, key, traceID string) error
	UpdateDocumentStatus(ctx context.Context, key, status string) error
	GetSourceByName(ctx context.Context, name string) (int, string, error)
	Ping(ctx context.Context) error
}

// ObjectStore abstracts the MinIO operations needed by IngestionService.
type ObjectStore interface {
	UploadJSON(ctx context.Context, bucket, key, data string) error
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

// Document represents a single document to be ingested into the bronze layer.
type Document struct {
	Key  string // Object key in MinIO (e.g. "article-123.json")
	Data string // Raw JSON payload
}

// IngestResult summarizes the outcome of a batch ingestion.
type IngestResult struct {
	JobID    int    `json:"job_id"`
	Accepted int    `json:"accepted"`
	Failed   int    `json:"failed"`
	Status   string `json:"status"`
}

// IngestionService orchestrates the data collection and storage processes.
type IngestionService struct {
	db    MetadataStore
	minio ObjectStore
}

// NewIngestionService creates a new core service via Dependency Injection.
func NewIngestionService(db MetadataStore, minio ObjectStore) *IngestionService {
	return &IngestionService{
		db:    db,
		minio: minio,
	}
}

// IngestDocuments processes a batch of documents: tracks them in PostgreSQL,
// uploads them to the MinIO bronze layer, and returns a summary.
func (s *IngestionService) IngestDocuments(ctx context.Context, sourceID int, docs []Document) (*IngestResult, error) {
	if len(docs) == 0 {
		return nil, fmt.Errorf("no documents provided")
	}

	jobID, err := s.db.CreateIngestionJob(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("could not initialize ingestion job: %w", err)
	}
	slog.Info("Created new ingestion job", "job_id", jobID, "document_count", len(docs))

	tracer := otel.Tracer("aer.ingestion")
	errorCount := 0

	for _, doc := range docs {
		ctxSpan, span := tracer.Start(ctx, "IngestDocument")
		traceID := span.SpanContext().TraceID().String()

		// 1. Write to DB FIRST (Status: pending) to prevent dark data
		logErr := s.db.LogDocument(ctxSpan, jobID, doc.Key, traceID)
		if logErr != nil {
			slog.Error("Failed to log pending document metadata. Skipping upload to prevent dark data.",
				"key", doc.Key, "error", logErr)
			span.RecordError(logErr)
			span.End()
			errorCount++
			continue
		}

		// 2. Upload to MinIO (Bronze Layer)
		uploadErr := s.minio.UploadJSON(ctxSpan, "bronze", doc.Key, doc.Data)
		if uploadErr != nil {
			slog.Error("Failed to upload to bronze", "key", doc.Key, "error", uploadErr)
			span.RecordError(uploadErr)
			errorCount++
			_ = s.db.UpdateDocumentStatus(ctxSpan, doc.Key, "failed")
		} else {
			updateErr := s.db.UpdateDocumentStatus(ctxSpan, doc.Key, "uploaded")
			if updateErr != nil {
				slog.Error("Failed to update document status to uploaded", "key", doc.Key, "error", updateErr)
				errorCount++
			} else {
				slog.Info("Successfully ingested document", "key", doc.Key, "trace_id", traceID)
			}
		}

		span.End()
	}

	// Determine final job status
	finalStatus := "completed"
	if errorCount > 0 {
		if errorCount == len(docs) {
			finalStatus = "failed"
		} else {
			finalStatus = "completed_with_errors"
		}
	}

	if err := s.db.UpdateJobStatus(ctx, jobID, finalStatus); err != nil {
		slog.Error("Failed to update job status", "job_id", jobID, "error", err)
	} else {
		slog.Info("Ingestion job finished", "job_id", jobID, "status", finalStatus,
			"errors", errorCount, "total", len(docs))
	}

	return &IngestResult{
		JobID:    jobID,
		Accepted: len(docs) - errorCount,
		Failed:   errorCount,
		Status:   finalStatus,
	}, nil
}

// LookupSource returns the ID and name of a source by its name.
func (s *IngestionService) LookupSource(ctx context.Context, name string) (int, string, error) {
	return s.db.GetSourceByName(ctx, name)
}

// CheckPostgres verifies the PostgreSQL connection is alive.
func (s *IngestionService) CheckPostgres(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return s.db.Ping(ctx)
}

// CheckMinio verifies the MinIO connection by checking if the bronze bucket exists.
func (s *IngestionService) CheckMinio(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	exists, err := s.minio.BucketExists(ctx, "bronze")
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("bronze bucket not found")
	}
	return nil
}
