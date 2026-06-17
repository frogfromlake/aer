package storage

import (
	"testing"
	"time"
)

// Phase 103b storage-layer integration tests against the same Memory /
// Testcontainer ClickHouse used by the Phase 102 view-mode tests. The
// fixtures seed `aer_silver.documents` with a small Probe-0-style sample
// and exercise each aggregation kind end-to-end.

func seedSilverFixture(t *testing.T, s *ClickHouseStorage, ctx hasContext) {
	t.Helper()
	base := mustParse(t, "2026-04-20T10:00:00Z")
	rows := [][]any{
		// (timestamp, source, article_id, language, cleaned_text_length, word_count, raw_entity_count, ingestion_version)
		{base, "tagesschau", "a1", "de", uint32(120), uint32(20), uint32(5), uint64(1)},
		{base.Add(30 * time.Minute), "tagesschau", "a2", "de", uint32(240), uint32(40), uint32(8), uint64(2)},
		{base.Add(2 * time.Hour), "tagesschau", "a3", "de", uint32(60), uint32(10), uint32(2), uint64(3)},
		{base.Add(24 * time.Hour), "tagesschau", "a4", "de", uint32(360), uint32(60), uint32(12), uint64(4)},
		// Off-source row to prove scope filtering kicks in.
		{base, "wikipedia", "w1", "en", uint32(9999), uint32(9999), uint32(9999), uint64(5)},
	}
	if err := bulkInsert(ctx.Ctx(), s, "aer_silver.documents",
		[]string{"timestamp", "source", "article_id", "language", "cleaned_text_length", "word_count", "raw_entity_count", "ingestion_version"},
		rows); err != nil {
		t.Fatalf("seed silver documents: %v", err)
	}
}

