package storage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// MetadataCoverageCell is one (source, field, method) bucket of the
// per-source coverage matrix surfaced by Phase 122f.
type MetadataCoverageCell struct {
	Source   string    `ch:"source"`
	Field    string    `ch:"field"`
	Method   string    `ch:"method"`
	Articles uint64    `ch:"articles"`
	LastSeen time.Time `ch:"last_seen"`
}

// nullCoverageMethod is the literal-string sentinel the worker writes
// for fields it did not populate. The Phase 122f surface treats this as
// the Negative-Space marker, not a SQL NULL — see
// `services/analysis-worker/internal/metadata_coverage.py`.
const nullCoverageMethod = "null"

// MetadataCoverageQuerier is the storage-side interface for the
// Phase 122f metadata-coverage endpoints. Implemented by
// ClickHouseStorage.
type MetadataCoverageQuerier interface {
	GetMetadataCoverage(ctx context.Context, sources []string) ([]MetadataCoverageCell, error)
	GetFieldCardinality(ctx context.Context, sources []string) (map[FieldKey]FieldCardinality, error)
}

// FieldKey identifies a (source, coverage-field) pair. The field name is the
// COVERAGE field name (e.g. "images"), not the Gold metric name (e.g.
// "image_count") — the scalar query below remaps so the dossier can mark by the
// same field names it already renders.
type FieldKey struct {
	Source string
	Field  string
}

// FieldCardinality is the distinct-value count for one (source, field) over the
// source's whole corpus, plus a sample value (well-defined as THE value only
// when Distinct == 1). Backs the Task-A "constant → no signal" dossier marker.
type FieldCardinality struct {
	Distinct uint64
	Value    string
}

// scalarMetadataMetricToCoverageField mirrors the worker's
// `_SCALAR_METADATA_FIELDS` (analysis-worker/internal/metadata_metrics.py):
// metric_name → the coverage `field` name. Two names differ (the count metrics);
// the rest are identical. Kept in sync by hand — a tiny, rarely-changing config
// mirror, like the language manifest copied into the BFF image. If the worker
// adds a scalar-metadata metric, add the pair here so the dossier can mark it.
var scalarMetadataMetricToCoverageField = map[string]string{
	"paywall_status":          "paywall_status",
	"reading_time_minutes":    "reading_time_minutes",
	"comment_count":           "comment_count",
	"image_count":             "images",
	"external_citation_count": "external_citations",
}

// GetMetadataCoverage returns the per-source-per-field-per-method
// coverage cells for the requested source set. Reads
// `aer_gold.metadata_coverage`, the AggregatingMergeTree MV populated by
// the worker on every Silver write (migration 022). An empty `sources`
// slice returns an empty result — every coverage call is scoped, never
// global, so the BFF cannot accidentally return the entire corpus.
func (s *ClickHouseStorage) GetMetadataCoverage(ctx context.Context, sources []string) ([]MetadataCoverageCell, error) {
	if len(sources) == 0 {
		return nil, nil
	}
	const queryFmt = `
		SELECT
			source,
			field,
			method,
			uniqExactMerge(articles_state) AS articles,
			maxMerge(last_seen_state)      AS last_seen
		  FROM aer_gold.metadata_coverage
		 WHERE source IN (%s)
		 GROUP BY source, field, method
	`
	placeholders := make([]string, len(sources))
	args := make([]any, len(sources))
	for i, src := range sources {
		placeholders[i] = "?"
		args[i] = src
	}
	query := fmt.Sprintf(queryFmt, strings.Join(placeholders, ", "))

	var rows []MetadataCoverageCell
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("metadata coverage query: %w", err)
	}
	return rows, nil
}

