package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// This file holds shared test fixtures used across the storage suite: the
// extra ClickHouse tables that setupTestStore does not create (article_metadata,
// metadata_coverage) and a minimal Postgres `sources` schema for the PG-backed
// stores. Keeping them here lets each query-test file stay focused on assertions.

// createArticleMetadataTable creates aer_gold.article_metadata mirroring
// migration 000030 (ReplacingMergeTree, queried with FINAL). The view-mode
// categorical/cross-tab/sankey/parallel queries read this table.
func createArticleMetadataTable(t *testing.T, ctx context.Context, s *ClickHouseStorage) {
	t.Helper()
	err := s.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.article_metadata (
			timestamp DateTime,
			source LowCardinality(String),
			article_id String,
			field LowCardinality(String),
			value Array(String),
			discourse_function String DEFAULT '',
			timestamp_source String DEFAULT '',
			ingestion_version UInt64 DEFAULT 0
		) ENGINE = ReplacingMergeTree(ingestion_version)
		ORDER BY (source, field, article_id)
	`)
	if err != nil {
		t.Fatalf("create article_metadata table: %v", err)
	}
}

// metadataValueRow is a single (article, field) row for article_metadata.
type metadataValueRow struct {
	ts       time.Time
	source   string
	article  string
	field    string
	values   []string
	tsSource string
}

// seedArticleMetadata bulk-inserts the given article_metadata rows.
func seedArticleMetadata(t *testing.T, ctx context.Context, s *ClickHouseStorage, rows []metadataValueRow) {
	t.Helper()
	batch, err := s.conn.PrepareBatch(ctx,
		"INSERT INTO aer_gold.article_metadata (timestamp, source, article_id, field, value, timestamp_source, ingestion_version)")
	if err != nil {
		t.Fatalf("prepare article_metadata batch: %v", err)
	}
	for _, r := range rows {
		if err := batch.Append(r.ts, r.source, r.article, r.field, r.values, r.tsSource, uint64(1000)); err != nil {
			t.Fatalf("append article_metadata row: %v", err)
		}
	}
	if err := batch.Send(); err != nil {
		t.Fatalf("send article_metadata batch: %v", err)
	}
}

// createMetadataCoverageTable creates the aer_gold.metadata_coverage
// AggregatingMergeTree backing table (mirrors migration 000022's MV shape;
// a plain table is used so the test can seed -State columns directly).
func createMetadataCoverageTable(t *testing.T, ctx context.Context, s *ClickHouseStorage) {
	t.Helper()
	err := s.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metadata_coverage (
			source LowCardinality(String),
			field LowCardinality(String),
			method LowCardinality(String),
			articles_state AggregateFunction(uniqExact, String),
			last_seen_state AggregateFunction(max, DateTime)
		) ENGINE = AggregatingMergeTree
		ORDER BY (source, field, method)
	`)
	if err != nil {
		t.Fatalf("create metadata_coverage table: %v", err)
	}
}

// seedMetadataCoverage inserts one coverage cell by replaying the article ids
// through uniqExactState (so uniqExactMerge at read time yields their distinct
// count) and the timestamp through maxState.
func seedMetadataCoverage(t *testing.T, ctx context.Context, s *ClickHouseStorage,
	source, field, method string, articleIDs []string, lastSeen time.Time) {
	t.Helper()
	// Build a VALUES list of (article_id) tuples to fold into the state.
	tuples := ""
	args := []any{source, field, method}
	for i, id := range articleIDs {
		if i > 0 {
			tuples += " UNION ALL "
		}
		tuples += fmt.Sprintf("SELECT $%d AS aid", len(args)+1)
		args = append(args, id)
	}
	args = append(args, lastSeen)
	query := fmt.Sprintf(`
		INSERT INTO aer_gold.metadata_coverage
		SELECT $1 AS source, $2 AS field, $3 AS method,
		       uniqExactState(aid) AS articles_state,
		       maxState($%d) AS last_seen_state
		FROM (%s)
	`, len(args), tuples)
	if err := s.conn.Exec(ctx, query, args...); err != nil {
		t.Fatalf("seed metadata_coverage (%s/%s/%s): %v", source, field, method, err)
	}
}

// ---------------------------------------------------------------------------
// Postgres `sources` fixture (for DossierStore / SourceStore / discovery).
// ---------------------------------------------------------------------------

// pgSourcesCounter hands each setupSourcesDB call a unique database name.
var pgSourcesCounter int64

