package storage

import (
	"testing"
	"time"
)

// TestResolveArticle_FromSilverDocuments_NoPostgresRow asserts that
// ResolveArticle resolves (article_id) → (bronze_object_key, source)
// from aer_silver.documents *without* a Postgres documents row. This is
// the regression guard for Phase 113b / ADR-022: between days 90 and
// 365 of an article's life the Postgres row is gone but the analytical
// projection still exists, and "View article" must succeed.
func TestResolveArticle_FromSilverDocuments_NoPostgresRow(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{
			"timestamp", "source", "article_id", "language",
			"cleaned_text_length", "word_count", "raw_entity_count",
			"ingestion_version", "bronze_object_key",
		},
		[][]any{
			{
				mustParse(t, "2026-01-15T08:00:00Z"),
				"tagesschau", "art-pre-retention", "de",
				uint32(1200), uint32(180), uint32(20),
				uint64(1), "tagesschau/2026/01/15/art-pre-retention.json",
			},
		}); err != nil {
		t.Fatalf("seed silver documents: %v", err)
	}

	// Pass nil for the Postgres pool — ResolveArticle must not consult
	// it under ADR-022. If the implementation regresses to a Postgres
	// query the nil pointer dereferences and this test fails loudly.
	store := &DossierStore{db: nil, chCn: s.conn}
	res, err := store.ResolveArticle(ctx, "art-pre-retention")
	if err != nil {
		t.Fatalf("ResolveArticle: %v", err)
	}
	if res.SourceName != "tagesschau" {
		t.Errorf("source mismatch: got %q want %q", res.SourceName, "tagesschau")
	}
	if res.BronzeObjectKey != "tagesschau/2026/01/15/art-pre-retention.json" {
		t.Errorf("bronze key mismatch: got %q", res.BronzeObjectKey)
	}
}

// TestResolveArticle_NotFound covers the empty-result path: an
// article_id that has no Silver projection row resolves to
// ErrSourceNotFound (the recycled sentinel the handler maps to 404).
func TestResolveArticle_NotFound(t *testing.T) {
	s, ctx := setupTestStore(t)
	store := &DossierStore{db: nil, chCn: s.conn}
	if _, err := store.ResolveArticle(ctx, "missing-article"); err != ErrSourceNotFound {
		t.Fatalf("expected ErrSourceNotFound, got %v", err)
	}
}

// TestResolveArticle_IgnoresEmptyBronzeKey covers the migration-window
// path: a row written before Migration 013 carries the empty-string
// DEFAULT for bronze_object_key. ResolveArticle must treat that as
// "not yet repopulated" rather than handing the BFF an empty key that
// would 404 on MinIO.
func TestResolveArticle_IgnoresEmptyBronzeKey(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{
			"timestamp", "source", "article_id", "language",
			"cleaned_text_length", "word_count", "raw_entity_count",
			"ingestion_version", "bronze_object_key",
		},
		[][]any{
			{
				mustParse(t, "2026-01-10T08:00:00Z"),
				"tagesschau", "art-legacy", "de",
				uint32(900), uint32(140), uint32(10),
				uint64(1), "",
			},
		}); err != nil {
		t.Fatalf("seed silver documents: %v", err)
	}

	store := &DossierStore{db: nil, chCn: s.conn}
	if _, err := store.ResolveArticle(ctx, "art-legacy"); err != ErrSourceNotFound {
		t.Fatalf("expected ErrSourceNotFound for empty key, got %v", err)
	}
}

// TestFetchSourceCounts_FromSilverDocuments confirms that per-source
// totals, in-window totals, and publication-frequency-per-day are read
// from the analytical layer rather than Postgres. Two articles a day
// apart on the same source → freq ≈ 2/1 = 2.0 (1-day floor).
func TestFetchSourceCounts_FromSilverDocuments(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{
			"timestamp", "source", "article_id", "language",
			"cleaned_text_length", "word_count", "raw_entity_count",
			"ingestion_version", "bronze_object_key",
		},
		[][]any{
			{
				mustParse(t, "2026-04-20T10:00:00Z"),
				"tagesschau", "art-1", "de",
				uint32(1000), uint32(150), uint32(15),
				uint64(1), "tagesschau/2026/04/20/art-1.json",
			},
			{
				mustParse(t, "2026-04-21T10:00:00Z"),
				"tagesschau", "art-2", "de",
				uint32(1100), uint32(160), uint32(18),
				uint64(1), "tagesschau/2026/04/21/art-2.json",
			},
			// Off-source row to prove the IN filter kicks in.
			{
				mustParse(t, "2026-04-21T10:00:00Z"),
				"wikipedia", "art-wiki", "en",
				uint32(2000), uint32(300), uint32(40),
				uint64(1), "wikipedia/2026/04/21/art-wiki.json",
			},
		}); err != nil {
		t.Fatalf("seed silver documents: %v", err)
	}

	store := &DossierStore{db: nil, chCn: s.conn}

	// Window covering both tagesschau articles → in_window == total.
	winStart := mustParse(t, "2026-04-19T00:00:00Z")
	winEnd := mustParse(t, "2026-04-22T00:00:00Z")
	counts, err := store.fetchSourceCounts(ctx, []string{"tagesschau"}, &winStart, &winEnd)
	if err != nil {
		t.Fatalf("fetchSourceCounts: %v", err)
	}
	c, ok := counts["tagesschau"]
	if !ok {
		t.Fatalf("missing tagesschau count")
	}
	if c.total != 2 || c.inWindow != 2 {
		t.Errorf("count mismatch: %+v", c)
	}
	if c.freqPerDay <= 0 {
		t.Errorf("expected positive freqPerDay, got %v", c.freqPerDay)
	}

	// Narrow window catching only the first article → in_window=1, total=2.
	narrowEnd := mustParse(t, "2026-04-20T23:00:00Z")
	counts, err = store.fetchSourceCounts(ctx, []string{"tagesschau"}, &winStart, &narrowEnd)
	if err != nil {
		t.Fatalf("fetchSourceCounts narrow: %v", err)
	}
	c = counts["tagesschau"]
	if c.total != 2 {
		t.Errorf("total should still be 2, got %d", c.total)
	}
	if c.inWindow != 1 {
		t.Errorf("inWindow should be 1, got %d", c.inWindow)
	}
}

// TestFetchSourceCounts_NoChConn confirms the nil-connection escape
// hatch returns an empty map cleanly so handler tests that don't wire
// ClickHouse can still call FetchSources.
func TestFetchSourceCounts_NoChConn(t *testing.T) {
	store := &DossierStore{db: nil, chCn: nil}
	counts, err := store.fetchSourceCounts(t.Context(), []string{"tagesschau"}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %v", counts)
	}
	_ = time.Now()
}
