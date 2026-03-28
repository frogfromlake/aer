package storage

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresStorage(t *testing.T) {
	ctx := context.Background()

	// 1. Start ephemeral PostgreSQL container
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("aer_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(10*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	// Ensure the container is destroyed after the test
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate pgContainer: %v", err)
		}
	}()

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// 2. Initialize our Adapter
	db, err := NewPostgresDB(connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	defer db.DB.Close()

	// 3. Apply schema directly for test isolation
	schema := `
	CREATE TABLE IF NOT EXISTS sources (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		type VARCHAR(50) NOT NULL,
		url VARCHAR(500) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS ingestion_jobs (
		id SERIAL PRIMARY KEY,
		source_id INTEGER REFERENCES sources(id),
		status VARCHAR(50) NOT NULL,
		started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
		finished_at TIMESTAMP WITH TIME ZONE
	);
	CREATE TABLE IF NOT EXISTS documents (
		id SERIAL PRIMARY KEY,
		job_id INTEGER REFERENCES ingestion_jobs(id),
		bronze_object_key VARCHAR(500) UNIQUE NOT NULL,
		trace_id VARCHAR(255),
		ingested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO sources (name, type, url) VALUES ('Test Source', 'test', 'http://localhost');
	`
	if _, err = db.DB.ExecContext(ctx, schema); err != nil {
		t.Fatalf("failed to apply test schema: %v", err)
	}

	// 4. TEST: Create Ingestion Job
	jobID, err := db.CreateIngestionJob(ctx, 1)
	if err != nil {
		t.Errorf("expected no error creating job, got %v", err)
	}
	if jobID <= 0 {
		t.Errorf("expected positive job ID, got %d", jobID)
	}

	// 5. TEST: Log Document (Happy Path)
	err = db.LogDocument(ctx, jobID, "test-bronze-path.json", "trace-12345")
	if err != nil {
		t.Errorf("expected no error logging document, got %v", err)
	}

	// 6. TEST: Log Document (Idempotency / ON CONFLICT DO NOTHING)
	err = db.LogDocument(ctx, jobID, "test-bronze-path.json", "trace-99999")
	if err != nil {
		t.Errorf("expected duplicate log to be ignored safely, got error: %v", err)
	}
}
