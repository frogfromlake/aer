package storage

import (
	"context"
	"fmt"
	"testing"
	"time"
)

var (
	eqStart = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	eqEnd   = time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	eqTS    = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
)

// seedLangDetection inserts one rank-1 language detection.
func seedLangDetection(t *testing.T, ctx hasContext, s *ClickHouseStorage, source, articleID, lang string) {
	t.Helper()
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.language_detections",
		[]string{"timestamp", "source", "article_id", "detected_language", "confidence", "rank"},
		[][]any{{eqTS, source, articleID, lang, 0.99, uint8(1)}}); err != nil {
		t.Fatalf("seed language detection: %v", err)
	}
}

// seedEquivalenceGrant inserts one metric_equivalence row.
func seedEquivalenceGrant(t *testing.T, ctx context.Context, s *ClickHouseStorage, metric, lang, level string) {
	t.Helper()
	err := s.conn.Exec(ctx, `INSERT INTO aer_gold.metric_equivalence
		(etic_construct, metric_name, language, source_type, equivalence_level, validated_by, validation_date, confidence, notes)
		VALUES ('evaluative_polarity', ?, ?, 'web', ?, 'researcher_x', now(), 0.9, 'grant note')`,
		metric, lang, level)
	if err != nil {
		t.Fatalf("seed equivalence grant: %v", err)
	}
}

// ---------------------------------------------------------------------------
// CountLanguagesForSources / LanguagesForScope
// ---------------------------------------------------------------------------

func TestCountAndListLanguagesForScope(t *testing.T) {
	s, ctx := setupTestStore(t)

	seedLangDetection(t, contextWrap{ctx}, s, "tagesschau", "a1", "de")
	seedLangDetection(t, contextWrap{ctx}, s, "tagesschau", "a2", "de")
	seedLangDetection(t, contextWrap{ctx}, s, "franceinfo", "f1", "fr")
	// Off-source row.
	seedLangDetection(t, contextWrap{ctx}, s, "wikipedia", "w1", "en")

	n, err := s.CountLanguagesForSources(ctx, eqStart, eqEnd, []string{"tagesschau", "franceinfo"})
	if err != nil {
		t.Fatalf("CountLanguagesForSources: %v", err)
	}
	if n != 2 {
		t.Errorf("distinct languages: want 2 (de, fr), got %d", n)
	}

	langs, err := s.LanguagesForScope(ctx, eqStart, eqEnd, []string{"tagesschau", "franceinfo"})
	if err != nil {
		t.Fatalf("LanguagesForScope: %v", err)
	}
	if len(langs) != 2 || langs[0] != "de" || langs[1] != "fr" {
		t.Errorf("languages (ordered): want [de fr], got %v", langs)
	}
}

// ---------------------------------------------------------------------------
// GetScopeAvailableMetrics — intersection vs partial.
// ---------------------------------------------------------------------------

