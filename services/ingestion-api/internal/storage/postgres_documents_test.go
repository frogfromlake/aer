package storage

import (
	"testing"
	"time"
)

func TestPostgresDocuments(t *testing.T) {
	db, ctx := setupTestDB(t)

	// Create a job to attach documents to
	jobID, err := db.CreateIngestionJob(ctx, 1)
	if err != nil {
		t.Fatalf("failed to create job for document tests: %v", err)
	}

	t.Run("LogDocument happy path", func(t *testing.T) {
		err := db.LogDocument(ctx, jobID, "test-bronze-path.json", "trace-12345")
		if err != nil {
			t.Errorf("expected no error logging document, got %v", err)
		}
	})

	t.Run("LogDocument idempotency", func(t *testing.T) {
		err := db.LogDocument(ctx, jobID, "test-bronze-path.json", "trace-99999")
		if err != nil {
			t.Errorf("expected duplicate log to update safely, got error: %v", err)
		}
	})

	t.Run("UpdateDocumentStatus", func(t *testing.T) {
		err := db.UpdateDocumentStatus(ctx, "test-bronze-path.json", "uploaded")
		if err != nil {
			t.Errorf("expected no error updating document status, got %v", err)
		}

		var status string
		err = db.DB.QueryRowContext(ctx, "SELECT status FROM documents WHERE bronze_object_key = $1", "test-bronze-path.json").Scan(&status)
		if err != nil {
			t.Fatalf("failed to query document status: %v", err)
		}
		if status != "uploaded" {
			t.Errorf("expected status 'uploaded', got %s", status)
		}
	})

	t.Run("DeleteOldDocuments no-op with past cutoff", func(t *testing.T) {
		err := db.LogDocument(ctx, jobID, "recent-doc.json", "trace-recent")
		if err != nil {
			t.Errorf("expected no error logging recent document, got %v", err)
		}

		pastCutoff := time.Now().Add(-30 * 24 * time.Hour)
		deleted, err := db.DeleteOldDocuments(ctx, pastCutoff)
		if err != nil {
			t.Errorf("expected no error deleting old documents, got %v", err)
		}
		if deleted != 0 {
			t.Errorf("expected 0 documents deleted with past cutoff, got %d", deleted)
		}
	})

	t.Run("DeleteOldDocuments removes with future cutoff", func(t *testing.T) {
		futureCutoff := time.Now().Add(24 * time.Hour)
		deleted, err := db.DeleteOldDocuments(ctx, futureCutoff)
		if err != nil {
			t.Errorf("expected no error deleting old documents, got %v", err)
		}
		if deleted < 1 {
			t.Errorf("expected at least 1 document deleted with future cutoff, got %d", deleted)
		}
	})
}
