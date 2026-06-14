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
	// WindowStart/WindowEnd carry the data window of the BERTopic sweep this
	// row came from. In LatestSweep mode every row shares the single newest
	// sweep's window, which the handler echoes so the UI can label the cell.
	WindowStart time.Time
	WindowEnd   time.Time
}

// TopicDistributionParams collects the query knobs from the BFF handler so
// the storage signature stays small as the endpoint grows new optional
// filters (Phase 120 + future iterations).
type TopicDistributionParams struct {
	Sources  []string
	Language *string
	Start    time.Time
	End      time.Time
	// LatestSweep selects ONLY the single newest sweep (max window_start) for
	// the scope instead of every sweep overlapping [Start, End]. The synchronic
	// topic_distribution cell uses this so it shows one coherent topic model
	// (BERTopic topic_ids are unique only within a sweep, so aggregating across
	// sweeps would conflate distinct topics and double-count articles). The
	// diachronic evolution view leaves it false and supplies explicit per-bucket
	// windows. When true, Start/End are ignored.
	LatestSweep    bool
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

	// Topic/scope filters shared by both modes.
	var args []any
	var clauses []string
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

	if params.LatestSweep {
		// Synchronic mode: pin to the single newest sweep for this scope so the
		// distribution reflects ONE coherent topic model. BERTopic topic_ids are
		// unique only within a sweep, so aggregating across sweeps would conflate
		// distinct topics and double-count articles.
		maxWS, err := s.latestTopicSweep(ctx, params)
		if err != nil {
			slog.Error("Failed to resolve latest topic sweep", "error", err)
			return nil, err
		}
		args = append(args, maxWS)
		clauses = append(clauses, fmt.Sprintf("window_start = $%d", len(args)))
	} else {
		// Diachronic / explicit-window mode (evolution view): every sweep whose
		// data window [window_start, window_end) overlaps the requested
		// [Start, End). Overlap (not `window_start >= Start`) so a sweep whose
		// 30-day window opens minutes before a same-shaped query window is not
		// silently dropped.
		args = append(args, params.End, params.Start)
		clauses = append(clauses,
			fmt.Sprintf("window_start < $%d", len(args)-1),
			fmt.Sprintf("window_end > $%d", len(args)),
		)
	}

	query := fmt.Sprintf(`
		SELECT
			topic_id,
			any(topic_label)              AS label,
			language,
			uniqExact(article_id)         AS article_count,
			avg(topic_confidence)         AS avg_confidence,
			argMax(model_hash, window_start) AS model_hash,
			any(window_start)             AS ws,
			any(window_end)               AS we
		FROM aer_gold.topic_assignments FINAL
		WHERE %s
		GROUP BY topic_id, language
		ORDER BY article_count DESC, language ASC, topic_id ASC
		LIMIT %d
	`, strings.Join(clauses, " AND "), params.Limit)

	var rows []struct {
		TopicID      int32     `ch:"topic_id"`
		Label        string    `ch:"label"`
		Language     string    `ch:"language"`
		ArticleCount uint64    `ch:"article_count"`
		AvgConf      float64   `ch:"avg_confidence"`
		ModelHash    string    `ch:"model_hash"`
		WS           time.Time `ch:"ws"`
		WE           time.Time `ch:"we"`
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
			WindowStart:  r.WS,
			WindowEnd:    r.WE,
		}
	}
	return out, nil
}

// latestTopicSweep returns the newest sweep's window_start for the given scope
// (source/language filters only). With no matching data ClickHouse returns the
// DateTime epoch, which the caller's pinned query then matches to zero rows.
func (s *ClickHouseStorage) latestTopicSweep(ctx context.Context, params TopicDistributionParams) (time.Time, error) {
	var args []any
	var clauses []string
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
	q := "SELECT max(window_start) FROM aer_gold.topic_assignments"
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	var ws time.Time
	if err := s.conn.QueryRow(ctx, q, args...).Scan(&ws); err != nil {
		return time.Time{}, err
	}
	return ws, nil
}
