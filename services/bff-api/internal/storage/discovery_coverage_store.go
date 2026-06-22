package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// DiscoveryCoverageRow is one (source × channel) record summarised
// across the requested trailing window (Phase 122g).
type DiscoveryCoverageRow struct {
	Channel                 string
	LastRunDiscovered       int64
	LastRunAfterDedup       int64
	AverageDiscoveredPerRun float64
	// Phase 148d (WP-007 §4.1) — the publisher-declared in-window
	// denominator and its trust flag, from the most recent run.
	Declared              sql.NullInt64
	DeclaredIndeterminate bool
}

// DiscoveryCoverageSummary is the per-source assemblage of telemetry
// the BFF handler renders into the OpenAPI response shape.
type DiscoveryCoverageSummary struct {
	WindowDays              int
	PerChannel              []DiscoveryCoverageRow
	TotalDiscoveredLastRun  int64
	UniqueAfterDedupLastRun int64
	UnderflowAlertActive    bool
	ExpectedFloorPerRun     sql.NullInt64
	// Phase 148d (WP-007) — completeness verdict + the per-source funnel.
	Completeness CompletenessResult
	Funnel       FunnelSummary
}

// GetDiscoveryCoverage queries crawler_discovery_runs +
// crawler_discovery_alerts for one source over the trailing window.
// The query returns one row per channel — the dashboard expects every
// channel that has telemetry in the window, plus a synthetic "last
// run" row identifying the most recent pass's URL counts.
//
// `expectedFloor` is read from the source's sources.yaml-derived
// configuration via the caller (the storage layer doesn't see YAML);
// today we surface only what's recorded in the alerts table, which
// captures the floor in effect at the time the alert was emitted.
func (s *DossierStore) GetDiscoveryCoverage(
	ctx context.Context,
	sourceID int64,
	sourceName string,
	windowDays int,
) (*DiscoveryCoverageSummary, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	cutoff := time.Now().UTC().Add(-time.Duration(windowDays) * 24 * time.Hour)

	// Per-channel aggregate over the window, plus the most-recent
	// run's per-channel snapshot. Single round-trip via two CTEs.
	const q = `
		WITH window_rows AS (
			SELECT channel,
			       urls_discovered,
			       urls_after_dedup,
			       run_started_at
			  FROM crawler_discovery_runs
			 WHERE source_id = $1
			   AND run_started_at >= $2
		),
		last_run AS (
			SELECT channel, urls_discovered, urls_after_dedup,
			       declared, declared_indeterminate
			  FROM crawler_discovery_runs
			 WHERE source_id = $1
			   AND run_started_at = (
			       SELECT MAX(run_started_at)
			         FROM crawler_discovery_runs
			        WHERE source_id = $1
			   )
		),
		channels AS (
			SELECT DISTINCT channel FROM window_rows
			UNION
			SELECT DISTINCT channel FROM last_run
		)
		SELECT c.channel,
		       COALESCE(lr.urls_discovered, 0)  AS last_discovered,
		       COALESCE(lr.urls_after_dedup, 0) AS last_after_dedup,
		       COALESCE(AVG(wr.urls_discovered), 0)::float8 AS avg_discovered,
		       lr.declared,
		       COALESCE(lr.declared_indeterminate, false) AS declared_indeterminate
		  FROM channels c
		  LEFT JOIN last_run    lr ON lr.channel = c.channel
		  LEFT JOIN window_rows wr ON wr.channel = c.channel
		 GROUP BY c.channel, lr.urls_discovered, lr.urls_after_dedup,
		          lr.declared, lr.declared_indeterminate
		 ORDER BY c.channel
	`
	rows, err := s.db.QueryContext(ctx, q, sourceID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("query discovery_coverage rows: %w", err)
	}
	defer rows.Close()

	summary := &DiscoveryCoverageSummary{
		WindowDays: windowDays,
		PerChannel: []DiscoveryCoverageRow{},
	}
	for rows.Next() {
		var row DiscoveryCoverageRow
		if err := rows.Scan(
			&row.Channel,
			&row.LastRunDiscovered,
			&row.LastRunAfterDedup,
			&row.AverageDiscoveredPerRun,
			&row.Declared,
			&row.DeclaredIndeterminate,
		); err != nil {
			return nil, fmt.Errorf("scan discovery_coverage row: %w", err)
		}
		summary.PerChannel = append(summary.PerChannel, row)
		summary.TotalDiscoveredLastRun += row.LastRunDiscovered
		summary.UniqueAfterDedupLastRun += row.LastRunAfterDedup
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate discovery_coverage rows: %w", err)
	}

	// Alert state — one query, fail-open (telemetry table absent ⇒
	// no alert, not an error).
	const alertQ = `
		SELECT expected_floor
		  FROM crawler_discovery_alerts
		 WHERE source_id = $1
		   AND alert_type = 'underflow'
		 LIMIT 1
	`
	var floor sql.NullInt64
	switch err := s.db.QueryRowContext(ctx, alertQ, sourceID).Scan(&floor); {
	case err == sql.ErrNoRows:
		summary.UnderflowAlertActive = false
	case err != nil:
		return nil, fmt.Errorf("query discovery_coverage alert: %w", err)
	default:
		summary.UnderflowAlertActive = true
		summary.ExpectedFloorPerRun = floor
	}

	// When no alert is active, we still want to surface the floor if
	// one is configured. The floor lives in sources.yaml (not in the
	// Postgres alerts table when not firing), so the BFF handler
	// substitutes the YAML-declared value at render time. Storage
	// layer leaves `ExpectedFloorPerRun` zero-valued in that case.

	// Phase 148d (WP-007 §5) — the per-source funnel (latest run) +
	// the reconciled Gold tail. Both fail-open: a missing funnel row (a
	// never-crawled or pre-148d source) or an unreachable ClickHouse leaves
	// the funnel absent rather than failing the whole coverage call.
	funnel, err := s.queryFunnel(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("query crawler_funnel_runs: %w", err)
	}
	funnel.GoldRows = s.countGoldArticles(ctx, sourceName)
	FillFunnelRates(&funnel)
	summary.Funnel = funnel

	// Completeness verdict (WP-007 §4.1): Gold rows over the trustworthy
	// declared denominator. The Gold count is cumulative (point-in-time),
	// not per-run — so a re-crawl that submits zero new documents (all
	// already_collected) still reports the corpus it actually holds, never
	// a misleading 0 %.
	summary.Completeness = DeriveCompleteness(summary.PerChannel, funnel.GoldRows)
	return summary, nil
}

