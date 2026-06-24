package storage

import (
	"testing"
	"time"
)

// Phase 102 storage-layer integration tests against a Memory-backed
// ClickHouse Testcontainer. The tests share `setupTestStore` with the
// existing storage suite and seed a Probe-0 fixture (tagesschau +
// bundesregierung) before each query. Shared seed/context helpers live in
// fixtures_test.go.

// ---------------------------------------------------------------------------
// Distribution
// ---------------------------------------------------------------------------

func TestGetMetricDistribution_HistogramAndSummary(t *testing.T) {
	s, ctx := setupTestStore(t)
	base := mustParse(t, "2026-04-24T10:00:00Z")
	rows := [][]any{}
	for i, v := range []float64{-0.8, -0.4, -0.1, 0.0, 0.2, 0.3, 0.5, 0.7, 0.9, 0.95} {
		rows = append(rows, []any{base.Add(time.Duration(i) * time.Minute), v, "tagesschau", "sentiment_score", nil})
	}
	rows = append(rows, []any{base, 99.0, "wikipedia", "sentiment_score", nil}) // off-source
	if err := bulkInsert(ctx, s, "aer_gold.metrics", metricCols, rows); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	res, err := s.GetMetricDistribution(ctx, "sentiment_score", []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 5, nil)
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

func seedHeatmapFixture(t *testing.T, ctx contextOnly, s *ClickHouseStorage) {
	t.Helper()
	rows := [][]any{
		{mustParse(t, "2026-04-20T10:00:00Z"), 0.5, "tagesschau", "sentiment_score", "a1"},      // Mon 10:00
		{mustParse(t, "2026-04-20T10:30:00Z"), 0.3, "tagesschau", "sentiment_score", "a2"},      // Mon 10:00
		{mustParse(t, "2026-04-20T11:00:00Z"), -0.1, "tagesschau", "sentiment_score", "a3"},     // Mon 11:00
		{mustParse(t, "2026-04-21T10:00:00Z"), 0.4, "bundesregierung", "sentiment_score", "a4"}, // Tue 10:00
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics", metricCols, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetMetricHeatmap_DayOfWeekByHour(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedHeatmapFixture(t, ctx, s)

	cells, err := s.GetMetricHeatmap(ctx, "sentiment_score", []string{"tagesschau", "bundesregierung"},
		HeatmapDimDayOfWeek, HeatmapDimHour,
		mustParse(t, "2026-04-19T00:00:00Z"), mustParse(t, "2026-04-22T00:00:00Z"))
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
	seedHeatmapFixture(t, ctx, s)

	cells, err := s.GetMetricHeatmap(ctx, "sentiment_score", []string{"tagesschau", "bundesregierung"},
		HeatmapDimSource, HeatmapDimSource,
		mustParse(t, "2026-04-19T00:00:00Z"), mustParse(t, "2026-04-22T00:00:00Z"))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(cells) != 2 { // diagonal: each source paired with itself
		t.Fatalf("expected 2 source cells, got %d: %+v", len(cells), cells)
	}
}

// ---------------------------------------------------------------------------
// Correlation
// ---------------------------------------------------------------------------

func TestGetMetricCorrelation_PerfectPositive(t *testing.T) {
	s, ctx := setupTestStore(t)
	base := mustParse(t, "2026-04-24T10:00:00Z")
	rows := [][]any{}
	for i := range 6 { // two metrics moving in lockstep across 6 buckets
		ts := base.Add(time.Duration(i*5) * time.Minute)
		v := float64(i)
		rows = append(rows, []any{ts, v, "tagesschau", "sentiment_score", nil})
		rows = append(rows, []any{ts, 2 * v, "tagesschau", "word_count", nil})
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics", metricCols, rows); err != nil {
		t.Fatalf("seed: %v", err)
	}

	res, err := s.GetMetricCorrelation(ctx, []string{"sentiment_score", "word_count"}, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), nil)
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

func TestGetEntityCoOccurrence_AggregatesAndRanks(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "tagesschau", "art-1", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "tagesschau", "art-2", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "tagesschau", "art-3", "Berlin", "LOC", "Scholz", "PER", uint32(2), uint64(1000)},
		{w0, w1, "wikipedia", "art-w", "Berlin", "LOC", "Merkel", "PER", uint32(99), uint64(1000)}, // off-source
	})

	res, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 10, "", "", 0, false, "", 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d: %+v", len(res.Edges), res.Edges)
	}
	// Ordered by weight DESC; tie broken lexicographically → Berlin/Merkel first.
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

