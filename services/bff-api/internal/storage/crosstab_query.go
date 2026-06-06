package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// CrossTabBucket is one categorical value of the field with the per-article
// aggregate of a metric over the articles in that category (Phase 125).
type CrossTabBucket struct {
	Value    string  `ch:"Value"`
	Articles uint64  `ch:"Articles"`
	Mean     float64 `ch:"Mean"`
	Std      float64 `ch:"Std"`
}

// CrossTabResult is the top-N cross-tab of a categorical metadata FIELD against
// a numeric METRIC: for each category value, how many distinct articles carry
// both, and the mean/spread of the metric among them. DistinctValues drives the
// honest "showing top N of M" disclosure (no silent truncation).
type CrossTabResult struct {
	Buckets        []CrossTabBucket
	DistinctValues int64
}

// GetCrossTab joins aer_gold.article_metadata (the categorical FIELD, expanded
// via arrayJoin) with aer_gold.metrics (the numeric METRIC, pivoted per article)
// on article_id, then aggregates the metric per category value.
//
// FINAL on the metadata side (exact distinct-article counts; the documented
// exception, mirroring GetCategoricalDistribution); the metrics side omits FINAL
// (avg tolerates transient re-ingest skew, the established convention). An empty
// field/metric/source set returns an empty result (the handler resolves scope).
//
// Caveat (disclosed in the cell's how-to-read): for a LIST field an article may
// carry the same value more than once; uniqExact keeps Articles a distinct count,
// but the mean is over value-occurrences — negligible for scalar fields, where
// each article contributes once.
func (s *ClickHouseStorage) GetCrossTab(
	ctx context.Context,
	field, metric string,
	sources []string,
	start, end time.Time,
	topN int,
	metadataFilter *MetadataFilter,
) (CrossTabResult, error) {
	out := CrossTabResult{Buckets: []CrossTabBucket{}}
	if field == "" || metric == "" || len(sources) == 0 {
		return out, nil
	}
	if topN < 1 {
		topN = 1
	}
	if topN > 200 {
		topN = 200
	}

	// Positional args (window + sources appear in both subqueries, so they are
	// bound twice — ClickHouse positional params are not deduplicated).
	sa := newScopeArgs()
	// Faceting (Phase 125a): narrow the metadata side to facet-matching articles.
	// The INNER JOIN on article_id propagates the restriction to the metric side.
	metaWhere := fmt.Sprintf(
		"field = %s AND timestamp >= %s AND timestamp < %s AND source IN (%s)",
		sa.ph(field), sa.ph(start), sa.ph(end), sa.srcIn(sources),
	) + sa.metadataFilterClause(metadataFilter, start, end, sources)
	metricWhere := fmt.Sprintf(
		"metric_name = %s AND timestamp >= %s AND timestamp < %s AND source IN (%s) AND article_id IS NOT NULL AND article_id != ''",
		sa.ph(metric), sa.ph(start), sa.ph(end), sa.srcIn(sources),
	)

	query := fmt.Sprintf(`
		SELECT am.v AS Value,
		       uniqExact(am.article_id) AS Articles,
		       avg(mv.MetricValue) AS Mean,
		       stddevSamp(mv.MetricValue) AS Std
		FROM (
			SELECT article_id, arrayJoin(value) AS v
			FROM aer_gold.article_metadata FINAL
			WHERE %s
		) am
		INNER JOIN (
			SELECT article_id, avg(value) AS MetricValue
			FROM aer_gold.metrics
			WHERE %s
			GROUP BY article_id
		) mv ON am.article_id = mv.article_id
		GROUP BY Value
		ORDER BY Articles DESC, Value ASC
	`, metaWhere, metricWhere)

	var rows []CrossTabBucket
	if err := s.conn.Select(ctx, &rows, query, sa.Args...); err != nil {
		slog.Error("Failed to query cross-tab", "error", err, "field", field, "metric", metric)
		return out, err
	}

	out.DistinctValues = int64(len(rows))
	if len(rows) > topN {
		rows = rows[:topN]
	}
	out.Buckets = rows
	return out, nil
}