func TestGetScopeAvailableMetrics_IntersectionVsPartial(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			// word_count present for both → Available.
			{eqTS, 100.0, "tagesschau", "word_count", "a1"},
			{eqTS, 120.0, "franceinfo", "word_count", "f1"},
			// sentiment_score only tagesschau → Partial.
			{eqTS, 0.4, "tagesschau", "sentiment_score", "a1"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetScopeAvailableMetrics(ctx, eqStart, eqEnd, []string{"tagesschau", "franceinfo"})
	if err != nil {
		t.Fatalf("GetScopeAvailableMetrics: %v", err)
	}
	if len(res.Available) != 1 || res.Available[0] != "word_count" {
		t.Errorf("Available: want [word_count], got %v", res.Available)
	}
	if len(res.Partial) != 1 || res.Partial[0].MetricName != "sentiment_score" {
		t.Fatalf("Partial: want [sentiment_score], got %v", res.Partial)
	}
	if len(res.Partial[0].Sources) != 1 || res.Partial[0].Sources[0] != "tagesschau" {
		t.Errorf("partial sources: want [tagesschau], got %v", res.Partial[0].Sources)
	}
}

// TestGetScopeAvailableMetrics_Degenerate verifies the Task-A zero-variance
// detection: a metric whose value is constant across the whole scope is
// disclosed in Degenerate (with its constant value) while a varied metric is
// not. Degenerate is additive — a constant metric still appears in Available.
func TestGetScopeAvailableMetrics_Degenerate(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			// paywall_status constant (0) across both sources → Degenerate.
			{eqTS, 0.0, "tagesschau", "paywall_status", "a1"},
			{eqTS, 0.0, "franceinfo", "paywall_status", "f1"},
			// word_count varies → NOT degenerate.
			{eqTS, 100.0, "tagesschau", "word_count", "a1"},
			{eqTS, 250.0, "franceinfo", "word_count", "f1"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetScopeAvailableMetrics(ctx, eqStart, eqEnd, []string{"tagesschau", "franceinfo"})
	if err != nil {
		t.Fatalf("GetScopeAvailableMetrics: %v", err)
	}
	if len(res.Degenerate) != 1 || res.Degenerate[0].MetricName != "paywall_status" {
		t.Fatalf("Degenerate: want [paywall_status], got %+v", res.Degenerate)
	}
	if res.Degenerate[0].Value != 0.0 {
		t.Errorf("Degenerate value: want 0, got %v", res.Degenerate[0].Value)
	}
	// paywall_status is constant but still present for every source → Available.
	if !containsString(res.Available, "paywall_status") || !containsString(res.Available, "word_count") {
		t.Errorf("Available must keep its pure has-data semantics, got %v", res.Available)
	}
}

// TestGetScopeAvailableMetrics_LowSignal verifies the near-constant detection
// (image_count = 3 on ~99 % of articles): distinct ≥ 2 but a dominant value
// ≥ threshold → LowSignal (dropped from the picker on the frontend, disclosed),
// NOT Degenerate.
func TestGetScopeAvailableMetrics_LowSignal(t *testing.T) {
	s, ctx := setupTestStore(t)

	rows := [][]any{}
	// image_count: 99× value 3, 1× value 1 → distinct=2, dominantShare=0.99.
	for i := 0; i < 99; i++ {
		rows = append(rows, []any{eqTS, 3.0, "tagesschau", "image_count", fmt.Sprintf("a%d", i)})
	}
	rows = append(rows, []any{eqTS, 1.0, "tagesschau", "image_count", "a99"})
	// word_count varied → neither degenerate nor low-signal.
	rows = append(rows,
		[]any{eqTS, 100.0, "tagesschau", "word_count", "a0"},
		[]any{eqTS, 250.0, "tagesschau", "word_count", "a1"},
		[]any{eqTS, 600.0, "tagesschau", "word_count", "a2"},
	)
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"}, rows); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetScopeAvailableMetrics(ctx, eqStart, eqEnd, []string{"tagesschau"})
	if err != nil {
		t.Fatalf("GetScopeAvailableMetrics: %v", err)
	}
	if len(res.Degenerate) != 0 {
		t.Errorf("nothing should be degenerate (image_count has 2 distinct values), got %+v", res.Degenerate)
	}
	if len(res.LowSignal) != 1 || res.LowSignal[0].MetricName != "image_count" {
		t.Fatalf("LowSignal: want [image_count], got %+v", res.LowSignal)
	}
	ls := res.LowSignal[0]
	if ls.DistinctValues != 2 || ls.DominantValue != 3.0 {
		t.Errorf("image_count low-signal: want 2 distinct / dominant 3, got %+v", ls)
	}
	if ls.DominantShare < 0.95 {
		t.Errorf("image_count dominantShare: want ≥0.95, got %v", ls.DominantShare)
	}
}

// TestGetScopeAvailableMetrics_TwoValueBalanced verifies the ≤2-distinct rule
// (operator decision 2026-06-24): a metric with exactly two distinct values is
// low-signal even when they split evenly (no dominant value) — a near-binary
// discrete metric is too weak to read as a distribution.
func TestGetScopeAvailableMetrics_TwoValueBalanced(t *testing.T) {
	s, ctx := setupTestStore(t)

	rows := [][]any{}
	// has_image: 50× 0, 50× 1 → distinct=2, dominantShare=0.5 (< 0.85) → still
	// low-signal via the ≤2-distinct rule.
	for i := 0; i < 50; i++ {
		rows = append(rows, []any{eqTS, 0.0, "tagesschau", "has_image", fmt.Sprintf("z%d", i)})
		rows = append(rows, []any{eqTS, 1.0, "tagesschau", "has_image", fmt.Sprintf("o%d", i)})
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"}, rows); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetScopeAvailableMetrics(ctx, eqStart, eqEnd, []string{"tagesschau"})
	if err != nil {
		t.Fatalf("GetScopeAvailableMetrics: %v", err)
	}
	if len(res.Degenerate) != 0 {
		t.Errorf("has_image has 2 distinct values → not degenerate, got %+v", res.Degenerate)
	}
	if len(res.LowSignal) != 1 || res.LowSignal[0].MetricName != "has_image" {
		t.Fatalf("LowSignal: want [has_image] (≤2 distinct), got %+v", res.LowSignal)
	}
	if res.LowSignal[0].DistinctValues != 2 {
		t.Errorf("has_image: want 2 distinct, got %+v", res.LowSignal[0])
	}
}

// TestGetScopeAvailableMetrics_StructuralImageCount verifies the structural
// override for a PARTIAL metric: image_count present on only SOME scoped sources
// (tagesschau, missing on elysee) is still classified no-signal AND pruned from
// Partial — so "show anyway" can never resurrect it. It describes document FORMAT
// (a mis-read per-article count), not an editorial measure.
func TestGetScopeAvailableMetrics_StructuralImageCount(t *testing.T) {
	s, ctx := setupTestStore(t)

	rows := [][]any{}
	// image_count only on tagesschau (→ partial): 10× 1, 6× 2, 5× 3 → distinct=3,
	// dominantShare 10/21 ≈ 0.48 (< 0.85) → only the structural rule trips.
	for i := 0; i < 10; i++ {
		rows = append(rows, []any{eqTS, 1.0, "tagesschau", "image_count", fmt.Sprintf("p%d", i)})
	}
	for i := 0; i < 6; i++ {
		rows = append(rows, []any{eqTS, 2.0, "tagesschau", "image_count", fmt.Sprintf("q%d", i)})
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, []any{eqTS, 3.0, "tagesschau", "image_count", fmt.Sprintf("r%d", i)})
	}
	// elysee carries a different metric so image_count is genuinely partial.
	rows = append(rows, []any{eqTS, 250.0, "elysee", "word_count", "e1"})
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"}, rows); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetScopeAvailableMetrics(ctx, eqStart, eqEnd, []string{"tagesschau", "elysee"})
	if err != nil {
		t.Fatalf("GetScopeAvailableMetrics: %v", err)
	}
	if len(res.LowSignal) != 1 || res.LowSignal[0].MetricName != "image_count" {
		t.Fatalf("LowSignal: want [image_count] via structural override, got %+v", res.LowSignal)
	}
	if res.LowSignal[0].DominantShare >= lowSignalDominanceThreshold {
		t.Errorf("test invalid: dominant share %v should be below the statistical threshold", res.LowSignal[0].DominantShare)
	}
	// Pruned from Partial — must NOT be offerable even under "show anyway".
	for _, p := range res.Partial {
		if p.MetricName == "image_count" {
			t.Errorf("image_count must be pruned from Partial (no-signal), got %+v", res.Partial)
		}
	}
	if containsString(res.Available, "image_count") {
		t.Errorf("image_count is partial → must not be Available, got %+v", res.Available)
	}
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// GetPercentileNormalizedMetrics
// ---------------------------------------------------------------------------

func TestGetPercentileNormalizedMetrics_RanksWithinGroup(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Five articles in one hourly bucket with ascending values; percentile of
	// the group ranges 0..1 and averages to 0.5.
	base := time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
	for i, v := range []float64{1, 2, 3, 4, 5} {
		aid := "a" + itoa(i)
		if err := bulkInsert(ctx, s, "aer_gold.metrics",
			[]string{"timestamp", "value", "source", "metric_name", "article_id"},
			[][]any{{base.Add(time.Duration(i) * time.Minute), v, "tagesschau", "word_count", aid}}); err != nil {
			t.Fatalf("seed metric: %v", err)
		}
		seedLangDetection(t, contextWrap{ctx}, s, "tagesschau", aid, "de")
	}
	// One row with NO language detection → excluded count = 1.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{{base, 9.0, "tagesschau", "word_count", "no-lang"}}); err != nil {
		t.Fatalf("seed no-lang metric: %v", err)
	}

	metric := "word_count"
	rows, excluded, err := s.GetPercentileNormalizedMetrics(ctx, eqStart, eqEnd, []string{"tagesschau"}, &metric, ResolutionHourly)
	if err != nil {
		t.Fatalf("GetPercentileNormalizedMetrics: %v", err)
	}
	if excluded != 1 {
		t.Errorf("excluded: want 1 (no-lang row), got %d", excluded)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("want 1 hourly bucket, got %d", len(rows.Rows))
	}
	// Mean percentile of a uniformly ranked group is 0.5.
	if rows.Rows[0].Value < 0.49 || rows.Rows[0].Value > 0.51 {
		t.Errorf("mean percentile: want ~0.5, got %v", rows.Rows[0].Value)
	}
	if rows.Rows[0].Count != 5 {
		t.Errorf("count: want 5 (lang-bearing rows), got %d", rows.Rows[0].Count)
	}
}

// ---------------------------------------------------------------------------
// GetProbeEquivalence + checkAbsoluteEquivalenceForLanguages
// ---------------------------------------------------------------------------

func TestGetProbeEquivalence_LevelLadder(t *testing.T) {
	s, ctx := setupTestStore(t)

	// One metric with data + a de+fr language scope.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{eqTS, 0.4, "tagesschau", "sentiment_score", "a1"},
			{eqTS, 0.5, "franceinfo", "sentiment_score", "f1"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}
	seedLangDetection(t, contextWrap{ctx}, s, "tagesschau", "a1", "de")
	seedLangDetection(t, contextWrap{ctx}, s, "franceinfo", "f1", "fr")
	// Deviation grant for both languages → Level2 true, Level3 false.
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "de", "deviation")
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "fr", "deviation")

	rows, err := s.GetProbeEquivalence(ctx, eqStart, eqEnd, []string{"tagesschau", "franceinfo"})
	if err != nil {
		t.Fatalf("GetProbeEquivalence: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 metric row, got %d", len(rows))
	}
	r := rows[0]
	if r.MetricName != "sentiment_score" {
		t.Errorf("metric: want sentiment_score, got %q", r.MetricName)
	}
	if !r.Level1Available {
		t.Error("Level1 (temporal) should always be available with data")
	}
	if !r.Level2Available {
		t.Error("Level2 (deviation) should be available with de+fr deviation grants")
	}
	if r.Level3Available {
		t.Error("Level3 (absolute) should be false with only deviation grants")
	}
	if r.EquivalenceStatus == nil || r.EquivalenceStatus.Level == nil || *r.EquivalenceStatus.Level != "deviation" {
		t.Errorf("equivalence status level: want deviation, got %+v", r.EquivalenceStatus)
	}
}

