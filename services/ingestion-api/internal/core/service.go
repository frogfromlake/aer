package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel"
)

// StatusUpdateFailures counts cases where a document was uploaded to the
// bronze bucket but the subsequent PostgreSQL status update failed. These
// documents are live in MinIO — the job must not be marked as failed. The
// counter surfaces the inconsistency for operators so they can reconcile.
var StatusUpdateFailures = promauto.NewCounter(prometheus.CounterOpts{
	Name: "ingestion_status_update_failures_total",
	Help: "Documents uploaded to the bronze bucket whose PostgreSQL status update failed.",
})

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
	db     MetadataStore
	minio  ObjectStore
	bucket string
}

// NewIngestionService creates a new core service via Dependency Injection.
// bucket names the MinIO bronze bucket; empty string falls back to "bronze".
func NewIngestionService(db MetadataStore, minio ObjectStore, bucket string) *IngestionService {
	if bucket == "" {
		bucket = "bronze"
	}
	return &IngestionService{
		db:     db,
		minio:  minio,
		bucket: bucket,
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
	// uploadFailures counts documents that never reached the bronze bucket
	// (log-step failure or upload-step failure). Only this counter drives
	// the terminal job status. statusUpdateFailures counts documents that
	// were uploaded successfully but whose PostgreSQL row could not be
	// flipped to "uploaded" — the data is live in MinIO, so the job is
	// still a success. See Phase 77 for the contract.
	uploadFailures := 0
	statusUpdateFailures := 0

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
			uploadFailures++
			continue
		}

		// 2. Upload to MinIO (Bronze Layer)
		uploadErr := s.minio.UploadJSON(ctxSpan, s.bucket, doc.Key, doc.Data)
		if uploadErr != nil {
			slog.Error("Failed to upload to bronze", "key", doc.Key, "error", uploadErr)
			span.RecordError(uploadErr)
			uploadFailures++
			_ = s.db.UpdateDocumentStatus(ctxSpan, doc.Key, "failed")
		} else {
			updateErr := s.db.UpdateDocumentStatus(ctxSpan, doc.Key, "uploaded")
			if updateErr != nil {
				slog.Error("Document uploaded to bronze but PostgreSQL status update failed. Job continues; operator must reconcile.",
					"op", "status_update", "key", doc.Key, "error", updateErr)
				span.RecordError(updateErr)
				statusUpdateFailures++
				StatusUpdateFailures.Inc()
			} else {
				slog.Info("Successfully ingested document", "key", doc.Key, "trace_id", traceID)
			}
		}

		span.End()
	}

	// Terminal job status is derived solely from uploadFailures: a document
	// that is live in MinIO counts as accepted even if its PostgreSQL row
	// lags behind.
	finalStatus := "completed"
	if uploadFailures > 0 {
		if uploadFailures == len(docs) {
			finalStatus = "failed"
		} else {
			finalStatus = "completed_with_errors"
		}
	}

	if err := s.db.UpdateJobStatus(ctx, jobID, finalStatus); err != nil {
		slog.Error("Failed to update job status", "job_id", jobID, "error", err)
	} else {
		slog.Info("Ingestion job finished", "job_id", jobID, "status", finalStatus,
			"upload_failures", uploadFailures,
			"status_update_failures", statusUpdateFailures,
			"total", len(docs))
	}

	return &IngestResult{
		JobID:    jobID,
		Accepted: len(docs) - uploadFailures,
		Failed:   uploadFailures,
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
	exists, err := s.minio.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("bronze bucket %q not found", s.bucket)
	}
	return nil
}
