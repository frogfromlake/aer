package feed

import (
	"os"
	"testing"
)

func TestParseString_ValidRSS(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sample_rss.xml")
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	feedTitle, items, err := ParseString(string(data))
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	if feedTitle != "Test Feed — Bundespresseamt" {
		t.Errorf("unexpected feed title: %q", feedTitle)
	}

	// The fixture has 3 items (including one with empty title)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// First item
	item := items[0]
	if item.Title != "Bundesregierung beschließt neues Klimaschutzpaket" {
		t.Errorf("unexpected title: %q", item.Title)
	}
	if item.Link != "https://www.example.gov.de/artikel/klimaschutz-2026" {
		t.Errorf("unexpected link: %q", item.Link)
	}
	if item.GUID != "https://www.example.gov.de/artikel/klimaschutz-2026" {
		t.Errorf("unexpected GUID: %q", item.GUID)
	}
	if len(item.Categories) != 2 {
		t.Errorf("expected 2 categories, got %d", len(item.Categories))
	}
	if item.RawText == "" {
		t.Error("raw_text should not be empty")
	}
	if item.Published.IsZero() {
		t.Error("published time should not be zero")
	}
}

func TestParseString_ExtractsAuthor(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sample_rss.xml")
	if err != nil {
		t.Fatalf("failed to read test fixture: %v", err)
	}

	_, items, err := ParseString(string(data))
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// First item has an author
	if items[0].Author == "" {
		t.Error("expected non-empty author for first item")
	}
}

func TestParseString_GUIDFallback(t *testing.T) {
	// RSS item without explicit GUID should fall back to link
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <item>
      <title>No GUID Item</title>
      <link>https://example.com/no-guid</link>
      <description>Test content</description>
    </item>
  </channel>
</rss>`

	_, items, err := ParseString(xmlData)
	if err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].GUID != "https://example.com/no-guid" {
		t.Errorf("GUID should fall back to link, got: %q", items[0].GUID)
	}
}

func TestParseString_InvalidXML(t *testing.T) {
	_, _, err := ParseString("this is not XML")
	if err == nil {
		t.Error("expected error for invalid XML")
	}
}
