package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrRevisionDiffPending signals that the requested diff has not yet
// been computed by the worker's revision-diff sweep. Distinct from
// "article not found" and from "snapshots identical" so the BFF
// handler returns an honest user-facing status.
var ErrRevisionDiffPending = errors.New("revision diff pending; worker has not yet processed this pair")

// BUG-11: ErrRevisionDiffHeadRequested removed — revisionIndex=0 is
// now a legal pair anchored on (Silver-now → Wayback[0]). The
// handler no longer 404s on revisionIndex=0.

// sentinelIdentityOp is the `op` discriminator the worker writes when
// the two snapshots parse to identical paragraph content (BUG-B). The
// worker serialises it with Python's `json.dumps` default separators
// (`{"op": "identical"}` — note the space after the colon), so the
// sentinel MUST be detected by decoding the op and comparing the `Op`
// field, NOT by matching a hand-written byte-for-byte JSON string: a
// raw-string compare against `{"op":"identical"}` (no space) never
// matched the stored value, leaking the sentinel through as a malformed
// diff op the frontend rendered as a blank row.
const sentinelIdentityOp = "identical"

// ArticleRevisionDiffRow is the ClickHouse-side projection of one
// `aer_gold.article_revisions` row plus the prior row's
// `snapshot_at` (the diff's "before" anchor). Returned by
// `GetArticleRevisionDiff`.
type ArticleRevisionDiffRow struct {
	ArticleID        string    `ch:"article_id"`
	RevisionIndex    uint32    `ch:"revision_index"`
	SnapshotAtBefore time.Time `ch:"snapshot_at_before"`
	SnapshotAtAfter  time.Time `ch:"snapshot_at_after"`
	HeadlineChanged  bool      `ch:"headline_changed"`
	HeadlineBefore   string    `ch:"headline_before"`
	HeadlineAfter    string    `ch:"headline_after"`
	DiffParagraphs   []string  `ch:"diff_paragraphs"`
	Source           string    `ch:"source"`
}

