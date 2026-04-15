package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// EntityRow represents an aggregated entity result from ClickHouse.
type EntityRow struct {
	EntityText  string
	EntityLabel string
	Count       uint64
	Sources     []string
}

// GetEntities retrieves aggregated named entities from the gold layer.
func (s *ClickHouseStorage) GetEntities(ctx context.Context, start, end time.Time, source, label *string, limit int) ([]EntityRow, error) {
	cacheKey := hotQueryKey("entities",
		start.UnixNano(), end.UnixNano(), derefString(source), derefString(label), limit)
	if cached, ok := s.entitiesCache.get(cacheKey, s.metricsCacheTTL); ok {
		return cached, nil
	}

	query := `
		SELECT
			entity_text as EntityText,
			entity_label as EntityLabel,
			count() as Count,
			groupArray(DISTINCT source) as Sources
		FROM aer_gold.entities
		WHERE timestamp >= $1 AND timestamp <= $2
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if label != nil {
		query += fmt.Sprintf(" AND entity_label = $%d", argIdx)
		args = append(args, *label)
	}

	// The ClickHouse Go driver (clickhouse-go/v2) does not support parameterized
	// LIMIT clauses via the $N positional syntax. limit is validated in the handler
	// layer (1–1000) before reaching this function.
	query += fmt.Sprintf(`
		GROUP BY EntityText, EntityLabel
		ORDER BY Count DESC
		LIMIT %d
	`, limit)

	var results []EntityRow
	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query entities from ClickHouse", "error", err)
		return nil, err
	}

	s.entitiesCache.put(cacheKey, results)
	return results, nil
}

// LanguageDetectionRow represents an aggregated language detection result from ClickHouse.
type LanguageDetectionRow struct {
	DetectedLanguage string
	Count            uint64
	AvgConfidence    float64
	Sources          []string
}

// GetLanguageDetections retrieves aggregated language detections from the gold layer.
// Only rank=1 (top candidate per document) detections are included.
func (s *ClickHouseStorage) GetLanguageDetections(ctx context.Context, start, end time.Time, source, language *string, limit int) ([]LanguageDetectionRow, error) {
	cacheKey := hotQueryKey("languages",
		start.UnixNano(), end.UnixNano(), derefString(source), derefString(language), limit)
	if cached, ok := s.languagesCache.get(cacheKey, s.metricsCacheTTL); ok {
		return cached, nil
	}

	query := `
		SELECT
			detected_language as DetectedLanguage,
			count() as Count,
			avg(confidence) as AvgConfidence,
			groupArray(DISTINCT source) as Sources
		FROM aer_gold.language_detections
		WHERE timestamp >= $1 AND timestamp <= $2
		  AND rank = 1
	`
	args := []any{start, end}
	argIdx := 3

	if source != nil {
		query += fmt.Sprintf(" AND source = $%d", argIdx)
		args = append(args, *source)
		argIdx++
	}
	if language != nil {
		query += fmt.Sprintf(" AND detected_language = $%d", argIdx)
		args = append(args, *language)
	}

	// The ClickHouse Go driver (clickhouse-go/v2) does not support parameterized
	// LIMIT clauses via the $N positional syntax. limit is validated in the handler
	// layer (1–1000) before reaching this function.
	query += fmt.Sprintf(`
		GROUP BY DetectedLanguage
		ORDER BY Count DESC
		LIMIT %d
	`, limit)

	var results []LanguageDetectionRow
	err := s.conn.Select(ctx, &results, query, args...)
	if err != nil {
		slog.Error("Failed to query language detections from ClickHouse", "error", err)
		return nil, err
	}

	s.languagesCache.put(cacheKey, results)
	return results, nil
}
