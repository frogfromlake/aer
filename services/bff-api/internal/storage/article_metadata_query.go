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
	metadataFilter *MetadataFilter,
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
	// Faceting (Phase 125a): restrict both subqueries to facet-matching articles.
	facetSA := &scopeArgs{Args: args}
	where += facetSA.metadataFilterClause(metadataFilter, start, end, sources)
	args = facetSA.Args

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

// DegenerateField is a categorical field whose value is CONSTANT across the
// whole scope (exactly one distinct value) — present, but carrying no signal.
// Disclosed (with the constant value) rather than silently dropped (ADR-039).
type DegenerateField struct {
	Field string
	Value string
}

// LowSignalField is a categorical field that carries effectively no signal in
// the scope: either it has only 2 distinct values (a near-binary categorical is
// too weak to read as a distribution — operator decision 2026-06-24) or one
// value dominates ≥ lowSignalDominanceThreshold of in-scope articles. Like
// DegenerateField, the dashboard now DROPS it from the picker (never auto-shown)
// and discloses it under "no signal" — only the disclosure detail differs (a
// dominant value + share rather than a single constant). The flag is
// scope-relative: a future source with real variance clears it automatically.
type LowSignalField struct {
	Field          string
	DistinctValues int
	DominantShare  float64
	DominantValue  string
}

// lowSignalDominanceThreshold is the disclosed engineering-default *display*
// cutoff (Task A): a field with ≥3 distinct values is reported as low-signal
// when its single most common value covers at least this fraction of in-scope
// articles. (Fields with ≤2 distinct values are low-signal regardless of share —
// a near-binary categorical carries too little signal to plot.) It is a
// presentation affordance, NOT a methodological constant. Raising real variance
// (a future source) clears the flag automatically.
const lowSignalDominanceThreshold = 0.85

// lowSignalMaxDistinct: a categorical dimension with at most this many distinct
// values in scope is low-signal regardless of how evenly they split — a 1- or
// 2-value categorical (e.g. article_type = newsArticle/NewsArticle) is too weak
// to read as a distribution (operator decision 2026-06-24).
const lowSignalMaxDistinct = 2

// structuralNoSignalFields are categorical fields that describe a document's
// FORMAT (schema.org structural annotations) rather than its editorial content,
// so they carry no discourse signal by construction and are ALWAYS classified
// no-signal — regardless of how their values split in a scope (operator decision
// 2026-06-24). `article_type` is the schema.org @type, whose values
// (Article / NewsArticle and case/synonym variants) are a publishing-format
// artefact, not an editorial dimension; its variants split the dominant share
// just under the statistical threshold, so the statistical rule alone misses it.
// This is a deliberate, documented exception to the otherwise drift-free
// "distinct field set IS the dimension set" rule — it classifies field SEMANTICS
// (format vs content), not a transient value distribution.
var structuralNoSignalFields = map[string]bool{
	"article_type": true,
}

// ScopeMetadataAvailability splits the categorical metadata fields observed in a
// scope's window into those present for every scoped source (Available) and
// those present for only some (Partial). The categorical analog of
// ScopeMetricAvailability — it gates which metadata dimensions a panel may offer
// so a cross-probe panel never binds a field that silently yields empty cells
// (Phase 123c C1 discipline applied to metadata).
// Degenerate and LowSignal are ADDITIVE advisory lists (Task A): Available and
// Partial keep their pure "has data" intersection semantics, while Degenerate
// (constant fields) and LowSignal (near-constant fields) annotate the no-/low-
// signal cases. The dashboard drops Degenerate from the picker and flags
// LowSignal inline; neither is silently filtered (ADR-039).
type ScopeMetadataAvailability struct {
	ScopedSources []string
	Available     []string
	Partial       []PartialMetadataField
	Degenerate    []DegenerateField
	LowSignal     []LowSignalField
	// Phase 148g — per-source constancy: a field that carries signal across the
	// scope (NOT degenerate/low-signal) yet is CONSTANT for one source (that
	// source emits a single distinct value, e.g. Élysée's institutional `author`).
	// Advisory disclosure only; the field stays offerable.
	PerSourceConstant []PerSourceConstantField
}