// GetFieldCardinality returns the distinct-value count + a sample value per
// (source, coverage-field) over each source's whole corpus, for both categorical
// fields (aer_gold.article_metadata) and scalar-metadata fields (aer_gold.metrics,
// remapped to coverage field names). Task A: the dossier marks fields with
// Distinct == 1 as "constant → no signal" so a 100 %-populated-but-suppressed
// field is explained rather than confusing. Windowless to match the coverage MV's
// whole-corpus posture. An empty source set returns an empty map.
func (s *ClickHouseStorage) GetFieldCardinality(ctx context.Context, sources []string) (map[FieldKey]FieldCardinality, error) {
	out := map[FieldKey]FieldCardinality{}
	if len(sources) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(sources))
	srcArgs := make([]any, len(sources))
	for i, src := range sources {
		placeholders[i] = "?"
		srcArgs[i] = src
	}
	inClause := strings.Join(placeholders, ", ")

	// Categorical fields (one row per (article, field) with an Array(String)
	// `value`). FINAL mirrors the sibling distribution read so a transient
	// cross-version row never inflates the distinct count.
	catQuery := fmt.Sprintf(`
		SELECT Source, Field, uniqExact(v) AS Distinct, any(v) AS Value
		FROM (
			SELECT source AS Source, field AS Field, arrayJoin(value) AS v
			FROM aer_gold.article_metadata FINAL
			WHERE source IN (%s)
		)
		GROUP BY Source, Field
	`, inClause)
	var catRows []struct {
		Source   string `ch:"Source"`
		Field    string `ch:"Field"`
		Distinct uint64 `ch:"Distinct"`
		Value    string `ch:"Value"`
	}
	if err := s.conn.Select(ctx, &catRows, catQuery, srcArgs...); err != nil {
		return nil, fmt.Errorf("field cardinality (categorical): %w", err)
	}
	for _, r := range catRows {
		out[FieldKey{Source: r.Source, Field: r.Field}] = FieldCardinality{Distinct: r.Distinct, Value: r.Value}
	}

	// Scalar-metadata fields ride aer_gold.metrics under their metric_name;
	// remap to the coverage field name so the dossier marks by what it renders.
	metricNames := make([]string, 0, len(scalarMetadataMetricToCoverageField))
	for m := range scalarMetadataMetricToCoverageField {
		metricNames = append(metricNames, m)
	}
	metricPlaceholders := make([]string, len(metricNames))
	scalarArgs := make([]any, 0, len(sources)+len(metricNames))
	scalarArgs = append(scalarArgs, srcArgs...)
	for i, mn := range metricNames {
		metricPlaceholders[i] = "?"
		scalarArgs = append(scalarArgs, mn)
	}
	scalarQuery := fmt.Sprintf(`
		SELECT source AS Source, metric_name AS Metric, uniqExact(value) AS Distinct, any(value) AS Value
		FROM aer_gold.metrics
		WHERE source IN (%s) AND metric_name IN (%s)
		GROUP BY Source, Metric
	`, inClause, strings.Join(metricPlaceholders, ", "))
	var scalarRows []struct {
		Source   string  `ch:"Source"`
		Metric   string  `ch:"Metric"`
		Distinct uint64  `ch:"Distinct"`
		Value    float64 `ch:"Value"`
	}
	if err := s.conn.Select(ctx, &scalarRows, scalarQuery, scalarArgs...); err != nil {
		return nil, fmt.Errorf("field cardinality (scalar): %w", err)
	}
	for _, r := range scalarRows {
		field, ok := scalarMetadataMetricToCoverageField[r.Metric]
		if !ok {
			continue
		}
		out[FieldKey{Source: r.Source, Field: field}] = FieldCardinality{
			Distinct: r.Distinct,
			Value:    formatScalarConstant(r.Value),
		}
	}
	return out, nil
}

