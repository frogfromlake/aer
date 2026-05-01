package handler

// metricNameAliases rewrites legacy/superseded metric names to their current
// canonical form at the request boundary. Phase 117 introduces the only
// current entry: `sentiment_score` was renamed to `sentiment_score_sentiws`
// to make ADR-016's dual-metric (Tier 1 / Tier 2) pattern lexically explicit.
// The alias is intentionally read-side only — once the cached dashboard URL
// state has rotated through one release cycle, the entry can be removed.
var metricNameAliases = map[string]string{
	"sentiment_score": "sentiment_score_sentiws",
}

// canonicalMetricName resolves an alias if present; otherwise returns the
// input unchanged. Empty strings pass through (the absence of the
// metricName parameter is meaningful elsewhere — it is not the same as the
// empty-string special case).
func canonicalMetricName(name string) string {
	if name == "" {
		return name
	}
	if v, ok := metricNameAliases[name]; ok {
		return v
	}
	return name
}

// canonicalMetricNamePtr rewrites *in place*. Safe on nil pointers and on
// pointers to empty strings.
func canonicalMetricNamePtr(p *string) {
	if p == nil || *p == "" {
		return
	}
	if v, ok := metricNameAliases[*p]; ok {
		*p = v
	}
}

// canonicalMetricNames returns a new slice with every entry alias-resolved.
// Used by the correlation endpoint, which takes a list of metric names.
func canonicalMetricNames(names []string) []string {
	if len(names) == 0 {
		return names
	}
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = canonicalMetricName(n)
	}
	return out
}
