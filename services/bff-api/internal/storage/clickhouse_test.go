package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/frogfromlake/aer/pkg/testutils"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

func setupTestStore(t *testing.T) (*ClickHouseStorage, context.Context) {
	t.Helper()
	ctx := context.Background()

	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		t.Fatalf("failed to get clickhouse image from compose: %v", err)
	}

	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		t.Fatalf("failed to start clickhouse container: %v", err)
	}
	t.Cleanup(func() {
		if err := chContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate clickhouse container: %v", err)
		}
	})

	host, err := chContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := chContainer.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	addr := host + ":" + port.Port()

	store, err := NewClickHouseStorage(ctx, addr, "aer_admin", "aer_secret", "aer_gold", 10000, 60*time.Second)
	if err != nil {
		t.Fatalf("failed to initialize clickhouse storage: %v", err)
	}

	// Create test tables with Memory engine for fast ephemeral testing
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metrics (
			timestamp DateTime,
			value Float64,
			source String DEFAULT '',
			metric_name String DEFAULT '',
			article_id Nullable(String)
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create metrics table: %v", err)
	}

	// Phase 122c — multi-resolution MV backing tables. Production schema
	// uses three AggregatingMergeTree-backed materialized views populated
	// by triggers on aer_gold.metrics (see migration 000019). For tests
	// we keep the same engine + state-column shape but populate the
	// tables via direct INSERT INTO ... SELECT in TestGetMetrics_Phase122c
	// rather than relying on MV trigger machinery — that keeps the test
	// fast and isolated from MV-trigger edge cases. The query layer
	// being verified (Resolution.queryShape + GetMetrics routing) is
	// unaffected by the population mechanism.
	for table, ttl := range map[string]string{
		"aer_gold.metrics_hourly":  "TTL bucket + INTERVAL 365 DAY",
		"aer_gold.metrics_daily":   "TTL bucket + INTERVAL 1825 DAY",
		"aer_gold.metrics_monthly": "",
	} {
		ddl := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				bucket DateTime,
				source String,
				metric_name String,
				value_avg_state AggregateFunction(avg, Float64),
				sample_count_state AggregateFunction(count)
			) ENGINE = AggregatingMergeTree
			ORDER BY (bucket, source, metric_name)
			%s
		`, table, ttl)
		if err := store.conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("failed to create %s: %v", table, err)
		}
	}

	// Phase 102 / mirrors production migration 000010: ReplacingMergeTree,
	// not Memory. The articles-in-scope subquery in cooccurrence_query reads
	// `FROM aer_gold.entities FINAL` to collapse re-sweep duplicates, which
	// the Memory engine rejects ("Storage Memory doesn't support FINAL") on
	// ClickHouse 26.x. `article_id` is Nullable and leads the ORDER BY (as in
	// production), so allow_nullable_key is required.
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.entities (
			timestamp DateTime,
			source String,
			article_id Nullable(String),
			entity_text String,
			entity_label String,
			start_char UInt32,
			end_char UInt32
		) ENGINE = ReplacingMergeTree()
		ORDER BY (article_id, entity_label, start_char, end_char)
		SETTINGS allow_nullable_key = 1
	`)
	if err != nil {
		t.Fatalf("failed to create entities table: %v", err)
	}

	// Phase 118: entity_links is the LEFT JOIN target of GetEntities and the
	// per-node lookup target for the cooccurrence handler. Test schema mirrors
	// migration 000017 except for engine type (Memory keeps the test fast).
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.entity_links (
			timestamp DateTime,
			article_id String,
			entity_text String,
			entity_label String,
			wikidata_qid String,
			link_confidence Float32,
			link_method LowCardinality(String),
			ingestion_version UInt64
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create entity_links table: %v", err)
	}

	// Phase 123b: wikidata_labels backs the cross-lingual relabel join in the
	// cooccurrence handler. Mirrors migration 000026 (ReplacingMergeTree so the
	// query's FINAL collapses duplicates).
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.wikidata_labels (
			wikidata_qid String,
			language LowCardinality(String),
			label String,
			updated_at DateTime DEFAULT now()
		) ENGINE = ReplacingMergeTree(updated_at)
		ORDER BY (wikidata_qid, language)
	`)
	if err != nil {
		t.Fatalf("failed to create wikidata_labels table: %v", err)
	}

	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.language_detections (
			timestamp DateTime,
			source String,
			article_id Nullable(String),
			detected_language String,
			confidence Float64,
			rank UInt8
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create language_detections table: %v", err)
	}

	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_validity (
			metric_name String,
			context_key String,
			validation_date DateTime,
			alpha_score Float32,
			correlation Float32,
			n_annotated UInt32,
			error_taxonomy String,
			valid_until DateTime
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create metric_validity table: %v", err)
	}

	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_baselines (
			metric_name String,
			source String,
			language String,
			baseline_value Float64,
			baseline_std Float64,
			window_start DateTime,
			window_end DateTime,
			n_documents UInt32,
			compute_date DateTime
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create metric_baselines table: %v", err)
	}

	// Phase 115: mirror production schema. Migration 000008 uses
	// ReplacingMergeTree(validation_date) and migration 000014 adds the
	// `notes` column. Production queries (`GetAvailableMetrics`, the
	// cross-cultural equivalence gates) read FINAL to collapse re-validations,
	// which Memory engine does not support — keep this table on the
	// MergeTree family.
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence (
			etic_construct String,
			metric_name String,
			language String,
			source_type String,
			equivalence_level String,
			validated_by String,
			validation_date DateTime,
			confidence Float32,
			notes String DEFAULT ''
		) ENGINE = ReplacingMergeTree(validation_date)
		ORDER BY (etic_construct, metric_name, language)
	`)
	if err != nil {
		t.Fatalf("failed to create metric_equivalence table: %v", err)
	}

	// Phase 102: entity co-occurrence table for view-mode queries. Uses
	// ReplacingMergeTree to mirror production semantics (the query reads
	// FINAL to collapse re-sweep duplicates) — Memory engine does not
	// support the FINAL modifier.
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.entity_cooccurrences (
			window_start DateTime,
			window_end DateTime,
			source String,
			article_id String,
			entity_a_text String,
			entity_a_label String,
			entity_b_text String,
			entity_b_label String,
			cooccurrence_count UInt32,
			ingestion_version UInt64 DEFAULT 0
		) ENGINE = ReplacingMergeTree(ingestion_version)
		ORDER BY (window_start, source, article_id, entity_a_text, entity_b_text)
	`)
	if err != nil {
		t.Fatalf("failed to create entity_cooccurrences table: %v", err)
	}

	// Phase 103b: silver projection table. Uses ReplacingMergeTree to
	// mirror production semantics (queries read FINAL).
	err = store.conn.Exec(ctx, `
		CREATE DATABASE IF NOT EXISTS aer_silver
	`)
	if err != nil {
		t.Fatalf("failed to create aer_silver database: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_silver.documents (
			timestamp DateTime,
			source String,
			article_id String,
			language String,
			cleaned_text_length UInt32,
			word_count UInt32,
			raw_entity_count UInt32,
			ingestion_version UInt64 DEFAULT 0,
			bronze_object_key String DEFAULT '',
			timestamp_source String DEFAULT ''
		) ENGINE = ReplacingMergeTree(ingestion_version)
		ORDER BY (timestamp, source, article_id)
	`)
	if err != nil {
		t.Fatalf("failed to create silver documents table: %v", err)
	}

	// Phase 120 topic assignments. Mirrors migration 000018 —
	// ReplacingMergeTree, queried with FINAL; topic_id is unique only within a
	// (window_start, language) sweep, which the latest-sweep query relies on.
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.topic_assignments (
			window_start      DateTime,
			window_end        DateTime,
			source            String,
			article_id        String,
			language          String,
			topic_id          Int32,
			topic_label       String,
			topic_confidence  Float32,
			model_hash        String,
			ingestion_version UInt64 DEFAULT 0
		) ENGINE = ReplacingMergeTree(ingestion_version)
		ORDER BY (window_start, source, article_id, language, topic_id)
	`)
	if err != nil {
		t.Fatalf("failed to create topic_assignments table: %v", err)
	}

	// ADR-032 / ADR-036: revision chain. Mirrors migration 000023/000024 —
	// ReplacingMergeTree(ingestion_version) keyed on
	// (article_id, snapshot_at, content_hash). MUST be a MergeTree (not
	// Memory) so GetRevisionActivity's `FINAL` collapses the duplicate
	// versions the re-attempt loop writes when it re-heals an article.
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.article_revisions (
			article_id String,
			source LowCardinality(String),
			discourse_function LowCardinality(String) DEFAULT '',
			snapshot_at DateTime,
			content_hash String,
			prev_content_hash String DEFAULT '',
			revision_index UInt32 DEFAULT 0,
			time_since_prev_hours Float64 DEFAULT 0,
			revision_trigger LowCardinality(String) DEFAULT 'unknown',
			ingestion_version UInt64,
			archive_url String DEFAULT '',
			diff_paragraphs Array(String) DEFAULT [],
			headline_changed Bool DEFAULT false,
			headline_before String DEFAULT '',
			headline_after String DEFAULT '',
			sentiment_delta Float64 DEFAULT 0,
			entities_added Array(String) DEFAULT [],
			entities_removed Array(String) DEFAULT [],
			topic_shift_score Float64 DEFAULT 0,
			deltas_computed Bool DEFAULT false
		) ENGINE = ReplacingMergeTree(ingestion_version)
		ORDER BY (article_id, snapshot_at, content_hash)
	`)
	if err != nil {
		t.Fatalf("failed to create article_revisions table: %v", err)
	}

	return store, ctx
}