// formatScalarConstant renders a constant scalar value for disclosure: integers
// without decimals, otherwise trimmed. Honest — a constant 0 stays "0", never a
// fabricated "false" (the BFF has no per-metric boolean knowledge).
func formatScalarConstant(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// GlobalFieldStat is the corpus-wide (all-source) extraction + variance status
// for one Tier-B / Tier-C metadata field. Backs the Task-C Reflection surface.
type GlobalFieldStat struct {
	Field             string
	TotalArticles     int64
	PopulatedArticles int64
	PopulationRate    float64
	SourcesObserved   int
	SourcesPopulated  int
	DistinctValues    int64
	Constant          bool
	ConstantValue     string
}

// GetGlobalFieldStats aggregates the metadata-coverage matrix over EVERY source
// (no scope filter) into per-field corpus-wide fill statistics, plus a global
// distinct-value pass for the constant flag. Task C — the Reflection "metadata
// fields" page reports the real fill rate so its "most fields are empty by
// publisher choice, not by AĒR defect" claim is live, never a stale assertion.
func (s *ClickHouseStorage) GetGlobalFieldStats(ctx context.Context) ([]GlobalFieldStat, error) {
	var cells []MetadataCoverageCell
	if err := s.conn.Select(ctx, &cells, `
		SELECT
			source,
			field,
			method,
			uniqExactMerge(articles_state) AS articles,
			maxMerge(last_seen_state)      AS last_seen
		  FROM aer_gold.metadata_coverage
		 GROUP BY source, field, method
	`); err != nil {
		return nil, fmt.Errorf("global field coverage: %w", err)
	}

	cardinality, err := s.globalFieldCardinality(ctx)
	if err != nil {
		return nil, err
	}
	return aggregateGlobalFieldStats(AssembleCoverage(cells), cardinality), nil
}

// globalFieldCardinality returns the distinct-value count + sample value per
// coverage-field name over the WHOLE corpus (categorical from article_metadata,
// scalar metadata from metrics, remapped to coverage field names). Keyed by the
// coverage field name so it joins the coverage aggregate directly.
func (s *ClickHouseStorage) globalFieldCardinality(ctx context.Context) (map[string]FieldCardinality, error) {
	out := map[string]FieldCardinality{}

	var catRows []struct {
		Field    string `ch:"Field"`
		Distinct uint64 `ch:"Distinct"`
		Value    string `ch:"Value"`
	}
	if err := s.conn.Select(ctx, &catRows, `
		SELECT Field, uniqExact(v) AS Distinct, any(v) AS Value
		FROM (
			SELECT field AS Field, arrayJoin(value) AS v
			FROM aer_gold.article_metadata FINAL
		)
		GROUP BY Field
	`); err != nil {
		return nil, fmt.Errorf("global field cardinality (categorical): %w", err)
	}
	for _, r := range catRows {
		out[r.Field] = FieldCardinality{Distinct: r.Distinct, Value: r.Value}
	}

	metricNames := make([]string, 0, len(scalarMetadataMetricToCoverageField))
	for m := range scalarMetadataMetricToCoverageField {
		metricNames = append(metricNames, m)
	}
	placeholders := make([]string, len(metricNames))
	args := make([]any, len(metricNames))
	for i, mn := range metricNames {
		placeholders[i] = "?"
		args[i] = mn
	}
	scalarQuery := fmt.Sprintf(`
		SELECT metric_name AS Metric, uniqExact(value) AS Distinct, any(value) AS Value
		FROM aer_gold.metrics
		WHERE metric_name IN (%s)
		GROUP BY Metric
	`, strings.Join(placeholders, ", "))
	var scalarRows []struct {
		Metric   string  `ch:"Metric"`
		Distinct uint64  `ch:"Distinct"`
		Value    float64 `ch:"Value"`
	}
	if err := s.conn.Select(ctx, &scalarRows, scalarQuery, args...); err != nil {
		return nil, fmt.Errorf("global field cardinality (scalar): %w", err)
	}
	for _, r := range scalarRows {
		if field, ok := scalarMetadataMetricToCoverageField[r.Metric]; ok {
			out[field] = FieldCardinality{Distinct: r.Distinct, Value: formatScalarConstant(r.Value)}
		}
	}
	return out, nil
}

// aggregateGlobalFieldStats folds the per-source coverage summaries into one
// corpus-wide row per field and joins the global cardinality. Pure — exported
// for tests. A field is `constant` only when it is actually populated AND
// carries a single distinct value across the corpus.
func aggregateGlobalFieldStats(summaries []CoverageSourceSummary, cardinality map[string]FieldCardinality) []GlobalFieldStat {
	type acc struct {
		total, populated       int64
		observed, populatedSrc int
	}
	byField := map[string]*acc{}
	for _, src := range summaries {
		for _, f := range src.Fields {
			a, ok := byField[f.Field]
			if !ok {
				a = &acc{}
				byField[f.Field] = a
			}
			null := f.ByMethod[nullCoverageMethod]
			populated := f.TotalArticles - null
			if populated < 0 {
				populated = 0
			}
			a.total += f.TotalArticles
			a.populated += populated
			if f.TotalArticles > 0 {
				a.observed++
			}
			if populated > 0 {
				a.populatedSrc++
			}
		}
	}

	fields := make([]string, 0, len(byField))
	for f := range byField {
		fields = append(fields, f)
	}
	sort.Strings(fields)

	out := make([]GlobalFieldStat, 0, len(fields))
	for _, field := range fields {
		a := byField[field]
		var rate float64
		if a.total > 0 {
			rate = float64(a.populated) / float64(a.total)
		}
		stat := GlobalFieldStat{
			Field:             field,
			TotalArticles:     a.total,
			PopulatedArticles: a.populated,
			PopulationRate:    rate,
			SourcesObserved:   a.observed,
			SourcesPopulated:  a.populatedSrc,
		}
		if card, ok := cardinality[field]; ok {
			stat.DistinctValues = int64(card.Distinct) //nolint:gosec // bounded by field cardinality
			if card.Distinct == 1 && a.populated > 0 {
				stat.Constant = true
				stat.ConstantValue = card.Value
			}
		}
		out = append(out, stat)
	}
	return out
}

// Compile-time check that ClickHouseStorage implements the interface.
var _ MetadataCoverageQuerier = (*ClickHouseStorage)(nil)

// ----------------------------------------------------------------------
// Aggregation helpers (pure — exported for handler & tests).
// ----------------------------------------------------------------------

// CoverageFieldSummary is the assembled per-field record consumed by the
// BFF handler. The Phase 122f `structurallyAbsent` rule
// (≥ 50 articles observed AND 0 % populated) is computed here so the
// handler stays a thin marshalling layer over the storage result.
type CoverageFieldSummary struct {
	Field              string
	TotalArticles      int64
	ByMethod           map[string]int64
	PopulationRate     float64
	StructurallyAbsent bool
}

// CoverageSourceSummary is the per-source aggregate.
type CoverageSourceSummary struct {
	Name   string
	Fields []CoverageFieldSummary
}

// StructurallyAbsentMinSamples is the minimum article count over which
// 0 % population is read as "publisher's choice" rather than sampling
// variance (ROADMAP Phase 122f).
const StructurallyAbsentMinSamples = 50

// AssembleCoverage groups raw cells into per-source / per-field
// summaries, computes population rates, and flags structurally absent
// fields. The result is sorted deterministically (source name → field
// name) for stable client rendering.
func AssembleCoverage(cells []MetadataCoverageCell) []CoverageSourceSummary {
	type fieldKey struct {
		source string
		field  string
	}

	totals := map[fieldKey]int64{}
	byMethod := map[fieldKey]map[string]int64{}
	sourceFields := map[string]map[string]struct{}{}

	for _, c := range cells {
		key := fieldKey{source: c.Source, field: c.Field}
		count := int64(c.Articles) //nolint:gosec // bounded by the 30-day raw-table TTL
		totals[key] += count
		bm, ok := byMethod[key]
		if !ok {
			bm = make(map[string]int64)
			byMethod[key] = bm
		}
		bm[c.Method] = count
		sf, ok := sourceFields[c.Source]
		if !ok {
			sf = make(map[string]struct{})
			sourceFields[c.Source] = sf
		}
		sf[c.Field] = struct{}{}
	}

	sourceNames := make([]string, 0, len(sourceFields))
	for name := range sourceFields {
		sourceNames = append(sourceNames, name)
	}
	sort.Strings(sourceNames)

	out := make([]CoverageSourceSummary, 0, len(sourceNames))
	for _, name := range sourceNames {
		fieldNames := make([]string, 0, len(sourceFields[name]))
		for f := range sourceFields[name] {
			fieldNames = append(fieldNames, f)
		}
		sort.Strings(fieldNames)

		fields := make([]CoverageFieldSummary, 0, len(fieldNames))
		for _, fname := range fieldNames {
			key := fieldKey{source: name, field: fname}
			total := totals[key]
			bm := byMethod[key]
			nullCount := bm[nullCoverageMethod]

			var rate float64
			var absent bool
			if total > 0 {
				populated := total - nullCount
				if populated < 0 {
					populated = 0
				}
				rate = float64(populated) / float64(total)
			}
			if total >= StructurallyAbsentMinSamples && bm[nullCoverageMethod] == total {
				absent = true
			}

			fields = append(fields, CoverageFieldSummary{
				Field:              fname,
				TotalArticles:      total,
				ByMethod:           bm,
				PopulationRate:     rate,
				StructurallyAbsent: absent,
			})
		}
		out = append(out, CoverageSourceSummary{Name: name, Fields: fields})
	}
	return out
}
