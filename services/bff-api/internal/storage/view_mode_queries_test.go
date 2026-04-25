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
		{mustParse(t, "2026-04-20T10:00:00Z"), 0.5, "tagesschau", "sentiment_score", "a1"}, // Mon, 10:00
		{mustParse(t, "2026-04-20T10:30:00Z"), 0.3, "tagesschau", "sentiment_score", "a2"}, // Mon, 10:00
		{mustParse(t, "2026-04-20T11:00:00Z"), -0.1, "tagesschau", "sentiment_score", "a3"}, // Mon, 11:00
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