func TestGetProbeEquivalence_EmptyScope(t *testing.T) {
	s, ctx := setupTestStore(t)
	rows, err := s.GetProbeEquivalence(ctx, eqStart, eqEnd, nil)
	if err != nil {
		t.Fatalf("GetProbeEquivalence: %v", err)
	}
	if rows != nil {
		t.Errorf("empty scope must return nil, got %v", rows)
	}
}

func TestCheckAbsoluteEquivalenceForLanguages(t *testing.T) {
	s, ctx := setupTestStore(t)

	seedEquivalenceGrant(t, ctx, s, "publication_hour", "de", "absolute")
	seedEquivalenceGrant(t, ctx, s, "publication_hour", "fr", "deviation")

	// de has absolute but fr only deviation → not covered for both.
	ok, err := s.checkAbsoluteEquivalenceForLanguages(ctx, "publication_hour", []string{"de", "fr"})
	if err != nil {
		t.Fatalf("checkAbsolute: %v", err)
	}
	if ok {
		t.Error("want false: fr has no absolute grant")
	}

	// de alone is covered.
	ok, err = s.checkAbsoluteEquivalenceForLanguages(ctx, "publication_hour", []string{"de"})
	if err != nil {
		t.Fatalf("checkAbsolute: %v", err)
	}
	if !ok {
		t.Error("want true: de has an absolute grant")
	}

	// Empty language set is vacuously true.
	ok, err = s.checkAbsoluteEquivalenceForLanguages(ctx, "publication_hour", nil)
	if err != nil {
		t.Fatalf("checkAbsolute: %v", err)
	}
	if !ok {
		t.Error("empty language set should be vacuously true")
	}
}

