package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ArticleQueryFilter narrows the article-listing query.
type ArticleQueryFilter struct {
	Start         *time.Time
	End           *time.Time
	Language      *string
	EntityMatch   *string
	SentimentBand *string // "negative" | "neutral" | "positive"
	Limit         int
	Offset        int
	// Phase 122d.1 — opt in to per-row revision fields (chainLength,
	// hasHeadlineChange, latestRevisionAt). Server-side cost is one
	// extra `aer_gold.article_revisions` lookup per page; rows with
	// no revisions get the zero-values and the dashboard hides the
	// badges. Default false so existing callers receive the legacy shape.
	IncludeRevisions bool
}

// ArticleAggRow is the per-article row materialised by the article-listing
// query. Sourced from `aer_gold.metrics` keyed on article_id, with
// language and sentiment joined in via subqueries on the rank-1 language
// detection and the `sentiment_score` metric respectively.
type ArticleAggRow struct {
	ArticleID      string
	Source         string
	Timestamp      time.Time
	Language       string
	WordCount      int64
	SentimentScore float64
	HasLanguage    bool
	HasWordCount   bool
	HasSentiment   bool
	// Phase 122d.2 — timestamp provenance. 'fetch_at_fallback' means the
	// article's timestamp is the crawler fetch time, not a real publication
	// date (Temporal-Provenance-Absence NS-class). Empty = legacy/non-web row
	// that predates this provenance dimension; other values are real dates.
	TimestampSource string
	// Phase 122d.1 — revision fields. Populated only when
	// `ArticleQueryFilter.IncludeRevisions=true` AND the article has
	// at least one row in `aer_gold.article_revisions`.
	// ChainLength is the raw Wayback CAPTURE count (coverage detail, not a
	// revision count). EditorialChangeCount is the number of CONFIRMED
	// editorial edits (Phase 133) — the only count surfaced as "revisions".
	ChainLength          uint32
	EditorialChangeCount uint32
	HasHeadlineChange    bool
	HasRevisions         bool
	LatestRevisionAt     time.Time
}