func TestGetSilverDistribution_HistogramAndSummary(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedSilverFixture(t, s, contextWrap{ctx})

	res, err := s.GetSilverDistribution(
		ctx,
		"cleaned_text_length",
		"tagesschau",
		mustParse(t, "2026-04-20T00:00:00Z"),
		mustParse(t, "2026-04-22T00:00:00Z"),
		4,
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.Summary.Count != 4 {
		t.Fatalf("expected 4 rows in scope (off-source filtered), got %d", res.Summary.Count)
	}
	if res.Summary.Min < 59 || res.Summary.Max > 361 {
		t.Fatalf("min/max outside seeded range: %+v", res.Summary)
	}
	var total int64
	for _, b := range res.Bins {
		total += b.Count
	}
	if total != 4 {
		t.Fatalf("bin counts must sum to row count: %d", total)
	}
}

func TestGetSilverHeatmap_CleanedTextLengthByHour(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedSilverFixture(t, s, contextWrap{ctx})

	cells, xDim, yDim, err := s.GetSilverHeatmap(
		ctx,
		SilverAggCleanedTextLengthByHour,
		"tagesschau",
		mustParse(t, "2026-04-19T00:00:00Z"),
		mustParse(t, "2026-04-22T00:00:00Z"),
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if xDim != "dayOfWeek" || yDim != "hour" {
		t.Fatalf("unexpected dims: %s, %s", xDim, yDim)
	}
	var totalCount int64
	for _, c := range cells {
		totalCount += c.Count
	}
	if totalCount != 4 {
		t.Fatalf("cell counts must sum to row count: got %d", totalCount)
	}
}

func TestGetSilverCorrelation_PairwiseLengthVsWordCount(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedSilverFixture(t, s, contextWrap{ctx})

	res, err := s.GetSilverCorrelation(
		ctx,
		"tagesschau",
		mustParse(t, "2026-04-19T00:00:00Z"),
		mustParse(t, "2026-04-22T00:00:00Z"),
	)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(res.Fields) != 2 || res.Fields[0] != "cleaned_text_length" || res.Fields[1] != "word_count" {
		t.Fatalf("unexpected fields: %v", res.Fields)
	}
	if res.SampleCount != 4 {
		t.Fatalf("expected sampleCount=4, got %d", res.SampleCount)
	}
	// The fixture is exactly proportional (length ≈ 6 × word_count) so
	// correlation must be very close to 1.0; we leave a generous tolerance
	// because ClickHouse's exact corr() is rounding-stable.
	if res.Matrix[0][1] == nil || *res.Matrix[0][1] < 0.99 {
		t.Fatalf("expected ~1.0 correlation on perfectly linear fixture, got: %+v", res.Matrix[0][1])
	}
}

// TestGetSilverHeatmap_WordCountBySource exercises the second heatmap kind
// (source × dayOfWeek) and the unsupported-kind error branch.
func TestGetSilverHeatmap_WordCountBySource(t *testing.T) {
	s, ctx := setupTestStore(t)
	seedSilverFixture(t, s, contextWrap{ctx})

	cells, xDim, yDim, err := s.GetSilverHeatmap(
		ctx, SilverAggWordCountBySource, "tagesschau",
		mustParse(t, "2026-04-19T00:00:00Z"), mustParse(t, "2026-04-22T00:00:00Z"))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if xDim != "source" || yDim != "dayOfWeek" {
		t.Fatalf("unexpected dims: %s, %s", xDim, yDim)
	}
	var total int64
	for _, c := range cells {
		if c.X != "tagesschau" {
			t.Errorf("x dim should be the scoped source, got %q", c.X)
		}
		total += c.Count
	}
	if total != 4 {
		t.Errorf("cell counts must sum to scoped row count, got %d", total)
	}

	// Unsupported kind → error.
	if _, _, _, err := s.GetSilverHeatmap(ctx, SilverAggWordCount, "tagesschau",
		mustParse(t, "2026-04-19T00:00:00Z"), mustParse(t, "2026-04-22T00:00:00Z")); err == nil {
		t.Error("a non-heatmap kind must return an error")
	}
}

// TestGetSilverDistribution_EmptyAndDegenerate covers the empty-scope path and
// the zero-span single-bin path (every row carries the same value).
func TestGetSilverDistribution_EmptyAndDegenerate(t *testing.T) {
	s, ctx := setupTestStore(t)

	// No rows → empty result (count 0), no bins.
	empty, err := s.GetSilverDistribution(ctx, "word_count", "tagesschau",
		mustParse(t, "2026-04-20T00:00:00Z"), mustParse(t, "2026-04-22T00:00:00Z"), 4)
	if err != nil {
		t.Fatalf("empty query: %v", err)
	}
	if empty.Summary.Count != 0 || len(empty.Bins) != 0 {
		t.Errorf("empty scope must yield zero count and no bins, got %+v", empty.Summary)
	}

	// Three rows with an identical value → zero span → a single collapsed bin.
	base := mustParse(t, "2026-04-20T10:00:00Z")
	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{"timestamp", "source", "article_id", "language", "cleaned_text_length", "word_count", "raw_entity_count", "ingestion_version"},
		[][]any{
			{base, "tagesschau", "a1", "de", uint32(100), uint32(50), uint32(5), uint64(1)},
			{base, "tagesschau", "a2", "de", uint32(100), uint32(50), uint32(5), uint64(1)},
			{base, "tagesschau", "a3", "de", uint32(100), uint32(50), uint32(5), uint64(1)},
		}); err != nil {
		t.Fatalf("seed degenerate: %v", err)
	}
	res, err := s.GetSilverDistribution(ctx, "word_count", "tagesschau",
		mustParse(t, "2026-04-20T00:00:00Z"), mustParse(t, "2026-04-21T00:00:00Z"), 4)
	if err != nil {
		t.Fatalf("degenerate query: %v", err)
	}
	if res.Summary.Count != 3 {
		t.Fatalf("expected count 3, got %d", res.Summary.Count)
	}
	if len(res.Bins) != 1 || res.Bins[0].Count != 3 {
		t.Fatalf("zero-span distribution must collapse to one bin of 3, got %+v", res.Bins)
	}
	if res.Bins[0].Lower != res.Bins[0].Upper {
		t.Errorf("collapsed bin should have equal bounds, got %+v", res.Bins[0])
	}
}

// TestGetSilverCorrelation_InsufficientSamples covers the n<2 path: the
// off-diagonal stays nil (n/a) while diagonals remain 1.0.
func TestGetSilverCorrelation_InsufficientSamples(t *testing.T) {
	s, ctx := setupTestStore(t)

	base := mustParse(t, "2026-04-20T10:00:00Z")
	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{"timestamp", "source", "article_id", "language", "cleaned_text_length", "word_count", "raw_entity_count", "ingestion_version"},
		[][]any{{base, "tagesschau", "a1", "de", uint32(120), uint32(20), uint32(5), uint64(1)}}); err != nil {
		t.Fatalf("seed single row: %v", err)
	}
	res, err := s.GetSilverCorrelation(ctx, "tagesschau",
		mustParse(t, "2026-04-20T00:00:00Z"), mustParse(t, "2026-04-21T00:00:00Z"))
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if res.SampleCount != 0 {
		t.Errorf("single-sample correlation must report SampleCount=0, got %d", res.SampleCount)
	}
	if res.Matrix[0][1] != nil || res.Matrix[1][0] != nil {
		t.Errorf("off-diagonal must stay nil for n<2, got %+v", res.Matrix)
	}
	if res.Matrix[0][0] == nil || *res.Matrix[0][0] != 1.0 {
		t.Errorf("diagonal must remain 1.0, got %+v", res.Matrix[0][0])
	}
}
