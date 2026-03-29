package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

const wikipediaRandomSummaryAPI = "https://en.wikipedia.org/api/rest_v1/page/random/summary"

// wikipediaSummary holds the fields we use from the Wikipedia REST API response.
type wikipediaSummary struct {
	Title       string `json:"title"`
	Extract     string `json:"extract"`
	Timestamp   string `json:"timestamp"`
	ContentURLs struct {
		Desktop struct {
			Page string `json:"page"`
		} `json:"desktop"`
	} `json:"content_urls"`
}

// articlePayload is the raw document stored in the Bronze layer.
// It conforms to the AĒR Ingestion Contract — source-agnostic fields only.
type articlePayload struct {
	Source    string `json:"source"`
	Title     string `json:"title"`
	RawText   string `json:"raw_text"`
	URL       string `json:"url"`
	Timestamp string `json:"timestamp"`
}

// ingestDocument represents a single document in the ingestion request.
type ingestDocument struct {
	Key  string          `json:"key"`
	Data json.RawMessage `json:"data"`
}

// ingestRequest is the payload sent to POST /api/v1/ingest.
type ingestRequest struct {
	SourceID  int              `json:"source_id"`
	Documents []ingestDocument `json:"documents"`
}

func main() {
	ingestionURL := flag.String("ingestion-url", getEnv("INGESTION_URL", "http://localhost:8081/api/v1/ingest"), "URL of the ingestion API endpoint")
	sourceID := flag.Int("source-id", 1, "Source ID registered in PostgreSQL for the Wikipedia source")
	flag.Parse()

	slog.Info("Fetching random Wikipedia article summary...")
	article, err := fetchRandomArticle()
	if err != nil {
		slog.Error("Failed to fetch Wikipedia article", "error", err)
		os.Exit(1)
	}
	slog.Info("Fetched article", "title", article.Title, "timestamp", article.Timestamp)

	payload := articlePayload{
		Source:    "wikipedia",
		Title:     article.Title,
		RawText:   article.Extract, // adapter: map Wikipedia-specific field to generic contract
		URL:       article.ContentURLs.Desktop.Page,
		Timestamp: article.Timestamp,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal article payload", "error", err)
		os.Exit(1)
	}

	docKey := buildObjectKey(article.Title, article.Timestamp)
	req := ingestRequest{
		SourceID: *sourceID,
		Documents: []ingestDocument{
			{Key: docKey, Data: json.RawMessage(payloadBytes)},
		},
	}

	if err := postToIngestionAPI(*ingestionURL, req); err != nil {
		slog.Error("Failed to post to ingestion API", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully submitted article to ingestion pipeline", "key", docKey)
}

// fetchRandomArticle fetches a random article summary from the Wikipedia REST API.
func fetchRandomArticle() (*wikipediaSummary, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, wikipediaRandomSummaryAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	// Wikipedia API recommends setting a descriptive User-Agent.
	req.Header.Set("User-Agent", "aer-wikipedia-scraper/1.0 (github.com/frogfromlake/aer)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Wikipedia API returned unexpected status: %s", resp.Status)
	}

	var summary wikipediaSummary
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		return nil, fmt.Errorf("failed to decode Wikipedia response: %w", err)
	}

	if summary.Title == "" || summary.Extract == "" {
		return nil, fmt.Errorf("article is missing required fields (title or extract)")
	}

	return &summary, nil
}

// buildObjectKey creates a deterministic, URL-safe MinIO object key for partitioned storage.
// Format: wikipedia/{slug}/{date}.json
func buildObjectKey(title, timestamp string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	var sb strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		}
	}

	date := time.Now().UTC().Format("2006-01-02")
	if len(timestamp) >= 10 {
		date = timestamp[:10]
	}

	return fmt.Sprintf("wikipedia/%s/%s.json", sb.String(), date)
}

// postToIngestionAPI sends the ingestion request to the API and logs the response.
func postToIngestionAPI(url string, req ingestRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("HTTP POST failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		return fmt.Errorf("ingestion API returned %s: %v", resp.Status, result)
	}

	slog.Info("Ingestion API accepted document", "http_status", resp.Status, "result", result)
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
