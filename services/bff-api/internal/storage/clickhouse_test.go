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

	store, err := NewClickHouseStorage(ctx, addr, "aer_admin", "aer_secret", "aer_gold", 10000)
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

	return store, ctx
}

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

func TestGetEntities(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert test entities
	for _, e := range []struct {
		ts     time.Time
		source string
		text   string
		label  string
	}{
		{now.Add(-1 * time.Hour), "tagesschau", "Bundesregierung", "ORG"},
		{now.Add(-1 * time.Hour), "tagesschau", "Bundesregierung", "ORG"},
		{now.Add(-30 * time.Minute), "bundesregierung", "Bundesregierung", "ORG"},
		{now.Add(-1 * time.Hour), "tagesschau", "Berlin", "LOC"},
		{now.Add(-3 * time.Hour), "tagesschau", "OutOfRange", "PER"}, // outside range
	} {
		err := store.conn.Exec(ctx, "INSERT INTO aer_gold.entities (timestamp, source, article_id, entity_text, entity_label, start_char, end_char) VALUES (?, ?, ?, ?, ?, ?, ?)",
			e.ts, e.source, nil, e.text, e.label, 0, 0)
		if err != nil {
			t.Fatalf("failed to insert entity: %v", err)
		}
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// TEST: GetEntities without filters
	results, err := store.GetEntities(ctx, start, end, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 distinct entities, got %d", len(results))
	}
	// Ordered by count DESC — Bundesregierung (3) then Berlin (1)
	if results[0].EntityText != "Bundesregierung" {
		t.Errorf("expected first entity Bundesregierung, got %q", results[0].EntityText)
	}
	if results[0].Count != 3 {
		t.Errorf("expected count 3, got %d", results[0].Count)
	}
	if len(results[0].Sources) != 2 {
		t.Errorf("expected 2 distinct sources for Bundesregierung, got %d", len(results[0].Sources))
	}

	// TEST: GetEntities filtered by label
	orgLabel := "ORG"
	results, err = store.GetEntities(ctx, start, end, nil, &orgLabel, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entity for label=ORG, got %d", len(results))
	}

	// TEST: GetEntities filtered by source
	tagesschauSrc := "tagesschau"
	results, err = store.GetEntities(ctx, start, end, &tagesschauSrc, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 entities for source=tagesschau, got %d", len(results))
	}

	// TEST: GetEntities respects limit
	results, err = store.GetEntities(ctx, start, end, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 entity with limit=1, got %d", len(results))
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

	results, err := store.GetAvailableMetrics(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 distinct metric names, got %d: %v", len(results), results)
	}
	// Ordered alphabetically
	expected := []string{"entity_count", "sentiment_score", "word_count"}
	for i, name := range expected {
		if results[i] != name {
			t.Errorf("expected results[%d]=%q, got %q", i, name, results[i])
		}
	}
}

func TestGetLanguageDetections(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now().UTC().Truncate(time.Second)

	// Insert test language detections (rank 1 = top candidate)
	for _, d := range []struct {
		ts     time.Time
		source string
		lang   string
		conf   float64
		rank   uint8
	}{
		{now.Add(-1 * time.Hour), "tagesschau", "de", 0.9999, 1},
		{now.Add(-1 * time.Hour), "tagesschau", "en", 0.0001, 2},    // rank 2, should be excluded from aggregation
		{now.Add(-30 * time.Minute), "bundesregierung", "de", 0.985, 1},
		{now.Add(-30 * time.Minute), "tagesschau", "en", 0.92, 1},
		{now.Add(-3 * time.Hour), "tagesschau", "de", 0.99, 1},      // outside range
	} {
		err := store.conn.Exec(ctx, "INSERT INTO aer_gold.language_detections (timestamp, source, article_id, detected_language, confidence, rank) VALUES (?, ?, ?, ?, ?, ?)",
			d.ts, d.source, nil, d.lang, d.conf, d.rank)
		if err != nil {
			t.Fatalf("failed to insert language detection: %v", err)
		}
	}

	start := now.Add(-90 * time.Minute)
	end := now

	// TEST: GetLanguageDetections without filters (rank=1 only)
	results, err := store.GetLanguageDetections(ctx, start, end, nil, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 distinct languages, got %d", len(results))
	}
	// Ordered by count DESC — de (2) then en (1)
	if results[0].DetectedLanguage != "de" {
		t.Errorf("expected first language de, got %q", results[0].DetectedLanguage)
	}
	if results[0].Count != 2 {
		t.Errorf("expected count 2, got %d", results[0].Count)
	}
	if len(results[0].Sources) != 2 {
		t.Errorf("expected 2 distinct sources for de, got %d", len(results[0].Sources))
	}

	// TEST: GetLanguageDetections filtered by language
	deLang := "de"
	results, err = store.GetLanguageDetections(ctx, start, end, nil, &deLang, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 language for language=de, got %d", len(results))
	}

	// TEST: GetLanguageDetections filtered by source
	tagesschauSrc := "tagesschau"
	results, err = store.GetLanguageDetections(ctx, start, end, &tagesschauSrc, nil, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 languages for source=tagesschau, got %d", len(results))
	}

	// TEST: GetLanguageDetections respects limit
	results, err = store.GetLanguageDetections(ctx, start, end, nil, nil, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 language with limit=1, got %d", len(results))
	}
}
