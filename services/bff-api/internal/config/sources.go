package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// SourceEntry mirrors a row of the PostgreSQL `sources` table plus the
// `documentation_url` column added in migration 000007.
type SourceEntry struct {
	Name             string  `yaml:"name"`
	Type             string  `yaml:"type"`
	URL              *string `yaml:"url"`
	DocumentationURL *string `yaml:"documentation_url"`
}

type sourceDocFile struct {
	Sources []SourceEntry `yaml:"sources"`
}

// LoadSources parses configs/source_documentation.yaml into a slice.
func LoadSources(path string) ([]SourceEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source documentation config: %w", err)
	}
	var f sourceDocFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse source documentation config: %w", err)
	}
	return f.Sources, nil
}
