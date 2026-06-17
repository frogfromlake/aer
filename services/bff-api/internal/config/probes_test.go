package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProbeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestLoadProbeRegistry_LoadsValidProbe(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "probe-0.yaml", `
probeId: probe-0-de-institutional-web
language: de
sources: [tagesschau, bundesregierung]
emissionPoints:
  - latitude: 53.5511
    longitude: 9.9937
    label: "Hamburg"
  - latitude: 52.5170
    longitude: 13.3888
    label: "Berlin"
`)

	registry, err := LoadProbeRegistry(dir)
	if err != nil {
		t.Fatalf("LoadProbeRegistry: %v", err)
	}
	if len(registry) != 1 {
		t.Fatalf("expected 1 probe, got %d", len(registry))
	}
	p := registry["probe-0-de-institutional-web"]
	if p.Language != "de" {
		t.Errorf("language: %s", p.Language)
	}
	if len(p.EmissionPoints) != 2 {
		t.Errorf("emission points: %d", len(p.EmissionPoints))
	}
	if len(p.Sources) != 2 {
		t.Errorf("sources: %d", len(p.Sources))
	}
}

func TestLoadProbeRegistry_RejectsMissingProbeID(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
language: de
sources: [x]
emissionPoints:
  - latitude: 0
    longitude: 0
    label: "x"
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for missing probeId")
	}
}

func TestLoadProbeRegistry_RejectsOutOfRangeCoordinates(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
language: de
sources: [x]
emissionPoints:
  - latitude: 95
    longitude: 0
    label: "x"
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for latitude out of range")
	}
}

func TestLoadProbeRegistry_RejectsEmptyEmissionPoints(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
language: de
sources: [x]
emissionPoints: []
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for empty emissionPoints")
	}
}

func TestLoadProbeRegistry_RejectsDuplicateProbeID(t *testing.T) {
	dir := t.TempDir()
	body := `
probeId: dup
language: de
sources: [x]
emissionPoints:
  - latitude: 0
    longitude: 0
    label: "x"
`
	writeProbeYAML(t, dir, "a.yaml", body)
	writeProbeYAML(t, dir, "b.yaml", body)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected duplicate-probeId error")
	}
}

func TestProbeRegistry_OrderedIsStable(t *testing.T) {
	r := ProbeRegistry{
		"b": ProbeEntry{ProbeID: "b"},
		"a": ProbeEntry{ProbeID: "a"},
		"c": ProbeEntry{ProbeID: "c"},
	}
	ordered := r.Ordered()
	if ordered[0].ProbeID != "a" || ordered[1].ProbeID != "b" || ordered[2].ProbeID != "c" {
		t.Errorf("not sorted: %+v", ordered)
	}
}

func TestProbeEntry_CorpusClass(t *testing.T) {
	cases := map[string]string{
		"probe-0-de-institutional-web": "institutional-web",
		"probe-1-fr-institutional-web": "institutional-web",
		"probe-12-en-social-twitter":   "social-twitter",
		"malformed":                    "",
		"probe-0-de":                   "",
	}
	for id, want := range cases {
		if got := (ProbeEntry{ProbeID: id}).CorpusClass(); got != want {
			t.Errorf("CorpusClass(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestProbeEntry_Display(t *testing.T) {
	withName := ProbeEntry{ProbeID: "probe-0-de-institutional-web", DisplayName: "German Institutional Web"}
	if got := withName.Display(); got != "German Institutional Web" {
		t.Errorf("Display() = %q, want the displayName", got)
	}
	fallback := ProbeEntry{ProbeID: "probe-0-de-institutional-web"}
	if got := fallback.Display(); got != "probe-0-de-institutional-web" {
		t.Errorf("Display() = %q, want fallback to probeId", got)
	}
}

func TestProbeEntry_Short(t *testing.T) {
	withShort := ProbeEntry{ProbeID: "p", DisplayName: "Long Name", ShortName: "Short"}
	if got := withShort.Short(); got != "Short" {
		t.Errorf("Short() = %q, want the shortName", got)
	}
	// No shortName ⇒ fall back to Display() (the displayName).
	noShort := ProbeEntry{ProbeID: "p", DisplayName: "Long Name"}
	if got := noShort.Short(); got != "Long Name" {
		t.Errorf("Short() = %q, want fallback to displayName", got)
	}
	// Neither shortName nor displayName ⇒ fall all the way back to probeId.
	bare := ProbeEntry{ProbeID: "p"}
	if got := bare.Short(); got != "p" {
		t.Errorf("Short() = %q, want fallback to probeId", got)
	}
}

func TestLoadProbeRegistry_RejectsMissingLanguage(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
sources: [x]
emissionPoints:
  - latitude: 0
    longitude: 0
    label: "x"
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for missing language")
	}
}

func TestLoadProbeRegistry_RejectsEmptySources(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
language: de
sources: []
emissionPoints:
  - latitude: 0
    longitude: 0
    label: "x"
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for empty sources")
	}
}

func TestLoadProbeRegistry_RejectsOutOfRangeLongitude(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
language: de
sources: [x]
emissionPoints:
  - latitude: 0
    longitude: 200
    label: "x"
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for longitude out of range")
	}
}

func TestLoadProbeRegistry_RejectsMissingLabel(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", `
probeId: x
language: de
sources: [x]
emissionPoints:
  - latitude: 0
    longitude: 0
`)
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected error for missing emission-point label")
	}
}

func TestLoadProbeRegistry_RejectsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "bad.yaml", "probeId: [unterminated")
	if _, err := LoadProbeRegistry(dir); err == nil {
		t.Fatal("expected parse error for malformed YAML")
	}
}

func TestLoadProbeRegistry_IgnoresNonYAMLFiles(t *testing.T) {
	dir := t.TempDir()
	writeProbeYAML(t, dir, "probe-0.yaml", `
probeId: probe-0-de-institutional-web
language: de
sources: [tagesschau]
emissionPoints:
  - latitude: 52.5
    longitude: 13.4
    label: "Berlin"
`)
	// A non-YAML sibling must be skipped, not parsed.
	writeProbeYAML(t, dir, "README.md", "not yaml")

	registry, err := LoadProbeRegistry(dir)
	if err != nil {
		t.Fatalf("LoadProbeRegistry: %v", err)
	}
	if len(registry) != 1 {
		t.Fatalf("expected 1 probe (non-yaml ignored), got %d", len(registry))
	}
}

func TestLoadProbeRegistry_RejectsMissingRoot(t *testing.T) {
	if _, err := LoadProbeRegistry(filepath.Join(t.TempDir(), "no-such-dir")); err == nil {
		t.Fatal("expected error walking a non-existent root")
	}
}

func TestIsPublicCorpusClass(t *testing.T) {
	if !IsPublicCorpusClass("institutional-web") {
		t.Error("institutional-web must be a public corpus class (k-anon exempt)")
	}
	if IsPublicCorpusClass("social-twitter") {
		t.Error("social-twitter must NOT be public — k-anon gate stays enforced")
	}
	if IsPublicCorpusClass("") {
		t.Error("empty corpus class must NOT be treated as public (fail safe)")
	}
}
