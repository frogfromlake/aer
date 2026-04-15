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

	db, err := NewPostgresDB(ctx, connStr, PoolConfig{})
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

// Phase 62 — source_classifications schema and query semantics.
//
// The ingestion-api does not directly consume source_classifications, but
// it owns the Testcontainers-based Go integration surface closest to the
// PostgreSQL migrations. These tests verify the behaviors that the
// analysis-worker's get_source_classification() helper depends on:
//   - latest-wins ordering by classification_date
//   - NULL for sources without any classification row
//   - foreign-key integrity against sources(id)
//
// The table DDL mirrors infra/postgres/migrations/000005_source_classifications.up.sql.
func setupSourceClassifications(t *testing.T, db *PostgresDB, ctx context.Context) {
	t.Helper()
	ddl := `
	CREATE TABLE IF NOT EXISTS source_classifications (
		source_id INTEGER REFERENCES sources(id),
		primary_function VARCHAR(30) NOT NULL,
		secondary_function VARCHAR(30),
		function_weights JSONB,
		emic_designation TEXT NOT NULL,
		emic_context TEXT NOT NULL,
		emic_language VARCHAR(10),
		classified_by VARCHAR(100) NOT NULL,
		classification_date DATE NOT NULL,
		review_status VARCHAR(30) DEFAULT 'pending',
		PRIMARY KEY (source_id, classification_date),
		CONSTRAINT chk_primary_function CHECK (
			primary_function IN ('epistemic_authority', 'power_legitimation', 'cohesion_identity', 'subversion_friction')
		),
		CONSTRAINT chk_secondary_function CHECK (
			secondary_function IS NULL OR secondary_function IN ('epistemic_authority', 'power_legitimation', 'cohesion_identity', 'subversion_friction')
		),
		CONSTRAINT chk_review_status CHECK (
			review_status IN ('provisional_engineering', 'pending', 'reviewed', 'contested')
		)
	);`
	if _, err := db.DB.ExecContext(ctx, ddl); err != nil {
		t.Fatalf("failed to create source_classifications: %v", err)
	}
}

// latestClassificationQuery mirrors the analysis-worker's
// get_source_classification() SQL so the Go test validates the same shape.
const latestClassificationQuery = `
	SELECT sc.primary_function, sc.secondary_function, sc.emic_designation
	FROM source_classifications sc
	JOIN sources s ON sc.source_id = s.id
	WHERE s.name = $1
	ORDER BY sc.classification_date DESC
	LIMIT 1
`

func TestSourceClassifications_LatestClassificationReturned(t *testing.T) {
	db, ctx := setupTestDB(t)
	setupSourceClassifications(t, db, ctx)

	var sourceID int
	if err := db.DB.QueryRowContext(ctx, "SELECT id FROM sources WHERE name = 'Test Source'").Scan(&sourceID); err != nil {
		t.Fatalf("failed to look up seeded source: %v", err)
	}

	// Insert an older classification.
	if _, err := db.DB.ExecContext(ctx,
		`INSERT INTO source_classifications
			(source_id, primary_function, secondary_function, emic_designation, emic_context, emic_language, classified_by, classification_date, review_status)
		 VALUES ($1, 'power_legitimation', NULL, 'Old Label', 'ctx', 'de', 'researcher_a', '2025-01-01', 'provisional_engineering')`,
		sourceID,
	); err != nil {
		t.Fatalf("failed to insert old classification: %v", err)
	}
	// Insert a newer classification — should win.
	if _, err := db.DB.ExecContext(ctx,
		`INSERT INTO source_classifications
			(source_id, primary_function, secondary_function, emic_designation, emic_context, emic_language, classified_by, classification_date, review_status)
		 VALUES ($1, 'epistemic_authority', 'power_legitimation', 'New Label', 'ctx', 'de', 'researcher_b', '2026-02-01', 'provisional_engineering')`,
		sourceID,
	); err != nil {
		t.Fatalf("failed to insert newer classification: %v", err)
	}

	var primary, emic string
	var secondary *string
	err := db.DB.QueryRowContext(ctx, latestClassificationQuery, "Test Source").Scan(&primary, &secondary, &emic)
	if err != nil {
		t.Fatalf("latest-classification query failed: %v", err)
	}
	if primary != "epistemic_authority" {
		t.Errorf("expected primary=epistemic_authority (latest), got %q", primary)
	}
	if secondary == nil || *secondary != "power_legitimation" {
		t.Errorf("expected secondary=power_legitimation, got %v", secondary)
	}
	if emic != "New Label" {
		t.Errorf("expected emic=New Label, got %q", emic)
	}
}

func TestSourceClassifications_UnknownSourceYieldsNoRow(t *testing.T) {
	db, ctx := setupTestDB(t)
	setupSourceClassifications(t, db, ctx)

	var primary, emic string
	var secondary *string
	err := db.DB.QueryRowContext(ctx, latestClassificationQuery, "nonexistent-source").Scan(&primary, &secondary, &emic)
	if err == nil {
		t.Errorf("expected sql.ErrNoRows for unknown source, got success")
	}
	// Any error is acceptable (sql.ErrNoRows is the expected one) — we just
	// need the query to return no row rather than a phantom match.
}

func TestSourceClassifications_ForeignKeyIntegrity(t *testing.T) {
	db, ctx := setupTestDB(t)
	setupSourceClassifications(t, db, ctx)

	// Insert against a non-existent source_id — must violate FK.
	_, err := db.DB.ExecContext(ctx,
		`INSERT INTO source_classifications
			(source_id, primary_function, emic_designation, emic_context, emic_language, classified_by, classification_date)
		 VALUES ($1, 'epistemic_authority', 'ghost', 'ctx', 'de', 'researcher_x', '2026-01-01')`,
		99999,
	)
	if err == nil {
		t.Fatal("expected FK violation for unknown source_id, got nil error")
	}
}

func TestSourceClassifications_NullFunctionWeightsAccepted(t *testing.T) {
	db, ctx := setupTestDB(t)
	setupSourceClassifications(t, db, ctx)

	var sourceID int
	if err := db.DB.QueryRowContext(ctx, "SELECT id FROM sources WHERE name = 'Test Source'").Scan(&sourceID); err != nil {
		t.Fatalf("failed to look up seeded source: %v", err)
	}

	// function_weights = NULL is the provisional-engineering default.
	if _, err := db.DB.ExecContext(ctx,
		`INSERT INTO source_classifications
			(source_id, primary_function, secondary_function, function_weights, emic_designation, emic_context, emic_language, classified_by, classification_date, review_status)
		 VALUES ($1, 'epistemic_authority', 'power_legitimation', NULL, 'Label', 'ctx', 'de', 'researcher_a', '2026-01-01', 'provisional_engineering')`,
		sourceID,
	); err != nil {
		t.Fatalf("insert with NULL function_weights failed: %v", err)
	}

	var weights *string
	err := db.DB.QueryRowContext(ctx,
		"SELECT function_weights::text FROM source_classifications WHERE source_id = $1",
		sourceID,
	).Scan(&weights)
	if err != nil {
		t.Fatalf("readback failed: %v", err)
	}
	if weights != nil {
		t.Errorf("expected NULL function_weights, got %v", *weights)
	}
}
