package storage

import (
	"context"
	"testing"
	"time"
)

var (
	artStart = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	artEnd   = time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	artTS    = time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)
)

// ---------------------------------------------------------------------------
// GetSourceArticles + lookupTopLanguages
// ---------------------------------------------------------------------------

func TestGetSourceArticles_PivotsPerArticleWithLanguage(t *testing.T) {
	s, ctx := setupTestStore(t)

	// a1: word_count + sentiment; a2: word_count only.
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{artTS, 180.0, "tagesschau", "word_count", "a1"},
			{artTS, 0.4, "tagesschau", "sentiment_score", "a1"},
			{artTS.Add(time.Hour), 120.0, "tagesschau", "word_count", "a2"},
			// Off-source row.
			{artTS, 50.0, "wikipedia", "word_count", "w1"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}
	if err := bulkInsert(ctx, s, "aer_gold.language_detections",
		[]string{"timestamp", "source", "article_id", "detected_language", "confidence", "rank"},
		[][]any{{artTS, "tagesschau", "a1", "de", 0.99, uint8(1)}}); err != nil {
		t.Fatalf("seed language detections: %v", err)
	}

	start, end := artStart, artEnd
	rows, err := s.GetSourceArticles(ctx, "tagesschau", ArticleQueryFilter{Start: &start, End: &end, Limit: 10})
	if err != nil {
		t.Fatalf("GetSourceArticles: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 tagesschau articles, got %d: %+v", len(rows), rows)
	}
	byID := map[string]ArticleAggRow{}
	for _, r := range rows {
		byID[r.ArticleID] = r
	}
	a1 := byID["a1"]
	if a1.WordCount != 180 || !a1.HasWordCount {
		t.Errorf("a1 word count: want 180/present, got %d/%v", a1.WordCount, a1.HasWordCount)
	}
	if a1.SentimentScore < 0.39 || a1.SentimentScore > 0.41 || !a1.HasSentiment {
		t.Errorf("a1 sentiment: want ~0.4/present, got %v/%v", a1.SentimentScore, a1.HasSentiment)
	}
	if a1.Language != "de" || !a1.HasLanguage {
		t.Errorf("a1 language: want de/present, got %q/%v", a1.Language, a1.HasLanguage)
	}
	a2 := byID["a2"]
	if a2.HasSentiment {
		t.Errorf("a2 should have no sentiment, got %v", a2.SentimentScore)
	}
	if a2.HasLanguage {
		t.Errorf("a2 should have no language, got %q", a2.Language)
	}
	// Ordered newest-first: a2 (later) before a1.
	if rows[0].ArticleID != "a2" {
		t.Errorf("ordering: want a2 first (newest), got %q", rows[0].ArticleID)
	}
}

