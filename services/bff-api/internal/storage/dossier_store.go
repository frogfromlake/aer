package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

// DossierSourceRow is one source card on the Probe Dossier.
type DossierSourceRow struct {
	Name                  string
	Type                  string
	URL                   sql.NullString
	DocumentationURL      sql.NullString
	ArticlesTotal         int64
	ArticlesInWindow      int64
	PublicationFreqPerDay sql.NullFloat64
	PrimaryFunction       sql.NullString
	SecondaryFunction     sql.NullString
	EmicDesignation       sql.NullString
	EmicContext           sql.NullString
	SilverEligible        bool
	SilverReviewDate      sql.NullTime
}

// ProbeDossierData is the composite per-probe payload built by the BFF.
type ProbeDossierData struct {
	WindowStart *time.Time
	WindowEnd   *time.Time
	Sources     []DossierSourceRow
}

// DossierStore reads Probe Dossier data. Source metadata (name, type,
// classification, Silver-eligibility) comes from PostgreSQL — the BFF
// owns only SELECT on the relevant tables (see infra/postgres/init-roles.sh).
// Per-source article counts and (article_id) → (bronze_object_key, source)
// resolution come from ClickHouse aer_silver.documents (ADR-022): the
// analytical layer is the source of truth on the analytical horizon, so
// the dossier no longer drifts when the operational 90-day Postgres
// retention prunes rows whose Silver/Gold record is still live.
type DossierStore struct {
	db   *sql.DB
	chCn clickhouse.Conn
}

// NewDossierStore wires a DossierStore over the existing read-only Postgres
// pool. ch may be nil in tests that do not exercise article-resolution or
// Gold-backed counts; the affected code paths fall back to the legacy
// Postgres queries in that case.
func NewDossierStore(db *sql.DB, ch clickhouse.Conn) *DossierStore {
	return &DossierStore{db: db, chCn: ch}
}

