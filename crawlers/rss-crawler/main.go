package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/frogfromlake/aer/crawlers/rss-crawler/internal/feed"
	"github.com/frogfromlake/aer/crawlers/rss-crawler/internal/state"
	"gopkg.in/yaml.v3"
)

// feedConfig represents a single feed entry in feeds.yaml.
type feedConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// feedsFile is the top-level structure of feeds.yaml.
type feedsFile struct {
	Feeds []feedConfig `yaml:"feeds"`
}

// sourceResponse is the JSON returned by GET /api/v1/sources?name=<name>.
type sourceResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	configPath := flag.String("config", "feeds.yaml", "Path to the feed configuration file")
	apiURL := flag.String("api-url", getEnv("INGESTION_URL", "http://localhost:8081/api/v1/ingest"), "URL of the ingestion API endpoint")
	sourcesURL := flag.String("sources-url", getEnv("SOURCES_URL", "http://localhost:8081/api/v1/sources"), "URL of the sources lookup endpoint")
	apiKey := flag.String("api-key", getEnv("INGESTION_API_KEY", ""), "API key for the ingestion API")
	statePath := flag.String("state", getEnv("STATE_FILE", ".rss-crawler-state.json"), "Path to the dedup state file")
	delayMs := flag.Int("delay", 1000, "Delay in milliseconds between feed fetches")
	flag.Parse()

	// Load feed configuration
	configData, err := os.ReadFile(*configPath)
	if err != nil {
		slog.Error("Failed to read feed config", "path", *configPath, "error", err)
		os.Exit(1)
	}

	var config feedsFile
	if err := yaml.Unmarshal(configData, &config); err != nil {
		slog.Error("Failed to parse feed config", "error", err)
		os.Exit(1)
	}

	if len(config.Feeds) == 0 {
		slog.Error("No feeds configured")
		os.Exit(1)
	}

	// Load dedup state
	store, err := state.NewStore(*statePath)
	if err != nil {
		slog.Error("Failed to load dedup state", "error", err)
		os.Exit(1)
	}

	slog.Info("RSS Crawler starting", "feeds", len(config.Feeds), "state_file", *statePath)

	totalSubmitted := 0
	totalSkipped := 0

	for i, fc := range config.Feeds {
		if i > 0 {
			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}

		slog.Info("Processing feed", "name", fc.Name, "url", fc.URL)

		// Resolve source_id dynamically
		sourceID, err := resolveSourceID(*sourcesURL, fc.Name, *apiKey)
		if err != nil {
			slog.Error("Failed to resolve source ID", "name", fc.Name, "error", err)
			continue
		}
		slog.Info("Resolved source ID", "name", fc.Name, "source_id", sourceID)

		// Parse feed
		feedTitle, items, err := feed.Parse(fc.URL)
		if err != nil {
			slog.Error("Failed to parse feed", "name", fc.Name, "url", fc.URL, "error", err)
			continue
		}
		slog.Info("Parsed feed", "name", fc.Name, "title", feedTitle, "items", len(items))

		// Translate and deduplicate
		var documents []feed.IngestDocument
		skipped := 0
		for _, item := range items {
			if store.HasSeen(fc.Name, item.GUID) {
				skipped++
				continue
			}

			doc, err := feed.Translate(item, fc.Name, fc.URL, feedTitle)
			if err != nil {
				slog.Warn("Skipping item", "title", item.Title, "error", err)
				continue
			}
			documents = append(documents, doc)
		}

		if len(documents) == 0 {
			slog.Info("No new items to submit", "name", fc.Name, "skipped", skipped)
			totalSkipped += skipped
			continue
		}

		// Submit batch to ingestion API
		req := feed.IngestRequest{
			SourceID:  sourceID,
			Documents: documents,
		}

		if err := postToIngestionAPI(*apiURL, *apiKey, req); err != nil {
			slog.Error("Failed to submit to ingestion API", "name", fc.Name, "error", err)
			continue
		}

		// Mark all submitted items as seen
		for _, item := range items {
			if !store.HasSeen(fc.Name, item.GUID) {
				store.MarkSeen(fc.Name, item.GUID)
			}
		}

		slog.Info("Submitted items", "name", fc.Name, "submitted", len(documents), "skipped", skipped)
		totalSubmitted += len(documents)
		totalSkipped += skipped
	}

	// Persist dedup state
	if err := store.Save(); err != nil {
		slog.Error("Failed to save dedup state", "error", err)
		os.Exit(1)
	}

	slog.Info("RSS Crawler finished", "total_submitted", totalSubmitted, "total_skipped", totalSkipped)
}

// resolveSourceID queries the ingestion API to look up a source ID by name.
func resolveSourceID(sourcesURL, name, apiKey string) (int, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, sourcesURL+"?name="+name, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to build request: %w", err)
	}
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("sources API returned %s for name=%q", resp.Status, name)
	}

	var src sourceResponse
	if err := json.NewDecoder(resp.Body).Decode(&src); err != nil {
		return 0, fmt.Errorf("failed to decode sources response: %w", err)
	}

	if src.ID <= 0 {
		return 0, fmt.Errorf("invalid source ID %d for name=%q", src.ID, name)
	}

	return src.ID, nil
}

// postToIngestionAPI sends the ingestion request to the API.
func postToIngestionAPI(url, apiKey string, req feed.IngestRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("HTTP POST failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusMultiStatus {
		return fmt.Errorf("ingestion API returned %s: %v", resp.Status, result)
	}

	slog.Info("Ingestion API accepted documents", "http_status", resp.Status, "result", result)
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
