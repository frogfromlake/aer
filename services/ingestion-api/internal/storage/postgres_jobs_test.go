package storage

import (
	"testing"
	"time"
)

func TestPostgresJobs(t *testing.T) {
	db, ctx := setupTestDB(t)

	t.Run("GetSourceByName happy path", func(t *testing.T) {
		sourceID, sourceName, err := db.GetSourceByName(ctx, "Test Source")
		if err != nil {
			t.Errorf("expected no error looking up source, got %v", err)
		}
		if sourceID <= 0 {
			t.Errorf("expected positive source ID, got %d", sourceID)
		}
		if sourceName != "Test Source" {
			t.Errorf("expected source name 'Test Source', got %q", sourceName)
		}
	})

	t.Run("GetSourceByName not found", func(t *testing.T) {
		_, _, err := db.GetSourceByName(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent source, got nil")
		}
	})

	t.Run("CreateIngestionJob", func(t *testing.T) {
		jobID, err := db.CreateIngestionJob(ctx, 1)
		if err != nil {
			t.Errorf("expected no error creating job, got %v", err)
		}
		if jobID <= 0 {
			t.Errorf("expected positive job ID, got %d", jobID)
		}
	})

	t.Run("UpdateJobStatus", func(t *testing.T) {
		jobID, err := db.CreateIngestionJob(ctx, 1)
		if err != nil {
			t.Fatalf("failed to create job: %v", err)
		}
		err = db.UpdateJobStatus(ctx, jobID, "completed")
		if err != nil {
			t.Errorf("expected no error updating job status, got %v", err)
		}
	})

	t.Run("DeleteOldIngestionJobs removes orphaned completed jobs", func(t *testing.T) {
		jobID, err := db.CreateIngestionJob(ctx, 1)
		if err != nil {
			t.Fatalf("failed to create job: %v", err)
		}
		err = db.UpdateJobStatus(ctx, jobID, "completed")
		if err != nil {
			t.Fatalf("failed to update job status: %v", err)
		}

		futureCutoff := time.Now().Add(24 * time.Hour)
		deleted, err := db.DeleteOldIngestionJobs(ctx, futureCutoff)
		if err != nil {
			t.Errorf("expected no error deleting old jobs, got %v", err)
		}
		if deleted < 1 {
			t.Errorf("expected at least 1 job deleted, got %d", deleted)
		}
	})
}
