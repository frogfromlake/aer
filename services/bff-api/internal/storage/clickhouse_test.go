package storage

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/pkg/testutils"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

func TestClickHouseStorage(t *testing.T) {
	ctx := context.Background()

	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		t.Fatalf("failed to get clickhouse image from compose: %v", err)
	}

	// 1. Start ephemeral ClickHouse container
	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		t.Fatalf("failed to start clickhouse container: %v", err)
	}
	defer func() {
		if err := chContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate clickhouse container: %v", err)
		}
	}()

	host, err := chContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	// clickhouse-go uses the native protocol on port 9000
	port, err := chContainer.MappedPort(ctx, "9000/tcp")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	addr := host + ":" + port.Port()

	// 2. Initialize our Adapter
	store, err := NewClickHouseStorage(ctx, addr, "aer_admin", "aer_secret", "aer_gold")
	if err != nil {
		t.Fatalf("failed to initialize clickhouse storage: %v", err)
	}

	// 3. Apply test schema (We use Memory engine for fast ephemeral testing)
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
		t.Fatalf("failed to create test table: %v", err)
	}

	// 4. Setup Dummy Data
	now := time.Now().UTC().Truncate(time.Second) // ClickHouse DateTime resolution is in seconds

	// Insert one point outside of our test range (2 hours ago)
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		now.Add(-2*time.Hour), 10.5, "wikipedia", "word_count", "outside-article")
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Insert one point INSIDE our test range (1 hour ago) — wikipedia source
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		now.Add(-1*time.Hour), 42.0, "wikipedia", "word_count", "test-article")
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	// Insert another point INSIDE our test range — different source
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		now.Add(-30*time.Minute), 99.0, "newsapi", "word_count", "news-article")
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// 5a. TEST: GetMetrics without dimension filters (returns all in-range rows)
	results, err := store.GetMetrics(ctx, start, end, nil, nil)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results inside time range (no filters), got %d", len(results))
	}

	// 5b. TEST: GetMetrics filtered by source
	wikiSource := "wikipedia"
	results, err = store.GetMetrics(ctx, start, end, &wikiSource, nil)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with source filter, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for source=wikipedia, got %d", len(results))
	}
	if results[0].Value != 42.0 {
		t.Errorf("expected value 42.0, got %f", results[0].Value)
	}

	// 5c. TEST: GetMetrics filtered by metricName
	metricName := "word_count"
	results, err = store.GetMetrics(ctx, start, end, nil, &metricName)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with metricName filter, got: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results for metric_name=word_count, got %d", len(results))
	}

	// 5d. TEST: GetMetrics filtered by both source and metricName
	results, err = store.GetMetrics(ctx, start, end, &wikiSource, &metricName)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with both filters, got: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for source=wikipedia AND metric_name=word_count, got %d", len(results))
	}
}
