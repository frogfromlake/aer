package storage

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Shared metric-test helpers.
// ---------------------------------------------------------------------------

// seedMetric inserts one aer_gold.metrics row.
func seedMetric(t *testing.T, ctx context.Context, s *ClickHouseStorage, ts time.Time, value float64, source, metric, articleID string) {
	t.Helper()
	if err := s.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name, article_id) VALUES (?, ?, ?, ?, ?)",
		ts, value, source, metric, articleID); err != nil {
		t.Fatalf("seed metric (%s/%s): %v", source, metric, err)
	}
}

// seedBaseline inserts one metric_baselines row.
func seedBaseline(t *testing.T, ctx context.Context, s *ClickHouseStorage, metric, source, lang string, mean, std float64) {
	t.Helper()
	if err := s.conn.Exec(ctx, `INSERT INTO aer_gold.metric_baselines
		(metric_name, source, language, baseline_value, baseline_std, window_start, window_end, n_documents, compute_date)
		VALUES (?, ?, ?, ?, ?, now() - INTERVAL 30 DAY, now(), 100, now())`,
		metric, source, lang, mean, std); err != nil {
		t.Fatalf("seed baseline: %v", err)
	}
}

// seedLangForArticle inserts a rank-1 language detection for an article.
func seedLangForArticle(t *testing.T, ctx context.Context, s *ClickHouseStorage, ts time.Time, source, articleID, lang string) {
	t.Helper()
	if err := s.conn.Exec(ctx, "INSERT INTO aer_gold.language_detections (timestamp, source, article_id, detected_language, confidence, rank) VALUES (?, ?, ?, ?, ?, ?)",
		ts, source, articleID, lang, 0.99, 1); err != nil {
		t.Fatalf("seed language detection: %v", err)
	}
}

