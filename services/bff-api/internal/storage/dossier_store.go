package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
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

// DossierStore reads Probe Dossier data from PostgreSQL. The BFF owns
// only SELECT on the relevant tables — see infra/postgres/init-roles.sh.
type DossierStore struct {
	db *sql.DB
}

// NewDossierStore wires a DossierStore over the existing read-only pool.
func NewDossierStore(db *sql.DB) *DossierStore {
	return &DossierStore{db: db}
}

// FetchSources returns the per-source dossier rows for the given source
// names in name order. windowStart/windowEnd are optional — when nil the
// `articlesInWindow` count equals the total processed-document count and
// `publicationFreqPerDay` is computed across all-time.
//
// The query left-joins source_classifications via the latest classification
// row per source (MAX(classification_date)) so sources without a
// classification still surface (with null function fields).
func (s *DossierStore) FetchSources(ctx context.Context, sourceNames []string, windowStart, windowEnd *time.Time) ([]DossierSourceRow, error) {
	if len(sourceNames) == 0 {
		return nil, nil
	}

	// Build a $1, $2, ... placeholder list for the IN clause.
	placeholders := make([]string, len(sourceNames))
	args := make([]any, 0, len(sourceNames)+2)
	for i, name := range sourceNames {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args = append(args, name)
	}

	var winStartArg, winEndArg any
	if windowStart != nil && windowEnd != nil {
		winStartArg = *windowStart
		winEndArg = *windowEnd
	}
	args = append(args, winStartArg, winEndArg)
	winStartIdx := len(sourceNames) + 1
	winEndIdx := len(sourceNames) + 2

	// Counts join via documents → ingestion_jobs → sources (Phase 101).
	// `articlesInWindow` falls back to the total when window args are NULL.
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
		),
		doc_counts AS (
			SELECT j.source_id,
			       COUNT(*) FILTER (WHERE d.status = 'processed') AS total,
			       COUNT(*) FILTER (
			           WHERE d.status = 'processed'
			             AND ($%d::timestamptz IS NULL OR d.ingested_at >= $%d::timestamptz)
			             AND ($%d::timestamptz IS NULL OR d.ingested_at <= $%d::timestamptz)
			       ) AS in_window,
			       MIN(d.ingested_at) AS first_ingested,
			       MAX(d.ingested_at) AS last_ingested
			  FROM documents d
			  JOIN ingestion_jobs j ON j.id = d.job_id
			 GROUP BY j.source_id
		)
		SELECT s.name,
		       s.type,
		       s.url,
		       s.documentation_url,
		       COALESCE(c.total, 0) AS articles_total,
		       COALESCE(c.in_window, 0) AS articles_in_window,
		       CASE
		           WHEN c.total IS NULL OR c.total = 0 THEN NULL
		           WHEN c.first_ingested IS NULL OR c.last_ingested IS NULL THEN NULL
		           WHEN EXTRACT(EPOCH FROM (c.last_ingested - c.first_ingested)) <= 0 THEN NULL
		           ELSE c.total::float8 / GREATEST(EXTRACT(EPOCH FROM (c.last_ingested - c.first_ingested)) / 86400.0, 1.0)
		       END AS publication_freq_per_day,
		       lc.primary_function,
		       lc.secondary_function,
		       lc.emic_designation,
		       lc.emic_context,
		       s.silver_eligible,
		       s.silver_review_date
		  FROM sources s
		  LEFT JOIN latest_classification lc ON lc.source_id = s.id
		  LEFT JOIN doc_counts c ON c.source_id = s.id
		 WHERE s.name IN (%s)
		 ORDER BY s.name
	`, winStartIdx, winStartIdx, winEndIdx, winEndIdx, strings.Join(placeholders, ", "))

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
			&r.ArticlesTotal, &r.ArticlesInWindow,
			&r.PublicationFreqPerDay,
			&r.PrimaryFunction, &r.SecondaryFunction,
			&r.EmicDesignation, &r.EmicContext,
			&r.SilverEligible, &r.SilverReviewDate,
		); err != nil {
			return nil, fmt.Errorf("scan dossier row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dossier rows: %w", err)
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
func (s *DossierStore) ResolveArticle(ctx context.Context, articleID string) (*ArticleResolution, error) {
	const q = `
		SELECT d.bronze_object_key, s.name
		  FROM documents d
		  JOIN ingestion_jobs j ON j.id = d.job_id
		  JOIN sources s ON s.id = j.source_id
		 WHERE d.article_id = $1
		 LIMIT 1
	`
	var res ArticleResolution
	row := s.db.QueryRowContext(ctx, q, articleID)
	if err := row.Scan(&res.BronzeObjectKey, &res.SourceName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSourceNotFound
		}
		return nil, fmt.Errorf("resolve article: %w", err)
	}
	return &res, nil
}
