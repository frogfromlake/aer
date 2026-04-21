package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// EmissionPoint mirrors the OpenAPI EmissionPoint schema. It is a
// geographic emission origin — explicitly not a reach claim.
type EmissionPoint struct {
	Latitude  float64 `yaml:"latitude"`
	Longitude float64 `yaml:"longitude"`
	Label     string  `yaml:"label"`
}

// ProbeEntry mirrors the OpenAPI Probe schema. It carries structural
// data only; editorial Dual-Register content is served separately
// through the content catalog.
type ProbeEntry struct {
	ProbeID        string          `yaml:"probeId"`
	Language       string          `yaml:"language"`
	EmissionPoints []EmissionPoint `yaml:"emissionPoints"`
	Sources        []string        `yaml:"sources"`
}

// ProbeRegistry is the in-memory registry keyed by probeId.
type ProbeRegistry map[string]ProbeEntry

// LoadProbeRegistry walks rootPath, parses every *.yaml file into a
// ProbeEntry, validates it, and returns the populated registry. A
// malformed or invalid file aborts startup so a broken probe does not
// silently vanish from the Atmosphere surface.
func LoadProbeRegistry(rootPath string) (ProbeRegistry, error) {
	registry := make(ProbeRegistry)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("probe registry: reading %s: %w", path, readErr)
		}

		var entry ProbeEntry
		if parseErr := yaml.Unmarshal(data, &entry); parseErr != nil {
			return fmt.Errorf("probe registry: parsing %s: %w", path, parseErr)
		}

		if validateErr := validateProbeEntry(entry, path); validateErr != nil {
			return validateErr
		}

		if _, exists := registry[entry.ProbeID]; exists {
			return fmt.Errorf("probe registry: duplicate probeId %q (file: %s)", entry.ProbeID, path)
		}
		registry[entry.ProbeID] = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	return registry, nil
}

// Ordered returns the probes in a deterministic order (by probeId) so the
// /probes response is stable across restarts — important for cache keys
// and visual-regression tests.
func (r ProbeRegistry) Ordered() []ProbeEntry {
	ids := make([]string, 0, len(r))
	for id := range r {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ProbeEntry, 0, len(ids))
	for _, id := range ids {
		out = append(out, r[id])
	}
	return out
}

func validateProbeEntry(e ProbeEntry, path string) error {
	loc := func(msg string) error {
		return fmt.Errorf("probe registry: %s (file: %s)", msg, path)
	}
	if e.ProbeID == "" {
		return loc("probeId is required")
	}
	if e.Language == "" {
		return loc("language is required")
	}
	if len(e.Sources) == 0 {
		return loc("sources must list at least one source")
	}
	if len(e.EmissionPoints) == 0 {
		return loc("emissionPoints must list at least one point")
	}
	for i, p := range e.EmissionPoints {
		if p.Latitude < -90 || p.Latitude > 90 {
			return loc(fmt.Sprintf("emissionPoints[%d].latitude out of range [-90, 90]", i))
		}
		if p.Longitude < -180 || p.Longitude > 180 {
			return loc(fmt.Sprintf("emissionPoints[%d].longitude out of range [-180, 180]", i))
		}
		if p.Label == "" {
			return loc(fmt.Sprintf("emissionPoints[%d].label is required", i))
		}
	}
	return nil
}
