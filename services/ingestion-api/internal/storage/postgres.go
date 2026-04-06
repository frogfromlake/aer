package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/cenkalti/backoff/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresDB struct {
	DB *sql.DB
}

func NewPostgresDB(ctx context.Context, connStr string) (*PostgresDB, error) {
	// 1. Define an operation returning the connection directly (thanks to v5 generics!)
	operation := func() (*sql.DB, error) {
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			return nil, err
		}

		// PingContext ensures the actual network connection is established
		if err = db.PingContext(ctx); err != nil {
			db.Close()
			return nil, err
		}
		return db, nil
	}

	notify := func(err error, d time.Duration) {
		slog.Warn("PostgreSQL not ready, retrying...", "error", err, "backoff", d)
	}

	// v5 uses functional options for MaxElapsedTime
	db, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
		backoff.WithMaxElapsedTime(30*time.Second),
		backoff.WithNotify(notify),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres after retries: %w", err)
	}

	return &PostgresDB{DB: db}, nil
}

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

// GetSourceByName returns the ID and name of a source matching the given name.
func (p *PostgresDB) GetSourceByName(ctx context.Context, name string) (int, string, error) {
	var id int
	var sourceName string
	query := `SELECT id, name FROM sources WHERE name = $1`

	err := p.DB.QueryRowContext(ctx, query, name).Scan(&id, &sourceName)
	if err != nil {
		return 0, "", fmt.Errorf("failed to find source by name: %w", err)
	}
	return id, sourceName, nil
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

// Ping verifies the PostgreSQL connection is alive.
func (p *PostgresDB) Ping(ctx context.Context) error {
	return p.DB.PingContext(ctx)
}
