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

// --- Validation branch coverage (temp-file driven) ---

// writeContentYAML writes a content record file into dir.
func writeContentYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// validContentBody is a complete, schema-conformant record used as the baseline
// the failure cases below mutate.
const validContentBody = `
entityId: word_count
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: tester
lastReviewedDate: "2026-01-01"
registers:
  semantic:
    short: "A short semantic blurb."
    long: "A longer semantic explanation of the metric."
  methodological:
    short: "A short methodological note."
    long: "A longer methodological explanation of how it is computed."
`

func TestLoadContentCatalog_ValidTempFile(t *testing.T) {
	dir := t.TempDir()
	writeContentYAML(t, dir, "word_count.yaml", validContentBody)

	catalog, err := LoadContentCatalog(dir)
	if err != nil {
		t.Fatalf("LoadContentCatalog: %v", err)
	}
	key := CatalogKey("en", "metric", "word_count")
	rec, ok := catalog[key]
	if !ok {
		t.Fatalf("record %q not in catalog", key)
	}
	if rec.Registers.Semantic.Short != "A short semantic blurb." {
		t.Errorf("semantic.short = %q", rec.Registers.Semantic.Short)
	}
}

func TestLoadContentCatalog_RejectsMissingRoot(t *testing.T) {
	if _, err := LoadContentCatalog(filepath.Join(t.TempDir(), "no-such-dir")); err == nil {
		t.Fatal("expected error walking a non-existent root")
	}
}

func TestLoadContentCatalog_RejectsMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeContentYAML(t, dir, "bad.yaml", "entityId: [unterminated")
	if _, err := LoadContentCatalog(dir); err == nil {
		t.Fatal("expected parse error for malformed YAML")
	}
}

func TestLoadContentCatalog_RejectsDuplicateKey(t *testing.T) {
	dir := t.TempDir()
	writeContentYAML(t, dir, "a.yaml", validContentBody)
	writeContentYAML(t, dir, "b.yaml", validContentBody)
	_, err := LoadContentCatalog(dir)
	if err == nil {
		t.Fatal("expected duplicate-key error")
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		t.Errorf("error = %q, want 'duplicate key'", err)
	}
}

func TestLoadContentCatalog_IgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	writeContentYAML(t, dir, "word_count.yaml", validContentBody)
	writeContentYAML(t, dir, "notes.txt", "ignored")

	catalog, err := LoadContentCatalog(dir)
	if err != nil {
		t.Fatalf("LoadContentCatalog: %v", err)
	}
	if len(catalog) != 1 {
		t.Fatalf("expected 1 record (non-yaml ignored), got %d", len(catalog))
	}
}

// TestValidateContentRecord_RejectsBadField runs the record-level validation
// branches by writing a record that violates exactly one rule each.
func TestValidateContentRecord_RejectsBadField(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		wantSub string
	}{
		{
			name: "missing entityId",
			body: `
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "entityId is required",
		},
		{
			name: "invalid entityType",
			body: `
entityId: x
entityType: not_a_type
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "invalid entityType",
		},
		{
			name: "invalid locale",
			body: `
entityId: x
entityType: metric
locale: es
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "invalid locale",
		},
		{
			name: "missing contentVersion",
			body: `
entityId: x
entityType: metric
locale: en
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "contentVersion is required",
		},
		{
			name: "missing lastReviewedBy",
			body: `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "lastReviewedBy is required",
		},
		{
			name: "missing lastReviewedDate",
			body: `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
registers:
  semantic: {short: s, long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "lastReviewedDate is required",
		},
		{
			name: "semantic register missing short",
			body: `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {long: l}
  methodological: {short: s, long: l}
`,
			wantSub: "registers.semantic.short is required",
		},
		{
			name: "semantic register missing long",
			body: `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s}
  methodological: {short: s, long: l}
`,
			wantSub: "registers.semantic.long is required",
		},
		{
			name: "methodological register missing short",
			body: `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: l}
  methodological: {long: l}
`,
			wantSub: "registers.methodological.short is required",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeContentYAML(t, dir, "rec.yaml", c.body)
			_, err := LoadContentCatalog(dir)
			if err == nil {
				t.Fatalf("expected error containing %q", c.wantSub)
			}
			if !strings.Contains(err.Error(), c.wantSub) {
				t.Errorf("error = %q, want substring %q", err, c.wantSub)
			}
		})
	}
}

// TestValidateRegister_RejectsOverlongText exercises the length-cap branches:
// short > 300 runes and long > 4000 runes.
func TestValidateRegister_RejectsOverlongText(t *testing.T) {
	overlongShort := strings.Repeat("a", 301)
	bodyShort := `
entityId: x
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: "` + overlongShort + `", long: l}
  methodological: {short: s, long: l}
`
	dir := t.TempDir()
	writeContentYAML(t, dir, "rec.yaml", bodyShort)
	_, err := LoadContentCatalog(dir)
	if err == nil || !strings.Contains(err.Error(), "short exceeds 300 characters") {
		t.Fatalf("expected 'short exceeds 300 characters', got %v", err)
	}

	overlongLong := strings.Repeat("b", 4001)
	bodyLong := `
entityId: y
entityType: metric
locale: en
contentVersion: "1.0"
lastReviewedBy: t
lastReviewedDate: "2026-01-01"
registers:
  semantic: {short: s, long: "` + overlongLong + `"}
  methodological: {short: s, long: l}
`
	dir2 := t.TempDir()
	writeContentYAML(t, dir2, "rec.yaml", bodyLong)
	_, err = LoadContentCatalog(dir2)
	if err == nil || !strings.Contains(err.Error(), "long exceeds 4000 characters") {
		t.Fatalf("expected 'long exceeds 4000 characters', got %v", err)
	}
}

func TestCatalogKey(t *testing.T) {
	if got := CatalogKey("en", "metric", "word_count"); got != "en:metric:word_count" {
		t.Errorf("CatalogKey = %q", got)
	}
}
