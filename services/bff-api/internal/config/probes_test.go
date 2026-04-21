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
probeId: probe-0-de-institutional-rss
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
	p := registry["probe-0-de-institutional-rss"]
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