// Phase 148g — node-FIRST breadth mode: maxNodes selects the top-N entities by
// summed weight; edges are restricted to pairs among them; TotalCount is the
// entity's FULL summed weight (not just the returned edges).
func TestGetEntityCoOccurrence_NodeFirstBreadth(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	// Totals by summed weight: Berlin=6, Merkel=6, Scholz=1, Habeck=1.
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "tagesschau", "a1", "Berlin", "LOC", "Merkel", "PER", uint32(5), uint64(1)},
		{w0, w1, "tagesschau", "a2", "Berlin", "LOC", "Scholz", "PER", uint32(1), uint64(1)},
		{w0, w1, "tagesschau", "a3", "Merkel", "PER", "Habeck", "PER", uint32(1), uint64(1)},
	})
	qs := mustParse(t, "2026-04-24T00:00:00Z")
	qe := mustParse(t, "2026-04-25T00:00:00Z")

	// node-first, maxNodes=2 → universe {Berlin, Merkel}; only the Berlin–Merkel
	// edge survives (Scholz/Habeck excluded from the universe).
	res, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau"}, qs, qe, 10, "", "", 0, false, "", 2)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Nodes) != 2 {
		t.Fatalf("node-first maxNodes=2: want 2 nodes, got %d: %+v", len(res.Nodes), res.Nodes)
	}
	nodeBy := map[string]CoOccurrenceNode{}
	for _, n := range res.Nodes {
		nodeBy[n.Text] = n
	}
	if _, ok := nodeBy["Berlin"]; !ok {
		t.Fatalf("Berlin must be in the top-2 node universe: %+v", res.Nodes)
	}
	if _, ok := nodeBy["Merkel"]; !ok {
		t.Fatalf("Merkel must be in the top-2 node universe: %+v", res.Nodes)
	}
	if _, ok := nodeBy["Scholz"]; ok {
		t.Fatalf("Scholz must be excluded by the maxNodes=2 universe: %+v", res.Nodes)
	}
	// TotalCount is the FULL summed weight (Berlin=6), not just the returned edge.
	if nodeBy["Berlin"].TotalCount != 6 {
		t.Errorf("Berlin TotalCount: want 6 (full summed weight), got %d", nodeBy["Berlin"].TotalCount)
	}
	if len(res.Edges) != 1 || res.Edges[0].A != "Berlin" || res.Edges[0].B != "Merkel" {
		t.Fatalf("node-first edges: want [Berlin–Merkel], got %+v", res.Edges)
	}

	// Sanity: edge-first (maxNodes=0) on the same seed surfaces all 4 entities.
	resEdge, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau"}, qs, qe, 10, "", "", 0, false, "", 0)
	if err != nil {
		t.Fatalf("edge-first query: %v", err)
	}
	if len(resEdge.Nodes) != 4 || len(resEdge.Edges) != 3 {
		t.Fatalf("edge-first: want 4 nodes / 3 edges, got %d / %d", len(resEdge.Nodes), len(resEdge.Edges))
	}
}

