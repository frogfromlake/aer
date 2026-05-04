// Package config — Phase 118a / ADR-024.
//
// Go reader for the Language Capability Manifest. The manifest is the
// single system-of-record for per-language analytical capability and is
// authored in services/analysis-worker/configs/language_capabilities.yaml.
// The BFF copy lives at /app/configs/language_capabilities.yaml in the
// runtime image (see services/bff-api/Dockerfile).
//
// The reader is intentionally minimal: it only parses the keys the BFF
// needs for the ?language= validator. Schema fields the worker uses
// (negation config, Tier-2 placeholders) are unmarshalled into typed
// structs but not surfaced through the public API.

package config

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v3"
)

// LanguageManifest is the parsed in-memory representation of the manifest
// as consumed by the BFF.
type LanguageManifest struct {
	ManifestVersion int
	Languages       map[string]LanguageManifestEntry
}

// LanguageManifestEntry holds the subset of per-language capability data
// the BFF currently surfaces. Field tags match the YAML keys so the
// manifest can be unmarshalled directly into this type.
type LanguageManifestEntry struct {
	IsoCode     string `yaml:"iso_code"`
	DisplayName string `yaml:"display_name"`
}

// LanguageCodes returns the sorted list of language codes declared by the
// manifest. Used as the alternatives list in the invalid_language refusal
// payload.
func (m *LanguageManifest) LanguageCodes() []string {
	out := make([]string, 0, len(m.Languages))
	for code := range m.Languages {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

// IsKnown returns true if the manifest declares the given (lower-cased)
// language code.
func (m *LanguageManifest) IsKnown(code string) bool {
	_, ok := m.Languages[code]
	return ok
}

// rawManifest mirrors the YAML shape the worker writes.
type rawManifest struct {
	ManifestVersion int                              `yaml:"manifest_version"`
	Languages       map[string]LanguageManifestEntry `yaml:"languages"`
}

// LoadLanguageManifest reads and validates the manifest at the given path.
//
// Returns an error if the file is missing, malformed, or declares an
// unsupported manifest_version. The BFF refuses to start in any of those
// cases, mirroring the worker's fatal-startup behaviour (ADR-024).
func LoadLanguageManifest(path string) (*LanguageManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read language manifest: %w", err)
	}
	var raw rawManifest
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse language manifest: %w", err)
	}
	if raw.ManifestVersion != 1 {
		return nil, fmt.Errorf(
			"language manifest: unsupported manifest_version %d (expected 1)",
			raw.ManifestVersion,
		)
	}
	if len(raw.Languages) == 0 {
		return nil, fmt.Errorf("language manifest: at least one language is required")
	}
	for code, entry := range raw.Languages {
		if entry.IsoCode == "" {
			return nil, fmt.Errorf("language manifest: %q is missing iso_code", code)
		}
	}
	return &LanguageManifest{
		ManifestVersion: raw.ManifestVersion,
		Languages:       raw.Languages,
	}, nil
}
