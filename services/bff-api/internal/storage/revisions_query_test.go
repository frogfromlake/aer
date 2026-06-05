package storage

import (
	"context"
	"math"
	"testing"
	"time"
)

// TestGetRevisionActivity_CollapsesReattemptVersions pins the data-integrity
// invariant that revision totals are monotonic, never oscillating. The
// ADR-036 enrichment re-attempt loop re-writes an article's revision rows
// with a fresh ingestion_version each time it re-heals an incomplete Wayback
// lookup. `aer_gold.article_revisions` is a ReplacingMergeTree, so between the
// INSERT and the next background merge the old + new versions of a
// (article_id, snapshot_at, content_hash) tuple coexist as physical rows.
//
// Without `FINAL`, `count()` counts those physical rows — so the revision
// total transiently over-counts and then drops as merges settle (the bug:
// dashboard revision counts that rise then fall). This test writes the same
// revision tuple twice under different versions and asserts it is counted
// exactly once.
//
// Phase 133 — the activity count is now EDITORIAL CHANGES, not raw captures:
// a row counts only when its `diff_paragraphs` is a real change (not the
// `identical` re-archival sentinel and not empty/pending). The test seeds
// change-diffs on the counted rows and an identical re-archival that must be
// excluded.
func TestGetRevisionActivity_CollapsesReattemptVersions(t *testing.T) {
	store, ctx := setupTestStore(t)

	snap := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}
	identicalDiff := []string{`{"op": "identical"}`}

	insert := func(snapshotAt time.Time, hash string, version uint64, diff []string) {
		t.Helper()
		err := store.conn.Exec(ctx, `
			INSERT INTO aer_gold.article_revisions
				(article_id, source, snapshot_at, content_hash, revision_trigger, ingestion_version, diff_paragraphs)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"art-1", "elysee", snapshotAt, hash, "cdx_snapshot", version, diff)
		if err != nil {
			t.Fatalf("insert revision (%s, v%d): %v", hash, version, err)
		}
	}

	// Same editorial-change tuple written twice — exactly what the re-attempt
	// loop does when it re-heals "art-1". FINAL must collapse these to ONE.
	insert(snap, "deadbeef", 1000, changeDiff)
	insert(snap, "deadbeef", 2000, changeDiff)
	// A second, genuinely distinct editorial change of the same article. The
	// true editorial-revision count for the article is therefore 2.
	insert(snap.Add(time.Hour), "cafef00d", 1500, changeDiff)
	// An identical re-archival — a capture with no editorial change. It must
	// NOT be counted as a revision (Phase 133 edits-only semantics).
	insert(snap.Add(2*time.Hour), "f00dface", 1500, identicalDiff)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cells, err := store.GetRevisionActivity(
		ctx, []string{"elysee"}, start, end, RevisionResolutionSnapshot,
	)
	if err != nil {
		t.Fatalf("GetRevisionActivity: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("want 1 (source, bucket) cell, got %d", len(cells))
	}
	// 2 editorial revisions — the double-versioned change counted once (FINAL),
	// plus the distinct second change. The identical re-archival is excluded;
	// without FINAL the deduped change would also double-count to 3.
	if cells[0].Revisions != 2 {
		t.Errorf("revisions: want 2 (edits only, duplicate versions collapsed, identical excluded), got %d", cells[0].Revisions)
	}
	if cells[0].ArticlesAffected != 1 {
		t.Errorf("articles_affected: want 1, got %d", cells[0].ArticlesAffected)
	}
	if cells[0].CdxSnapshotCount != 2 {
		t.Errorf("cdx_snapshot_count: want 2 (edit rows via cdx), got %d", cells[0].CdxSnapshotCount)
	}
}

// insertRevisionWithDeltas inserts one article_revisions row including the
// Phase-122d.3 discourse-shift delta columns.
func insertRevisionWithDeltas(
	t *testing.T,
	store *ClickHouseStorage,
	ctx context.Context,
	articleID, source string,
	snapshotAt time.Time,
	hash string,
	diff []string,
	sentimentDelta, topicShift float64,
	added, removed []string,
	deltasComputed bool,
) {
	t.Helper()
	err := store.conn.Exec(ctx, `
		INSERT INTO aer_gold.article_revisions
			(article_id, source, snapshot_at, content_hash, revision_trigger, ingestion_version,
			 diff_paragraphs, sentiment_delta, topic_shift_score, entities_added, entities_removed, deltas_computed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		articleID, source, snapshotAt, hash, "cdx_snapshot", uint64(1000),
		diff, sentimentDelta, topicShift, added, removed, deltasComputed)
	if err != nil {
		t.Fatalf("insert revision-with-deltas (%s/%s): %v", source, hash, err)
	}
}

// TestGetRevisionDiscourseShift_AggregatesComputedDeltasOnly runtime-verifies
// the Phase-122d.3 aggregation SQL against a real ClickHouse: only edits with
// deltas_computed=true contribute, and the means/sums are correct.
func TestGetRevisionDiscourseShift_AggregatesComputedDeltasOnly(t *testing.T) {
	store, ctx := setupTestStore(t)

	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}
	identicalDiff := []string{`{"op": "identical"}`}

	// Two scored edits for elysee.
	insertRevisionWithDeltas(t, store, ctx, "a1", "elysee", base, "h1", changeDiff,
		0.4, 0.2, []string{"X", "Y"}, nil, true)
	insertRevisionWithDeltas(t, store, ctx, "a2", "elysee", base.Add(time.Hour), "h2", changeDiff,
		-0.2, 0.6, nil, []string{"Z"}, true)
	// A real edit but deltas NOT computed — must be excluded from the averages.
	insertRevisionWithDeltas(t, store, ctx, "a3", "elysee", base.Add(2*time.Hour), "h3", changeDiff,
		0.0, 0.0, nil, nil, false)
	// An identical re-archival — not an edit at all.
	insertRevisionWithDeltas(t, store, ctx, "a4", "elysee", base.Add(3*time.Hour), "h4", identicalDiff,
		0.0, 0.0, nil, nil, false)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	cells, err := store.GetRevisionDiscourseShift(ctx, []string{"elysee"}, start, end, RevisionResolutionSnapshot)
	if err != nil {
		t.Fatalf("GetRevisionDiscourseShift: %v", err)
	}
	if len(cells) != 1 {
		t.Fatalf("want 1 (source, bucket) cell, got %d", len(cells))
	}
	c := cells[0]
	if c.EditsWithDeltas != 2 {
		t.Errorf("editsWithDeltas: want 2 (only computed edits), got %d", c.EditsWithDeltas)
	}
	if !approx(c.AvgSentimentDelta, 0.1) { // (0.4 + -0.2) / 2
		t.Errorf("avgSentimentDelta: want 0.1, got %v", c.AvgSentimentDelta)
	}
	if !approx(c.NetSentimentDrift, 0.2) { // 0.4 + -0.2
		t.Errorf("netSentimentDrift: want 0.2, got %v", c.NetSentimentDrift)
	}
	if !approx(c.AvgTopicShift, 0.4) { // (0.2 + 0.6) / 2
		t.Errorf("avgTopicShift: want 0.4, got %v", c.AvgTopicShift)
	}
	if c.EntitiesAddedTotal != 2 { // {X,Y} + {}
		t.Errorf("entitiesAddedTotal: want 2, got %d", c.EntitiesAddedTotal)
	}
	if c.EntitiesRemovedTotal != 1 { // {} + {Z}
		t.Errorf("entitiesRemovedTotal: want 1, got %d", c.EntitiesRemovedTotal)
	}
}