// PerSourceConstantField names a (field, source) pair where the source emits a
// single distinct value for an otherwise-varying field — the per-source analogue
// of structural absence (the source carries no WITHIN-source signal for it).
type PerSourceConstantField struct {
	Field  string
	Source string
	Value  string
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
	// Shared scope predicate, reused verbatim by the concentration queries below
	// so availability and degeneracy never disagree on the window/source set.
	where := "timestamp >= $1 AND timestamp < $2 AND source IN (" + strings.Join(placeholders, ", ") + ")"
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
	availableSet := map[string]bool{}
	for _, field := range order {
		srcSet := bySrc[field]
		if len(srcSet) == total {
			out.Available = append(out.Available, field)
			availableSet[field] = true
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

	if err := s.classifyMetadataConcentration(ctx, where, args, availableSet, &out); err != nil {
		return out, err
	}
	if err := s.classifyPerSourceConstant(ctx, where, args, &out); err != nil {
		return out, err
	}
	return out, nil
}

// classifyPerSourceConstant fills the PerSourceConstant advisory list: for each
// (field, source) where the source emits exactly ONE distinct value, the field is
// constant for that source (no within-source signal). Fields that are constant
// scope-WIDE (already in Degenerate/LowSignal) are excluded — there the whole
// field is no-signal and the per-source detail is redundant. One cheap aggregation
// (cardinality = #fields × #sources). `where`/`args` are the caller's shared scope
// predicate so the window/source set never disagrees with availability.
func (s *ClickHouseStorage) classifyPerSourceConstant(
	ctx context.Context,
	where string,
	args []any,
	out *ScopeMetadataAvailability,
) error {
	noSignal := map[string]bool{}
	for _, d := range out.Degenerate {
		noSignal[d.Field] = true
	}
	for _, l := range out.LowSignal {
		noSignal[l.Field] = true
	}

	// `value` is Array(String) (list-valued fields), so unnest with arrayJoin
	// before counting distinct values per (field, source) — exactly the rollup the
	// concentration query uses. A source is constant for the field when it emits a
	// single distinct value across all its articles' value arrays. FINAL mirrors
	// the sibling reads so a transient cross-version row never skews the count.
	query := `
		SELECT Field, Source, any(v) AS Value
		FROM (
			SELECT field AS Field, source AS Source, arrayJoin(value) AS v
			FROM aer_gold.article_metadata FINAL
			WHERE ` + where + `
		)
		GROUP BY Field, Source
		HAVING uniqExact(v) = 1
		ORDER BY Field, Source
	`
	var rows []struct {
		Field  string `ch:"Field"`
		Source string `ch:"Source"`
		Value  string `ch:"Value"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query per-source-constant metadata", "error", err)
		return err
	}
	for _, r := range rows {
		// Skip a field that is constant scope-wide (no-signal) — the per-source
		// breakdown adds nothing there; this list is for fields that DO vary across
		// the scope but are constant on an individual source (e.g. Élysée's author).
		if noSignal[r.Field] {
			continue
		}
		out.PerSourceConstant = append(out.PerSourceConstant, PerSourceConstantField{
			Field:  r.Field,
			Source: r.Source,
			Value:  r.Value,
		})
	}
	return nil
}

// classifyMetadataConcentration computes, per field in scope, the distinct-value
// count + the dominant value's distinct-article share, and fills the Degenerate
// (constant) and LowSignal (near-constant) advisory lists (Task A). `where` is
// the already-built "timestamp + source IN (...)" predicate and `args` its bind
// values, shared verbatim with the caller's availability query so the two never
// disagree on the scope.
//
// Two field-grained aggregations (cardinality = #fields, so cheap):
//   - perValue: distinct values, the dominant value, and its distinct-article
//     count, via a nested (field,value) → uniqExact(article_id) rollup;
//   - total: distinct articles carrying ANY value for the field.
//
// dominantShare = dominantArticles / totalArticles is an article fraction (exact
// for single-valued fields like article_type; for multi-value list fields an
// article carrying several values is counted once in the denominator and in the
// numerator only when it carries the dominant value — a faithful "share of
// field-carrying articles", never inflated). FINAL mirrors the sibling
// distribution read so a transient cross-version row never double-counts.
func (s *ClickHouseStorage) classifyMetadataConcentration(
	ctx context.Context,
	where string,
	args []any,
	availableSet map[string]bool,
	out *ScopeMetadataAvailability,
) error {
	perValueQuery := fmt.Sprintf(`
		SELECT Field, uniqExact(v) AS Distinct, max(Cnt) AS Dominant, argMax(v, Cnt) AS DominantValue
		FROM (
			SELECT field AS Field, arrayJoin(value) AS v, uniqExact(article_id) AS Cnt
			FROM aer_gold.article_metadata FINAL
			WHERE %s
			GROUP BY Field, v
		)
		GROUP BY Field
		ORDER BY Field
	`, where)
	var pvRows []struct {
		Field         string `ch:"Field"`
		Distinct      uint64 `ch:"Distinct"`
		Dominant      uint64 `ch:"Dominant"`
		DominantValue string `ch:"DominantValue"`
	}
	if err := s.conn.Select(ctx, &pvRows, perValueQuery, args...); err != nil {
		slog.Error("Failed to query metadata concentration (per-value)", "error", err)
		return err
	}

	totalQuery := fmt.Sprintf(`
		SELECT field AS Field, uniqExact(article_id) AS Total
		FROM aer_gold.article_metadata FINAL
		WHERE %s
		GROUP BY Field
	`, where)
	var totalRows []struct {
		Field string `ch:"Field"`
		Total uint64 `ch:"Total"`
	}
	if err := s.conn.Select(ctx, &totalRows, totalQuery, args...); err != nil {
		slog.Error("Failed to query metadata concentration (total)", "error", err)
		return err
	}
	totals := make(map[string]uint64, len(totalRows))
	for _, r := range totalRows {
		totals[r.Field] = r.Total
	}

	for _, r := range pvRows {
		structural := structuralNoSignalFields[r.Field]
		// Phase 148g — no-signal classification (degenerate AND low-signal) applies
		// to OFFERABLE (available) fields + STRUCTURAL fields only. A PARTIAL content
		// field (present on some sources, not all) is already surfaced as "withheld"
		// by the intersection gate; classifying it no-signal here — and pruning it
		// from Partial below — wrongly EMPTIES the withheld list when a partial field
		// happens to be constant over the scope union in the window, hiding a field
		// like `author` that the user can still opt into (show-anyway). Its per-source
		// constancy is disclosed separately (PerSourceConstant). A STRUCTURAL field
		// (document format, e.g. article_type) is no-signal on ANY source set, so it
		// IS classified even when partial — otherwise "show anyway" would resurrect it.
		if !structural && !availableSet[r.Field] {
			continue
		}
		if r.Distinct <= 1 {
			out.Degenerate = append(out.Degenerate, DegenerateField{Field: r.Field, Value: r.DominantValue})
			continue
		}
		total := totals[r.Field]
		if total == 0 {
			continue
		}
		share := float64(r.Dominant) / float64(total)
		if structural || r.Distinct <= lowSignalMaxDistinct || share >= lowSignalDominanceThreshold {
			out.LowSignal = append(out.LowSignal, LowSignalField{
				Field:          r.Field,
				DistinctValues: int(r.Distinct), //nolint:gosec // bounded by field cardinality
				DominantShare:  share,
				DominantValue:  r.DominantValue,
			})
		}
	}

	// A no-signal field (constant / near-constant / structural) belongs ONLY under
	// the "no signal" disclosure — never the withheld (Partial) list, where "show
	// anyway" would resurrect it and re-seed it as a default. Prune it from Partial
	// so a partial-but-no-signal field (e.g. article_type missing on one source)
	// can never be re-offered.
	noSignal := make(map[string]bool, len(out.Degenerate)+len(out.LowSignal))
	for _, d := range out.Degenerate {
		noSignal[d.Field] = true
	}
	for _, l := range out.LowSignal {
		noSignal[l.Field] = true
	}
	if len(noSignal) > 0 {
		kept := out.Partial[:0]
		for _, p := range out.Partial {
			if !noSignal[p.Field] {
				kept = append(kept, p)
			}
		}
		out.Partial = kept
	}
	return nil
}
