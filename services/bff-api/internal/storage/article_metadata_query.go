package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// CategoryCount is one categorical value and the number of DISTINCT articles in
// scope that carry it.
type CategoryCount struct {
	Value    string `ch:"Value"`
	Articles uint64 `ch:"Articles"`
}

// CategoricalDistributionResult is the top-N category counts for one metadata
// field over a scope, plus the disclosed long-tail so a Top-N truncation is
// never silent (Phase 133).
type CategoricalDistributionResult struct {
	Categories []CategoryCount
	// TotalArticles is the distinct-article count in scope that carry ANY value
	// for the field.
	TotalArticles int64
	// DistinctValues is the number of distinct values for the field in scope —
	// drives the honest "showing top N of M categories" disclosure.
	DistinctValues int64
	// OtherArticles is the summed per-value article weight beyond the Top-N,
	// retained as a disclosed datum (response + export). For list fields one
	// article can carry several values, so this is a value-occurrence weight,
	// NOT a distinct-article count — the cell discloses the truncation via
	// DistinctValues (the unambiguous count) rather than drawing it as a bar on
	// the distinct-article axis.
	OtherArticles int64
}

// GetCategoricalDistribution returns, for one categorical metadata `field`, the
// top-N values by distinct-article count over the window + source set, plus the
// disclosed long-tail.
//
// Reads aer_gold.article_metadata (one row per (article, field) with an
// Array(String) `value`). `arrayJoin(value)` expands the per-field value array;
// `uniqExact(article_id)` per value counts DISTINCT articles so a duplicate
// element within one article never double-counts. An empty source set returns an
// empty result (the handler resolves scope → sources).
//
// Why FINAL here (the sibling metrics reads deliberately omit it): a re-ingest
// that CHANGES an article's value array writes a higher-version row that, before
// the background merge, coexists with the prior version. Without FINAL,
// arrayJoin would expand BOTH versions and uniqExact would attribute the article
// to its OLD and NEW value at once (a transient cross-version double-count that
// distorts an EXACT distinct-article distribution). The avg/sum metrics reads
// tolerate that transient skew (it self-heals on merge), but a category
// distribution is a count, so we pay the merge-on-read to keep it correct. This
// is the documented exception to the "no FINAL on Gold reads" convention.
func (s *ClickHouseStorage) GetCategoricalDistribution(
	ctx context.Context,
	field string,
	sources []string,
	start, end time.Time,
	topN int,
) (CategoricalDistributionResult, error) {
	out := CategoricalDistributionResult{Categories: []CategoryCount{}}
	if field == "" || len(sources) == 0 {
		return out, nil
	}
	if topN < 1 {
		topN = 1
	}
	if topN > 200 {
		topN = 200
	}

	// Shared WHERE: field + window + source IN (...).
	args := []any{field, start, end}
	clauses := []string{"field = $1", "timestamp >= $2", "timestamp < $3"}
	placeholders := make([]string, len(sources))
	for i, src := range sources {
		placeholders[i] = fmt.Sprintf("$%d", i+4)
		args = append(args, src)
	}
	clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ", ")))
	where := strings.Join(clauses, " AND ")

	perValueQuery := fmt.Sprintf(`
		SELECT v AS Value, uniqExact(article_id) AS Articles
		FROM (
			SELECT article_id, arrayJoin(value) AS v
			FROM aer_gold.article_metadata FINAL
			WHERE %s
		)
		GROUP BY v
		ORDER BY Articles DESC, Value ASC
	`, where)

	var rows []CategoryCount
	if err := s.conn.Select(ctx, &rows, perValueQuery, args...); err != nil {
		slog.Error("Failed to query categorical distribution", "error", err, "field", field)
		return out, err
	}

	// Distinct-article total for the field (an article with any value).
	totalQuery := fmt.Sprintf(`
		SELECT uniqExact(article_id) AS N
		FROM aer_gold.article_metadata FINAL
		WHERE %s
	`, where)
	var totalRows []struct {
		N uint64 `ch:"N"`
	}
	if err := s.conn.Select(ctx, &totalRows, totalQuery, args...); err != nil {
		slog.Error("Failed to query categorical distribution total", "error", err, "field", field)
		return out, err
	}
	if len(totalRows) > 0 {
		out.TotalArticles = int64(totalRows[0].N) //nolint:gosec // bounded by 365-day TTL
	}

	out.DistinctValues = int64(len(rows))
	if len(rows) > topN {
		var other int64
		for _, r := range rows[topN:] {
			other += int64(r.Articles) //nolint:gosec // bounded by 365-day TTL
		}
		out.OtherArticles = other
		rows = rows[:topN]
	}
	out.Categories = rows
	return out, nil
}