// setupSourcesDB creates a fresh Postgres database on the shared container with
// the minimal sources / source_classifications / crawler_discovery_* schema the
// PG-backed stores read. Mirrors the production column shape (migrations 001,
// 005, 007, 011, 018) without the seed inserts so tests own their fixtures.
func setupSourcesDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	ctx := context.Background()

	dbName := fmt.Sprintf("srctest_%d", atomic.AddInt64(&pgSourcesCounter, 1))
	adminDSN := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=aer_test sslmode=disable",
		sharedPGHost, sharedPGPort)
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin postgres: %v", err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+dbName); err != nil {
		_ = admin.Close()
		t.Fatalf("create test database %s: %v", dbName, err)
	}
	t.Cleanup(func() {
		_, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+dbName+" WITH (FORCE)")
		_ = admin.Close()
	})

	testDSN := fmt.Sprintf("host=%s port=%s user=testuser password=testpass dbname=%s sslmode=disable",
		sharedPGHost, sharedPGPort, dbName)
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		t.Fatalf("open pgx: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	schema := []string{
		`CREATE TABLE sources (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(50) NOT NULL,
			url VARCHAR(500) NOT NULL,
			documentation_url VARCHAR(255),
			silver_eligible BOOLEAN NOT NULL DEFAULT false,
			silver_review_reviewer VARCHAR(255),
			silver_review_date DATE,
			silver_review_rationale TEXT,
			silver_review_reference VARCHAR(500),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE source_classifications (
			source_id INTEGER REFERENCES sources(id),
			primary_function VARCHAR(30) NOT NULL,
			secondary_function VARCHAR(30),
			emic_designation TEXT NOT NULL,
			emic_context TEXT NOT NULL,
			classified_by VARCHAR(100) NOT NULL,
			classification_date DATE NOT NULL,
			PRIMARY KEY (source_id, classification_date)
		)`,
		`CREATE TABLE crawler_discovery_runs (
			run_id UUID PRIMARY KEY,
			source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
			channel TEXT NOT NULL,
			urls_discovered INTEGER NOT NULL,
			urls_after_dedup INTEGER NOT NULL,
			run_started_at TIMESTAMPTZ NOT NULL,
			run_completed_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE TABLE crawler_discovery_alerts (
			source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
			alert_type TEXT NOT NULL,
			first_observed_at TIMESTAMPTZ NOT NULL,
			last_observed_at TIMESTAMPTZ NOT NULL,
			consecutive_runs INTEGER NOT NULL,
			expected_floor INTEGER NOT NULL,
			last_urls_observed INTEGER NOT NULL,
			PRIMARY KEY (source_id, alert_type)
		)`,
	}
	for _, ddl := range schema {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("apply sources schema: %v", err)
		}
	}
	return db, ctx
}

// insertSource adds a source row and returns its generated id.
func insertSource(t *testing.T, db *sql.DB, ctx context.Context, name, sourceType, url string, silverEligible bool) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO sources (name, type, url, documentation_url, silver_eligible)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		name, sourceType, url, "docs/"+name+".md", silverEligible).Scan(&id)
	if err != nil {
		t.Fatalf("insert source %q: %v", name, err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Shared ClickHouse seed helpers used across the query-test files.
// ---------------------------------------------------------------------------

// mustParse parses an RFC3339 timestamp or fails the test.
func mustParse(t *testing.T, raw string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return v
}

// bulkInsert appends every row to a prepared batch and sends it.
func bulkInsert(ctx contextOnly, s *ClickHouseStorage, table string, columns []string, rows [][]any) error {
	batch, err := s.conn.PrepareBatch(ctx, "INSERT INTO "+table+" ("+joinCols(columns)+")")
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.Append(r...); err != nil {
			return err
		}
	}
	return batch.Send()
}

func joinCols(cols []string) string { return strings.Join(cols, ", ") }

func strPtrTest(s string) *string { return &s }

// hasContext lets seed helpers accept either the storage-test wrapper or a raw
// context without churning the call sites.
type hasContext interface{ Ctx() contextOnly }

type contextWrap struct{ inner contextOnly }

func (c contextWrap) Ctx() contextOnly { return c.inner }

// contextOnly is a compatibility alias matching context.Context's method set,
// avoiding a context import in the thin seed wrappers.
type contextOnly = interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// coocCols / metricCols are the column lists the view-mode seeders reuse.
var (
	coocCols = []string{
		"window_start", "window_end", "source", "article_id",
		"entity_a_text", "entity_a_label", "entity_b_text", "entity_b_label",
		"cooccurrence_count", "ingestion_version",
	}
	metricCols     = []string{"timestamp", "value", "source", "metric_name", "article_id"}
	entityLinkCols = []string{
		"timestamp", "article_id", "entity_text", "entity_label",
		"wikidata_qid", "link_confidence", "link_method", "ingestion_version",
	}
)

// seedCooc bulk-inserts entity_cooccurrence rows from the shared column list.
func seedCooc(t *testing.T, ctx contextOnly, s *ClickHouseStorage, rows [][]any) {
	t.Helper()
	if err := bulkInsert(ctx, s, "aer_gold.entity_cooccurrences", coocCols, rows); err != nil {
		t.Fatalf("seed cooccurrences: %v", err)
	}
}
