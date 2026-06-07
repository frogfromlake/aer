package storage

import (
	"strings"
	"testing"
	"time"
)

// Phase 102 storage-layer integration tests against a Memory-backed
// ClickHouse Testcontainer. The tests share `setupTestStore` with the
// existing storage suite and seed a Probe-0 fixture (tagesschau +
// bundesregierung) before each query.

func seedDistributionFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	rows := [][]any{}
	base := mustParse(t, "2026-04-24T10:00:00Z")
	values := []float64{-0.8, -0.4, -0.1, 0.0, 0.2, 0.3, 0.5, 0.7, 0.9, 0.95}
	for i, v := range values {
		rows = append(rows, []any{base.Add(time.Duration(i) * time.Minute), v, "tagesschau", "sentiment_score", nil})
	}
	// Off-source row to prove scope filtering kicks in.
	rows = append(rows, []any{base, 99.0, "wikipedia", "sentiment_score", nil})
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		rows); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}
}

func TestGetMetricDistribution_HistogramAndSummary(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedDistributionFixture(t, s, contextWrap{ctx})

	res, err := s.GetMetricDistribution(
		ctx,
		"sentiment_score",
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		5,
		nil,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.Summary.Count != 10 {
		t.Fatalf("expected 10 rows in scope, got %d", res.Summary.Count)
	}
	if res.Summary.Min < -0.81 || res.Summary.Max > 0.96 {
		t.Fatalf("min/max outside seeded range: %+v", res.Summary)
	}
	if len(res.Bins) != 5 {
		t.Fatalf("expected 5 bins, got %d", len(res.Bins))
	}
	var total int64
	for _, b := range res.Bins {
		total += b.Count
	}
	if total != 10 {
		t.Fatalf("bin counts must sum to row count: %d", total)
	}
}

// ---------------------------------------------------------------------------
// Heatmap
// ---------------------------------------------------------------------------

func seedHeatmapFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	rows := [][]any{
		{mustParse(t, "2026-04-20T10:00:00Z"), 0.5, "tagesschau", "sentiment_score", "a1"},      // Mon, 10:00
		{mustParse(t, "2026-04-20T10:30:00Z"), 0.3, "tagesschau", "sentiment_score", "a2"},      // Mon, 10:00
		{mustParse(t, "2026-04-20T11:00:00Z"), -0.1, "tagesschau", "sentiment_score", "a3"},     // Mon, 11:00
		{mustParse(t, "2026-04-21T10:00:00Z"), 0.4, "bundesregierung", "sentiment_score", "a4"}, // Tue, 10:00
	}
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetMetricHeatmap_DayOfWeekByHour(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedHeatmapFixture(t, s, contextWrap{ctx})

	cells, err := s.GetMetricHeatmap(
		ctx,
		"sentiment_score",
		[]string{"tagesschau", "bundesregierung"},
		HeatmapDimDayOfWeek,
		HeatmapDimHour,
		mustParse(t, "2026-04-19T00:00:00Z"),
		mustParse(t, "2026-04-22T00:00:00Z"),
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(cells) != 3 {
		t.Fatalf("expected 3 distinct (dow, hour) cells, got %d: %+v", len(cells), cells)
	}
	var totalCount int64
	for _, c := range cells {
		totalCount += c.Count
	}
	if totalCount != 4 {
		t.Fatalf("cell counts must sum to row count: %d", totalCount)
	}
}

func TestGetMetricHeatmap_SourceBySource(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedHeatmapFixture(t, s, contextWrap{ctx})

	cells, err := s.GetMetricHeatmap(
		ctx,
		"sentiment_score",
		[]string{"tagesschau", "bundesregierung"},
		HeatmapDimSource,
		HeatmapDimSource,
		mustParse(t, "2026-04-19T00:00:00Z"),
		mustParse(t, "2026-04-22T00:00:00Z"),
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// Diagonal: each source paired with itself yields one cell.
	if len(cells) != 2 {
		t.Fatalf("expected 2 source cells, got %d: %+v", len(cells), cells)
	}
}

// ---------------------------------------------------------------------------
// Correlation
// ---------------------------------------------------------------------------

func seedCorrelationFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	base := mustParse(t, "2026-04-24T10:00:00Z")
	rows := [][]any{}
	// Two metrics that move in lockstep across 6 buckets.
	for i := range 6 {
		ts := base.Add(time.Duration(i*5) * time.Minute)
		v := float64(i)
		rows = append(rows, []any{ts, v, "tagesschau", "sentiment_score", nil})
		rows = append(rows, []any{ts, 2 * v, "tagesschau", "word_count", nil})
	}
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetMetricCorrelation_PerfectPositive(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedCorrelationFixture(t, s, contextWrap{ctx})

	res, err := s.GetMetricCorrelation(
		ctx,
		[]string{"sentiment_score", "word_count"},
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		nil,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.BucketCount < 6 {
		t.Fatalf("expected >= 6 buckets, got %d", res.BucketCount)
	}
	if len(res.Matrix) != 2 || len(res.Matrix[0]) != 2 {
		t.Fatalf("matrix shape: %v", res.Matrix)
	}
	if res.Matrix[0][1] == nil || *res.Matrix[0][1] < 0.999 {
		t.Fatalf("expected correlation ~1.0, got %v", res.Matrix[0][1])
	}
}

// ---------------------------------------------------------------------------
// Co-occurrence
// ---------------------------------------------------------------------------

func seedCoOccurrenceFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	rows := [][]any{
		// Two articles, both mentioning Berlin+Merkel.
		{w0, w1, "tagesschau", "art-1", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "tagesschau", "art-2", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		// One article mentioning Berlin+Scholz.
		{w0, w1, "tagesschau", "art-3", "Berlin", "LOC", "Scholz", "PER", uint32(2), uint64(1000)},
		// Off-source row (excluded by scope).
		{w0, w1, "wikipedia", "art-w", "Berlin", "LOC", "Merkel", "PER", uint32(99), uint64(1000)},
	}
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.entity_cooccurrences",
		[]string{
			"window_start", "window_end", "source", "article_id",
			"entity_a_text", "entity_a_label", "entity_b_text", "entity_b_label",
			"cooccurrence_count", "ingestion_version",
		}, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetEntityCoOccurrence_AggregatesAndRanks(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedCoOccurrenceFixture(t, s, contextWrap{ctx})

	res, err := s.GetEntityCoOccurrence(
		ctx,
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		10,
		"",
		"",
		0,
		false,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d: %+v", len(res.Edges), res.Edges)
	}
	// Ordered by weight DESC: Berlin/Scholz has weight 2; Berlin/Merkel has weight 2 (1+1).
	// Tiebreak is lexicographic on A then B → Berlin/Merkel first.
	if res.Edges[0].A != "Berlin" || res.Edges[0].B != "Merkel" {
		t.Fatalf("expected Berlin/Merkel first by tie-break, got %+v", res.Edges[0])
	}
	if res.Edges[0].Weight != 2 || res.Edges[0].ArticleCount != 2 {
		t.Fatalf("Berlin/Merkel weight/articleCount: %+v", res.Edges[0])
	}
	if res.Edges[1].Weight != 2 || res.Edges[1].ArticleCount != 1 {
		t.Fatalf("Berlin/Scholz weight/articleCount: %+v", res.Edges[1])
	}

	nodeBy := map[string]CoOccurrenceNode{}
	for _, n := range res.Nodes {
		nodeBy[n.Text] = n
	}
	if nodeBy["Berlin"].Degree != 2 || nodeBy["Berlin"].TotalCount != 4 {
		t.Fatalf("Berlin node mismatch: %+v", nodeBy["Berlin"])
	}
	if nodeBy["Merkel"].Degree != 1 {
		t.Fatalf("Merkel degree mismatch: %+v", nodeBy["Merkel"])
	}
}

// Phase 131a — per-edge source presence + pipeline-gap diagnostic.
func TestGetEntityCoOccurrence_EdgePresenceAcrossSources(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	rows := [][]any{
		// Berlin/Merkel appears in BOTH sources — presence should list both.
		{w0, w1, "tagesschau", "art-1", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "bundesregierung", "art-2", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		// Berlin/Scholz appears in tagesschau only — presence should list one.
		{w0, w1, "tagesschau", "art-3", "Berlin", "LOC", "Scholz", "PER", uint32(2), uint64(1000)},
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entity_cooccurrences",
		[]string{
			"window_start", "window_end", "source", "article_id",
			"entity_a_text", "entity_a_label", "entity_b_text", "entity_b_label",
			"cooccurrence_count", "ingestion_version",
		}, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(
		ctx,
		[]string{"tagesschau", "bundesregierung"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		10,
		"",
		"",
		0,
		false,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	edgePresence := map[string][]string{}
	for _, e := range res.Edges {
		edgePresence[e.A+"|"+e.B] = e.Presence
	}
	bm := edgePresence["Berlin|Merkel"]
	if len(bm) != 2 {
		t.Fatalf("Berlin/Merkel should appear in 2 sources, got %d: %v", len(bm), bm)
	}
	bs := edgePresence["Berlin|Scholz"]
	if len(bs) != 1 || bs[0] != "tagesschau" {
		t.Fatalf("Berlin/Scholz should appear only in tagesschau, got %v", bs)
	}
}

// Phase 131a — articlesInScope is the pipeline-gap diagnostic. Articles
// with ≥2 entities in aer_gold.entities must be counted regardless of
// whether co-occurrence rows exist for them.
func TestGetEntityCoOccurrence_ArticlesInScopePipelineGap(t *testing.T) {
	s, ctx := setupTestStore(t)
	base := mustParse(t, "2026-04-24T10:00:00Z")
	// Seed three articles with ≥2 entities, but NO co-occurrence rows
	// — this is exactly the Phase 131a failure mode. The test schema
	// for aer_gold.entities only carries the 7 columns the existing
	// view-mode test suite needs (`clickhouse_test.go`).
	entityRows := [][]any{
		{base, "tagesschau", "art-1", "Berlin", "LOC", uint32(0), uint32(6)},
		{base, "tagesschau", "art-1", "Merkel", "PER", uint32(10), uint32(16)},
		{base, "tagesschau", "art-2", "Hamburg", "LOC", uint32(0), uint32(7)},
		{base, "tagesschau", "art-2", "Scholz", "PER", uint32(8), uint32(14)},
		// One entity-only article — excluded from the ≥2 set.
		{base, "tagesschau", "art-3", "Bayern", "LOC", uint32(0), uint32(6)},
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entities",
		[]string{
			"timestamp", "source", "article_id", "entity_text", "entity_label",
			"start_char", "end_char",
		}, entityRows); err != nil {
		t.Fatalf("seed entities: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(
		ctx,
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		10,
		"",
		"",
		0,
		false,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.ArticlesInScope != 2 {
		t.Fatalf("expected ArticlesInScope=2 (entity-bearing articles), got %d", res.ArticlesInScope)
	}
	if len(res.Edges) != 0 {
		t.Fatalf("expected 0 edges (no co-occurrence rows seeded), got %d", len(res.Edges))
	}
}

// Phase 123b — viewerLanguage relabels the QID-linked subset to the viewer's
// display label, leaves unlinked nodes on their source surface form, and
// surfaces the linked / labeled coverage counts.
func TestGetEntityCoOccurrence_ViewerLanguageRelabel(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	// French co-occurrence edge: Russie–Macron (both linkable) plus an
	// unlinked span "Moïse Kouame" paired with Russie.
	coocRows := [][]any{
		{w0, w1, "franceinfo", "art-1", "Macron", "PER", "Russie", "LOC", uint32(3), uint64(1000)},
		{w0, w1, "franceinfo", "art-2", "Moïse Kouame", "PER", "Russie", "LOC", uint32(1), uint64(1000)},
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entity_cooccurrences",
		[]string{
			"window_start", "window_end", "source", "article_id",
			"entity_a_text", "entity_a_label", "entity_b_text", "entity_b_label",
			"cooccurrence_count", "ingestion_version",
		}, coocRows); err != nil {
		t.Fatalf("seed cooccurrences: %v", err)
	}
	// entity_links resolves the two linkable spans to QIDs.
	linkRows := [][]any{
		{w0, "art-1", "Russie", "LOC", "Q159", float32(1.0), "exact_match", uint64(1000)},
		{w0, "art-1", "Macron", "PER", "Q3052772", float32(1.0), "exact_match", uint64(1000)},
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entity_links",
		[]string{
			"timestamp", "article_id", "entity_text", "entity_label",
			"wikidata_qid", "link_confidence", "link_method", "ingestion_version",
		}, linkRows); err != nil {
		t.Fatalf("seed entity_links: %v", err)
	}
	// wikidata_labels carries the German display label for Q159 but NOT for
	// Q3052772 — so Macron is linked-but-unlabeled in de.
	labelRows := [][]any{
		{"Q159", "de", "Russland", w0},
		{"Q159", "fr", "Russie", w0},
		{"Q3052772", "fr", "Emmanuel Macron", w0},
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.wikidata_labels",
		[]string{"wikidata_qid", "language", "label", "updated_at"}, labelRows); err != nil {
		t.Fatalf("seed wikidata_labels: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(
		ctx,
		[]string{"franceinfo"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		10,
		"de",
		"",
		0,
		false,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	nodeBy := map[string]CoOccurrenceNode{}
	for _, n := range res.Nodes {
		nodeBy[n.Text] = n
	}
	// Russie (Q159) relabels to Russland in de.
	if got := nodeBy["Russie"].ViewerLabel; got != "Russland" {
		t.Fatalf("Russie should relabel to Russland in de, got %q", got)
	}
	// Macron is linked (Q3052772) but has no de label → no ViewerLabel.
	if nodeBy["Macron"].WikidataQid == "" {
		t.Fatalf("Macron should be linked")
	}
	if got := nodeBy["Macron"].ViewerLabel; got != "" {
		t.Fatalf("Macron has no de label, expected empty ViewerLabel, got %q", got)
	}
	// Unlinked span keeps its source surface form, no QID, no ViewerLabel.
	if nodeBy["Moïse Kouame"].WikidataQid != "" || nodeBy["Moïse Kouame"].ViewerLabel != "" {
		t.Fatalf("unlinked node must stay on source form: %+v", nodeBy["Moïse Kouame"])
	}
	// Coverage counts: 2 linked (Russie, Macron), 1 labeled (Russie) in de.
	if res.LinkedNodeCount != 2 {
		t.Fatalf("expected LinkedNodeCount=2, got %d", res.LinkedNodeCount)
	}
	if res.LabeledNodeCount != 1 {
		t.Fatalf("expected LabeledNodeCount=1, got %d", res.LabeledNodeCount)
	}
}

// Phase 123b — without a viewer language nothing relabels, but the linked
// coverage count is still surfaced (and labeled count is zero).
func TestGetEntityCoOccurrence_NoViewerLanguageNoRelabel(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entity_cooccurrences",
		[]string{
			"window_start", "window_end", "source", "article_id",
			"entity_a_text", "entity_a_label", "entity_b_text", "entity_b_label",
			"cooccurrence_count", "ingestion_version",
		}, [][]any{
			{w0, w1, "franceinfo", "art-1", "Macron", "PER", "Russie", "LOC", uint32(3), uint64(1000)},
		}); err != nil {
		t.Fatalf("seed cooccurrences: %v", err)
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.entity_links",
		[]string{
			"timestamp", "article_id", "entity_text", "entity_label",
			"wikidata_qid", "link_confidence", "link_method", "ingestion_version",
		}, [][]any{
			{w0, "art-1", "Russie", "LOC", "Q159", float32(1.0), "exact_match", uint64(1000)},
		}); err != nil {
		t.Fatalf("seed entity_links: %v", err)
	}
	if err := bulkInsert(contextWrap{ctx}.Ctx(), s, "aer_gold.wikidata_labels",
		[]string{"wikidata_qid", "language", "label", "updated_at"},
		[][]any{{"Q159", "de", "Russland", w0}}); err != nil {
		t.Fatalf("seed wikidata_labels: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(
		ctx,
		[]string{"franceinfo"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		10,
		"", // no viewer language
		"",
		0,
		false,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for _, n := range res.Nodes {
		if n.ViewerLabel != "" {
			t.Fatalf("no viewer language → no ViewerLabel, got %q on %q", n.ViewerLabel, n.Text)
		}
	}
	if res.LinkedNodeCount != 1 {
		t.Fatalf("expected LinkedNodeCount=1, got %d", res.LinkedNodeCount)
	}
	if res.LabeledNodeCount != 0 {
		t.Fatalf("expected LabeledNodeCount=0 without viewer language, got %d", res.LabeledNodeCount)
	}
}

// ---------------------------------------------------------------------------
// helpers (test-only)
// ---------------------------------------------------------------------------

func mustParse(t *testing.T, raw string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return v
}

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

func joinCols(cols []string) string {
	return strings.Join(cols, ", ")
}

// ---------------------------------------------------------------------------
// Scatter (Phase 131) — paired-metric pivot by article.
// ---------------------------------------------------------------------------

func seedScatterFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	base := mustParse(t, "2026-04-24T10:00:00Z")
	rows := [][]any{
		// a1: full triple (x word_count, y sentiment, size entity_count).
		{base, 100.0, "tagesschau", "word_count", "a1"},
		{base, 0.5, "tagesschau", "sentiment_score", "a1"},
		{base, 3.0, "tagesschau", "entity_count", "a1"},
		// a2: x + y only (size channel absent for this article).
		{base, 200.0, "tagesschau", "word_count", "a2"},
		{base, -0.2, "tagesschau", "sentiment_score", "a2"},
		// a3: x only → excluded by the HAVING (no y).
		{base, 300.0, "tagesschau", "word_count", "a3"},
		// a4: full pair but off-source → excluded by scope filter.
		{base, 50.0, "wikipedia", "word_count", "a4"},
		{base, 0.1, "wikipedia", "sentiment_score", "a4"},
	}
	if err := bulkInsert(ctx.Ctx(), s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		rows); err != nil {
		t.Fatalf("seed scatter: %v", err)
	}
}

func TestGetMetricScatter_PivotsByArticleAndBindsSize(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedScatterFixture(t, s, contextWrap{ctx})

	size := "entity_count"
	res, err := s.GetMetricScatter(
		ctx,
		"word_count", "sentiment_score",
		&size, nil,
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		2000,
		nil,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// a1 + a2 contribute; a3 (no y) and a4 (off-source) drop out.
	if len(res.Points) != 2 {
		t.Fatalf("expected 2 points, got %d: %+v", len(res.Points), res.Points)
	}
	if res.Truncated {
		t.Fatalf("did not expect truncation under a 2000 cap")
	}
	// Ordered by article id: a1 first.
	p1 := res.Points[0]
	if p1.ArticleID != "a1" || p1.X != 100.0 || p1.Y != 0.5 {
		t.Fatalf("a1 point wrong: %+v", p1)
	}
	if p1.Size == nil || *p1.Size != 3.0 {
		t.Fatalf("a1 size channel should be bound to 3.0, got %v", p1.Size)
	}
	p2 := res.Points[1]
	if p2.ArticleID != "a2" || p2.Size != nil {
		t.Fatalf("a2 should have no size value: %+v", p2)
	}
}

func TestGetMetricScatter_TruncationFlag(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedScatterFixture(t, s, contextWrap{ctx})

	res, err := s.GetMetricScatter(
		ctx,
		"word_count", "sentiment_score",
		nil, nil,
		[]string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		1,
		nil,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !res.Truncated {
		t.Fatalf("expected truncation at maxPoints=1 with 2 eligible articles")
	}
	if len(res.Points) != 1 {
		t.Fatalf("expected exactly 1 point after truncation, got %d", len(res.Points))
	}
}

// ---------------------------------------------------------------------------
// Time-series spread (Phase 131) — per-bucket sample stddev.
// ---------------------------------------------------------------------------

func TestGetMetricsWithSpread_ComputesStddev(t *testing.T) {
	s, ctx := setupTestStore(t)
	base := mustParse(t, "2026-04-24T10:00:00Z")
	// Four values in one hourly bucket: mean 0.5, sample stddev of
	// {0.2,0.4,0.6,0.8} = 0.2581988897…
	rows := [][]any{
		{base, 0.2, "tagesschau", "sentiment_score", "s1"},
		{base.Add(time.Minute), 0.4, "tagesschau", "sentiment_score", "s2"},
		{base.Add(2 * time.Minute), 0.6, "tagesschau", "sentiment_score", "s3"},
		{base.Add(3 * time.Minute), 0.8, "tagesschau", "sentiment_score", "s4"},
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		rows); err != nil {
		t.Fatalf("seed spread: %v", err)
	}

	got, err := s.GetMetricsWithSpread(
		ctx,
		mustParse(t, "2026-04-24T00:00:00Z"),
		mustParse(t, "2026-04-25T00:00:00Z"),
		[]string{"tagesschau"},
		strPtrTest("sentiment_score"),
		ResolutionHourly,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one hourly bucket, got %d", len(got))
	}
	if got[0].Count != 4 {
		t.Fatalf("expected count 4, got %d", got[0].Count)
	}
	if got[0].Stddev < 0.25 || got[0].Stddev > 0.27 {
		t.Fatalf("sample stddev out of expected range (~0.258): %f", got[0].Stddev)
	}
}

func strPtrTest(s string) *string { return &s }

// hasContext lets the seed helpers accept either the storage-test wrapper or a
// raw context in the future without churning the call sites.
type hasContext interface{ Ctx() contextOnly }

type contextWrap struct{ inner contextOnly }

func (c contextWrap) Ctx() contextOnly { return c.inner }

// contextOnly is a compatibility alias to avoid pulling context just for
// this thin wrapper — the test imports it transitively via the storage pkg.
type contextOnly = interface {
	Deadline() (time.Time, bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}
