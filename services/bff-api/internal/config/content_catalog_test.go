package config

// Phase 104 — Content catalog validation tests.
//
// Three checks:
//   (a) LoadContentCatalog succeeds on the real configs/content/ tree (schema conformance).
//   (b) Locale parity: every en/{type}/{id}.yaml has a matching de/{type}/{id}.yaml.
//   (c) Cross-reference integrity: every WP anchor ("WP-NNN §M" or "WP-NNN §M.K") in any
//       short/long/workingPaperAnchors field resolves to a heading in the English WP markdown.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot walks upward from this file's directory to find the repository root
// (identified by go.work being present).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root (go.work not found)")
		}
		dir = parent
	}
}

func contentRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "services", "bff-api", "configs", "content")
}

func methodologyRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "docs", "methodology", "en")
}

// TestContentCatalogLoads verifies the full catalog loads without error.
func TestContentCatalogLoads(t *testing.T) {
	root := contentRoot(t)
	catalog, err := LoadContentCatalog(root)
	if err != nil {
		t.Fatalf("LoadContentCatalog failed: %v", err)
	}
	if len(catalog) == 0 {
		t.Fatal("catalog is empty")
	}
	t.Logf("loaded %d content records", len(catalog))
}

// TestContentCatalogLocaleParity verifies every en record has a de counterpart.
func TestContentCatalogLocaleParity(t *testing.T) {
	root := contentRoot(t)
	catalog, err := LoadContentCatalog(root)
	if err != nil {
		t.Fatalf("LoadContentCatalog failed: %v", err)
	}

	var missing []string
	for _, rec := range catalog {
		if rec.Locale != "en" {
			continue
		}
		deKey := CatalogKey("de", rec.EntityType, rec.EntityID)
		if _, ok := catalog[deKey]; !ok {
			missing = append(missing, fmt.Sprintf("en:%s:%s has no de counterpart", rec.EntityType, rec.EntityID))
		}
	}
	for _, m := range missing {
		t.Errorf("locale parity violation: %s", m)
	}
}

// wpAnchorRe matches WP anchor tokens in text: "WP-NNN §M" or "WP-NNN §M.K".
var wpAnchorRe = regexp.MustCompile(`WP-(\d+)\s*§(\d+(?:\.\d+)?)`)

// wpHeadingRe matches markdown headings that start a section number: "## 3." or "### 3.1".
var wpHeadingRe = regexp.MustCompile(`(?m)^#{1,6}\s+(\d+(?:\.\d+)*)\b`)

// loadWPHeadings reads a WP markdown file and returns a set of all section numbers found
// in headings (e.g., "3", "3.1", "5.2").
func loadWPHeadings(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sections := make(map[string]bool)
	for _, m := range wpHeadingRe.FindAllSubmatch(data, -1) {
		sections[string(m[1])] = true
	}
	return sections, nil
}

// TestContentCatalogCrossReferences verifies every WP anchor in every content record
// resolves to a section that exists in the corresponding WP markdown file.
func TestContentCatalogCrossReferences(t *testing.T) {
	root := contentRoot(t)
	catalog, err := LoadContentCatalog(root)
	if err != nil {
		t.Fatalf("LoadContentCatalog failed: %v", err)
	}

	methodologyDir := methodologyRoot(t)

	// Build index: wp number → set of section numbers.
	wpIndex := make(map[string]map[string]bool) // "001" → {"3": true, "3.1": true, ...}
	entries, err := os.ReadDir(methodologyDir)
	if err != nil {
		t.Fatalf("reading methodology dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		// Extract WP number from filename (e.g., WP-002-en-...).
		parts := strings.SplitN(e.Name(), "-", 3)
		if len(parts) < 2 || parts[0] != "WP" {
			continue
		}
		wpNum := parts[1] // e.g., "002"
		sections, err := loadWPHeadings(filepath.Join(methodologyDir, e.Name()))
		if err != nil {
			t.Fatalf("loading WP headings from %s: %v", e.Name(), err)
		}
		wpIndex[wpNum] = sections
	}

	// Check every anchor in the catalog.
	for key, rec := range catalog {
		checkText := func(field, text string) {
			for _, m := range wpAnchorRe.FindAllStringSubmatch(text, -1) {
				wpNum := m[1]   // e.g., "001"
				section := m[2] // e.g., "3" or "5.2"

				// Zero-pad to 3 digits for WP filename lookup.
				if len(wpNum) < 3 {
					wpNum = strings.Repeat("0", 3-len(wpNum)) + wpNum
				}

				sections, knownWP := wpIndex[wpNum]
				if !knownWP {
					t.Errorf("record %s field %s: references WP-%s which has no English markdown in docs/methodology/en/", key, field, wpNum)
					return
				}
				if !sections[section] {
					t.Errorf("record %s field %s: WP-%s §%s not found in WP markdown headings (known: %v)",
						key, field, wpNum, section, sectionKeys(sections))
				}
			}
		}
		checkText("semantic.short", rec.Registers.Semantic.Short)
		checkText("semantic.long", rec.Registers.Semantic.Long)
		checkText("methodological.short", rec.Registers.Methodological.Short)
		checkText("methodological.long", rec.Registers.Methodological.Long)
		for _, anchor := range rec.WorkingPaperAnchors {
			checkText("workingPaperAnchors", anchor)
		}
	}
}

func sectionKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
