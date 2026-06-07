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
	// Phase 122d.2 — Temporal-Provenance-Absence count (articles with no real
	// publication date; window-scoped when a window is supplied).
	TemporalProvenanceAbsent int64
	PrimaryFunction          sql.NullString
	SecondaryFunction        sql.NullString
	EmicDesignation          sql.NullString
	EmicContext              sql.NullString
	SilverEligible           bool
	SilverReviewDate         sql.NullTime
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
			r.TemporalProvenanceAbsent = c.nsTemporal
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
	// Phase 122d.2 — Temporal-Provenance-Absence count (articles with no real
	// publication date, window-scoped when a window is supplied).
	nsTemporal int64
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
	// DateTime, so we branch the predicate textually instead of binding
	// nil.
	//
	// Placeholder order in the rendered query (ClickHouse binds are positional,
	// not named — the CTE and outer query each take the IN-list independently):
	//   (1) sourceNames            — CTE `WHERE d_inner.source IN ?`
	//   (2) windowStart, windowEnd — in_window's `countDistinctIf` (Phase 131a)
	//   (3) windowStart, windowEnd — ns_temporal's `countDistinctIf` (Phase 122d.2)
	//   (4) sourceNames            — outer `WHERE d.source IN ?`
	// (2) and (3) are present only when a window is supplied (both branch on the
	// same window, so they are appended/omitted together).
	var windowFilter string
	var args []any
	args = append(args, sourceNames)
	if windowStart != nil && windowEnd != nil {
		windowFilter = "AND timestamp >= ? AND timestamp <= ?"
		args = append(args, *windowStart, *windowEnd)
	}
	// Phase 122d.2 — per-source Temporal-Provenance-Absence count (articles whose
	// timestamp is the crawler fetch time, not a real publication date). Reuses
	// the same window filter, so its window args are appended a SECOND time, in
	// source order (after in_window's, before the outer source IN).
	if windowStart != nil && windowEnd != nil {
		args = append(args, *windowStart, *windowEnd)
	}

	// FINAL collapses ReplacingMergeTree duplicates from NATS redelivery so
	// the count is per distinct (timestamp, source, article_id) tuple,
	// matching the dedupe Gold/Silver readers see at query time.
	//
	// Phase 131a — `recent_7d` counts articles whose `published_date`
	// falls within 7 days of the source's NEWEST `published_date` (not
	// of `now()`). That makes the per-day rate robust to two failure
	// modes: (a) a crawl that runs once and then idles — `now()-7d`
	// would drift past the latest article and the rate would collapse
	// to 0; (b) the recently-re-listed-old-article artefact that
	// stretched the published-date span and deflated the long-run
	// `total / span` rate to a misleadingly small number (the value
	// users saw as "0.9 / day" when the actual cadence was ~35/day).
	query := fmt.Sprintf(`
		WITH source_max AS (
		    SELECT source, max(timestamp) AS max_ts
		      FROM aer_silver.documents AS d_inner FINAL
		     WHERE d_inner.source IN ?
		     GROUP BY d_inner.source
		)
		SELECT
			d.source                                          AS source,
			count(DISTINCT d.article_id)                      AS total,
			countDistinctIf(d.article_id, 1=1 %s)             AS in_window,
			countDistinctIf(
				d.article_id,
				d.timestamp_source = 'fetch_at_fallback' %s
			)                                                 AS ns_temporal,
			countDistinctIf(
				d.article_id,
				d.timestamp >= sm.max_ts - INTERVAL 7 DAY
			)                                                 AS recent_7d,
			toUnixTimestamp(min(d.timestamp))                 AS first_ts,
			toUnixTimestamp(max(d.timestamp))                 AS last_ts
		  FROM aer_silver.documents AS d FINAL
		  INNER JOIN source_max sm ON d.source = sm.source
		 WHERE d.source IN ?
		 GROUP BY d.source
	`, windowFilter, windowFilter)

	// Outer-query sourceNames IN-list (positional placeholder group #4 in
	// the rendered SQL — see args ordering comment at function top).
	args = append(args, sourceNames)

	type row struct {
		Source     string `ch:"source"`
		Total      uint64 `ch:"total"`
		InWindow   uint64 `ch:"in_window"`
		NsTemporal uint64 `ch:"ns_temporal"`
		Recent7d   uint64 `ch:"recent_7d"`
		FirstTS    uint32 `ch:"first_ts"`
		LastTS     uint32 `ch:"last_ts"`
	}
	var rows []row
	if err := s.chCn.Select(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("source counts query: %w", err)
	}

	// Window length in days for the in-window publication rate, floored at
	// 1 so a sub-day window never divides by zero. Only meaningful when both
	// bounds are bound.
	hasWindow := windowStart != nil && windowEnd != nil
	var windowDays float64
	if hasWindow {
		windowDays = windowEnd.Sub(*windowStart).Hours() / 24.0
		if windowDays < 1.0 {
			windowDays = 1.0
		}
	}

	out := make(map[string]sourceCount, len(rows))
	for _, r := range rows {
		c := sourceCount{
			total:      int64(r.Total),      //nolint:gosec // bounded by source rowcount
			inWindow:   int64(r.InWindow),   //nolint:gosec // bounded by source rowcount
			nsTemporal: int64(r.NsTemporal), //nolint:gosec // bounded by source rowcount
		}
		// Publication-frequency-per-day reports the cadence the user is
		// actually looking at, in two modes:
		//
		//   - With an explicit window: `in_window / window_days`
		//     (a 1-day floor guards the same-day-burst divide-by-zero).
		//
		//   - Without a window (Phase 131a default): a 7-day rolling
		//     rate anchored on the SOURCE'S NEWEST `published_date`,
		//     i.e. `recent_7d / 7`. This robustly reports the natural
		//     publication cadence even when (a) the crawl is idle and
		//     `now()` has drifted past the latest article and (b)
		//     the published-date span includes recently-re-listed-old
		//     articles that would deflate a `total / span` average to
		//     a misleadingly small number (the "0.9 / day" artefact
		//     observed on tagesschau at 288 total / 10-month span).
		switch {
		case hasWindow:
			if r.InWindow > 0 {
				c.freqPerDay = float64(r.InWindow) / windowDays
			}
		case r.Recent7d > 0:
			c.freqPerDay = float64(r.Recent7d) / 7.0
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