// ---------------------------------------------------------------------------
// GetEquivalenceStatus
// ---------------------------------------------------------------------------

func TestGetEquivalenceStatus_StrongestGrant(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Two grants for one metric — temporal + deviation; strongest is deviation.
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "de", "temporal")
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "fr", "deviation")

	status, err := s.GetEquivalenceStatus(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("GetEquivalenceStatus: %v", err)
	}
	if status == nil || status.Level == nil {
		t.Fatal("expected a status with a level")
	}
	if *status.Level != "deviation" {
		t.Errorf("level: want deviation (strongest), got %q", *status.Level)
	}
	if status.ValidatedBy == nil || *status.ValidatedBy != "researcher_x" {
		t.Errorf("validatedBy not populated: %+v", status.ValidatedBy)
	}

	// Unknown metric → nil status, no error.
	missing, err := s.GetEquivalenceStatus(ctx, "no_such_metric")
	if err != nil {
		t.Fatalf("GetEquivalenceStatus(missing): %v", err)
	}
	if missing != nil {
		t.Errorf("unknown metric must yield nil status, got %+v", missing)
	}
}

// ---------------------------------------------------------------------------
// GetMetricValidationStatus / GetMetricCulturalContextNotes
// ---------------------------------------------------------------------------

func TestGetMetricValidationStatus(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Unvalidated: no row at all.
	st, err := s.GetMetricValidationStatus(ctx, "word_count")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st != "unvalidated" {
		t.Errorf("no row: want unvalidated, got %q", st)
	}

	// Validated: an entry with valid_until in the future.
	if err := s.conn.Exec(ctx, `INSERT INTO aer_gold.metric_validity
		(metric_name, context_key, validation_date, alpha_score, correlation, n_annotated, error_taxonomy, valid_until)
		VALUES ('sentiment_score', 'de', now(), 0.8, 0.7, 100, '', now() + INTERVAL 365 DAY)`); err != nil {
		t.Fatalf("insert valid: %v", err)
	}
	st, err = s.GetMetricValidationStatus(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st != "validated" {
		t.Errorf("future valid_until: want validated, got %q", st)
	}

	// Expired: an entry already past its valid_until.
	if err := s.conn.Exec(ctx, `INSERT INTO aer_gold.metric_validity
		(metric_name, context_key, validation_date, alpha_score, correlation, n_annotated, error_taxonomy, valid_until)
		VALUES ('entity_count', 'de', now() - INTERVAL 700 DAY, 0.8, 0.7, 100, '', now() - INTERVAL 1 DAY)`); err != nil {
		t.Fatalf("insert expired: %v", err)
	}
	st, err = s.GetMetricValidationStatus(ctx, "entity_count")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if st != "expired" {
		t.Errorf("past valid_until: want expired, got %q", st)
	}
}

func TestGetMetricCulturalContextNotes(t *testing.T) {
	s, ctx := setupTestStore(t)

	// No equivalence → empty notes.
	notes, err := s.GetMetricCulturalContextNotes(ctx, "word_count")
	if err != nil {
		t.Fatalf("notes: %v", err)
	}
	if notes != "" {
		t.Errorf("no equivalence: want empty, got %q", notes)
	}

	// Two grants — temporal + absolute; the summary picks the strongest level.
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "de", "temporal")
	seedEquivalenceGrant(t, ctx, s, "sentiment_score", "fr", "absolute")

	notes, err = s.GetMetricCulturalContextNotes(ctx, "sentiment_score")
	if err != nil {
		t.Fatalf("notes: %v", err)
	}
	if notes == "" {
		t.Fatal("expected a non-empty cultural-context summary")
	}
	if !contains(notes, "absolute") || !contains(notes, "evaluative_polarity") {
		t.Errorf("summary should name strongest level + construct, got %q", notes)
	}
}

