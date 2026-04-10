package storage

import (
	"context"
	"fmt"
	"time"
)

// LogDocument records the INTENT to ingest a document (Status: pending).
func (p *PostgresDB) LogDocument(ctx context.Context, jobID int, bronzeKey string, traceID string) error {
	// We use ON CONFLICT DO UPDATE to handle duplicate test cases gracefully and reset them to pending
	query := `
		INSERT INTO documents (job_id, bronze_object_key, trace_id, status)
		VALUES ($1, $2, $3, 'pending')
		ON CONFLICT (bronze_object_key) DO UPDATE SET status = 'pending', trace_id = EXCLUDED.trace_id
	`
	_, err := p.DB.ExecContext(ctx, query, jobID, bronzeKey, traceID)
	if err != nil {
		return fmt.Errorf("failed to log pending document metadata: %w", err)
	}
	return nil
}

// UpdateDocumentStatus updates the document's state after a MinIO upload attempt.
func (p *PostgresDB) UpdateDocumentStatus(ctx context.Context, bronzeKey string, status string) error {
	query := `UPDATE documents SET status = $1 WHERE bronze_object_key = $2`
	_, err := p.DB.ExecContext(ctx, query, status, bronzeKey)
	if err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}
	return nil
}

// DeleteOldDocuments removes documents older than cutoff. Documents must be
// deleted before their parent ingestion_jobs (FK constraint). Returns the
// number of rows deleted.
func (p *PostgresDB) DeleteOldDocuments(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := p.DB.ExecContext(ctx,
		`DELETE FROM documents WHERE ingested_at < $1`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old documents: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to read rows affected for document deletion: %w", err)
	}
	return n, nil
}
