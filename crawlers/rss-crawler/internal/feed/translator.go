package feed

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

// IngestDocument represents a single document in the ingestion request.
type IngestDocument struct {
	Key  string          `json:"key"`
	Data json.RawMessage `json:"data"`
}

// IngestRequest is the payload sent to POST /api/v1/ingest.
type IngestRequest struct {
	SourceID  int              `json:"source_id"`
	Documents []IngestDocument `json:"documents"`
}

// articlePayload is the raw document stored in the Bronze layer.
// It conforms to the AĒR Ingestion Contract with the addition of
// source_type for the Phase 39 adapter pattern.
type articlePayload struct {
	Source     string   `json:"source"`
	SourceType string  `json:"source_type"`
	Title      string  `json:"title"`
	RawText    string  `json:"raw_text"`
	URL        string  `json:"url"`
	Timestamp  string  `json:"timestamp"`
	FeedURL    string  `json:"feed_url"`
	Categories []string `json:"categories,omitempty"`
	Author     string  `json:"author,omitempty"`
	FeedTitle  string  `json:"feed_title,omitempty"`
}

// Translate converts a ParsedItem into an IngestDocument conforming to the AĒR Ingestion Contract.
// The key pattern is: rss/<source_name>/<guid-hash>/<date>.json
func Translate(item ParsedItem, sourceName, feedURL, feedTitle string) (IngestDocument, error) {
	if item.Title == "" || item.RawText == "" {
		return IngestDocument{}, fmt.Errorf("item missing required fields (title or raw_text)")
	}

	payload := articlePayload{
		Source:     sourceName,
		SourceType: "rss",
		Title:      item.Title,
		RawText:    item.RawText,
		URL:        item.Link,
		Timestamp:  item.Published.Format("2006-01-02T15:04:05Z"),
		FeedURL:    feedURL,
		Categories: item.Categories,
		Author:     item.Author,
		FeedTitle:  feedTitle,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return IngestDocument{}, fmt.Errorf("failed to marshal payload: %w", err)
	}

	key := BuildObjectKey(sourceName, item.GUID, item.Published.Format("2006-01-02"))

	return IngestDocument{
		Key:  key,
		Data: json.RawMessage(payloadBytes),
	}, nil
}

// BuildObjectKey creates a deterministic, URL-safe MinIO object key.
// Format: rss/<source_name>/<guid-hash>/<date>.json
func BuildObjectKey(sourceName, guid, date string) string {
	guidHash := fmt.Sprintf("%x", sha256.Sum256([]byte(guid)))[:16]

	slug := strings.ToLower(sourceName)
	slug = strings.ReplaceAll(slug, " ", "-")
	var sb strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}

	return fmt.Sprintf("rss/%s/%s/%s.json", sb.String(), guidHash, date)
}
