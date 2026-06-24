package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MetricProvenanceEntry holds the static methodological provenance for a
// single metric. Dynamic fields (validation_status, cultural_context_notes)
// are resolved at request time against ClickHouse.
//
// The prose fields (algorithm description, known limitations) carry an optional
// German variant (`*_de`); tier, extractor hash and validation status are
// locale-neutral machine facts. When a `_de` variant is absent the English text
// is served (graceful fallback), so partial translation degrades to English
// rather than to an empty surface — mirroring the content-catalogue EN-fallback
// (ADR-041/042).
type MetricProvenanceEntry struct {
	TierClassification     int      `yaml:"tier_classification"`
	AlgorithmDescription   string   `yaml:"algorithm_description"`
	AlgorithmDescriptionDe string   `yaml:"algorithm_description_de"`
	KnownLimitations       []string `yaml:"known_limitations"`
	KnownLimitationsDe     []string `yaml:"known_limitations_de"`
	ExtractorVersionHash   string   `yaml:"extractor_version_hash"`
}

// AlgorithmFor returns the algorithm description in the requested locale,
// falling back to English when no German variant is authored.
func (e MetricProvenanceEntry) AlgorithmFor(locale string) string {
	if locale == "de" && e.AlgorithmDescriptionDe != "" {
		return e.AlgorithmDescriptionDe
	}
	return e.AlgorithmDescription
}

// LimitationsFor returns the known-limitations list in the requested locale,
// falling back to English when no German variant is authored.
func (e MetricProvenanceEntry) LimitationsFor(locale string) []string {
	if locale == "de" && len(e.KnownLimitationsDe) > 0 {
		return e.KnownLimitationsDe
	}
	return e.KnownLimitations
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
