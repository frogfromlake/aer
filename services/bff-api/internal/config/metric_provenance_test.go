package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeProvenance writes a metric-provenance YAML into dir and returns its path.
// The body mirrors services/bff-api/configs/metric_provenance.yaml: a top-level
// map keyed by metric name, each value a MetricProvenanceEntry.
func writeProvenance(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "metric_provenance.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write provenance: %v", err)
	}
	return path
}

func TestLoadMetricProvenance_Valid(t *testing.T) {
	path := writeProvenance(t, t.TempDir(), `
word_count:
  tier_classification: 1
  algorithm_description: "Deterministic whitespace token count."
  known_limitations:
    - "Whitespace tokenisation fails on logographic languages."
  extractor_version_hash: v1
sentiment_score_sentiws:
  tier_classification: 1
  algorithm_description: "Lexicon-based polarity score using SentiWS v2.0."
  known_limitations:
    - "Negation blindness."
  extractor_version_hash: v2
`)

	m, err := LoadMetricProvenance(path)
	if err != nil {
		t.Fatalf("LoadMetricProvenance: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(m))
	}
	wc := m["word_count"]
	if wc.TierClassification != 1 {
		t.Errorf("word_count tier = %d, want 1", wc.TierClassification)
	}
	if len(wc.KnownLimitations) != 1 {
		t.Errorf("word_count limitations = %v", wc.KnownLimitations)
	}
	if wc.ExtractorVersionHash != "v1" {
		t.Errorf("word_count hash = %q", wc.ExtractorVersionHash)
	}
}

// TestLoadMetricProvenance_NilLimitationsBecomeEmptySlice verifies the
// normalisation branch: a metric with no known_limitations key yields a
// non-nil empty slice (so the JSON serialises to [] not null).
func TestLoadMetricProvenance_NilLimitationsBecomeEmptySlice(t *testing.T) {
	path := writeProvenance(t, t.TempDir(), `
some_metric:
  tier_classification: 2
  algorithm_description: "x"
  extractor_version_hash: v1
`)
	m, err := LoadMetricProvenance(path)
	if err != nil {
		t.Fatalf("LoadMetricProvenance: %v", err)
	}
	entry := m["some_metric"]
	if entry.KnownLimitations == nil {
		t.Fatal("KnownLimitations must be a non-nil empty slice, got nil")
	}
	if len(entry.KnownLimitations) != 0 {
		t.Errorf("KnownLimitations = %v, want empty", entry.KnownLimitations)
	}
}

func TestLoadMetricProvenance_FileNotFound(t *testing.T) {
	_, err := LoadMetricProvenance(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read metric provenance config") {
		t.Errorf("error = %q", err)
	}
}

func TestLoadMetricProvenance_MalformedYAML(t *testing.T) {
	path := writeProvenance(t, t.TempDir(), "word_count: [oops: not: a mapping")
	_, err := LoadMetricProvenance(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parse metric provenance config") {
		t.Errorf("error = %q", err)
	}
}

func TestLoadMetricProvenance_RejectsTierOutOfRange(t *testing.T) {
	cases := map[string]string{
		"tier zero": `
m:
  tier_classification: 0
  extractor_version_hash: v1
`,
		"tier four": `
m:
  tier_classification: 4
  extractor_version_hash: v1
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeProvenance(t, t.TempDir(), body)
			_, err := LoadMetricProvenance(path)
			if err == nil {
				t.Fatal("expected error for out-of-range tier_classification")
			}
			if !strings.Contains(err.Error(), "tier_classification must be 1, 2, or 3") {
				t.Errorf("error = %q", err)
			}
		})
	}
}

// TestLoadMetricProvenance_RealFile loads the bundled BFF config to guard
// against drift between the reader and the shipped YAML.
func TestLoadMetricProvenance_RealFile(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "services", "bff-api", "configs", "metric_provenance.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("real provenance not present: %v", err)
	}
	m, err := LoadMetricProvenance(path)
	if err != nil {
		t.Fatalf("LoadMetricProvenance(real): %v", err)
	}
	if _, ok := m["word_count"]; !ok {
		t.Error("real provenance must declare word_count")
	}
}

func TestLookupMinMeaningfulResolution(t *testing.T) {
	cases := []struct {
		metric string
		want   string
	}{
		{"word_count", "hourly"},
		{"sentiment_score", "hourly"},
		{"entity_count", "hourly"},
		{"language_confidence", "hourly"},
		{"publication_hour", "hourly"},
		{"publication_weekday", "daily"},
		{"unknown_metric", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := LookupMinMeaningfulResolution(c.metric); got != c.want {
			t.Errorf("LookupMinMeaningfulResolution(%q) = %q, want %q", c.metric, got, c.want)
		}
	}
}