// DiffOp is the decoded shape of one `diff_paragraphs` JSON entry.
// The worker writes ops as JSON-encoded `{op, before?, after?}`
// records (see `services/analysis-worker/internal/article_revisions_diff.py`).
type DiffOp struct {
	Op     string `json:"op"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// DecodeDiffParagraphs parses the worker's JSON-per-entry payload into
// typed ops. Returns an error only when the payload is structurally
// malformed; missing optional fields are silently dropped.
func DecodeDiffParagraphs(raw []string) ([]DiffOp, error) {
	ops := make([]DiffOp, 0, len(raw))
	for i, entry := range raw {
		var op DiffOp
		if err := json.Unmarshal([]byte(entry), &op); err != nil {
			return nil, fmt.Errorf("decode diff op %d: %w", i, err)
		}
		ops = append(ops, op)
	}
	return ops, nil
}

// GetArticleRevisionDiff returns the diff payload for one (articleId,
// revisionIndex) pair.
//
// Two pair kinds (BUG-11):
//   - revisionIndex == 0: chain-head pair. `snapshot_at_before` is
//     zero-time (no predecessor in `article_revisions`); the
//     comparison is "Silver-now → Wayback[0]". The handler labels
//     the pair as "current article → archived snapshot".
//   - revisionIndex > 0: mid-chain pair. The LEFT JOIN resolves the
//     predecessor row to populate `snapshot_at_before`.
//
// Returns:
//   - `ErrSourceNotFound` when the article_id+revision_index is unknown.
//   - `ErrRevisionDiffPending` when the row exists but diff_paragraphs
//     is empty AND no sentinel — sweep hasn't processed yet.
//   - A row whose `DiffParagraphs` contains ONLY the sentinel marker
//     when the snapshots parse to identical paragraph content.
//     `IsIdentical()` checks this case so the handler can render a
//     distinct user message (BUG-10).
func (s *ClickHouseStorage) GetArticleRevisionDiff(
	ctx context.Context,
	articleID string,
	revisionIndex int,
) (*ArticleRevisionDiffRow, error) {
	const query = `
		SELECT
			curr.article_id           AS article_id,
			curr.revision_index       AS revision_index,
			prev.snapshot_at          AS snapshot_at_before,
			curr.snapshot_at          AS snapshot_at_after,
			curr.headline_changed     AS headline_changed,
			curr.headline_before      AS headline_before,
			curr.headline_after       AS headline_after,
			curr.diff_paragraphs      AS diff_paragraphs,
			curr.source               AS source
		FROM aer_gold.article_revisions AS curr FINAL
		LEFT JOIN aer_gold.article_revisions AS prev FINAL
			ON prev.article_id     = curr.article_id
		   AND prev.revision_index = curr.revision_index - 1
		WHERE curr.article_id     = ?
		  AND curr.revision_index = ?
		LIMIT 1
	`
	var rows []ArticleRevisionDiffRow
	if err := s.conn.Select(ctx, &rows, query, articleID, revisionIndex); err != nil {
		return nil, fmt.Errorf("revision diff query: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrSourceNotFound
	}
	r := &rows[0]
	if len(r.DiffParagraphs) == 0 && !r.HeadlineChanged {
		// True pending — sweep has not yet written anything for this pair.
		return nil, ErrRevisionDiffPending
	}
	return r, nil
}

// IsIdentical returns true when the row's `DiffParagraphs` contains
// ONLY the BUG-B sentinel op — "snapshots parsed to identical
// content". Used by the BFF handler to render a distinct user message.
// Decodes the single op and compares its `Op` field so the check is
// independent of the writer's JSON whitespace (see `sentinelIdentityOp`).
func (r *ArticleRevisionDiffRow) IsIdentical() bool {
	if len(r.DiffParagraphs) != 1 {
		return false
	}
	var op DiffOp
	if err := json.Unmarshal([]byte(r.DiffParagraphs[0]), &op); err != nil {
		return false
	}
	return op.Op == sentinelIdentityOp
}

// ----------------------------------------------------------------------
// /revisions/articles — paginated drill-down list.
// ----------------------------------------------------------------------

// RevisionArticleRow is the joined projection of `aer_gold.metrics`
// (for `language`, `word_count`) + `aer_gold.article_revisions` (for
// `chainLength`, `hasHeadlineChange`, `latestRevisionAt`). Used by
// `GetRevisionsArticles` to back the Workbench drill-down list.
type RevisionArticleRow struct {
	ArticleID            string    `ch:"article_id"`
	Source               string    `ch:"source"`
	Timestamp            time.Time `ch:"timestamp"`
	Language             string    `ch:"language"`
	WordCount            uint64    `ch:"word_count"`
	ChainLength          uint32    `ch:"chain_length"`
	EditorialChangeCount uint32    `ch:"editorial_change_count"`
	HasHeadlineChange    bool      `ch:"has_headline_change"`
	LatestRevisionAt     time.Time `ch:"latest_revision_at"`
}

// RevisionsArticlesFilter captures the query parameters.
type RevisionsArticlesFilter struct {
	Sources           []string
	Start             time.Time
	End               time.Time
	HasHeadlineChange bool
	MinChainLength    int
	Limit             int
	Offset            int
}

// GetRevisionsArticles returns articles with ≥1 revision in the
// window, sorted newest-first. The query aggregates
// `aer_gold.article_revisions` by `article_id`, then joins
// `aer_gold.metrics` once to attach the article's metadata
// (language, word_count, published_at). Sources without revisions
// in the window do not appear.
//
// An empty `Sources` slice returns no rows (consistent with the rest
// of the BFF surface — every revision query is scoped).
func (s *ClickHouseStorage) GetRevisionsArticles(
	ctx context.Context,
	filter RevisionsArticlesFilter,
) ([]RevisionArticleRow, error) {
	if len(filter.Sources) == 0 {
		return nil, nil
	}
	if filter.MinChainLength < 1 {
		filter.MinChainLength = 1
	}
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	sourcePlaceholders := make([]string, len(filter.Sources))
	args := []any{filter.Start, filter.End}
	for i, src := range filter.Sources {
		sourcePlaceholders[i] = "?"
		args = append(args, src)
	}

	// Post-aggregation filters (HAVING, after the per-article reduction).
	//
	// Phase 133 — the revisions / "edited articles" drill-down is EDITS-ONLY:
	// an article belongs here only if it has ≥ 1 CONFIRMED editorial change
	// (`editorial_change_count >= 1`). Articles whose Wayback chain is purely
	// identical re-archivals (captures with no body change) are NOT revisions
	// and must not appear — this keeps the list consistent with the
	// revision_activity / revision_timeline counts, which also count only
	// editorial edits. (A pending-but-not-yet-diffed edit surfaces once the
	// sweep confirms it, exactly as it does in the activity counts.)
	havingClauses := []string{"editorial_change_count >= 1"}
	if filter.HasHeadlineChange {
		havingClauses = append(havingClauses, "any_headline_change = true")
	}
	if filter.MinChainLength > 1 {
		havingClauses = append(havingClauses, fmt.Sprintf("chain_length >= %d", filter.MinChainLength))
	}
	havingSQL := ""
	if len(havingClauses) > 0 {
		havingSQL = "HAVING " + joinAndClauses(havingClauses)
	}

	args = append(args, filter.Limit, filter.Offset)

	// Two-step query: aggregate `aer_gold.article_revisions` to one
	// row per (article_id, source) carrying chain_length +
	// any_headline_change + latest_revision_at; join against
	// `aer_silver.documents` (the canonical per-article projection —
	// Phase 103b) for timestamp, language, word_count. Sources
	// without revisions in the window simply don't appear in the
	// outer SELECT. ``LEFT JOIN`` so an article whose revisions row
	// exists but whose Silver projection was evicted by TTL still
	// shows up (with empty language / 0 word_count).
	query := fmt.Sprintf(`
		WITH
			revisions_window AS (
				SELECT
					article_id,
					source,
					toUInt32(count())              AS chain_length,
					toUInt32(countIf(
						(length(diff_paragraphs) > 0
						 AND NOT arrayExists(x -> JSONExtractString(x, 'op') = 'identical', diff_paragraphs))
						OR headline_changed
					))                             AS editorial_change_count,
					countIf(headline_changed) > 0  AS any_headline_change,
					max(snapshot_at)               AS latest_revision_at
				FROM aer_gold.article_revisions FINAL
				WHERE snapshot_at >= ?
				  AND snapshot_at <  ?
				  AND source IN (%s)
				GROUP BY article_id, source
				%s
			)
		SELECT
			r.article_id            AS article_id,
			r.source                AS source,
			d.timestamp             AS timestamp,
			d.language              AS language,
			toUInt64(d.word_count)  AS word_count,
			r.chain_length          AS chain_length,
			r.editorial_change_count AS editorial_change_count,
			r.any_headline_change   AS has_headline_change,
			r.latest_revision_at    AS latest_revision_at
		FROM revisions_window AS r
		LEFT JOIN aer_silver.documents AS d FINAL
			ON d.article_id = r.article_id
		ORDER BY latest_revision_at DESC, r.article_id
		LIMIT ? OFFSET ?
	`,
		joinPlaceholders(sourcePlaceholders),
		havingSQL,
	)

	var rows []RevisionArticleRow
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("revisions articles query: %w", err)
	}
	return rows, nil
}

func joinAndClauses(clauses []string) string {
	out := ""
	for i, c := range clauses {
		if i > 0 {
			out += " AND "
		}
		out += c
	}
	return out
}

func joinPlaceholders(placeholders []string) string {
	out := ""
	for i, p := range placeholders {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
