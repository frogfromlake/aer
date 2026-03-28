package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// PostgresDB is a wrapper around the SQL database connection.
type PostgresDB struct {
	DB *sql.DB
}

// NewPostgresDB establishes a connection to PostgreSQL and verifies it via Ping.
func NewPostgresDB(connStr string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
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

// LogDocument records a successfully ingested document in the metadata index.
func (p *PostgresDB) LogDocument(ctx context.Context, jobID int, bronzeKey string, traceID string) error {
	// We use ON CONFLICT DO NOTHING to gracefully handle our duplicate test cases
	query := `
		INSERT INTO documents (job_id, bronze_object_key, trace_id) 
		VALUES ($1, $2, $3)
		ON CONFLICT (bronze_object_key) DO NOTHING
	`
	_, err := p.DB.ExecContext(ctx, query, jobID, bronzeKey, traceID)
	if err != nil {
		return fmt.Errorf("failed to log document metadata: %w", err)
	}
	return nil
}
