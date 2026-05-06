package storage

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// TopicDistributionRow is one entry in a topic-distribution response —
// keyed by (language, topic_id) because BERTopic is fit per language
// partition (Phase 120 / WP-004 §3.4).
type TopicDistributionRow struct {
	TopicID      int32
	Label        string
	Language     string
	ArticleCount int64
	AvgConf      float64
	ModelHash    string
}

// TopicDistributionParams collects the query knobs from the BFF handler so
// the storage signature stays small as the endpoint grows new optional
// filters (Phase 120 + future iterations).
type TopicDistributionParams struct {
	Sources        []string
	Language       *string
	Start          time.Time
	End            time.Time
	MinConfidence  float32
	IncludeOutlier bool
	Limit          int
}

// GetTopicDistribution aggregates aer_gold.topic_assignments over a window
// restricted to the resolved scope, returning the top-N (topic_id, language)
// rows by distinct article count. The outlier class (-1) is filtered out by
// default; callers requesting `includeOutlier=true` get it back as a
// regular row that the BFF re-labels to "uncategorised" per Phase 121.
func (s *ClickHouseStorage) GetTopicDistribution(
	ctx context.Context,
	params TopicDistributionParams,
) ([]TopicDistributionRow, error) {
	if params.Limit <= 0 {
		params.Limit = 50
	}
	if params.Limit > 200 {
		params.Limit = 200
	}

	// Overlap semantics — a topic sweep is in scope when its data window
	// [window_start, window_end) overlaps the requested [Start, End).
	// The earlier `window_start >= Start AND window_start < End` form
	// silently dropped sweeps whose 30-day data window opens slightly
	// before a same-shaped query window — e.g. a sweep started at
	// 23:45:36 with window_start=now-30d is invisible to a query made
	// 6 minutes later with start=now-30d, because the sweep's window_start
	// is 6 minutes "in the past" relative to the query's start.
	args := []any{params.End, params.Start}
	clauses := []string{
		"window_start < $1",
		"window_end > $2",
	}
	if !params.IncludeOutlier {
		clauses = append(clauses, "topic_id != -1")
	}
	if params.MinConfidence > 0 {
		args = append(args, params.MinConfidence)
		clauses = append(clauses, fmt.Sprintf("topic_confidence >= $%d", len(args)))
	}
	if len(params.Sources) > 0 {
		placeholders := make([]string, len(params.Sources))
		for i, src := range params.Sources {
			args = append(args, src)
			placeholders[i] = fmt.Sprintf("$%d", len(args))
		}
		clauses = append(clauses, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ", ")))
	}
	if params.Language != nil && *params.Language != "" {
		args = append(args, *params.Language)
		clauses = append(clauses, fmt.Sprintf("language = $%d", len(args)))
	}

	query := fmt.Sprintf(`
		SELECT
			topic_id,
			any(topic_label)              AS label,
			language,
			uniqExact(article_id)         AS article_count,
			avg(topic_confidence)         AS avg_confidence,
			argMax(model_hash, window_start) AS model_hash
		FROM aer_gold.topic_assignments FINAL
		WHERE %s
		GROUP BY topic_id, language
		ORDER BY article_count DESC, language ASC, topic_id ASC
		LIMIT %d
	`, strings.Join(clauses, " AND "), params.Limit)

	var rows []struct {
		TopicID      int32   `ch:"topic_id"`
		Label        string  `ch:"label"`
		Language     string  `ch:"language"`
		ArticleCount uint64  `ch:"article_count"`
		AvgConf      float64 `ch:"avg_confidence"`
		ModelHash    string  `ch:"model_hash"`
	}
	if err := s.conn.Select(ctx, &rows, query, args...); err != nil {
		slog.Error("Failed to query topic distribution", "error", err)
		return nil, err
	}

	out := make([]TopicDistributionRow, len(rows))
	for i, r := range rows {
		out[i] = TopicDistributionRow{
			TopicID:      r.TopicID,
			Label:        r.Label,
			Language:     r.Language,
			ArticleCount: int64(r.ArticleCount), //nolint:gosec // bounded by aggregation
			AvgConf:      r.AvgConf,
			ModelHash:    r.ModelHash,
		}
	}
	return out, nil
}
