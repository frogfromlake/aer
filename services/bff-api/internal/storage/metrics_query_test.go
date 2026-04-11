package storage

import (
	"context"
	"testing"
	"time"

	"github.com/frogfromlake/aer/pkg/testutils"
	tcclickhouse "github.com/testcontainers/testcontainers-go/modules/clickhouse"
)

func TestGetMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert one point outside of our test range (2 hours ago)
	err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
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

	// Insert a different metric type inside range
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		now.Add(-1*time.Hour), 0.75, "wikipedia", "sentiment_score", "test-article")
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// TEST: GetMetrics without dimension filters (returns all in-range rows, grouped by source+metric)
	results, err := store.GetMetrics(ctx, start, end, nil, nil)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics, got: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results inside time range (no filters), got %d", len(results))
	}

	// TEST: GetMetrics filtered by source
	wikiSource := "wikipedia"
	results, err = store.GetMetrics(ctx, start, end, &wikiSource, nil)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with source filter, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for source=wikipedia, got %d", len(results))
	}

	// TEST: GetMetrics filtered by metricName
	metricName := "word_count"
	results, err = store.GetMetrics(ctx, start, end, nil, &metricName)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with metricName filter, got: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for metric_name=word_count, got %d", len(results))
	}

	// TEST: GetMetrics filtered by both source and metricName
	results, err = store.GetMetrics(ctx, start, end, &wikiSource, &metricName)
	if err != nil {
		t.Fatalf("expected no error from GetMetrics with both filters, got: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for source=wikipedia AND metric_name=word_count, got %d", len(results))
	}

	// TEST: Verify source and metricName are returned
	if results[0].Source != "wikipedia" {
		t.Errorf("expected source=wikipedia, got %q", results[0].Source)
	}
	if results[0].MetricName != "word_count" {
		t.Errorf("expected metricName=word_count, got %q", results[0].MetricName)
	}
}

func TestGetAvailableMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert metrics with different names
	for _, name := range []string{"word_count", "sentiment_score", "word_count", "entity_count"} {
		err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (?, ?, ?, ?)",
			now, 1.0, "test", name)
		if err != nil {
			t.Fatalf("failed to insert metric: %v", err)
		}
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	results, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 distinct metric names, got %d: %v", len(results), results)
	}
	// Ordered alphabetically, all unvalidated (empty metric_validity table)
	expected := []string{"entity_count", "sentiment_score", "word_count"}
	for i, name := range expected {
		if results[i].MetricName != name {
			t.Errorf("expected results[%d].MetricName=%q, got %q", i, name, results[i].MetricName)
		}
		if results[i].ValidationStatus != "unvalidated" {
			t.Errorf("expected results[%d].ValidationStatus=unvalidated, got %q", i, results[i].ValidationStatus)
		}
	}
}

// TestGetAvailableMetrics_CacheHitSkipsQuery verifies that two consecutive calls
// within the TTL window result in only one ClickHouse query.
func TestGetAvailableMetrics_CacheHitSkipsQuery(t *testing.T) {
	// Use a dedicated store with a long TTL so the cache never expires mid-test.
	ctx := context.Background()
	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		t.Fatalf("failed to get clickhouse image: %v", err)
	}
	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		t.Fatalf("failed to start clickhouse container: %v", err)
	}
	t.Cleanup(func() { _ = chContainer.Terminate(ctx) })

	host, _ := chContainer.Host(ctx)
	port, _ := chContainer.MappedPort(ctx, "9000/tcp")
	store, err := NewClickHouseStorage(ctx, host+":"+port.Port(), "aer_admin", "aer_secret", "aer_gold", 10000, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metrics (
			timestamp DateTime, value Float64,
			source String DEFAULT '', metric_name String DEFAULT '',
			article_id Nullable(String)
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_validity (
			metric_name String, context_key String,
			validation_date DateTime, alpha_score Float32,
			correlation Float32, n_annotated UInt32,
			error_taxonomy String, valid_until DateTime
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create metric_validity table: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence (
			etic_construct String, metric_name String, language String,
			source_type String, equivalence_level String, validated_by String,
			validation_date DateTime, confidence Float32
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create metric_equivalence table: %v", err)
	}
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 1.0, 'test', 'word_count')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)

	// First call — cache miss, hits ClickHouse.
	r1, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(r1) != 1 || r1[0].MetricName != "word_count" {
		t.Fatalf("unexpected first result: %v", r1)
	}

	// Drop the underlying tables so any second ClickHouse query would fail.
	if err := store.conn.Exec(ctx, "DROP TABLE aer_gold.metrics"); err != nil {
		t.Fatalf("failed to drop table: %v", err)
	}

	// Second call — must be served from cache without hitting ClickHouse.
	r2, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("second call (expected cache hit): %v", err)
	}
	if len(r2) != 1 || r2[0].MetricName != "word_count" {
		t.Fatalf("unexpected second result: %v", r2)
	}
}

