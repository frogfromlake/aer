package feed

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTranslate_ValidItem(t *testing.T) {
	item := ParsedItem{
		Title:      "Test Article Title",
		RawText:    "The article content goes here.",
		Link:       "https://example.com/article",
		Published:  time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
		GUID:       "https://example.com/article",
		Categories: []string{"Politik", "Wirtschaft"},
		Author:     "Test Author",
	}

	doc, err := Translate(item, "tagesschau", "https://tagesschau.de/rss", "Tagesschau Feed")
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	// Verify key pattern: rss/<source_name>/<guid-hash>/<date>.json
	if !strings.HasPrefix(doc.Key, "rss/tagesschau/") {
		t.Errorf("key should start with 'rss/tagesschau/', got: %q", doc.Key)
	}
	if !strings.HasSuffix(doc.Key, "/2026-04-05.json") {
		t.Errorf("key should end with '/2026-04-05.json', got: %q", doc.Key)
	}

	// Verify payload conforms to Ingestion Contract
	var payload map[string]any
	if err := json.Unmarshal(doc.Data, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	// Required Ingestion Contract fields
	if payload["source"] != "tagesschau" {
		t.Errorf("expected source 'tagesschau', got: %v", payload["source"])
	}
	if payload["source_type"] != "rss" {
		t.Errorf("expected source_type 'rss', got: %v", payload["source_type"])
	}
	if payload["title"] != "Test Article Title" {
		t.Errorf("unexpected title: %v", payload["title"])
	}
	if payload["raw_text"] != "The article content goes here." {
		t.Errorf("unexpected raw_text: %v", payload["raw_text"])
	}
	if payload["url"] != "https://example.com/article" {
		t.Errorf("unexpected url: %v", payload["url"])
	}
	if payload["timestamp"] != "2026-04-05T10:00:00Z" {
		t.Errorf("unexpected timestamp: %v", payload["timestamp"])
	}

	// RSS-specific metadata fields
	if payload["feed_url"] != "https://tagesschau.de/rss" {
		t.Errorf("unexpected feed_url: %v", payload["feed_url"])
	}
	if payload["feed_title"] != "Tagesschau Feed" {
		t.Errorf("unexpected feed_title: %v", payload["feed_title"])
	}
	if payload["author"] != "Test Author" {
		t.Errorf("unexpected author: %v", payload["author"])
	}
}

func TestTranslate_MissingTitle_ReturnsError(t *testing.T) {
	item := ParsedItem{
		Title:   "",
		RawText: "Some content",
		GUID:    "test-guid",
	}

	_, err := Translate(item, "test", "https://example.com/feed", "Test")
	if err == nil {
		t.Error("expected error for missing title")
	}
}

func TestTranslate_MissingRawText_ReturnsError(t *testing.T) {
	item := ParsedItem{
		Title:   "Has Title",
		RawText: "",
		GUID:    "test-guid",
	}

	_, err := Translate(item, "test", "https://example.com/feed", "Test")
	if err == nil {
		t.Error("expected error for missing raw_text")
	}
}

func TestBuildObjectKey_Determinism(t *testing.T) {
	key1 := BuildObjectKey("tagesschau", "guid-123", "2026-04-05")
	key2 := BuildObjectKey("tagesschau", "guid-123", "2026-04-05")

	if key1 != key2 {
		t.Errorf("object keys should be deterministic: %q != %q", key1, key2)
	}
}

func TestBuildObjectKey_DifferentGUIDs(t *testing.T) {
	key1 := BuildObjectKey("tagesschau", "guid-123", "2026-04-05")
	key2 := BuildObjectKey("tagesschau", "guid-456", "2026-04-05")

	if key1 == key2 {
		t.Error("different GUIDs should produce different keys")
	}
}

func TestBuildObjectKey_Format(t *testing.T) {
	key := BuildObjectKey("bundesregierung", "https://example.gov.de/art/123", "2026-04-05")

	if !strings.HasPrefix(key, "rss/bundesregierung/") {
		t.Errorf("key should start with 'rss/bundesregierung/', got: %q", key)
	}
	if !strings.HasSuffix(key, "/2026-04-05.json") {
		t.Errorf("key should end with date.json, got: %q", key)
	}
}
