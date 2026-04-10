package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	"github.com/frogfromlake/aer/pkg/testutils"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupTestDB(t *testing.T) (*PostgresDB, context.Context) {
	t.Helper()
	ctx := context.Background()

	pgImage, err := testutils.GetImageFromCompose("postgres")
	if err != nil {
		t.Fatalf("failed to get postgres image from compose: %v", err)
	}

	pgContainer, err := postgres.Run(ctx,
		pgImage,
		postgres.WithDatabase("aer_test"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "pgx/v5", func(host string, port nat.Port) string {
				return fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=aer_test sslmode=disable", host, port.Port())
			}).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate pgContainer: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	db, err := NewPostgresDB(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	t.Cleanup(func() { db.DB.Close() })

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
		status VARCHAR(50) DEFAULT 'pending',
		ingested_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	INSERT INTO sources (name, type, url) VALUES ('Test Source', 'test', 'http://localhost');
	`
	if _, err = db.DB.ExecContext(ctx, schema); err != nil {
		t.Fatalf("failed to apply test schema: %v", err)
	}

	return db, ctx
}
