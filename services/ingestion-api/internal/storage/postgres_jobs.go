package storage

import (
	"context"
	"fmt"
	"time"
)

// CreateIngestionJob creates a new job record and returns its ID.
func (p *PostgresDB) CreateIngestionJob(ctx context.Context, sourceID int) (int, error) {
	var jobID int
	query := `INSERT INTO ingestion_jobs (source_id, status) VALUES ($1, 'running') RETURNING id`

	err := p.DB.QueryRowContext(ctx, query, sourceID).Scan(&jobID)
	if err != nil {
		return 0, fmt.Errorf("failed to create ingestion job: %w", err)
	}
	return jobID, nil
}

// UpdateJobStatus updates the status and completion time of a job.
func (p *PostgresDB) UpdateJobStatus(ctx context.Context, jobID int, status string) error {
	query := `UPDATE ingestion_jobs SET status = $1, finished_at = $2 WHERE id = $3`
	_, err := p.DB.ExecContext(ctx, query, status, time.Now(), jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}
	return nil
}

// DeleteOldIngestionJobs removes completed or failed ingestion jobs older than
// cutoff that have no remaining child documents. Call after DeleteOldDocuments
// to respect the FK constraint. Returns the number of rows deleted.
func (p *PostgresDB) DeleteOldIngestionJobs(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := p.DB.ExecContext(ctx,
		`DELETE FROM ingestion_jobs
		 WHERE started_at < $1
		   AND status IN ('completed', 'failed')
		   AND id NOT IN (
		       SELECT DISTINCT job_id FROM documents WHERE job_id IS NOT NULL
		   )`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old ingestion jobs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read rows affected for job deletion: %w", err)
	}
	return n, nil
}
