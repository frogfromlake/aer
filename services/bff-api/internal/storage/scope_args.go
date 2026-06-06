package storage

import (
	"fmt"
	"strings"
	"time"
)

// MetadataFilter restricts a per-article query to articles whose categorical
// metadata field carries a specific value (Phase 125a faceting / small-
// multiples). A nil *MetadataFilter — or one with an empty Field/Value — means
// no restriction.
type MetadataFilter struct {
	Field string
	Value string
}

// scopeArgs accumulates positional query parameters for ClickHouse ($1, $2, …)
// while a per-article view-mode query assembles its WHERE clause. It centralises
// the window + source-IN placeholder bookkeeping that was copy-pasted across the
// Phase-125 query builders (Phase 125a cleanup).
//
// Every bound value goes through ph(), so the returned placeholder string always
// matches the value's 1-based position in Args. Binding the same value twice
// (e.g. the window appears in two subqueries) is fine — it simply appears twice
// in Args; ClickHouse positional parameters are not deduplicated.
//
// Pass sa.Args... as the trailing query parameters.
type scopeArgs struct {
	Args []any
}

// newScopeArgs returns an empty accumulator.
func newScopeArgs() *scopeArgs { return &scopeArgs{} }

// ph appends a value and returns its positional placeholder ($N).
func (sa *scopeArgs) ph(v any) string {
	sa.Args = append(sa.Args, v)
	return fmt.Sprintf("$%d", len(sa.Args))
}

// srcIn appends every source and returns the comma-joined placeholder list for a
// `source IN (...)` predicate. Returns "" for an empty set — callers guard the
// resolved scope upstream, so this only happens on a degenerate request.
func (sa *scopeArgs) srcIn(sources []string) string {
	if len(sources) == 0 {
		return ""
	}
	ps := make([]string, len(sources))
	for i, src := range sources {
		ps[i] = sa.ph(src)
	}
	return strings.Join(ps, ", ")
}

// metadataFilterClause returns an ` AND article_id IN (...)` membership predicate
// restricting a per-article query to articles whose `mf.Field` carries `mf.Value`
// over the same window and source set — the spine of Phase-125a faceting. It
// returns "" when mf is nil/empty (no faceting) or the scope is empty. The
// subquery FINALs `article_metadata` for exact membership (mirroring the
// crosstab/categorical-distribution convention) and uses `has(value, …)` because
// the metadata value column is an Array. Placeholders are appended in call order,
// so the clause composes onto whatever WHERE the caller is already assembling.
// The leading space lets callers concatenate it directly.
func (sa *scopeArgs) metadataFilterClause(mf *MetadataFilter, start, end time.Time, sources []string) string {
	if mf == nil || mf.Field == "" || mf.Value == "" || len(sources) == 0 {
		return ""
	}
	return fmt.Sprintf(
		" AND article_id IN (SELECT article_id FROM aer_gold.article_metadata FINAL"+
			" WHERE field = %s AND has(value, %s) AND timestamp >= %s AND timestamp < %s AND source IN (%s))",
		sa.ph(mf.Field), sa.ph(mf.Value), sa.ph(start), sa.ph(end), sa.srcIn(sources),
	)
}
