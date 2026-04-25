package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ContentRegister holds both text variants for one register.
type ContentRegister struct {
	Short string `yaml:"short"`
	Long  string `yaml:"long"`
}

// ContentRegisters pairs the semantic and methodological registers.
type ContentRegisters struct {
	Semantic       ContentRegister `yaml:"semantic"`
	Methodological ContentRegister `yaml:"methodological"`
}

// ContentRecord is the canonical in-memory representation of one content
// YAML file. All fields are required except WorkingPaperAnchors.
type ContentRecord struct {
	EntityID            string           `yaml:"entityId"`
	EntityType          string           `yaml:"entityType"`
	Locale              string           `yaml:"locale"`
	Registers           ContentRegisters `yaml:"registers"`
	ContentVersion      string           `yaml:"contentVersion"`
	LastReviewedBy      string           `yaml:"lastReviewedBy"`
	LastReviewedDate    string           `yaml:"lastReviewedDate"`
	WorkingPaperAnchors []string         `yaml:"workingPaperAnchors"`
}

// ContentCatalog is the in-memory catalog, keyed by "locale:entityType:entityId".
type ContentCatalog map[string]ContentRecord

var validEntityTypes = map[string]bool{
	"metric":                true,
	"probe":                 true,
	"discourse_function":    true,
	"refusal":               true,
	"view_mode":             true,
	"empty_lane":            true,
	"open_research_question": true,
	"primer":                true,
}

var validLocales = map[string]bool{
	"en": true,
	"de": true,
}

// CatalogKey returns the map key for a content record.
func CatalogKey(locale, entityType, entityID string) string {
	return locale + ":" + entityType + ":" + entityID
}

// LoadContentCatalog walks rootPath recursively, parses every *.yaml file into a
// ContentRecord, validates it, and returns the populated catalog. A malformed or
// invalid file causes an immediate error — the service must not start with broken
// content.
func LoadContentCatalog(rootPath string) (ContentCatalog, error) {
	catalog := make(ContentCatalog)

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("content catalog: reading %s: %w", path, readErr)
		}

		var record ContentRecord
		if parseErr := yaml.Unmarshal(data, &record); parseErr != nil {
			return fmt.Errorf("content catalog: parsing %s: %w", path, parseErr)
		}

		if validateErr := validateContentRecord(record, path); validateErr != nil {
			return validateErr
		}

		key := CatalogKey(record.Locale, record.EntityType, record.EntityID)
		if _, exists := catalog[key]; exists {
			return fmt.Errorf("content catalog: duplicate key %q (file: %s)", key, path)
		}
		catalog[key] = record
		return nil
	})
	if err != nil {
		return nil, err
	}

	return catalog, nil
}

func validateContentRecord(r ContentRecord, path string) error {
	loc := func(msg string) error {
		return fmt.Errorf("content catalog: %s (file: %s)", msg, path)
	}

	if r.EntityID == "" {
		return loc("entityId is required")
	}
	if !validEntityTypes[r.EntityType] {
		return loc(fmt.Sprintf("invalid entityType %q; must be one of metric, probe, discourse_function, refusal, view_mode, empty_lane, open_research_question, primer", r.EntityType))
	}
	if !validLocales[r.Locale] {
		return loc(fmt.Sprintf("invalid locale %q; must be one of en, de", r.Locale))
	}
	if r.ContentVersion == "" {
		return loc("contentVersion is required")
	}
	if r.LastReviewedBy == "" {
		return loc("lastReviewedBy is required")
	}
	if r.LastReviewedDate == "" {
		return loc("lastReviewedDate is required")
	}

	if err := validateRegister(r.Registers.Semantic, "registers.semantic", path); err != nil {
		return err
	}
	if err := validateRegister(r.Registers.Methodological, "registers.methodological", path); err != nil {
		return err
	}

	return nil
}

func validateRegister(reg ContentRegister, field, path string) error {
	loc := func(msg string) error {
		return fmt.Errorf("content catalog: %s.%s (file: %s)", field, msg, path)
	}
	if reg.Short == "" {
		return loc("short is required")
	}
	if len([]rune(reg.Short)) > 300 {
		return loc(fmt.Sprintf("short exceeds 300 characters (%d)", len([]rune(reg.Short))))
	}
	if reg.Long == "" {
		return loc("long is required")
	}
	if len([]rune(reg.Long)) > 4000 {
		return loc(fmt.Sprintf("long exceeds 4000 characters (%d)", len([]rune(reg.Long))))
	}
	return nil
}
