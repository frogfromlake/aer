package config

// SourceEntry mirrors a row of the PostgreSQL `sources` table plus the
// `documentation_url` column added in migration 000007. The BFF API reads
// this shape directly from Postgres via a read-only role (see
// internal/storage.SourceStore). It is no longer loaded from YAML — the
// Postgres table is the single source of truth (Phase 87).
type SourceEntry struct {
	Name             string
	Type             string
	URL              *string
	DocumentationURL *string
}