// TestGetAvailableMetrics_CacheExpiry verifies that a call after TTL expiry
// triggers a fresh ClickHouse query.
func TestGetAvailableMetrics_CacheExpiry(t *testing.T) {
	ctx := context.Background()
	chImage, err := testutils.GetImageFromCompose("clickhouse")
	if err != nil {
		t.Fatalf("failed to get clickhouse image: %v", err)
	}
	chContainer, err := tcclickhouse.Run(ctx, chImage,
		tcclickhouse.WithDatabase("aer_gold"),
		tcclickhouse.WithUsername("aer_admin"),
		tcclickhouse.WithPassword("aer_secret"),
	)
	if err != nil {
		t.Fatalf("failed to start clickhouse container: %v", err)
	}
	t.Cleanup(func() { _ = chContainer.Terminate(ctx) })

	host, _ := chContainer.Host(ctx)
	port, _ := chContainer.MappedPort(ctx, "9000/tcp")
	// Use a very short TTL so we can expire it without sleeping long.
	store, err := NewClickHouseStorage(ctx, host+":"+port.Port(), "aer_admin", "aer_secret", "aer_gold", 10000, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metrics (
			timestamp DateTime, value Float64,
			source String DEFAULT '', metric_name String DEFAULT '',
			article_id Nullable(String)
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_validity (
			metric_name String, context_key String,
			validation_date DateTime, alpha_score Float32,
			correlation Float32, n_annotated UInt32,
			error_taxonomy String, valid_until DateTime
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create metric_validity table: %v", err)
	}
	err = store.conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence (
			etic_construct String, metric_name String, language String,
			source_type String, equivalence_level String, validated_by String,
			validation_date DateTime, confidence Float32
		) ENGINE = Memory`)
	if err != nil {
		t.Fatalf("failed to create metric_equivalence table: %v", err)
	}
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 1.0, 'test', 'word_count')")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)

	// Prime the cache.
	if _, err := store.GetAvailableMetrics(ctx, start, end); err != nil {
		t.Fatalf("prime call: %v", err)
	}

	// Insert a new metric name while the cache is still warm.
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 2.0, 'test', 'sentiment_score')")
	if err != nil {
		t.Fatalf("failed to insert second metric: %v", err)
	}

	// Wait for TTL to expire.
	time.Sleep(100 * time.Millisecond)

	// After expiry, should query ClickHouse again and pick up the new metric.
	r2, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("post-expiry call: %v", err)
	}
	if len(r2) != 2 {
		t.Fatalf("expected 2 metrics after cache expiry, got %d: %v", len(r2), r2)
	}
}

func TestCheckBaselineExists(t *testing.T) {
	store, ctx := setupTestStore(t)

	// No baselines — should return false.
	exists, err := store.CheckBaselineExists(ctx, "word_count", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false when no baselines exist")
	}

	// Insert a baseline.
	err = store.conn.Exec(ctx, `INSERT INTO aer_gold.metric_baselines
		(metric_name, source, language, baseline_value, baseline_std, window_start, window_end, n_documents, compute_date)
		VALUES ('word_count', 'tagesschau', 'de', 150.0, 30.0, now() - INTERVAL 30 DAY, now(), 100, now())`)
	if err != nil {
		t.Fatalf("failed to insert baseline: %v", err)
	}

	// Now should return true.
	exists, err = store.CheckBaselineExists(ctx, "word_count", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true after inserting baseline")
	}

	// Filter by source — matching source.
	src := "tagesschau"
	exists, err = store.CheckBaselineExists(ctx, "word_count", &src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true for matching source")
	}

	// Filter by source — non-matching source.
	other := "nonexistent"
	exists, err = store.CheckBaselineExists(ctx, "word_count", &other)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for non-matching source")
	}
}

func TestCheckEquivalenceExists(t *testing.T) {
	store, ctx := setupTestStore(t)

	// No equivalence — should return false.
	exists, err := store.CheckEquivalenceExists(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false when no equivalence entries exist")
	}

	// Insert a temporal-level equivalence (below the threshold).
	err = store.conn.Exec(ctx, `INSERT INTO aer_gold.metric_equivalence
		(etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence)
		VALUES ('evaluative_polarity', 'sentiment_score', 'de', 'rss', 'temporal', 'researcher_a', now(), 0.8)`)
	if err != nil {
		t.Fatalf("failed to insert equivalence: %v", err)
	}

	// temporal is below deviation — should still return false.
	exists, err = store.CheckEquivalenceExists(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected false for temporal-only equivalence")
	}

	// Insert a deviation-level equivalence.
	err = store.conn.Exec(ctx, `INSERT INTO aer_gold.metric_equivalence
		(etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence)
		VALUES ('evaluative_polarity', 'sentiment_score', 'de', 'rss', 'deviation', 'researcher_b', now(), 0.9)`)
	if err != nil {
		t.Fatalf("failed to insert equivalence: %v", err)
	}

	// Now should return true.
	exists, err = store.CheckEquivalenceExists(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected true after inserting deviation-level equivalence")
	}
}

func TestGetNormalizedMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert a metric row.
	err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		now.Add(-1*time.Hour), 180.0, "tagesschau", "word_count", "art-1")
	if err != nil {
		t.Fatalf("failed to insert metric: %v", err)
	}

	// Insert a language detection for the article.
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.language_detections (timestamp, source, article_id, detected_language, confidence, rank) VALUES (?, ?, ?, ?, ?, ?)",
		now.Add(-1*time.Hour), "tagesschau", "art-1", "de", 0.99, 1)
	if err != nil {
		t.Fatalf("failed to insert language detection: %v", err)
	}

	// Insert a baseline: mean=150, std=30.
	err = store.conn.Exec(ctx, `INSERT INTO aer_gold.metric_baselines
		(metric_name, source, language, baseline_value, baseline_std, window_start, window_end, n_documents, compute_date)
		VALUES ('word_count', 'tagesschau', 'de', 150.0, 30.0, ?, ?, 100, ?)`,
		now.Add(-30*24*time.Hour), now, now)
	if err != nil {
		t.Fatalf("failed to insert baseline: %v", err)
	}

	start := now.Add(-2 * time.Hour)
	end := now

	results, err := store.GetNormalizedMetrics(ctx, start, end, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// z-score = (180 - 150) / 30 = 1.0
	if results[0].Value < 0.99 || results[0].Value > 1.01 {
		t.Errorf("expected zscore ~1.0, got %v", results[0].Value)
	}
	if results[0].Source != "tagesschau" {
		t.Errorf("expected source=tagesschau, got %q", results[0].Source)
	}
}

func TestGetAvailableMetrics_IncludesEquivalenceMetadata(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert metrics.
	err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (?, 1.0, 'test', 'sentiment_score')", now)
	if err != nil {
		t.Fatalf("failed to insert metric: %v", err)
	}
	err = store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (?, 1.0, 'test', 'word_count')", now)
	if err != nil {
		t.Fatalf("failed to insert metric: %v", err)
	}

	// Insert equivalence for sentiment_score only.
	err = store.conn.Exec(ctx, `INSERT INTO aer_gold.metric_equivalence
		(etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence)
		VALUES ('evaluative_polarity', 'sentiment_score', 'de', 'rss', 'deviation', 'researcher_a', now(), 0.9)`)
	if err != nil {
		t.Fatalf("failed to insert equivalence: %v", err)
	}

	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)
	results, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(results))
	}

	// sentiment_score should have equivalence metadata.
	var sentiment, wordCount *AvailableMetricRow
	for i := range results {
		switch results[i].MetricName {
		case "sentiment_score":
			sentiment = &results[i]
		case "word_count":
			wordCount = &results[i]
		}
	}
	if sentiment == nil || wordCount == nil {
		t.Fatalf("expected both sentiment_score and word_count in results")
	}
	if sentiment.EticConstruct == nil || *sentiment.EticConstruct != "evaluative_polarity" {
		t.Errorf("expected eticConstruct=evaluative_polarity for sentiment_score")
	}
	if sentiment.EquivalenceLevel == nil || *sentiment.EquivalenceLevel != "deviation" {
		t.Errorf("expected equivalenceLevel=deviation for sentiment_score")
	}
	if wordCount.EticConstruct != nil {
		t.Errorf("expected nil eticConstruct for word_count")
	}
	if wordCount.EquivalenceLevel != nil {
		t.Errorf("expected nil equivalenceLevel for word_count")
	}
}

// TestGetAvailableMetrics_ConcurrentAccess verifies thread safety under concurrent reads.
func TestGetAvailableMetrics_ConcurrentAccess(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC()
	err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (?, 1.0, 'test', 'word_count')", now)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	const goroutines = 50
	errs := make(chan error, goroutines)

	for range goroutines {
		go func() {
			_, err := store.GetAvailableMetrics(ctx, start, end)
			errs <- err
		}()
	}

	for range goroutines {
		if err := <-errs; err != nil {
			t.Errorf("concurrent call error: %v", err)
		}
	}
}
