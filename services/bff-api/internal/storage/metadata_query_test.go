package storage

import (
	"fmt"
	"testing"
	"time"
)

// metadataFixtureWindow is the standard window covering the seeded rows.
var (
	mdStart = time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	mdEnd   = time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	mdTS    = time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
)

// ---------------------------------------------------------------------------
// GetCategoricalDistribution
// ---------------------------------------------------------------------------

func TestGetCategoricalDistribution_TopNAndLongTail(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	// Politik x4, Wirtschaft x2, Sport x1, Kultur x1 — 4 distinct values.
	rows := []metadataValueRow{
		{mdTS, "tagesschau", "a1", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a2", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a3", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a4", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a5", "section", []string{"Wirtschaft"}, ""},
		{mdTS, "tagesschau", "a6", "section", []string{"Wirtschaft"}, ""},
		{mdTS, "tagesschau", "a7", "section", []string{"Sport"}, ""},
		{mdTS, "tagesschau", "a8", "section", []string{"Kultur"}, ""},
		// Off-source row, excluded by scope.
		{mdTS, "wikipedia", "w1", "section", []string{"Politik"}, ""},
	}
	seedArticleMetadata(t, ctx, s, rows)

	res, err := s.GetCategoricalDistribution(ctx, "section", []string{"tagesschau"}, mdStart, mdEnd, 2, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.TotalArticles != 8 {
		t.Errorf("TotalArticles: want 8 (off-source excluded), got %d", res.TotalArticles)
	}
	if res.DistinctValues != 4 {
		t.Errorf("DistinctValues: want 4, got %d", res.DistinctValues)
	}
	if len(res.Categories) != 2 {
		t.Fatalf("Categories: want top-2, got %d", len(res.Categories))
	}
	if res.Categories[0].Value != "Politik" || res.Categories[0].Articles != 4 {
		t.Errorf("first category: want Politik/4, got %+v", res.Categories[0])
	}
	if res.Categories[1].Value != "Wirtschaft" || res.Categories[1].Articles != 2 {
		t.Errorf("second category: want Wirtschaft/2, got %+v", res.Categories[1])
	}
	// Long tail = Sport (1) + Kultur (1) = 2.
	if res.OtherArticles != 2 {
		t.Errorf("OtherArticles: want 2, got %d", res.OtherArticles)
	}
}

func TestGetCategoricalDistribution_EmptyScopeReturnsEmpty(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	res, err := s.GetCategoricalDistribution(ctx, "section", nil, mdStart, mdEnd, 5, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Categories) != 0 || res.TotalArticles != 0 {
		t.Errorf("empty scope must yield empty result, got %+v", res)
	}
}

func TestGetCategoricalDistribution_ListFieldCountsArticlesDistinctly(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	// One article carries the same tag twice via its array — distinct-article
	// counting must not double-count it.
	rows := []metadataValueRow{
		{mdTS, "tagesschau", "a1", "tags", []string{"eu", "eu"}, ""},
		{mdTS, "tagesschau", "a2", "tags", []string{"eu", "klima"}, ""},
	}
	seedArticleMetadata(t, ctx, s, rows)

	res, err := s.GetCategoricalDistribution(ctx, "tags", []string{"tagesschau"}, mdStart, mdEnd, 10, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	byVal := map[string]uint64{}
	for _, c := range res.Categories {
		byVal[c.Value] = c.Articles
	}
	if byVal["eu"] != 2 {
		t.Errorf("eu should count 2 distinct articles, got %d", byVal["eu"])
	}
	if byVal["klima"] != 1 {
		t.Errorf("klima should count 1 article, got %d", byVal["klima"])
	}
}

// ---------------------------------------------------------------------------
// GetScopeAvailableMetadata
// ---------------------------------------------------------------------------

func TestGetScopeAvailableMetadata_IntersectionVsPartial(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	rows := []metadataValueRow{
		// section present for both sources → Available.
		{mdTS, "tagesschau", "a1", "section", []string{"Politik"}, ""},
		{mdTS, "elysee", "e1", "section", []string{"Communiqué"}, ""},
		// author present only for tagesschau → Partial.
		{mdTS, "tagesschau", "a1", "author", []string{"M. Mustermann"}, ""},
	}
	seedArticleMetadata(t, ctx, s, rows)

	res, err := s.GetScopeAvailableMetadata(ctx, mdStart, mdEnd, []string{"tagesschau", "elysee"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Available) != 1 || res.Available[0] != "section" {
		t.Errorf("Available: want [section], got %v", res.Available)
	}
	if len(res.Partial) != 1 || res.Partial[0].Field != "author" {
		t.Fatalf("Partial: want [author], got %v", res.Partial)
	}
	if len(res.Partial[0].Sources) != 1 || res.Partial[0].Sources[0] != "tagesschau" {
		t.Errorf("author partial sources: want [tagesschau], got %v", res.Partial[0].Sources)
	}
}

func TestGetScopeAvailableMetadata_DegenerateAndLowSignal(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	rows := []metadataValueRow{
		// article_type constant across the whole scope → Degenerate (still in
		// Available — the lists keep their pure "has data" semantics).
		{mdTS, "tagesschau", "a1", "article_type", []string{"NewsArticle"}, ""},
		{mdTS, "tagesschau", "a2", "article_type", []string{"NewsArticle"}, ""},
		{mdTS, "elysee", "e1", "article_type", []string{"NewsArticle"}, ""},
		// section: three balanced values → ≥3 distinct, no dominant → neither
		// degenerate nor low-signal (a genuine multi-value categorical).
		{mdTS, "tagesschau", "a1", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a2", "section", []string{"Wirtschaft"}, ""},
		{mdTS, "elysee", "e1", "section", []string{"Sport"}, ""},
	}
	// kicker: present on both sources, only two distinct values across scope
	// (20 "Politik" vs 1 "Spezial") → low-signal by the ≤2-distinct rule
	// (operator decision 2026-06-24); its share 20/21≈0.952 also clears ≥0.85.
	for i := 0; i < 19; i++ {
		rows = append(rows, metadataValueRow{mdTS, "tagesschau", fmt.Sprintf("k%d", i), "kicker", []string{"Politik"}, ""})
	}
	rows = append(rows,
		metadataValueRow{mdTS, "elysee", "ek1", "kicker", []string{"Politik"}, ""},
		metadataValueRow{mdTS, "elysee", "ek2", "kicker", []string{"Spezial"}, ""},
	)
	// rubrik: three distinct values but one dominates ≥85 % → low-signal by the
	// dominance rule (exercises the ≥0.85 path independently of the ≤2 rule).
	for i := 0; i < 17; i++ {
		rows = append(rows, metadataValueRow{mdTS, "tagesschau", fmt.Sprintf("r%d", i), "rubrik", []string{"Inland"}, ""})
	}
	rows = append(rows,
		metadataValueRow{mdTS, "elysee", "er1", "rubrik", []string{"Inland"}, ""},
		metadataValueRow{mdTS, "elysee", "er2", "rubrik", []string{"Ausland"}, ""},
		metadataValueRow{mdTS, "elysee", "er3", "rubrik", []string{"Kultur"}, ""},
	)
	seedArticleMetadata(t, ctx, s, rows)

	res, err := s.GetScopeAvailableMetadata(ctx, mdStart, mdEnd, []string{"tagesschau", "elysee"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	// Degenerate: exactly article_type, disclosed with its constant value.
	if len(res.Degenerate) != 1 || res.Degenerate[0].Field != "article_type" {
		t.Fatalf("Degenerate: want [article_type], got %+v", res.Degenerate)
	}
	if res.Degenerate[0].Value != "NewsArticle" {
		t.Errorf("Degenerate value: want NewsArticle, got %q", res.Degenerate[0].Value)
	}
	// LowSignal: kicker (≤2 distinct) AND rubrik (≥85 % dominant) — both dropped.
	lowByField := map[string]LowSignalField{}
	for _, l := range res.LowSignal {
		lowByField[l.Field] = l
	}
	if len(lowByField) != 2 || lowByField["kicker"].Field == "" || lowByField["rubrik"].Field == "" {
		t.Fatalf("LowSignal: want [kicker, rubrik], got %+v", res.LowSignal)
	}
	if lowByField["kicker"].DistinctValues != 2 || lowByField["kicker"].DominantValue != "Politik" {
		t.Errorf("LowSignal kicker: want 2 distinct / Politik, got %+v", lowByField["kicker"])
	}
	if lowByField["rubrik"].DistinctValues != 3 || lowByField["rubrik"].DominantShare < 0.85 {
		t.Errorf("LowSignal rubrik: want 3 distinct / share≥0.85, got %+v", lowByField["rubrik"])
	}
	// section has ≥3 balanced values → flagged by neither list.
	for _, d := range res.Degenerate {
		if d.Field == "section" {
			t.Errorf("section must not be degenerate")
		}
	}
	for _, l := range res.LowSignal {
		if l.Field == "section" {
			t.Errorf("section must not be low-signal")
		}
	}
}

// TestGetScopeAvailableMetadata_StructuralArticleType verifies the structural
// override for a PARTIAL field — the exact reported bug: article_type present on
// only SOME scoped sources (here tagesschau, missing on elysee). Without the
// override it would land only in Partial ("withheld"), and "show anyway" would
// resurrect it as a default. The override classifies it no-signal (it describes
// document FORMAT, not content) regardless of available/partial, AND prunes it
// from Partial so it is disclosed ONLY under "no signal".
func TestGetScopeAvailableMetadata_StructuralArticleType(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	// article_type only on tagesschau (→ partial), three values none ≥85 %:
	// 10 NewsArticle, 6 Article, 5 Blog → dominant share 10/21 ≈ 0.48 (the
	// statistical rule would NOT trip — only the structural rule does).
	rows := []metadataValueRow{}
	for i := 0; i < 10; i++ {
		rows = append(rows, metadataValueRow{mdTS, "tagesschau", fmt.Sprintf("n%d", i), "article_type", []string{"NewsArticle"}, ""})
	}
	for i := 0; i < 6; i++ {
		rows = append(rows, metadataValueRow{mdTS, "tagesschau", fmt.Sprintf("a%d", i), "article_type", []string{"Article"}, ""})
	}
	for i := 0; i < 5; i++ {
		rows = append(rows, metadataValueRow{mdTS, "tagesschau", fmt.Sprintf("b%d", i), "article_type", []string{"Blog"}, ""})
	}
	// elysee carries a different field so it is genuinely in scope (and article_type
	// is genuinely partial, not just the only field).
	rows = append(rows, metadataValueRow{mdTS, "elysee", "e1", "section", []string{"Politique"}, ""})
	seedArticleMetadata(t, ctx, s, rows)

	res, err := s.GetScopeAvailableMetadata(ctx, mdStart, mdEnd, []string{"tagesschau", "elysee"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// In LowSignal (structural), even though partial.
	if len(res.LowSignal) != 1 || res.LowSignal[0].Field != "article_type" {
		t.Fatalf("LowSignal: want [article_type] via structural override, got %+v", res.LowSignal)
	}
	if res.LowSignal[0].DominantShare >= lowSignalDominanceThreshold {
		t.Errorf("test invalid: dominant share %v should be below the statistical threshold so the structural rule is what trips", res.LowSignal[0].DominantShare)
	}
	// Pruned from Partial — must NOT be offerable even under "show anyway".
	for _, p := range res.Partial {
		if p.Field == "article_type" {
			t.Errorf("article_type must be pruned from Partial (no-signal), got %+v", res.Partial)
		}
	}
	// And never in Available (it is missing on elysee).
	if containsString(res.Available, "article_type") {
		t.Errorf("article_type is partial → must not be Available, got %+v", res.Available)
	}
}

func TestGetScopeAvailableMetadata_EmptyScope(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)
	res, err := s.GetScopeAvailableMetadata(ctx, mdStart, mdEnd, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Available) != 0 || len(res.Partial) != 0 {
		t.Errorf("empty scope must yield empty availability, got %+v", res)
	}
}

// ---------------------------------------------------------------------------
// GetCrossTab — categorical FIELD x numeric METRIC.
// ---------------------------------------------------------------------------

func TestGetCrossTab_PerCategoryMetricMean(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	// section metadata.
	seedArticleMetadata(t, ctx, s, []metadataValueRow{
		{mdTS, "tagesschau", "a1", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a2", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a3", "section", []string{"Sport"}, ""},
	})
	// sentiment metric per article.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{mdTS, 0.2, "tagesschau", "sentiment_score", "a1"},
			{mdTS, 0.4, "tagesschau", "sentiment_score", "a2"},
			{mdTS, -0.6, "tagesschau", "sentiment_score", "a3"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetCrossTab(ctx, "section", "sentiment_score", []string{"tagesschau"}, mdStart, mdEnd, 10, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.DistinctValues != 2 {
		t.Errorf("DistinctValues: want 2, got %d", res.DistinctValues)
	}
	byVal := map[string]CrossTabBucket{}
	for _, b := range res.Buckets {
		byVal[b.Value] = b
	}
	politik := byVal["Politik"]
	if politik.Articles != 2 {
		t.Errorf("Politik articles: want 2, got %d", politik.Articles)
	}
	if politik.Mean < 0.29 || politik.Mean > 0.31 { // mean of 0.2, 0.4
		t.Errorf("Politik mean: want ~0.3, got %v", politik.Mean)
	}
	sport := byVal["Sport"]
	if sport.Articles != 1 || sport.Mean > -0.59 {
		t.Errorf("Sport bucket: want 1 article ~-0.6, got %+v", sport)
	}
}

func TestGetCrossTab_EmptyArgsReturnsEmpty(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)
	res, err := s.GetCrossTab(ctx, "", "sentiment_score", []string{"tagesschau"}, mdStart, mdEnd, 10, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Buckets) != 0 {
		t.Errorf("empty field must yield empty result, got %+v", res)
	}
}

// ---------------------------------------------------------------------------
// GetSankey — alluvial flow across an ordered field chain.
// ---------------------------------------------------------------------------

func TestGetSankey_FlowBetweenTwoFields(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)

	// Two fields per article: section -> sentiment_band.
	seedArticleMetadata(t, ctx, s, []metadataValueRow{
		{mdTS, "tagesschau", "a1", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a1", "band", []string{"positive"}, ""},
		{mdTS, "tagesschau", "a2", "section", []string{"Politik"}, ""},
		{mdTS, "tagesschau", "a2", "band", []string{"positive"}, ""},
		{mdTS, "tagesschau", "a3", "section", []string{"Sport"}, ""},
		{mdTS, "tagesschau", "a3", "band", []string{"negative"}, ""},
	})

	res, err := s.GetSankey(ctx, []string{"section", "band"}, []string{"tagesschau"}, mdStart, mdEnd, 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// Two distinct flows: Politik->positive (2 articles), Sport->negative (1).
	if len(res.Links) != 2 {
		t.Fatalf("want 2 links, got %d: %+v", len(res.Links), res.Links)
	}
	// Ordered by N desc: Politik->positive first.
	if res.Links[0].Value != 2 {
		t.Errorf("first link value: want 2, got %d", res.Links[0].Value)
	}
	if res.Links[1].Value != 1 {
		t.Errorf("second link value: want 1, got %d", res.Links[1].Value)
	}
	// Four distinct nodes (Politik, Sport in layer 0; positive, negative in layer 1).
	if len(res.Nodes) != 4 {
		t.Errorf("want 4 nodes, got %d: %+v", len(res.Nodes), res.Nodes)
	}
}

func TestGetSankey_NeedsTwoFields(t *testing.T) {
	s, ctx := setupTestStore(t)
	createArticleMetadataTable(t, ctx, s)
	res, err := s.GetSankey(ctx, []string{"section"}, []string{"tagesschau"}, mdStart, mdEnd, 50)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Links) != 0 || len(res.Nodes) != 0 {
		t.Errorf("single field must yield empty flow, got %+v", res)
	}
}

// ---------------------------------------------------------------------------
// GetParallelCoords — per-article N-metric vector.
// ---------------------------------------------------------------------------

func TestGetParallelCoords_CompletePolylinesOnly(t *testing.T) {
	s, ctx := setupTestStore(t)

	// a1 carries all three metrics; a2 is missing entity_count (dropped);
	// a3 is off-source (dropped).
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{mdTS, 100.0, "tagesschau", "word_count", "a1"},
			{mdTS, 0.5, "tagesschau", "sentiment_score", "a1"},
			{mdTS, 4.0, "tagesschau", "entity_count", "a1"},
			{mdTS, 200.0, "tagesschau", "word_count", "a2"},
			{mdTS, 0.1, "tagesschau", "sentiment_score", "a2"},
			{mdTS, 50.0, "wikipedia", "word_count", "a3"},
			{mdTS, 0.9, "wikipedia", "sentiment_score", "a3"},
			{mdTS, 1.0, "wikipedia", "entity_count", "a3"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetParallelCoords(ctx,
		[]string{"word_count", "sentiment_score", "entity_count"},
		[]string{"tagesschau"}, mdStart, mdEnd, 1000, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Rows) != 1 {
		t.Fatalf("want 1 complete polyline (a1), got %d: %+v", len(res.Rows), res.Rows)
	}
	r := res.Rows[0]
	if r.ArticleID != "a1" {
		t.Errorf("article: want a1, got %q", r.ArticleID)
	}
	if len(r.Values) != 3 || r.Values[0] != 100.0 || r.Values[1] != 0.5 || r.Values[2] != 4.0 {
		t.Errorf("values in metric order want [100,0.5,4], got %v", r.Values)
	}
	if res.Truncated {
		t.Error("did not expect truncation")
	}
}

func TestGetParallelCoords_TruncationFlag(t *testing.T) {
	s, ctx := setupTestStore(t)
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{mdTS, 100.0, "tagesschau", "word_count", "a1"},
			{mdTS, 0.5, "tagesschau", "sentiment_score", "a1"},
			{mdTS, 200.0, "tagesschau", "word_count", "a2"},
			{mdTS, 0.1, "tagesschau", "sentiment_score", "a2"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}
	res, err := s.GetParallelCoords(ctx,
		[]string{"word_count", "sentiment_score"}, []string{"tagesschau"}, mdStart, mdEnd, 1, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !res.Truncated {
		t.Error("expected truncation at maxPoints=1 with 2 complete polylines")
	}
	if len(res.Rows) != 1 {
		t.Errorf("want exactly 1 row after truncation, got %d", len(res.Rows))
	}
}

func TestGetParallelCoords_NeedsTwoMetrics(t *testing.T) {
	s, ctx := setupTestStore(t)
	res, err := s.GetParallelCoords(ctx, []string{"word_count"}, []string{"tagesschau"}, mdStart, mdEnd, 100, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Rows) != 0 {
		t.Errorf("single metric must yield no rows, got %d", len(res.Rows))
	}
}

// ---------------------------------------------------------------------------
// GetMetadataCoverage
// ---------------------------------------------------------------------------

func TestGetMetadataCoverage_PerSourceFieldMethod(t *testing.T) {
	s, ctx := setupTestStore(t)
	createMetadataCoverageTable(t, ctx, s)

	// author populated for 3 distinct articles via json_ld; published_date null
	// for 2 articles.
	seedMetadataCoverage(t, ctx, s, "tagesschau", "author", "json_ld",
		[]string{"a1", "a2", "a3"}, mdTS)
	seedMetadataCoverage(t, ctx, s, "tagesschau", "published_date", "null",
		[]string{"a1", "a2"}, mdTS)
	// Off-source row, excluded by scope.
	seedMetadataCoverage(t, ctx, s, "wikipedia", "author", "json_ld",
		[]string{"w1"}, mdTS)

	cells, err := s.GetMetadataCoverage(ctx, []string{"tagesschau"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(cells) != 2 {
		t.Fatalf("want 2 cells for tagesschau, got %d: %+v", len(cells), cells)
	}
	byField := map[string]MetadataCoverageCell{}
	for _, c := range cells {
		byField[c.Field] = c
	}
	if byField["author"].Method != "json_ld" || byField["author"].Articles != 3 {
		t.Errorf("author cell: want json_ld/3, got %+v", byField["author"])
	}
	if byField["published_date"].Method != "null" || byField["published_date"].Articles != 2 {
		t.Errorf("published_date cell: want null/2, got %+v", byField["published_date"])
	}
}

func TestGetMetadataCoverage_EmptyScope(t *testing.T) {
	s, ctx := setupTestStore(t)
	createMetadataCoverageTable(t, ctx, s)
	cells, err := s.GetMetadataCoverage(ctx, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if cells != nil {
		t.Errorf("empty scope must return nil, got %v", cells)
	}
}
