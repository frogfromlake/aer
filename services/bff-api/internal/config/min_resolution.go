package config

// MinMeaningfulResolution is a static lookup of the finest temporal
// resolution at which a metric yields statistically meaningful aggregates.
//
// The values are seeded from the Probe 0 publication-rate heuristic
// described in WP-005 §3.3: tagesschau.de ≈ 50 articles/day, which
// makes hourly buckets the finest grain that consistently contains
// non-empty samples. The map is keyed by metric name; per-source
// granularity is deferred until /metrics/available exposes the source
// dimension.
//
// Returns an empty string when the metric has no recorded heuristic —
// the BFF surfaces this as a JSON null.
var MinMeaningfulResolution = map[string]string{
	"word_count":            "hourly",
	"sentiment_score":       "hourly",
	"entity_count":          "hourly",
	"language_confidence":   "hourly",
	"publication_hour":      "hourly",
	"publication_weekday":   "daily",
}

// LookupMinMeaningfulResolution returns the configured resolution for the
// given metric, or empty string if none is recorded.
func LookupMinMeaningfulResolution(metricName string) string {
	return MinMeaningfulResolution[metricName]
}