func TestGetSourceArticles_SentimentBandFilter(t *testing.T) {
	s, ctx := setupTestStore(t)

	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{artTS, 100.0, "tagesschau", "word_count", "pos"},
			{artTS, 0.5, "tagesschau", "sentiment_score", "pos"},
			{artTS, 100.0, "tagesschau", "word_count", "neg"},
			{artTS, -0.5, "tagesschau", "sentiment_score", "neg"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	band := "positive"
	rows, err := s.GetSourceArticles(ctx, "tagesschau", ArticleQueryFilter{SentimentBand: &band, Limit: 10})
	if err != nil {
		t.Fatalf("GetSourceArticles: %v", err)
	}
	if len(rows) != 1 || rows[0].ArticleID != "pos" {
		t.Fatalf("positive band: want [pos], got %+v", rows)
	}

	bad := "invalid"
	if _, err := s.GetSourceArticles(ctx, "tagesschau", ArticleQueryFilter{SentimentBand: &bad}); err == nil {
		t.Error("invalid sentiment band must return an error")
	}
}

func TestLookupTopLanguages_EmptyArticleIDs(t *testing.T) {
	s, ctx := setupTestStore(t)
	got, err := s.lookupTopLanguages(ctx, "tagesschau", nil)
	if err != nil {
		t.Fatalf("lookupTopLanguages: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty ids must return empty map, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// lookupArticleRevisions + GetSourceArticles with IncludeRevisions
// ---------------------------------------------------------------------------

func TestLookupArticleRevisions_CountsEditorialChanges(t *testing.T) {
	s, ctx := setupTestStore(t)

	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}
	identicalDiff := []string{`{"op": "identical"}`}
	// a1: one editorial change + one identical re-archival (chain_length 2, edits 1).
	insertRevisionWithDeltas(t, s, ctx, "a1", "tagesschau", artTS, "h1", changeDiff, 0, 0, nil, nil, true)
	insertRevisionWithDeltas(t, s, ctx, "a1", "tagesschau", artTS.Add(time.Hour), "h2", identicalDiff, 0, 0, nil, nil, true)

	got, err := s.lookupArticleRevisions(ctx, []string{"a1"})
	if err != nil {
		t.Fatalf("lookupArticleRevisions: %v", err)
	}
	rev, ok := got["a1"]
	if !ok {
		t.Fatal("a1 missing from revision map")
	}
	if rev.ChainLength != 2 {
		t.Errorf("chain_length: want 2 (all captures), got %d", rev.ChainLength)
	}
	if rev.EditorialChangeCount != 1 {
		t.Errorf("editorial_change_count: want 1 (identical excluded), got %d", rev.EditorialChangeCount)
	}

	if empty, err := s.lookupArticleRevisions(ctx, nil); err != nil || len(empty) != 0 {
		t.Errorf("empty ids: want empty map no error, got (%v, %v)", empty, err)
	}
}

// ---------------------------------------------------------------------------
// CountAggregationGroup (k-anonymity gate)
// ---------------------------------------------------------------------------

func TestCountAggregationGroup_DistinctArticlesPerDay(t *testing.T) {
	s, ctx := setupTestStore(t)

	// Three distinct articles same day, same (source, metric).
	if err := bulkInsert(ctx, s, "aer_gold.metrics",
		[]string{"timestamp", "value", "source", "metric_name", "article_id"},
		[][]any{
			{artTS, 1.0, "tagesschau", "sentiment_score", "a1"},
			{artTS.Add(time.Hour), 1.0, "tagesschau", "sentiment_score", "a2"},
			{artTS.Add(2 * time.Hour), 1.0, "tagesschau", "sentiment_score", "a3"},
			// Different day — excluded.
			{artTS.Add(48 * time.Hour), 1.0, "tagesschau", "sentiment_score", "a4"},
		}); err != nil {
		t.Fatalf("seed metrics: %v", err)
	}

	n, err := s.CountAggregationGroup(ctx, "tagesschau", "sentiment_score", artTS)
	if err != nil {
		t.Fatalf("CountAggregationGroup: %v", err)
	}
	if n != 3 {
		t.Errorf("want 3 distinct articles in the day, got %d", n)
	}

	// A day with no articles → 0.
	n, err = s.CountAggregationGroup(ctx, "tagesschau", "sentiment_score", artTS.Add(240*time.Hour))
	if err != nil {
		t.Fatalf("CountAggregationGroup empty: %v", err)
	}
	if n != 0 {
		t.Errorf("empty day: want 0, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// GetArticleProvenance (stub)
// ---------------------------------------------------------------------------

func TestGetArticleProvenance_EmptyMap(t *testing.T) {
	s, ctx := setupTestStore(t)
	got, err := s.GetArticleProvenance(ctx, "a1")
	if err != nil {
		t.Fatalf("GetArticleProvenance: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("want empty non-nil map, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetArticleRevisions (per-article chain)
// ---------------------------------------------------------------------------

func TestGetArticleRevisions_OrderedChainWithDiffStatus(t *testing.T) {
	s, ctx := setupTestStore(t)

	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}
	identicalDiff := []string{`{"op": "identical"}`}
	insertRevisionWithDeltas(t, s, ctx, "a1", "tagesschau", artTS, "h1", changeDiff, 0.3, 0.2, []string{"X"}, nil, true)
	insertRevisionWithDeltas(t, s, ctx, "a1", "tagesschau", artTS.Add(time.Hour), "h2", identicalDiff, 0, 0, nil, nil, true)

	rows, err := s.GetArticleRevisions(ctx, "a1")
	if err != nil {
		t.Fatalf("GetArticleRevisions: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 revisions, got %d", len(rows))
	}
	// Ordered by snapshot_at; first is the editorial change.
	if rows[0].DiffStatus != "changed" {
		t.Errorf("first revision diff status: want changed, got %q", rows[0].DiffStatus)
	}
	if rows[1].DiffStatus != "identical" {
		t.Errorf("second revision diff status: want identical, got %q", rows[1].DiffStatus)
	}
	if rows[0].SentimentDelta < 0.29 || rows[0].SentimentDelta > 0.31 {
		t.Errorf("first revision sentiment delta: want ~0.3, got %v", rows[0].SentimentDelta)
	}

	if empty, err := s.GetArticleRevisions(ctx, ""); err != nil || empty != nil {
		t.Errorf("empty article id: want nil no error, got (%v, %v)", empty, err)
	}
}

// ---------------------------------------------------------------------------
// GetArticleRevisionDiff (revisions_diff_query.go)
// ---------------------------------------------------------------------------

func TestGetArticleRevisionDiff_PairKinds(t *testing.T) {
	s, ctx := setupTestStore(t)

	changeDiff := []string{`{"op": "mod", "before": "old", "after": "new"}`}
	// revisionIndex 0 (chain head) and 1 (mid-chain).
	insertRevisionAtIndex(t, s, ctx, "a1", "tagesschau", artTS, "h0", 0, changeDiff, true, "Headline A", "Headline B")
	insertRevisionAtIndex(t, s, ctx, "a1", "tagesschau", artTS.Add(time.Hour), "h1", 1, changeDiff, false, "", "")

	// Mid-chain pair resolves the predecessor's snapshot_at.
	row, err := s.GetArticleRevisionDiff(ctx, "a1", 1)
	if err != nil {
		t.Fatalf("GetArticleRevisionDiff(idx 1): %v", err)
	}
	if row.RevisionIndex != 1 {
		t.Errorf("revision index: want 1, got %d", row.RevisionIndex)
	}
	if row.SnapshotAtBefore.IsZero() {
		t.Error("mid-chain pair must resolve a non-zero before-anchor")
	}
	if len(row.DiffParagraphs) == 0 {
		t.Error("expected diff paragraphs on a changed pair")
	}

	// Chain-head pair (index 0) with a headline change.
	head, err := s.GetArticleRevisionDiff(ctx, "a1", 0)
	if err != nil {
		t.Fatalf("GetArticleRevisionDiff(idx 0): %v", err)
	}
	if !head.HeadlineChanged || head.HeadlineAfter != "Headline B" {
		t.Errorf("head pair headline: want changed to 'Headline B', got %v/%q", head.HeadlineChanged, head.HeadlineAfter)
	}

	// Unknown pair → ErrSourceNotFound.
	if _, err := s.GetArticleRevisionDiff(ctx, "a1", 99); err != ErrSourceNotFound {
		t.Errorf("unknown pair: want ErrSourceNotFound, got %v", err)
	}
}

func TestGetArticleRevisionDiff_PendingWhenNoDiffNoHeadline(t *testing.T) {
	s, ctx := setupTestStore(t)
	// A row with empty diff_paragraphs and no headline change → pending.
	insertRevisionAtIndex(t, s, ctx, "a1", "tagesschau", artTS, "h0", 0, nil, false, "", "")
	if _, err := s.GetArticleRevisionDiff(ctx, "a1", 0); err != ErrRevisionDiffPending {
		t.Errorf("want ErrRevisionDiffPending, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetRevisionsArticles (paginated drill-down list)
// ---------------------------------------------------------------------------

func TestGetRevisionsArticles_EditsOnlyDrillDown(t *testing.T) {
	s, ctx := setupTestStore(t)

	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}
	identicalDiff := []string{`{"op": "identical"}`}
	// a1: a real editorial change → appears.
	insertRevisionWithDeltas(t, s, ctx, "a1", "tagesschau", artTS, "h1", changeDiff, 0, 0, nil, nil, true)
	// a2: only an identical re-archival → must NOT appear (edits-only).
	insertRevisionWithDeltas(t, s, ctx, "a2", "tagesschau", artTS.Add(time.Hour), "h2", identicalDiff, 0, 0, nil, nil, true)
	// Silver projection so language/word_count attach.
	if err := bulkInsert(ctx, s, "aer_silver.documents",
		[]string{"timestamp", "source", "article_id", "language", "cleaned_text_length", "word_count", "raw_entity_count", "ingestion_version", "bronze_object_key"},
		[][]any{{artTS, "tagesschau", "a1", "de", uint32(1000), uint32(150), uint32(10), uint64(1), "k"}}); err != nil {
		t.Fatalf("seed silver: %v", err)
	}

	rows, err := s.GetRevisionsArticles(ctx, RevisionsArticlesFilter{
		Sources: []string{"tagesschau"}, Start: artStart, End: artEnd, Limit: 50,
	})
	if err != nil {
		t.Fatalf("GetRevisionsArticles: %v", err)
	}
	if len(rows) != 1 || rows[0].ArticleID != "a1" {
		t.Fatalf("want only a1 (editorial change), got %+v", rows)
	}
	if rows[0].Language != "de" || rows[0].WordCount != 150 {
		t.Errorf("a1 silver join: want de/150, got %q/%d", rows[0].Language, rows[0].WordCount)
	}
	if rows[0].EditorialChangeCount != 1 {
		t.Errorf("editorial change count: want 1, got %d", rows[0].EditorialChangeCount)
	}

	// Empty source set → no rows.
	if empty, err := s.GetRevisionsArticles(ctx, RevisionsArticlesFilter{}); err != nil || empty != nil {
		t.Errorf("empty sources: want nil no error, got (%v, %v)", empty, err)
	}
}

// insertRevisionAtIndex inserts a revision row carrying a specific
// revision_index and optional headline-change fields.
func insertRevisionAtIndex(t *testing.T, s *ClickHouseStorage, ctx context.Context, articleID, source string, snapshotAt time.Time, hash string, revIndex uint32, diff []string, headlineChanged bool, headlineBefore, headlineAfter string) {
	t.Helper()
	err := s.conn.Exec(ctx, `
		INSERT INTO aer_gold.article_revisions
			(article_id, source, snapshot_at, content_hash, revision_index, revision_trigger,
			 ingestion_version, diff_paragraphs, headline_changed, headline_before, headline_after)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		articleID, source, snapshotAt, hash, revIndex, "cdx_snapshot", uint64(1000),
		diff, headlineChanged, headlineBefore, headlineAfter)
	if err != nil {
		t.Fatalf("insert revision at index %d: %v", revIndex, err)
	}
}
