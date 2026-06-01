package storage

import (
	"testing"
	"time"
)

// TestGetTopicDistribution_LatestSweepIgnoresOlderSweeps pins the synchronic
// topic_distribution invariant: with LatestSweep, only the single newest sweep
// (max window_start) is aggregated. BERTopic topic_ids are unique only within a
// (window_start, language) sweep, so aggregating across sweeps would conflate
// semantically-distinct topics and double-count re-assigned articles. The query
// must therefore never mix an older sweep into the latest-sweep result.
func TestGetTopicDistribution_LatestSweepIgnoresOlderSweeps(t *testing.T) {
	store, ctx := setupTestStore(t)

	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	insert := func(ws time.Time, article string, topicID int32, label string) {
		t.Helper()
		err := store.conn.Exec(ctx, `
			INSERT INTO aer_gold.topic_assignments
				(window_start, window_end, source, article_id, language, topic_id, topic_label, topic_confidence, model_hash, ingestion_version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			ws, ws.AddDate(0, 1, 0), "tagesschau", article, "de", topicID, label, float32(0.9), "h", uint64(1))
		if err != nil {
			t.Fatalf("insert (%s, t%d): %v", article, topicID, err)
		}
	}

	// Older sweep: article a1 → topic 0 "old-politics".
	insert(older, "a1", 0, "old-politics")
	// Newer sweep: a1 re-assigned to topic 0 "new-politics" + a2 → topic 1.
	insert(newer, "a1", 0, "new-politics")
	insert(newer, "a2", 1, "new-economy")

	rows, err := store.GetTopicDistribution(ctx, TopicDistributionParams{
		Sources:        []string{"tagesschau"},
		LatestSweep:    true,
		IncludeOutlier: true,
		Limit:          50,
	})
	if err != nil {
		t.Fatalf("GetTopicDistribution: %v", err)
	}

	// Exactly the newer sweep's two topics — the older sweep is excluded, so no
	// conflation and no double-count.
	if len(rows) != 2 {
		t.Fatalf("want 2 topics from the latest sweep, got %d: %+v", len(rows), rows)
	}
	for _, r := range rows {
		if !r.WindowStart.Equal(newer) {
			t.Errorf("row window_start: want newer sweep %v, got %v", newer, r.WindowStart)
		}
		if r.Label == "old-politics" {
			t.Errorf("latest-sweep result must not include the older sweep's topic")
		}
		if r.ArticleCount != 1 {
			t.Errorf("topic %d: want 1 article, got %d", r.TopicID, r.ArticleCount)
		}
	}
}