// GetSourceArticles returns paginated articles for a source. Filters
// (language, entity match, sentiment band) translate to SQL WHERE
// clauses against the relevant Gold tables. Pagination is offset-based
// at the storage layer; the handler wraps it in an opaque cursor.
func (s *ClickHouseStorage) GetSourceArticles(ctx context.Context, sourceName string, f ArticleQueryFilter) ([]ArticleAggRow, error) {
	conds := []string{"source = ?"}
	args := []any{sourceName}

	if f.Start != nil {
		conds = append(conds, "timestamp >= ?")
		args = append(args, *f.Start)
	}
	if f.End != nil {
		conds = append(conds, "timestamp <= ?")
		args = append(args, *f.End)
	}
	if f.EntityMatch != nil && *f.EntityMatch != "" {
		conds = append(conds, `article_id IN (
			SELECT article_id FROM aer_gold.entities
			 WHERE source = ? AND positionCaseInsensitive(entity_text, ?) > 0
		)`)
		args = append(args, sourceName, *f.EntityMatch)
	}
	if f.Language != nil && *f.Language != "" {
		conds = append(conds, `article_id IN (
			SELECT article_id FROM aer_gold.language_detections
			 WHERE source = ? AND rank = 1 AND detected_language = ?
		)`)
		args = append(args, sourceName, *f.Language)
	}
	if f.SentimentBand != nil {
		band := *f.SentimentBand
		var sentimentCond string
		switch band {
		case "negative":
			sentimentCond = "value <= -0.05"
		case "positive":
			sentimentCond = "value >= 0.05"
		case "neutral":
			sentimentCond = "value > -0.05 AND value < 0.05"
		default:
			return nil, fmt.Errorf("invalid sentimentBand: %q", band)
		}
		conds = append(conds, fmt.Sprintf(`article_id IN (
			SELECT article_id FROM aer_gold.metrics
			 WHERE source = ? AND metric_name = 'sentiment_score' AND %s
		)`, sentimentCond))
		args = append(args, sourceName)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := max(f.Offset, 0)

	// One row per article: pick the metric row with metric_name='word_count'
	// when present (every processed document writes one), falling back to
	// any other metric row. anyLast() is used for projection columns to
	// avoid GROUP BY on every column. The ClickHouse Go driver does not
	// parameterise LIMIT/OFFSET, so we interpolate the validated integers.
	query := fmt.Sprintf(`
		SELECT
			article_id        AS ArticleID,
			anyLast(source)   AS Source,
			min(timestamp)    AS Timestamp,
			anyIf(value, metric_name = 'word_count')      AS WordCount,
			anyIf(value, metric_name = 'sentiment_score') AS SentimentScore,
			countIf(metric_name = 'word_count')           AS HasWordCount,
			countIf(metric_name = 'sentiment_score')      AS HasSentiment,
			anyLast(timestamp_source)                     AS TimestampSource
		  FROM aer_gold.metrics
		 WHERE %s
		   AND article_id IS NOT NULL
		 GROUP BY article_id
		 ORDER BY Timestamp DESC
		 LIMIT %d OFFSET %d
	`, strings.Join(conds, " AND "), limit, offset)

	type row struct {
		ArticleID       string    `ch:"ArticleID"`
		Source          string    `ch:"Source"`
		Timestamp       time.Time `ch:"Timestamp"`
		WordCount       float64   `ch:"WordCount"`
		SentimentScore  float64   `ch:"SentimentScore"`
		HasWordCount    uint64    `ch:"HasWordCount"`
		HasSentiment    uint64    `ch:"HasSentiment"`
		TimestampSource string    `ch:"TimestampSource"`
	}
	var raw []row
	if err := s.conn.Select(ctx, &raw, query, args...); err != nil {
		slog.Error("Failed to query source articles", "error", err)
		return nil, err
	}

	if len(raw) == 0 {
		return nil, nil
	}

	// Resolve top language detection per article in a single follow-up
	// query so we don't run a correlated subquery per row.
	articleIDs := make([]string, len(raw))
	for i, r := range raw {
		articleIDs[i] = r.ArticleID
	}
	languages, err := s.lookupTopLanguages(ctx, sourceName, articleIDs)
	if err != nil {
		// Language is decorative — log and continue.
		slog.Warn("language lookup failed; continuing without language", "error", err)
		languages = nil
	}

	// Phase 122d.1 — optional revision lookup. Single follow-up query
	// keyed on the same articleIDs slice; no correlated subquery per row.
	var revisions map[string]revisionAggRow
	if f.IncludeRevisions {
		revisions, err = s.lookupArticleRevisions(ctx, articleIDs)
		if err != nil {
			// Revision fields are decorative for the list view — log
			// and continue without them.
			slog.Warn("article revisions lookup failed; continuing without revision badges", "error", err)
			revisions = nil
		}
	}

	out := make([]ArticleAggRow, 0, len(raw))
	for _, r := range raw {
		row := ArticleAggRow{
			ArticleID:       r.ArticleID,
			Source:          r.Source,
			Timestamp:       r.Timestamp,
			WordCount:       int64(r.WordCount),
			SentimentScore:  r.SentimentScore,
			HasWordCount:    r.HasWordCount > 0,
			HasSentiment:    r.HasSentiment > 0,
			TimestampSource: r.TimestampSource,
		}
		if lang, ok := languages[r.ArticleID]; ok {
			row.Language = lang
			row.HasLanguage = true
		}
		if rev, ok := revisions[r.ArticleID]; ok {
			row.ChainLength = rev.ChainLength
			row.EditorialChangeCount = rev.EditorialChangeCount
			row.HasHeadlineChange = rev.HasHeadlineChange
			row.LatestRevisionAt = rev.LatestRevisionAt
			row.HasRevisions = true
		}
		out = append(out, row)
	}
	return out, nil
}

// revisionAggRow is the per-article aggregate of
// `aer_gold.article_revisions` used to decorate the article-list view.
type revisionAggRow struct {
	ArticleID            string    `ch:"article_id"`
	ChainLength          uint32    `ch:"chain_length"`
	EditorialChangeCount uint32    `ch:"editorial_change_count"`
	HasHeadlineChange    bool      `ch:"has_headline_change"`
	LatestRevisionAt     time.Time `ch:"latest_revision_at"`
}

func (s *ClickHouseStorage) lookupArticleRevisions(ctx context.Context, articleIDs []string) (map[string]revisionAggRow, error) {
	if len(articleIDs) == 0 {
		return map[string]revisionAggRow{}, nil
	}
	const q = `
		SELECT
			article_id,
			toUInt32(count())              AS chain_length,
			toUInt32(countIf(
				(length(diff_paragraphs) > 0
				 AND NOT arrayExists(x -> JSONExtractString(x, 'op') = 'identical', diff_paragraphs))
				OR headline_changed
			))                             AS editorial_change_count,
			countIf(headline_changed) > 0  AS has_headline_change,
			max(snapshot_at)               AS latest_revision_at
		FROM aer_gold.article_revisions FINAL
		WHERE article_id IN ?
		GROUP BY article_id
	`
	var rows []revisionAggRow
	if err := s.conn.Select(ctx, &rows, q, articleIDs); err != nil {
		return nil, err
	}
	out := make(map[string]revisionAggRow, len(rows))
	for _, r := range rows {
		out[r.ArticleID] = r
	}
	return out, nil
}

func (s *ClickHouseStorage) lookupTopLanguages(ctx context.Context, sourceName string, articleIDs []string) (map[string]string, error) {
	if len(articleIDs) == 0 {
		return map[string]string{}, nil
	}
	const q = `
		SELECT article_id, detected_language
		  FROM aer_gold.language_detections
		 WHERE source = ? AND rank = 1 AND article_id IN ?
	`
	type row struct {
		ArticleID string `ch:"article_id"`
		Lang      string `ch:"detected_language"`
	}
	var rows []row
	if err := s.conn.Select(ctx, &rows, q, sourceName, articleIDs); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ArticleID] = r.Lang
	}
	return out, nil
}

