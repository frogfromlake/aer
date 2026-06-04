package storage

import (
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