// PartialMetadataField is a categorical field present for only a subset of the
// scoped sources.
type PartialMetadataField struct {
	Field   string
	Sources []string
}

// ScopeMetadataAvailability splits the categorical metadata fields observed in a
// scope's window into those present for every scoped source (Available) and
// those present for only some (Partial). The categorical analog of
// ScopeMetricAvailability — it gates which metadata dimensions a panel may offer
// so a cross-probe panel never binds a field that silently yields empty cells
// (Phase 123c C1 discipline applied to metadata).
type ScopeMetadataAvailability struct {
	ScopedSources []string
	Available     []string
	Partial       []PartialMetadataField
}

// GetScopeAvailableMetadata returns, for the given sources and window, which
// categorical metadata fields have data for every source (the intersection)
// versus only some.
//
// The signal is `DISTINCT field FROM aer_gold.article_metadata` — the exact
// parallel of GetScopeAvailableMetrics' `DISTINCT metric_name FROM metrics`.
// article_metadata holds ONLY categorical fields (the worker writes scalar
// metadata to aer_gold.metrics instead), so the distinct field set IS the
// available categorical dimension set — no hardcoded field list, drift-free.
// "Present" means "has at least one value-row in the window", which is exactly
// "selecting it would render non-empty". (The Phase-122f coverage matrix +
// structurallyAbsent remains the methodological Negative-Space surface; this is
// only the picker availability gate.)
func (s *ClickHouseStorage) GetScopeAvailableMetadata(
	ctx context.Context,
	start, end time.Time,
	sources []string,
) (ScopeMetadataAvailability, error) {
	out := ScopeMetadataAvailability{ScopedSources: sources, Available: []string{}, Partial: []PartialMetadataField{}}
	if len(sources) == 0 {
		return out, nil
	}

	query := `
		SELECT DISTINCT source AS Source, field AS Field
		FROM aer_gold.article_metadata
		WHERE timestamp >= $1 AND timestamp < $2
	`
	args := []any{start, end}
	placeholders := make([]string, len(sources))
	for i, src := range sources {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, src)
	}
	query += " AND source IN (" + strings.Join(placeholders, ", ") + ")"
	query += " ORDER BY Field, Source"

	var rows []struct {
		Source string `ch:"Source"`
		Field  string `ch:"Field"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query scope available metadata", "error", err)
		return out, err
	}

	bySrc := map[string]map[string]bool{}
	order := []string{}
	for _, r := range rows {
		if _, ok := bySrc[r.Field]; !ok {
			bySrc[r.Field] = map[string]bool{}
			order = append(order, r.Field)
		}
		bySrc[r.Field][r.Source] = true
	}

	total := len(sources)
	for _, field := range order {
		srcSet := bySrc[field]
		if len(srcSet) == total {
			out.Available = append(out.Available, field)
			continue
		}
		present := make([]string, 0, len(srcSet))
		for _, src := range sources {
			if srcSet[src] {
				present = append(present, src)
			}
		}
		out.Partial = append(out.Partial, PartialMetadataField{Field: field, Sources: present})
	}
	return out, nil
}
