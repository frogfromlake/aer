package storage

import (
	"context"
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

	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.entities (
			timestamp DateTime,
			source String,
			article_id Nullable(String),
			entity_text String,
			entity_label String,
			start_char UInt32,
			end_char UInt32
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create entities table: %v", err)
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

	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence (
			etic_construct String,
			metric_name String,
			language String,
			source_type String,
			equivalence_level String,
			validated_by String,
			validation_date DateTime,
			confidence Float32
		) ENGINE = Memory
	`)
	if err != nil {
		t.Fatalf("failed to create metric_equivalence table: %v", err)
	}

	return store, ctx
}