// queryFunnel reads the most recent crawler_funnel_runs row for a source.
// Returns a zero-value (Present=false) summary when no row exists.
func (s *DossierStore) queryFunnel(ctx context.Context, sourceID int64) (FunnelSummary, error) {
	const q = `
		SELECT discovered, url_filtered, already_collected, fetched,
		       not_modified, content_dropped, thin_content_dropped,
		       submitted, errored
		  FROM crawler_funnel_runs
		 WHERE source_id = $1
		 ORDER BY run_started_at DESC
		 LIMIT 1
	`
	var f FunnelSummary
	switch err := s.db.QueryRowContext(ctx, q, sourceID).Scan(
		&f.Discovered, &f.URLFiltered, &f.AlreadyCollected, &f.Fetched,
		&f.NotModified, &f.ContentDropped, &f.ThinContentDropped,
		&f.Submitted, &f.Errored,
	); {
	case err == sql.ErrNoRows:
		return FunnelSummary{Present: false}, nil
	case err != nil:
		return FunnelSummary{}, err
	default:
		f.Present = true
		return f, nil
	}
}

// countGoldArticles reconciles the funnel tail (extracted → Gold) against
// the current distinct-article count in ClickHouse Gold. Fail-open: a nil
// ClickHouse handle or a query error yields 0 (the completeness figure then
// reads as a lower bound, which the indeterminate/Negative-Space disclosure
// already covers) rather than failing the coverage call.
func (s *DossierStore) countGoldArticles(ctx context.Context, sourceName string) int64 {
	if s.chCn == nil || sourceName == "" {
		return 0
	}
	const q = `SELECT countDistinct(article_id) AS n FROM aer_gold.metrics WHERE source = ?`
	var rows []struct {
		N uint64 `ch:"n"`
	}
	if err := s.chCn.Select(ctx, &rows, q, sourceName); err != nil {
		slog.Warn("discovery_coverage gold reconcile failed", "source", sourceName, "error", err)
		return 0
	}
	if len(rows) == 0 {
		return 0
	}
	return int64(rows[0].N) //nolint:gosec // article counts are bounded well within int64
}
