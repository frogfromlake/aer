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