// ---------------------------------------------------------------------------
// queryNodeMetric (cooccurrence_subqueries.go)
// ---------------------------------------------------------------------------

func TestQueryNodeMetric_MeanOverArticlesWhereEntityAppears(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Two entities: Berlin appears in a1 + a2, Merkel only in a1.
	if err := bulkInsert(ctx, s, "aer_gold.entities",
		[]string{"timestamp", "source", "article_id", "entity_text", "entity_label", "start_char", "end_char"},
		[][]any{
			{eqTS, "tagesschau", "a1", "Berlin", "LOC", uint32(0), uint32(6)},
			{eqTS, "tagesschau", "a2", "Berlin", "LOC", uint32(0), uint32(6)},
			{eqTS, "tagesschau", "a1", "Merkel", "PER", uint32(10), uint32(16)},
		}); err != nil {
		t.Fatalf("seed entities: %v", err)
	}
	// sentiment: a1 = 0.2, a2 = 0.8.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{eqTS, 0.2, "tagesschau", "sentiment_score", "a1"},
			{eqTS, 0.8, "tagesschau", "sentiment_score", "a2"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	acc := map[string]*nodeAccumulator{
		"Berlin": {label: "LOC"},
		"Merkel": {label: "PER"},
	}
	got, err := s.queryNodeMetric(ctx, acc, "sentiment_score", []string{"tagesschau"}, eqStart, eqEnd)
	if err != nil {
		t.Fatalf("queryNodeMetric: %v", err)
	}
	// Berlin mean over a1, a2 = 0.5; Merkel over a1 = 0.2.
	if got["Berlin"] < 0.49 || got["Berlin"] > 0.51 {
		t.Errorf("Berlin mean: want ~0.5, got %v", got["Berlin"])
	}
	if got["Merkel"] < 0.19 || got["Merkel"] > 0.21 {
		t.Errorf("Merkel mean: want ~0.2, got %v", got["Merkel"])
	}
}

func TestQueryNodeMetric_EmptyInputs(t *testing.T) {
	s, ctx := setupTestStore(t)
	got, err := s.queryNodeMetric(ctx, map[string]*nodeAccumulator{}, "sentiment_score", []string{"tagesschau"}, eqStart, eqEnd)
	if err != nil || got != nil {
		t.Errorf("empty accumulator: want (nil, nil), got (%v, %v)", got, err)
	}
	got, err = s.queryNodeMetric(ctx, map[string]*nodeAccumulator{"X": {}}, "", []string{"tagesschau"}, eqStart, eqEnd)
	if err != nil || got != nil {
		t.Errorf("empty metric: want (nil, nil), got (%v, %v)", got, err)
	}
}
