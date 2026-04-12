package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MetricProvenanceEntry holds the static methodological provenance for a
// single metric. Dynamic fields (validation_status, cultural_context_notes)
// are resolved at request time against ClickHouse.
type MetricProvenanceEntry struct {
	TierClassification   int      `yaml:"tier_classification"`
	AlgorithmDescription string   `yaml:"algorithm_description"`
	KnownLimitations     []string `yaml:"known_limitations"`
	ExtractorVersionHash string   `yaml:"extractor_version_hash"`
}

// MetricProvenanceMap is the parsed in-memory representation of
// configs/metric_provenance.yaml, keyed by metric name.
type MetricProvenanceMap map[string]MetricProvenanceEntry

// LoadMetricProvenance reads and parses the provenance config at the given path.
func LoadMetricProvenance(path string) (MetricProvenanceMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read metric provenance config: %w", err)
	}
	var m MetricProvenanceMap
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse metric provenance config: %w", err)
	}
	for name, entry := range m {
		if entry.TierClassification < 1 || entry.TierClassification > 3 {
			return nil, fmt.Errorf("metric %q: tier_classification must be 1, 2, or 3", name)
		}
		if entry.KnownLimitations == nil {
			entry.KnownLimitations = []string{}
			m[name] = entry
		}
	}
	return m, nil
}
