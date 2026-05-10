package storage

import (
	"context"
	"fmt"
	"sort"
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