// newCacheStore wires a fresh store with the given metrics-cache TTL plus the
// minimal tables GetAvailableMetrics reads (metrics, metric_validity,
// metric_equivalence). Used by the cache-behaviour tests.
func newCacheStore(t *testing.T, ctx context.Context, ttl time.Duration) *ClickHouseStorage {
	t.Helper()
	store := newSharedCHStore(t, ctx, ttl)
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS aer_gold.metrics (
			timestamp DateTime, value Float64,
			source String DEFAULT '', metric_name String DEFAULT '',
			article_id Nullable(String)) ENGINE = Memory`,
		`CREATE TABLE IF NOT EXISTS aer_gold.metric_validity (
			metric_name String, context_key String,
			validation_date DateTime, alpha_score Float32,
			correlation Float32, n_annotated UInt32,
			error_taxonomy String, valid_until DateTime) ENGINE = Memory`,
		`CREATE TABLE IF NOT EXISTS aer_gold.metric_equivalence (
			etic_construct String, metric_name String, language String,
			source_type String, equivalence_level String, validated_by String,
			validation_date DateTime, confidence Float32, notes String DEFAULT '')
			ENGINE = ReplacingMergeTree(validation_date)
			ORDER BY (etic_construct, metric_name, language)`,
	}
	for _, ddl := range ddls {
		if err := store.conn.Exec(ctx, ddl); err != nil {
			t.Fatalf("create cache-store table: %v", err)
		}
	}
	return store
}

// ---------------------------------------------------------------------------
// GetMetrics
// ---------------------------------------------------------------------------

func TestGetMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now.Add(-2*time.Hour), 10.5, "wikipedia", "word_count", "outside")      // out of range
	seedMetric(t, ctx, store, now.Add(-1*time.Hour), 42.0, "wikipedia", "word_count", "test-article") // in range
	seedMetric(t, ctx, store, now.Add(-30*time.Minute), 99.0, "newsapi", "word_count", "news-article")
	seedMetric(t, ctx, store, now.Add(-1*time.Hour), 0.75, "wikipedia", "sentiment_score", "test-article")

	start, end := now.Add(-90*time.Minute), now

	all, err := store.GetMetrics(ctx, start, end, nil, nil, ResolutionFiveMinute)
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 results inside time range, got %d", len(all))
	}

	bySource, err := store.GetMetrics(ctx, start, end, []string{"wikipedia"}, nil, ResolutionFiveMinute)
	if err != nil || len(bySource) != 2 {
		t.Fatalf("expected 2 wikipedia results, got %d (err %v)", len(bySource), err)
	}

	metricName := "word_count"
	byMetric, err := store.GetMetrics(ctx, start, end, nil, &metricName, ResolutionFiveMinute)
	if err != nil || len(byMetric) != 2 {
		t.Fatalf("expected 2 word_count results, got %d (err %v)", len(byMetric), err)
	}

	both, err := store.GetMetrics(ctx, start, end, []string{"wikipedia"}, &metricName, ResolutionFiveMinute)
	if err != nil || len(both) != 1 {
		t.Fatalf("expected 1 wikipedia/word_count result, got %d (err %v)", len(both), err)
	}
	if both[0].Source != "wikipedia" || both[0].MetricName != "word_count" {
		t.Errorf("projection mismatch: %+v", both[0])
	}
}

func TestGetAvailableMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	for _, name := range []string{"word_count", "sentiment_score", "word_count", "entity_count"} {
		seedMetric(t, ctx, store, now, 1.0, "test", name, "")
	}

	results, err := store.GetAvailableMetrics(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 distinct metric names, got %d: %v", len(results), results)
	}
	for i, name := range []string{"entity_count", "sentiment_score", "word_count"} { // alphabetical
		if results[i].MetricName != name {
			t.Errorf("results[%d].MetricName = %q, want %q", i, results[i].MetricName, name)
		}
		if results[i].ValidationStatus != "unvalidated" {
			t.Errorf("results[%d].ValidationStatus = %q, want unvalidated", i, results[i].ValidationStatus)
		}
	}
}

// TestGetAvailableMetrics_CacheHitSkipsQuery verifies the second call within the
// TTL is served from cache (dropping the table mid-test would break a real query).
func TestGetAvailableMetrics_CacheHitSkipsQuery(t *testing.T) {
	ctx := context.Background()
	store := newCacheStore(t, ctx, 5*time.Minute)
	if err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 1.0, 'test', 'word_count')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)

	r1, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil || len(r1) != 1 || r1[0].MetricName != "word_count" {
		t.Fatalf("first call: %v / %v", r1, err)
	}
	// Drop the table — any second ClickHouse query would now fail.
	if err := store.conn.Exec(ctx, "DROP TABLE aer_gold.metrics"); err != nil {
		t.Fatalf("drop: %v", err)
	}
	r2, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil || len(r2) != 1 || r2[0].MetricName != "word_count" {
		t.Fatalf("second call (expected cache hit): %v / %v", r2, err)
	}
}

// TestGetAvailableMetrics_CacheExpiry verifies a post-TTL call re-queries.
func TestGetAvailableMetrics_CacheExpiry(t *testing.T) {
	ctx := context.Background()
	store := newCacheStore(t, ctx, 50*time.Millisecond)
	if err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 1.0, 'test', 'word_count')"); err != nil {
		t.Fatalf("insert: %v", err)
	}
	now := time.Now().UTC()
	start, end := now.Add(-time.Hour), now.Add(time.Hour)
	if _, err := store.GetAvailableMetrics(ctx, start, end); err != nil {
		t.Fatalf("prime: %v", err)
	}
	if err := store.conn.Exec(ctx, "INSERT INTO aer_gold.metrics (timestamp, value, source, metric_name) VALUES (now(), 2.0, 'test', 'sentiment_score')"); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	time.Sleep(100 * time.Millisecond) // let TTL expire

	r2, err := store.GetAvailableMetrics(ctx, start, end)
	if err != nil {
		t.Fatalf("post-expiry call: %v", err)
	}
	if len(r2) != 2 {
		t.Fatalf("expected 2 metrics after cache expiry, got %d: %v", len(r2), r2)
	}
}

// ---------------------------------------------------------------------------
// Baseline / equivalence existence gates.
// ---------------------------------------------------------------------------

func TestCheckBaselineExists(t *testing.T) {
	store, ctx := setupTestStore(t)

	exists, err := store.CheckBaselineExists(ctx, "word_count", nil)
	if err != nil || exists {
		t.Fatalf("no baselines: want (false, nil), got (%v, %v)", exists, err)
	}

	seedBaseline(t, ctx, store, "word_count", "tagesschau", "de", 150.0, 30.0)

	if exists, err = store.CheckBaselineExists(ctx, "word_count", nil); err != nil || !exists {
		t.Errorf("after insert: want true, got %v (err %v)", exists, err)
	}
	src := "tagesschau"
	if exists, err = store.CheckBaselineExists(ctx, "word_count", &src); err != nil || !exists {
		t.Errorf("matching source: want true, got %v (err %v)", exists, err)
	}
	other := "nonexistent"
	if exists, err = store.CheckBaselineExists(ctx, "word_count", &other); err != nil || exists {
		t.Errorf("non-matching source: want false, got %v (err %v)", exists, err)
	}
}

func TestCheckEquivalenceExists(t *testing.T) {
	store, ctx := setupTestStore(t)

	exists, err := store.CheckEquivalenceExists(ctx, "sentiment_score")
	if err != nil || exists {
		t.Fatalf("no equivalence: want false, got %v (err %v)", exists, err)
	}

	// temporal is below deviation — still false for an intensive metric.
	seedEquivalenceGrant(t, ctx, store, "sentiment_score", "de", "temporal")
	if exists, err = store.CheckEquivalenceExists(ctx, "sentiment_score"); err != nil || exists {
		t.Errorf("temporal-only on intensive metric: want false, got %v (err %v)", exists, err)
	}

	seedEquivalenceGrant(t, ctx, store, "sentiment_score", "de", "deviation")
	if exists, err = store.CheckEquivalenceExists(ctx, "sentiment_score"); err != nil || !exists {
		t.Errorf("after deviation grant: want true, got %v (err %v)", exists, err)
	}
}

// Phase 124: the normalization gate is metric-class-aware. A temporal-axis
// metric is satisfied by a temporal grant; an intensive metric still needs
// deviation/absolute. The strict reporting check stays strict.
func TestNormalizationGate_TemporalMetricAcceptsTemporalGrant(t *testing.T) {
	store, ctx := setupTestStore(t)
	for _, lang := range []string{"de", "fr"} {
		seedEquivalenceGrant(t, ctx, store, "publication_hour", lang, "temporal")
	}

	if ok, err := store.CheckEquivalenceExists(ctx, "publication_hour"); err != nil || !ok {
		t.Error("temporal metric should accept a temporal grant in the single-frame gate")
	}
	if ok, err := store.CheckNormalizationEquivalenceForLanguages(ctx, "publication_hour", []string{"de", "fr"}); err != nil || !ok {
		t.Error("temporal grant should satisfy the cross-frame normalization gate for de+fr")
	}
	if ok, err := store.CheckEquivalenceForLanguages(ctx, "publication_hour", []string{"de", "fr"}); err != nil || ok {
		t.Error("strict deviation/absolute reporting check must not be satisfied by a temporal grant")
	}
	if ok, err := store.CheckNormalizationEquivalenceForLanguages(ctx, "sentiment_score_sentiws", []string{"de", "fr"}); err != nil || ok {
		t.Error("intensive metric without a grant must be refused by the normalization gate")
	}
}

// ---------------------------------------------------------------------------
// Normalized metrics (z-score) + excluded-count semantics.
// ---------------------------------------------------------------------------

func TestGetNormalizedMetrics(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now.Add(-time.Hour), 180.0, "tagesschau", "word_count", "art-1")
	seedLangForArticle(t, ctx, store, now.Add(-time.Hour), "tagesschau", "art-1", "de")
	seedBaseline(t, ctx, store, "word_count", "tagesschau", "de", 150.0, 30.0)

	results, excluded, err := store.GetNormalizedMetrics(ctx, now.Add(-2*time.Hour), now, nil, nil, ResolutionFiveMinute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if excluded != 0 {
		t.Errorf("expected excluded=0, got %d", excluded)
	}
	if results[0].Value < 0.99 || results[0].Value > 1.01 { // (180-150)/30 = 1.0
		t.Errorf("expected zscore ~1.0, got %v", results[0].Value)
	}
	if results[0].Source != "tagesschau" {
		t.Errorf("expected source=tagesschau, got %q", results[0].Source)
	}
}

// Inner join drops metrics whose (source, language) pair has no baseline.
func TestGetNormalizedMetrics_NoBaselineMatchYieldsEmpty(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now.Add(-time.Hour), 180.0, "tagesschau", "word_count", "art-1")
	seedLangForArticle(t, ctx, store, now.Add(-time.Hour), "tagesschau", "art-1", "de")
	seedBaseline(t, ctx, store, "word_count", "other_source", "de", 150.0, 30.0) // wrong source

	results, excluded, err := store.GetNormalizedMetrics(ctx, now.Add(-2*time.Hour), now, nil, nil, ResolutionFiveMinute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty result, got %d rows", len(results))
	}
	if excluded != 0 {
		t.Errorf("expected excluded=0 (only baseline missing), got %d", excluded)
	}
}

// The baseline_std>0 predicate excludes degenerate baselines (no divide-by-zero).
func TestGetNormalizedMetrics_ZeroStdBaselineFiltered(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now.Add(-time.Hour), 180.0, "tagesschau", "word_count", "art-1")
	seedLangForArticle(t, ctx, store, now.Add(-time.Hour), "tagesschau", "art-1", "de")
	seedBaseline(t, ctx, store, "word_count", "tagesschau", "de", 150.0, 0.0) // zero std

	results, _, err := store.GetNormalizedMetrics(ctx, now.Add(-2*time.Hour), now, nil, nil, ResolutionFiveMinute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected zero-std baselines filtered, got %d rows", len(results))
	}
}

// Phase 78: metric rows whose article has no language detection are counted as
// excluded, not silently dropped.
func TestGetNormalizedMetrics_MissingLanguageDetectionIsCounted(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now.Add(-time.Hour), 180.0, "tagesschau", "word_count", "art-with-lang")
	seedLangForArticle(t, ctx, store, now.Add(-time.Hour), "tagesschau", "art-with-lang", "de")
	seedMetric(t, ctx, store, now.Add(-45*time.Minute), 210.0, "tagesschau", "word_count", "art-no-lang") // no detection
	seedBaseline(t, ctx, store, "word_count", "tagesschau", "de", 150.0, 30.0)

	results, excluded, err := store.GetNormalizedMetrics(ctx, now.Add(-2*time.Hour), now, nil, nil, ResolutionFiveMinute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 aggregated bucket, got %d", len(results))
	}
	if results[0].Value < 0.99 || results[0].Value > 1.01 {
		t.Errorf("expected zscore ~1.0, got %v", results[0].Value)
	}
	if excluded != 1 {
		t.Errorf("expected excluded=1, got %d", excluded)
	}
}

func TestGetAvailableMetrics_IncludesEquivalenceMetadata(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	seedMetric(t, ctx, store, now, 1.0, "test", "sentiment_score", "")
	seedMetric(t, ctx, store, now, 1.0, "test", "word_count", "")
	seedEquivalenceGrant(t, ctx, store, "sentiment_score", "de", "deviation")

	results, err := store.GetAvailableMetrics(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(results))
	}
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
		t.Fatalf("expected both metrics in results")
	}
	if sentiment.EticConstruct == nil || *sentiment.EticConstruct != "evaluative_polarity" {
		t.Errorf("expected eticConstruct=evaluative_polarity for sentiment_score")
	}
	if sentiment.EquivalenceLevel == nil || *sentiment.EquivalenceLevel != "deviation" {
		t.Errorf("expected equivalenceLevel=deviation for sentiment_score")
	}
	if wordCount.EticConstruct != nil || wordCount.EquivalenceLevel != nil {
		t.Errorf("expected nil equivalence metadata for word_count")
	}
}

// ---------------------------------------------------------------------------
// Resolution routing + bucketing.
// ---------------------------------------------------------------------------

// TestGetAvailableMetrics_ConcurrentAccess verifies thread safety under reads.
func TestGetAvailableMetrics_ConcurrentAccess(t *testing.T) {
	store, ctx := setupTestStore(t)
	now := time.Now().UTC()
	seedMetric(t, ctx, store, now, 1.0, "test", "word_count", "")

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
