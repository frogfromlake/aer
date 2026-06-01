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
func TestGetRevisionActivity_CollapsesReattemptVersions(t *testing.T) {
	store, ctx := setupTestStore(t)

	snap := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)

	insert := func(snapshotAt time.Time, hash string, version uint64) {
		t.Helper()
		err := store.conn.Exec(ctx, `
			INSERT INTO aer_gold.article_revisions
				(article_id, source, snapshot_at, content_hash, revision_trigger, ingestion_version)
			VALUES (?, ?, ?, ?, ?, ?)`,
			"art-1", "elysee", snapshotAt, hash, "cdx_snapshot", version)
		if err != nil {
			t.Fatalf("insert revision (%s, v%d): %v", hash, version, err)
		}
	}

	// Same revision tuple written twice — exactly what the re-attempt loop does
	// when it re-heals "art-1". FINAL must collapse these to ONE.
	insert(snap, "deadbeef", 1000)
	insert(snap, "deadbeef", 2000)
	// A second, genuinely distinct revision of the same article (single
	// version). The true revision count for the article is therefore 2.
	insert(snap.Add(time.Hour), "cafef00d", 1500)

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
	// 2 distinct revisions — the double-versioned tuple counted once, plus the
	// distinct second tuple. Without FINAL this would be 3.
	if cells[0].Revisions != 2 {
		t.Errorf("revisions: want 2 (duplicate ingestion_versions collapsed), got %d", cells[0].Revisions)
	}
	if cells[0].ArticlesAffected != 1 {
		t.Errorf("articles_affected: want 1, got %d", cells[0].ArticlesAffected)
	}
	if cells[0].CdxSnapshotCount != 2 {
		t.Errorf("cdx_snapshot_count: want 2, got %d", cells[0].CdxSnapshotCount)
	}
}
