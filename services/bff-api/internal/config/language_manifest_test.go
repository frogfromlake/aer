package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeManifest writes a manifest YAML into dir and returns its path. The body
// mirrors the real services/analysis-worker/configs/language_capabilities.yaml
// shape (manifest_version + per-language tier blocks), trimmed to the keys the
// BFF reader surfaces.
func writeManifest(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "language_capabilities.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

// validManifestBody is a two-language manifest: `de` carries a Tier-2 default
// plus a Tier-2.5 refinement (so it exercises both the backbone and the
// enrichment path); `fr` carries only Tier-1, so its backbone falls back to the
// `lexicon:` label and it has no enrichments.
const validManifestBody = `
manifest_version: 1
languages:
  de:
    iso_code: de
    display_name: German
    sentiment_tier1:
      method: lexicon
      lexicon: sentiws_v2.0
      metric_name: sentiment_score_sentiws
    sentiment_tier2_default:
      method: multilingual_bert
      metric_name: sentiment_score_bert_multilingual
    sentiment_tier2_refinement:
      method: news_domain_bert
      metric_name: sentiment_score_bert_de_news
  fr:
    iso_code: fr
    display_name: French
    sentiment_tier1:
      method: lexicon
      lexicon: feel_v1
      metric_name: sentiment_score_feel
`

func TestLoadLanguageManifest_ValidLoadsBothLanguages(t *testing.T) {
	path := writeManifest(t, t.TempDir(), validManifestBody)

	m, err := LoadLanguageManifest(path)
	if err != nil {
		t.Fatalf("LoadLanguageManifest: %v", err)
	}
	if m.ManifestVersion != 1 {
		t.Errorf("ManifestVersion = %d, want 1", m.ManifestVersion)
	}
	if len(m.Languages) != 2 {
		t.Fatalf("expected 2 languages, got %d", len(m.Languages))
	}
	de := m.Languages["de"]
	if de.IsoCode != "de" || de.DisplayName != "German" {
		t.Errorf("de entry = %+v", de)
	}
}

func TestLoadLanguageManifest_FileNotFound(t *testing.T) {
	_, err := LoadLanguageManifest(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "read language manifest") {
		t.Errorf("error = %q, want 'read language manifest' wrap", err)
	}
}

func TestLoadLanguageManifest_MalformedYAML(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "languages: [this is: not valid: mapping")
	_, err := LoadLanguageManifest(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
	if !strings.Contains(err.Error(), "parse language manifest") {
		t.Errorf("error = %q, want 'parse language manifest' wrap", err)
	}
}

func TestLoadLanguageManifest_UnsupportedVersion(t *testing.T) {
	path := writeManifest(t, t.TempDir(), `
manifest_version: 2
languages:
  de:
    iso_code: de
`)
	_, err := LoadLanguageManifest(path)
	if err == nil {
		t.Fatal("expected error for unsupported manifest_version")
	}
	if !strings.Contains(err.Error(), "unsupported manifest_version") {
		t.Errorf("error = %q, want 'unsupported manifest_version'", err)
	}
}

func TestLoadLanguageManifest_RejectsEmptyLanguages(t *testing.T) {
	path := writeManifest(t, t.TempDir(), "manifest_version: 1\nlanguages: {}\n")
	_, err := LoadLanguageManifest(path)
	if err == nil {
		t.Fatal("expected error for empty languages map")
	}
	if !strings.Contains(err.Error(), "at least one language is required") {
		t.Errorf("error = %q", err)
	}
}

func TestLoadLanguageManifest_RejectsMissingIsoCode(t *testing.T) {
	path := writeManifest(t, t.TempDir(), `
manifest_version: 1
languages:
  de:
    display_name: German
`)
	_, err := LoadLanguageManifest(path)
	if err == nil {
		t.Fatal("expected error for missing iso_code")
	}
	if !strings.Contains(err.Error(), "missing iso_code") {
		t.Errorf("error = %q, want 'missing iso_code'", err)
	}
}

// TestLoadLanguageManifest_RealFile loads the actual worker manifest to guard
// the BFF reader against drift in the system-of-record file shape.
func TestLoadLanguageManifest_RealFile(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "services", "analysis-worker", "configs", "language_capabilities.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("real manifest not present: %v", err)
	}
	m, err := LoadLanguageManifest(path)
	if err != nil {
		t.Fatalf("LoadLanguageManifest(real): %v", err)
	}
	if !m.IsKnown("de") {
		t.Error("real manifest must declare 'de'")
	}
}

func TestSentimentBackbone(t *testing.T) {
	cases := []struct {
		name  string
		entry LanguageManifestEntry
		want  string
	}{
		{
			name: "prefers tier2 default metric name",
			entry: LanguageManifestEntry{
				SentimentTier1:        manifestSentimentTier{Lexicon: "sentiws_v2.0"},
				SentimentTier2Default: manifestSentimentTier{MetricName: "sentiment_score_bert_multilingual"},
			},
			want: "sentiment_score_bert_multilingual",
		},
		{
			name: "falls back to lexicon label when no tier2",
			entry: LanguageManifestEntry{
				SentimentTier1: manifestSentimentTier{Lexicon: "feel_v1"},
			},
			want: "lexicon:feel_v1",
		},
		{
			name:  "empty when no sentiment capability",
			entry: LanguageManifestEntry{},
			want:  "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.entry.SentimentBackbone(); got != c.want {
				t.Errorf("SentimentBackbone() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSentimentEnrichments(t *testing.T) {
	withRefine := LanguageManifestEntry{
		SentimentTier2Refine: manifestSentimentTier{MetricName: "sentiment_score_bert_de_news"},
	}
	if got := withRefine.SentimentEnrichments(); !reflect.DeepEqual(got, []string{"sentiment_score_bert_de_news"}) {
		t.Errorf("SentimentEnrichments() = %v, want [sentiment_score_bert_de_news]", got)
	}

	none := LanguageManifestEntry{}
	got := none.SentimentEnrichments()
	if len(got) != 0 {
		t.Errorf("SentimentEnrichments() = %v, want empty", got)
	}
	if got == nil {
		t.Error("SentimentEnrichments() must return a non-nil empty slice")
	}
}

func TestLanguageCodes_SortedAndComplete(t *testing.T) {
	m := &LanguageManifest{Languages: map[string]LanguageManifestEntry{
		"fr": {IsoCode: "fr"},
		"de": {IsoCode: "de"},
		"en": {IsoCode: "en"},
	}}
	got := m.LanguageCodes()
	want := []string{"de", "en", "fr"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LanguageCodes() = %v, want %v", got, want)
	}
}

func TestLanguageCodes_EmptyManifest(t *testing.T) {
	m := &LanguageManifest{Languages: map[string]LanguageManifestEntry{}}
	if got := m.LanguageCodes(); len(got) != 0 {
		t.Errorf("LanguageCodes() = %v, want empty", got)
	}
}

func TestIsKnown(t *testing.T) {
	m := &LanguageManifest{Languages: map[string]LanguageManifestEntry{
		"de": {IsoCode: "de"},
	}}
	if !m.IsKnown("de") {
		t.Error("IsKnown(de) = false, want true")
	}
	if m.IsKnown("xx") {
		t.Error("IsKnown(xx) = true, want false")
	}
	if m.IsKnown("") {
		t.Error("IsKnown(empty) = true, want false")
	}
}
