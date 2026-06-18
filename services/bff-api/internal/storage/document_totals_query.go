package storage

import (
	"context"
	"log/slog"
)

// GetDocumentTotalsBySource returns the all-time distinct-document count per
// source over aer_silver.documents (count(DISTINCT article_id)). It is the
// dataset total — deliberately NOT window-scoped — and uses the same definition
// the Probe Dossier reports as `articlesTotal`. Sources with no processed
// documents are simply absent from the map (the caller treats absent as 0). An
// empty `sources` slice short-circuits to an empty map with no query.
//
// Backs the Probe.documentCount field that drives the Atmosphere
// dataset-overview readout (Design Brief §4.1).
func (s *ClickHouseStorage) GetDocumentTotalsBySource(ctx context.Context, sources []string) (map[string]int64, error) {
	out := make(map[string]int64, len(sources))
	if len(sources) == 0 {
		return out, nil
	}
	const query = `
		SELECT source, count(DISTINCT article_id) AS total
		FROM aer_silver.documents FINAL
		WHERE source IN ?
		GROUP BY source
	`
	var rows []struct {
		Source string `ch:"source"`
		Total  uint64 `ch:"total"`
	}
	if err := s.conn.Select(ctx, &rows, query, sources); err != nil {
		slog.Error("Failed to count documents by source", "error", err)
		return nil, err
	}
	for _, r := range rows {
		out[r.Source] = int64(r.Total) //nolint:gosec // document counts are bounded well within int64
	}
	return out, nil
}
