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
	// ArchiveURL is the Internet Archive playback URL for CDX snapshots
	// (empty for republication-trigger rows). Surfaced so the L5 reader's
	// "view snapshot" link resolves (Phase 133).
	ArchiveURL string `ch:"archive_url"`
	// DiffStatus is the editorial status of the diff for the pair ending
	// at this revision, derived from `diff_paragraphs`: `pending` (no diff
	// computed yet), `identical` (the sweep wrote the identical sentinel —
	// a re-archival with no editorial change), or `changed`. Lets the L5
	// reader walk the slider over editorial versions only (Phase 133).
	DiffStatus string `ch:"diff_status"`
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
// `FINAL` is REQUIRED. `aer_gold.article_revisions` is a
// `ReplacingMergeTree(ingestion_version)`, and the ADR-036 enrichment
// re-attempt loop re-writes an article's revision rows with a fresh
// `ingestion_version` whenever it heals an incomplete Wayback lookup.
// Between that INSERT and the next background merge, the old and new
// versions of a `(article_id, snapshot_at, content_hash)` tuple coexist
// as physical rows. `count()` counts physical rows, and grouping by
// (source, bucket) does NOT collapse those PK duplicates — so without
// `FINAL` the revision total transiently over-counts and then drops as
// merges settle (observed as revision counts that rise then fall). The
// table is small (hundreds–thousands of rows per scope), so the
// merge-on-read cost is negligible, matching `GetArticleRevisions` and
// the revision-diff query which already apply `FINAL`.
func (s *ClickHouseStorage) GetRevisionActivity(
	ctx context.Context,
	sources []string,
	start, end time.Time,
	resolution RevisionActivityResolution,
) ([]RevisionActivityCell, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	bucketExpr, err := revisionBucketExpr(resolution, start, end)
	if err != nil {
		return nil, err
	}

	placeholders := make([]string, len(sources))
	args := []any{start, end}
	for i, src := range sources {
		placeholders[i] = "?"
		args = append(args, src)
	}

	// Phase 133 — count EDITORIAL CHANGES, not raw captures. A revision is
	// a content-changing transition (`is_edit`): the pair has a computed
	// diff that is NOT the identical-sentinel. Identical re-archivals (the
	// Internet Archive re-capturing unchanged content) and not-yet-diffed
	// `pending` pairs are excluded — they are observation artefacts, not
	// edits by the source. `article_revisions` is the source of truth; the
	// diff classification lives in `diff_paragraphs`.
	query := fmt.Sprintf(`
		SELECT
			source,
			%s AS bucket,
			toUInt64(countIf(is_edit))                                                                    AS revisions,
			toUInt64(uniqExactIf(article_id, is_edit))                                                    AS articles_affected,
			toUInt64(countIf(is_edit AND revision_trigger = 'cdx_snapshot'))                              AS cdx_snapshot_count,
			toUInt64(countIf(is_edit AND revision_trigger = 'republication_trigger'))                     AS republication_count,
			toUInt64(countIf(is_edit AND revision_trigger NOT IN ('cdx_snapshot', 'republication_trigger'))) AS unknown_trigger_count
		FROM (
			SELECT
				source,
				snapshot_at,
				article_id,
				revision_trigger,
				-- An editorial edit = a real paragraph change OR a headline
				-- change. A headline-only change has empty diff_paragraphs but
				-- headline_changed=true, so it must be OR'd in (Phase 133).
				((length(diff_paragraphs) > 0
				  AND NOT arrayExists(x -> JSONExtractString(x, 'op') = 'identical', diff_paragraphs))
				 OR headline_changed) AS is_edit
			FROM aer_gold.article_revisions FINAL
			WHERE snapshot_at >= ?
			  AND snapshot_at <  ?
			  AND source IN (%s)
		)
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
			revision_trigger,
			archive_url,
			multiIf(
				arrayExists(x -> JSONExtractString(x, 'op') = 'identical', diff_paragraphs), 'identical',
				length(diff_paragraphs) > 0 OR headline_changed, 'changed',
				'pending'
			) AS diff_status
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
// For the synchronic `snapshot` resolution we project every row to a constant
// bucket pinned at the analysis-window END (the "as-of" instant) — the
// dashboard then renders one bar per source for the whole window without a
// per-bucket timeline. Pinning to the end rather than the start keeps the
// bucket label meaningful under an unbounded window, whose lower bound is the
// epoch sentinel (a start-pinned bucket would surface a nonsensical 1970 date).
func revisionBucketExpr(resolution RevisionActivityResolution, start, end time.Time) (string, error) {
	switch resolution {
	case "", RevisionResolutionSnapshot:
		return fmt.Sprintf("toDateTime('%s')", end.UTC().Format("2006-01-02 15:04:05")), nil
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