// TestGetRevisionEditClusters_CrossSourceCoincidence runtime-verifies the
// Phase-122d.3 Rhizome clustering SQL: an entity co-edited by ≥2 sources in
// the same bucket is a cluster; a single-source entity is not.
func TestGetRevisionEditClusters_CrossSourceCoincidence(t *testing.T) {
	store, ctx := setupTestStore(t)

	day := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	changeDiff := []string{`{"op": "mod", "before": "a", "after": "b"}`}

	// Two DIFFERENT sources both edit "Macron" on the same day → a cluster.
	insertRevisionWithDeltas(t, store, ctx, "a1", "elysee", day, "h1", changeDiff,
		0.1, 0.3, []string{"Macron"}, nil, true)
	insertRevisionWithDeltas(t, store, ctx, "a2", "franceinfo", day.Add(time.Hour), "h2", changeDiff,
		-0.1, 0.5, nil, []string{"Macron"}, true)
	// A single-source entity → NOT a cluster.
	insertRevisionWithDeltas(t, store, ctx, "a3", "elysee", day, "h3", changeDiff,
		0.2, 0.1, []string{"Solo"}, nil, true)

	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	clusters, err := store.GetRevisionEditClusters(
		ctx, []string{"elysee", "franceinfo"}, start, end, RevisionResolutionDaily, 2,
	)
	if err != nil {
		t.Fatalf("GetRevisionEditClusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("want 1 cluster (Macron, 2 sources), got %d", len(clusters))
	}
	cl := clusters[0]
	if cl.Entity != "Macron" {
		t.Errorf("entity: want Macron, got %q", cl.Entity)
	}
	if len(cl.Sources) != 2 {
		t.Errorf("sources: want 2 distinct, got %d (%v)", len(cl.Sources), cl.Sources)
	}
	if cl.EditCount != 2 {
		t.Errorf("editCount: want 2, got %d", cl.EditCount)
	}
	if !approx(cl.AvgTopicShift, 0.4) { // (0.3 + 0.5) / 2
		t.Errorf("avgTopicShift: want 0.4, got %v", cl.AvgTopicShift)
	}
}

func approx(got, want float64) bool {
	return math.Abs(got-want) < 1e-6
}
