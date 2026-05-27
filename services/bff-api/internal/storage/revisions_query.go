package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// RevisionActivityResolution selects the time-bucket grain for the
// `revision_activity` aggregation. `snapshot` collapses the whole
// window to one row per source (Aleph cell); the others bucket on a
// calendar grain (Episteme cell).
type RevisionActivityResolution string

const (
	RevisionResolutionSnapshot RevisionActivityResolution = "snapshot"
	RevisionResolutionDaily    RevisionActivityResolution = "daily"
	RevisionResolutionWeekly   RevisionActivityResolution = "weekly"
	RevisionResolutionMonthly  RevisionActivityResolution = "monthly"
)

// RevisionActivityCell is one (source, bucket) aggregation row over
// `aer_gold.article_revisions` (Phase 122d.0 / ADR-032).
type RevisionActivityCell struct {
	Source              string    `ch:"source"`
	Bucket              time.Time `ch:"bucket"`
	Revisions           uint64    `ch:"revisions"`
	ArticlesAffected    uint64    `ch:"articles_affected"`
	CdxSnapshotCount    uint64    `ch:"cdx_snapshot_count"`
	RepublicationCount  uint64    `ch:"republication_count"`
	UnknownTriggerCount uint64    `ch:"unknown_trigger_count"`
}

// ArticleRevisionRow is one detected revision for the per-article
// chain returned by `GET /articles/{id}/revisions`.
type ArticleRevisionRow struct {
	SnapshotAt         time.Time `ch:"snapshot_at"`
	ContentHash        string    `ch:"content_hash"`
	PrevContentHash    string    `ch:"prev_content_hash"`
	RevisionIndex      uint32    `ch:"revision_index"`
	TimeSincePrevHours float64   `ch:"time_since_prev_hours"`
	Trigger            string    `ch:"revision_trigger"`
}

// RevisionActivityQuerier abstracts the storage-side queries for the
// Silent-Edit Observability endpoints. Implemented by ClickHouseStorage.
type RevisionActivityQuerier interface {
	GetRevisionActivity(ctx context.Context, sources []string, start, end time.Time, resolution RevisionActivityResolution) ([]RevisionActivityCell, error)
	GetArticleRevisions(ctx context.Context, articleID string) ([]ArticleRevisionRow, error)
}

// GetRevisionActivity aggregates `aer_gold.article_revisions` by
// (source, bucket) for the requested scope and window.
//
// An empty `sources` slice yields no rows — every revision aggregation
// is scoped, never global, so the BFF cannot accidentally return the
// entire corpus.
//
// The CDX cache plus the `ReplacingMergeTree(ingestion_version)` engine
// guarantee idempotency: re-running the worker on the same Bronze
// payload converges on identical rows. No `FINAL` is needed at read
// time because the (source, bucket) grouping collapses duplicates
// naturally.
func (s *ClickHouseStorage) GetRevisionActivity(
	ctx context.Context,
	sources []string,
	start, end time.Time,
	resolution RevisionActivityResolution,
) ([]RevisionActivityCell, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	bucketExpr, err := revisionBucketExpr(resolution, start)
	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(sources))
	args := []any{start, end}
	for i, src := range sources {
		placeholders[i] = "?"
		args = append(args, src)
	}

	query := fmt.Sprintf(`
		SELECT
			source,
			%s AS bucket,
			toUInt64(count())                                                                    AS revisions,
			toUInt64(uniqExact(article_id))                                                      AS articles_affected,
			toUInt64(countIf(revision_trigger = 'cdx_snapshot'))                                 AS cdx_snapshot_count,
			toUInt64(countIf(revision_trigger = 'republication_trigger'))                        AS republication_count,
			toUInt64(countIf(revision_trigger NOT IN ('cdx_snapshot', 'republication_trigger'))) AS unknown_trigger_count
		FROM aer_gold.article_revisions
		WHERE snapshot_at >= ?
		  AND snapshot_at <  ?
		  AND source IN (%s)
		GROUP BY source, bucket
		ORDER BY bucket, source
	`, bucketExpr, strings.Join(placeholders, ", "))

	var rows []RevisionActivityCell
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("revision activity query: %w", err)
	}
	return rows, nil
}

// GetArticleRevisions returns the ordered revision chain for one
// article. ReplacingMergeTree merges may not have settled across the
// (article_id, snapshot_at, content_hash) primary tuple, so the query
// applies `FINAL` to collapse any straggling duplicates — the row
// count per article is bounded (tens, not thousands) so the cost is
// negligible compared to the aggregate path.
func (s *ClickHouseStorage) GetArticleRevisions(
	ctx context.Context,
	articleID string,
) ([]ArticleRevisionRow, error) {
	if articleID == "" {
		return nil, nil
	}
	const query = `
		SELECT
			snapshot_at,
			content_hash,
			prev_content_hash,
			revision_index,
			time_since_prev_hours,
			revision_trigger
		FROM aer_gold.article_revisions FINAL
		WHERE article_id = ?
		ORDER BY snapshot_at, content_hash
	`
	var rows []ArticleRevisionRow
	if err := s.conn.Select(ctx, &rows, query, articleID); err != nil {
		return nil, fmt.Errorf("article revisions query: %w", err)
	}
	return rows, nil
}

// revisionBucketExpr returns the ClickHouse SQL expression that maps
// `snapshot_at` to the requested aggregation bucket.
//
// For the synchronic `snapshot` resolution we project every row to a
// constant bucket pinned at the analysis-window start — the dashboard
// then renders one bar per source for the whole window without a per-
// bucket timeline.
func revisionBucketExpr(resolution RevisionActivityResolution, start time.Time) (string, error) {
	switch resolution {
	case "", RevisionResolutionSnapshot:
		return fmt.Sprintf("toDateTime('%s')", start.UTC().Format("2006-01-02 15:04:05")), nil
	case RevisionResolutionDaily:
		return "toStartOfDay(snapshot_at)", nil
	case RevisionResolutionWeekly:
		// Phase 122d aligns weekly buckets to ISO weeks (Monday start),
		// matching the BFF's existing weekly resolution convention.
		return "toStartOfWeek(snapshot_at, 1)", nil
	case RevisionResolutionMonthly:
		return "toStartOfMonth(snapshot_at)", nil
	default:
		return "", fmt.Errorf("unknown revision activity resolution: %q", resolution)
	}
}

// Compile-time check that ClickHouseStorage implements the interface.
var _ RevisionActivityQuerier = (*ClickHouseStorage)(nil)