// CountAggregationGroup returns the number of distinct articles that
// share the (source, metric_name) aggregation key in the article's
// time bucket. Used by the article-detail k-anonymity gate.
//
// The bucket is the article's UTC date — coarse enough that even a
// quiet source with a single article per day still aggregates cleanly
// against neighbouring days, but tight enough that a long-tail source
// archive does not dilute the count to meaninglessness.
func (s *ClickHouseStorage) CountAggregationGroup(ctx context.Context, sourceName, metricName string, articleTimestamp time.Time) (int, error) {
	const q = `
		SELECT count(DISTINCT article_id) AS c
		  FROM aer_gold.metrics
		 WHERE source = ?
		   AND metric_name = ?
		   AND toDate(timestamp) = toDate(?)
	`
	type row struct {
		C uint64 `ch:"c"`
	}
	var rows []row
	if err := s.conn.Select(ctx, &rows, q, sourceName, metricName, articleTimestamp); err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return int(rows[0].C), nil //nolint:gosec // bounded by source/day rowcount
}

// GetArticleProvenance returns the per-extractor provenance values
// recorded for the article (sourced from the metrics rows alongside
// the per-extractor versions written by the worker). For Phase 101
// the worker stores extractor provenance in the SilverEnvelope, so
// this BFF-side variant is currently a stub returning an empty map —
// retained as a hook for richer provenance once the metrics schema
// carries extractor versions per-row.
func (s *ClickHouseStorage) GetArticleProvenance(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{}, nil
}