// FetchSources returns the per-source dossier rows for the given source
// names in name order. windowStart/windowEnd are optional — when nil the
// `articlesInWindow` count equals the total processed-document count and
// `publicationFreqPerDay` is computed across all-time.
//
// Source metadata (name, type, URL, classification, Silver-eligibility)
// comes from PostgreSQL. Article counts and publication frequency come
// from ClickHouse aer_silver.documents (ADR-022): the analytical layer
// holds the same one-row-per-document projection on the 365-day horizon
// matching Silver/Gold, so dossier counts stay coherent with view-mode
// queries even when the operational 90-day Postgres documents/jobs rows
// have been pruned. Sources whose ClickHouse projection is empty fall
// through with zero counts and a NULL frequency — the same shape the
// previous Postgres-only query produced for an unprocessed source.
func (s *DossierStore) FetchSources(ctx context.Context, sourceNames []string, windowStart, windowEnd *time.Time) ([]DossierSourceRow, error) {
	if len(sourceNames) == 0 {
		return nil, nil
	}

	counts, err := s.fetchSourceCounts(ctx, sourceNames, windowStart, windowEnd)
	if err != nil {
		return nil, err
	}

	// Build a $1, $2, ... placeholder list for the IN clause.
	placeholders := make([]string, len(sourceNames))
	args := make([]any, 0, len(sourceNames))
	for i, name := range sourceNames {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, name)
	}

	query := fmt.Sprintf(`
		WITH latest_classification AS (
			SELECT DISTINCT ON (source_id)
			       source_id,
			       primary_function,
			       secondary_function,
			       emic_designation,
			       emic_context
			  FROM source_classifications
			 ORDER BY source_id, classification_date DESC
		)
		SELECT s.name,
		       s.type,
		       s.url,
		       s.documentation_url,
		       lc.primary_function,
		       lc.secondary_function,
		       lc.emic_designation,
		       lc.emic_context,
		       s.silver_eligible,
		       s.silver_review_date
		  FROM sources s
		  LEFT JOIN latest_classification lc ON lc.source_id = s.id
		 WHERE s.name IN (%s)
		 ORDER BY s.name
	`, strings.Join(placeholders, ", "))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("dossier query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []DossierSourceRow
	for rows.Next() {
		var r DossierSourceRow
		if err := rows.Scan(
			&r.Name, &r.Type, &r.URL, &r.DocumentationURL,
			&r.PrimaryFunction, &r.SecondaryFunction,
			&r.EmicDesignation, &r.EmicContext,
			&r.SilverEligible, &r.SilverReviewDate,
		); err != nil {
			return nil, fmt.Errorf("scan dossier row: %w", err)
		}
		if c, ok := counts[r.Name]; ok {
			r.ArticlesTotal = c.total
			r.ArticlesInWindow = c.inWindow
			if c.freqPerDay > 0 {
				r.PublicationFreqPerDay = sql.NullFloat64{Float64: c.freqPerDay, Valid: true}
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dossier rows: %w", err)
	}
	return out, nil
}

// sourceCount holds the ClickHouse-derived counts and publication frequency
// for one source, indexed by source name in fetchSourceCounts.
type sourceCount struct {
	total      int64
	inWindow   int64
	freqPerDay float64
}

// fetchSourceCounts returns the per-source totals, in-window totals, and
// publication-frequency-per-day computed from aer_silver.documents.
// Returns an empty map (not an error) when the ClickHouse connection is
// nil — tests that don't exercise counts can inject a nil ch and still
// fetch dossier metadata.
func (s *DossierStore) fetchSourceCounts(
	ctx context.Context,
	sourceNames []string,
	windowStart, windowEnd *time.Time,
) (map[string]sourceCount, error) {
	if s.chCn == nil || len(sourceNames) == 0 {
		return map[string]sourceCount{}, nil
	}

	// Window args reuse the Gold convention: when both are nil, in-window
	// equals total. ClickHouse-go does not support NULL placeholders for
	// DateTime, so we branch the predicate textually instead of binding nil.
	// Placeholder order in the rendered query is: windowStart, windowEnd
	// (in the SELECT projection), then sourceNames (in the WHERE clause).
	var windowFilter string
	var args []any
	if windowStart != nil && windowEnd != nil {
		windowFilter = "AND timestamp >= ? AND timestamp <= ?"
		args = append(args, *windowStart, *windowEnd)
	}
	args = append(args, sourceNames)

	// FINAL collapses ReplacingMergeTree duplicates from NATS redelivery so
	// the count is per distinct (timestamp, source, article_id) tuple,
	// matching the dedupe Gold/Silver readers see at query time.
	query := fmt.Sprintf(`
		SELECT
			source                                            AS source,
			count(DISTINCT article_id)                        AS total,
			countDistinctIf(article_id, 1=1 %s)               AS in_window,
			toUnixTimestamp(min(timestamp))                   AS first_ts,
			toUnixTimestamp(max(timestamp))                   AS last_ts
		  FROM aer_silver.documents FINAL
		 WHERE source IN ?
		 GROUP BY source
	`, windowFilter)

	type row struct {
		Source   string `ch:"source"`
		Total    uint64 `ch:"total"`
		InWindow uint64 `ch:"in_window"`
		FirstTS  uint32 `ch:"first_ts"`
		LastTS   uint32 `ch:"last_ts"`
	}
	var rows []row
	if err := s.chCn.Select(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("source counts query: %w", err)
	}

	out := make(map[string]sourceCount, len(rows))
	for _, r := range rows {
		c := sourceCount{
			total:    int64(r.Total),    //nolint:gosec // bounded by source rowcount
			inWindow: int64(r.InWindow), //nolint:gosec // bounded by source rowcount
		}
		// Publication-frequency-per-day mirrors the previous Postgres
		// computation: total over the active span in days, with a 1-day
		// floor so a same-day burst doesn't divide by zero.
		if r.Total > 0 && r.LastTS > r.FirstTS {
			spanDays := float64(int64(r.LastTS)-int64(r.FirstTS)) / 86400.0
			if spanDays < 1.0 {
				spanDays = 1.0
			}
			c.freqPerDay = float64(r.Total) / spanDays
		}
		out[r.Source] = c
	}
	return out, nil
}

// ArticleListRow is one row of the article-listing endpoint. Composed
// from PostgreSQL `documents` + ClickHouse `aer_gold.metrics` /
// `language_detections` to avoid waterfall round-trips client-side.
type ArticleListRow struct {
	ArticleID      string
	Source         string
	Timestamp      time.Time
	Language       sql.NullString
	WordCount      sql.NullInt64
	SentimentScore sql.NullFloat64
}

// ErrSourceNotFound signals that the source identifier did not resolve
// to a row in `public.sources`.
var ErrSourceNotFound = errors.New("source not found")

// ResolveSource returns the canonical name + integer id for either a
// numeric id ("1") or a name ("tagesschau"). The article-listing
// endpoint accepts both forms (Phase 101 §Article browsing endpoint).
func (s *DossierStore) ResolveSource(ctx context.Context, identifier string) (id int64, name string, err error) {
	const q = `
		SELECT id, name FROM sources
		 WHERE name = $1 OR (id::text = $1)
		 LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, q, identifier)
	if err := row.Scan(&id, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, "", ErrSourceNotFound
		}
		return 0, "", fmt.Errorf("resolve source: %w", err)
	}
	return id, name, nil
}

// SourceEligibilityRow carries the Silver-eligibility tuple for a single
// source — used by the Phase 103 Silver endpoints' eligibility gate and
// by the source-detail endpoint.
type SourceEligibilityRow struct {
	ID                    int64
	Name                  string
	Type                  string
	URL                   sql.NullString
	DocumentationURL      sql.NullString
	SilverEligible        bool
	SilverReviewReviewer  sql.NullString
	SilverReviewDate      sql.NullTime
	SilverReviewRationale sql.NullString
	SilverReviewReference sql.NullString
}

// ResolveSourceWithEligibility looks up a source by name or numeric id and
// returns the eligibility tuple. ErrSourceNotFound is recycled for the
// not-found case so callers can map cleanly to HTTP 404.
func (s *DossierStore) ResolveSourceWithEligibility(ctx context.Context, identifier string) (*SourceEligibilityRow, error) {
	const q = `
		SELECT id, name, type, url, documentation_url,
		       silver_eligible,
		       silver_review_reviewer, silver_review_date,
		       silver_review_rationale, silver_review_reference
		  FROM sources
		 WHERE name = $1 OR (id::text = $1)
		 LIMIT 1
	`
	var r SourceEligibilityRow
	row := s.db.QueryRowContext(ctx, q, identifier)
	if err := row.Scan(
		&r.ID, &r.Name, &r.Type, &r.URL, &r.DocumentationURL,
		&r.SilverEligible, &r.SilverReviewReviewer, &r.SilverReviewDate,
		&r.SilverReviewRationale, &r.SilverReviewReference,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSourceNotFound
		}
		return nil, fmt.Errorf("resolve source with eligibility: %w", err)
	}
	return &r, nil
}

// ArticleResolution maps an article_id back to its bronze object key
// and source so the article-detail handler can fetch Silver content
// and run the k-anonymity gate.
type ArticleResolution struct {
	BronzeObjectKey string
	SourceName      string
}

// ResolveArticle looks up the bronze object key for a given article_id.
// Returns ErrSourceNotFound (recycled sentinel — the meaning is "not
// found") when the article hasn't been processed.
//
// Phase 113b / ADR-022: resolution reads from aer_silver.documents in
// ClickHouse. The analytical layer keeps a one-row-per-document
// projection on the same 365-day horizon as Silver and Gold, so an
// article whose 90-day operational PostgreSQL row has been pruned still
// resolves as long as its Silver envelope is in MinIO. anyLast() picks
// the freshest non-empty bronze_object_key — older ReplacingMergeTree
// rows that pre-date Migration 013 carry the empty-string DEFAULT and
// must be ignored.
func (s *DossierStore) ResolveArticle(ctx context.Context, articleID string) (*ArticleResolution, error) {
	if s.chCn == nil {
		return nil, fmt.Errorf("resolve article: ClickHouse not configured")
	}
	const q = `
		SELECT
			anyLastIf(bronze_object_key, bronze_object_key != '') AS bronze_object_key,
			anyLast(source)                                       AS source
		  FROM aer_silver.documents
		 WHERE article_id = ?
		 GROUP BY article_id
		 LIMIT 1
	`
	type row struct {
		BronzeObjectKey string `ch:"bronze_object_key"`
		Source          string `ch:"source"`
	}
	var rows []row
	if err := s.chCn.Select(ctx, &rows, q, articleID); err != nil {
		return nil, fmt.Errorf("resolve article: %w", err)
	}
	if len(rows) == 0 || rows[0].BronzeObjectKey == "" {
		return nil, ErrSourceNotFound
	}
	return &ArticleResolution{
		BronzeObjectKey: rows[0].BronzeObjectKey,
		SourceName:      rows[0].Source,
	}, nil
}
