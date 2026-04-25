package config

import "time"

// SourceEntry mirrors a row of the PostgreSQL `sources` table plus the
// `documentation_url` column added in migration 000007 and the
// Silver-eligibility / review metadata added in migration 000011 (Phase
// 101). The BFF API reads this shape directly from Postgres via a
// read-only role (see internal/storage.SourceStore). It is no longer
// loaded from YAML — the Postgres table is the single source of truth
// (Phase 87).
type SourceEntry struct {
	Name             string
	Type             string
	URL              *string
	DocumentationURL *string
	// Silver-eligibility (Phase 103). SilverEligible defaults to false and
	// is flipped per source by an explicit one-off Postgres migration after
	// the WP-006 §5.2 review. The four review-metadata fields are populated
	// as a tuple at flip time.
	SilverEligible        bool
	SilverReviewReviewer  *string
	SilverReviewDate      *time.Time
	SilverReviewRationale *string
	SilverReviewReference *string
}