// Phase 148g — node-first CONNECTIVITY: per-node top-K keeps each node's
// strongest edge(s), so a weak peripheral pair is NOT starved by the dominant
// hub edges (the old "global top-N edges" left them isolated → central blob).
func TestGetEntityCoOccurrence_NodeFirstConnectivity(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	// A dominant hub (Berlin) + a weak peripheral pair (Habeck–Baerbock). A global
	// top-1-edge selection would keep only Berlin–Merkel and strand the others.
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "tagesschau", "a1", "Berlin", "LOC", "Merkel", "PER", uint32(100), uint64(1)},
		{w0, w1, "tagesschau", "a2", "Berlin", "LOC", "Scholz", "PER", uint32(50), uint64(1)},
		{w0, w1, "tagesschau", "a3", "Habeck", "PER", "Baerbock", "PER", uint32(5), uint64(1)},
	})
	// maxNodes=5 (all), topN=5 → K = round(5/5) = 1 (per-node top-1).
	res, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"),
		5, "", "", 0, false, "", 5)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Nodes) != 5 {
		t.Fatalf("want all 5 entities, got %d: %+v", len(res.Nodes), res.Nodes)
	}
	// EVERY node must keep at least one edge — no isolated nodes (the blob fix).
	for _, n := range res.Nodes {
		if n.Degree < 1 {
			t.Errorf("node %q is isolated (degree 0) — per-node top-K must connect it", n.Text)
		}
	}
	// The weak peripheral pair survives precisely because it is each other's top-1.
	var habeckBaerbock bool
	for _, e := range res.Edges {
		if (e.A == "Baerbock" && e.B == "Habeck") || (e.A == "Habeck" && e.B == "Baerbock") {
			habeckBaerbock = true
		}
	}
	if !habeckBaerbock {
		t.Errorf("the weak Habeck–Baerbock edge must survive per-node top-K, got edges %+v", res.Edges)
	}
}

// Phase 131a — per-edge source presence + pipeline-gap diagnostic.
func TestGetEntityCoOccurrence_EdgePresenceAcrossSources(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "tagesschau", "art-1", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "bundesregierung", "art-2", "Berlin", "LOC", "Merkel", "PER", uint32(1), uint64(1000)},
		{w0, w1, "tagesschau", "art-3", "Berlin", "LOC", "Scholz", "PER", uint32(2), uint64(1000)},
	})

	res, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau", "bundesregierung"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 10, "", "", 0, false, "", 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	edgePresence := map[string][]string{}
	for _, e := range res.Edges {
		edgePresence[e.A+"|"+e.B] = e.Presence
	}
	if bm := edgePresence["Berlin|Merkel"]; len(bm) != 2 {
		t.Fatalf("Berlin/Merkel should appear in 2 sources, got %d: %v", len(bm), bm)
	}
	if bs := edgePresence["Berlin|Scholz"]; len(bs) != 1 || bs[0] != "tagesschau" {
		t.Fatalf("Berlin/Scholz should appear only in tagesschau, got %v", bs)
	}
}

// Phase 131a — articlesInScope counts entity-bearing articles regardless of
// whether co-occurrence rows exist for them.
func TestGetEntityCoOccurrence_ArticlesInScopePipelineGap(t *testing.T) {
	s, ctx := setupTestStore(t)
	base := mustParse(t, "2026-04-24T10:00:00Z")
	entityRows := [][]any{
		{base, "tagesschau", "art-1", "Berlin", "LOC", uint32(0), uint32(6)},
		{base, "tagesschau", "art-1", "Merkel", "PER", uint32(10), uint32(16)},
		{base, "tagesschau", "art-2", "Hamburg", "LOC", uint32(0), uint32(7)},
		{base, "tagesschau", "art-2", "Scholz", "PER", uint32(8), uint32(14)},
		{base, "tagesschau", "art-3", "Bayern", "LOC", uint32(0), uint32(6)}, // entity-only → excluded
	}
	if err := bulkInsert(ctx, s, "aer_gold.entities",
		[]string{"timestamp", "source", "article_id", "entity_text", "entity_label", "start_char", "end_char"},
		entityRows); err != nil {
		t.Fatalf("seed entities: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(ctx, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 10, "", "", 0, false, "", 0)
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
// display label, leaves unlinked nodes on source surface form, and surfaces the
// linked/labeled coverage counts.
func TestGetEntityCoOccurrence_ViewerLanguageRelabel(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "franceinfo", "art-1", "Macron", "PER", "Russie", "LOC", uint32(3), uint64(1000)},
		{w0, w1, "franceinfo", "art-2", "Moïse Kouame", "PER", "Russie", "LOC", uint32(1), uint64(1000)},
	})
	// entity_links resolves the two linkable spans to QIDs.
	if err := bulkInsert(ctx, s, "aer_gold.entity_links", entityLinkCols, [][]any{
		{w0, "art-1", "Russie", "LOC", "Q159", float32(1.0), "exact_match", uint64(1000)},
		{w0, "art-1", "Macron", "PER", "Q3052772", float32(1.0), "exact_match", uint64(1000)},
	}); err != nil {
		t.Fatalf("seed entity_links: %v", err)
	}
	// wikidata_labels carries de for Q159 but NOT Q3052772 → Macron linked-unlabeled.
	if err := bulkInsert(ctx, s, "aer_gold.wikidata_labels",
		[]string{"wikidata_qid", "language", "label", "updated_at"}, [][]any{
			{"Q159", "de", "Russland", w0},
			{"Q159", "fr", "Russie", w0},
			{"Q3052772", "fr", "Emmanuel Macron", w0},
		}); err != nil {
		t.Fatalf("seed wikidata_labels: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(ctx, []string{"franceinfo"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 10, "de", "", 0, false, "", 0)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	nodeBy := map[string]CoOccurrenceNode{}
	for _, n := range res.Nodes {
		nodeBy[n.Text] = n
	}
	if got := nodeBy["Russie"].ViewerLabel; got != "Russland" {
		t.Fatalf("Russie should relabel to Russland in de, got %q", got)
	}
	if nodeBy["Macron"].WikidataQid == "" {
		t.Fatalf("Macron should be linked")
	}
	if got := nodeBy["Macron"].ViewerLabel; got != "" {
		t.Fatalf("Macron has no de label, expected empty ViewerLabel, got %q", got)
	}
	if nodeBy["Moïse Kouame"].WikidataQid != "" || nodeBy["Moïse Kouame"].ViewerLabel != "" {
		t.Fatalf("unlinked node must stay on source form: %+v", nodeBy["Moïse Kouame"])
	}
	if res.LinkedNodeCount != 2 {
		t.Fatalf("expected LinkedNodeCount=2, got %d", res.LinkedNodeCount)
	}
	if res.LabeledNodeCount != 1 {
		t.Fatalf("expected LabeledNodeCount=1, got %d", res.LabeledNodeCount)
	}
}

// Phase 123b — without a viewer language nothing relabels, but the linked
// coverage count is still surfaced (labeled count zero).
func TestGetEntityCoOccurrence_NoViewerLanguageNoRelabel(t *testing.T) {
	s, ctx := setupTestStore(t)
	w0 := mustParse(t, "2026-04-24T10:00:00Z")
	w1 := mustParse(t, "2026-04-24T11:00:00Z")
	seedCooc(t, ctx, s, [][]any{
		{w0, w1, "franceinfo", "art-1", "Macron", "PER", "Russie", "LOC", uint32(3), uint64(1000)},
	})
	if err := bulkInsert(ctx, s, "aer_gold.entity_links", entityLinkCols, [][]any{
		{w0, "art-1", "Russie", "LOC", "Q159", float32(1.0), "exact_match", uint64(1000)},
	}); err != nil {
		t.Fatalf("seed entity_links: %v", err)
	}
	if err := bulkInsert(ctx, s, "aer_gold.wikidata_labels",
		[]string{"wikidata_qid", "language", "label", "updated_at"},
		[][]any{{"Q159", "de", "Russland", w0}}); err != nil {
		t.Fatalf("seed wikidata_labels: %v", err)
	}

	res, err := s.GetEntityCoOccurrence(ctx, []string{"franceinfo"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 10, "", "", 0, false, "", 0)
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
// Scatter (Phase 131) — paired-metric pivot by article.
// ---------------------------------------------------------------------------

func seedScatterFixture(t *testing.T, ctx contextOnly, s *ClickHouseStorage) {
	t.Helper()
	base := mustParse(t, "2026-04-24T10:00:00Z")
	rows := [][]any{
		{base, 100.0, "tagesschau", "word_count", "a1"}, // full triple
		{base, 0.5, "tagesschau", "sentiment_score", "a1"},
		{base, 3.0, "tagesschau", "entity_count", "a1"},
		{base, 200.0, "tagesschau", "word_count", "a2"}, // x + y only
		{base, -0.2, "tagesschau", "sentiment_score", "a2"},
		{base, 300.0, "tagesschau", "word_count", "a3"}, // x only → excluded (no y)
		{base, 50.0, "wikipedia", "word_count", "a4"},   // off-source → excluded
		{base, 0.1, "wikipedia", "sentiment_score", "a4"},
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics", metricCols, rows); err != nil {
		t.Fatalf("seed scatter: %v", err)
	}
}

func TestGetMetricScatter_PivotsByArticleAndBindsSize(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedScatterFixture(t, ctx, s)

	size := "entity_count"
	res, err := s.GetMetricScatter(ctx, "word_count", "sentiment_score", &size, nil, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 2000, nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Points) != 2 { // a1 + a2; a3 (no y) and a4 (off-source) drop out
		t.Fatalf("expected 2 points, got %d: %+v", len(res.Points), res.Points)
	}
	if res.Truncated {
		t.Fatalf("did not expect truncation under a 2000 cap")
	}
	p1 := res.Points[0] // ordered by article id: a1 first
	if p1.ArticleID != "a1" || p1.X != 100.0 || p1.Y != 0.5 {
		t.Fatalf("a1 point wrong: %+v", p1)
	}
	if p1.Size == nil || *p1.Size != 3.0 {
		t.Fatalf("a1 size channel should be bound to 3.0, got %v", p1.Size)
	}
	if p2 := res.Points[1]; p2.ArticleID != "a2" || p2.Size != nil {
		t.Fatalf("a2 should have no size value: %+v", p2)
	}
}

func TestGetMetricScatter_TruncationFlag(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedScatterFixture(t, ctx, s)

	res, err := s.GetMetricScatter(ctx, "word_count", "sentiment_score", nil, nil, []string{"tagesschau"},
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"), 1, nil)
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
	// {0.2,0.4,0.6,0.8} ≈ 0.2582.
	rows := [][]any{
		{base, 0.2, "tagesschau", "sentiment_score", "s1"},
		{base.Add(time.Minute), 0.4, "tagesschau", "sentiment_score", "s2"},
		{base.Add(2 * time.Minute), 0.6, "tagesschau", "sentiment_score", "s3"},
		{base.Add(3 * time.Minute), 0.8, "tagesschau", "sentiment_score", "s4"},
	}
	if err := bulkInsert(ctx, s, "aer_gold.metrics", metricCols, rows); err != nil {
		t.Fatalf("seed spread: %v", err)
	}

	got, err := s.GetMetricsWithSpread(ctx,
		mustParse(t, "2026-04-24T00:00:00Z"), mustParse(t, "2026-04-25T00:00:00Z"),
		[]string{"tagesschau"}, strPtrTest("sentiment_score"), ResolutionHourly)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got.Rows) != 1 {
		t.Fatalf("expected one hourly bucket, got %d", len(got.Rows))
	}
	if got.Rows[0].Count != 4 {
		t.Fatalf("expected count 4, got %d", got.Rows[0].Count)
	}
	if got.Rows[0].Stddev < 0.25 || got.Rows[0].Stddev > 0.27 {
		t.Fatalf("sample stddev out of expected range (~0.258): %f", got.Rows[0].Stddev)
	}
}
