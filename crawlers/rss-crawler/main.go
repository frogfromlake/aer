package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	metricsPath := flag.String("metrics-file", getEnv("PROMETHEUS_TEXTFILE_PATH", ""), "Optional node_exporter textfile collector path to write Prometheus metrics")
	delayMs := flag.Int("delay", 1000, "Delay in milliseconds between feed fetches")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	start := time.Now()

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
	feedsCrawled := 0

	for i, fc := range config.Feeds {
		if i > 0 {
			time.Sleep(time.Duration(*delayMs) * time.Millisecond)
		}

		slog.Info("Processing feed", "name", fc.Name, "url", fc.URL)

		// Resolve source_id dynamically
		sourceID, err := resolveSourceID(ctx, *sourcesURL, fc.Name, *apiKey)
		if err != nil {
			slog.Error("Failed to resolve source ID", "name", fc.Name, "error", err)
			continue
		}
		slog.Info("Resolved source ID", "name", fc.Name, "source_id", sourceID)

		// Parse feed
		feedTitle, items, err := feed.Parse(ctx, fc.URL)
		if err != nil {
			slog.Error("Failed to parse feed", "name", fc.Name, "url", fc.URL, "error", err)
			continue
		}
		feedsCrawled++
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

		if err := postToIngestionAPI(ctx, *apiURL, *apiKey, req); err != nil {
			slog.Error("Failed to submit to ingestion API", "name", fc.Name, "error", err)
			continue
		}

		// Mark all submitted items as seen and immediately persist state so that
		// a crash or interruption on a later feed does not cause re-ingestion of
		// already-submitted items.
		for _, item := range items {
			if !store.HasSeen(fc.Name, item.GUID) {
				store.MarkSeen(fc.Name, item.GUID)
			}
		}
		if err := store.Save(); err != nil {
			slog.Error("Failed to save dedup state", "name", fc.Name, "error", err)
			os.Exit(1)
		}

		slog.Info("Submitted items", "name", fc.Name, "submitted", len(documents), "skipped", skipped)
		totalSubmitted += len(documents)
		totalSkipped += skipped
	}

	duration := time.Since(start)
	slog.Info("RSS Crawler finished", "total_submitted", totalSubmitted, "total_skipped", totalSkipped, "duration_seconds", duration.Seconds())

	if *metricsPath != "" {
		if err := writeTextfileMetrics(*metricsPath, feedsCrawled, totalSubmitted, totalSkipped, duration); err != nil {
			slog.Warn("Failed to write Prometheus textfile metrics", "path", *metricsPath, "error", err)
		}
	}
}

// writeTextfileMetrics writes the run's outcome as a Prometheus exposition
// file consumable by node_exporter's textfile collector. Rendered atomically
// (temp + rename) so a concurrent scrape never observes a partial file.
func writeTextfileMetrics(path string, feedsCrawled, submitted, skipped int, duration time.Duration) error {
	var buf bytes.Buffer
	fmt.Fprintln(&buf, "# HELP rss_crawler_feeds_crawled_total Number of feeds successfully parsed in the last run.")
	fmt.Fprintln(&buf, "# TYPE rss_crawler_feeds_crawled_total counter")
	fmt.Fprintf(&buf, "rss_crawler_feeds_crawled_total %d\n", feedsCrawled)
	fmt.Fprintln(&buf, "# HELP rss_crawler_items_submitted_total Number of items submitted to the ingestion API in the last run.")
	fmt.Fprintln(&buf, "# TYPE rss_crawler_items_submitted_total counter")
	fmt.Fprintf(&buf, "rss_crawler_items_submitted_total %d\n", submitted)
	fmt.Fprintln(&buf, "# HELP rss_crawler_items_skipped_total Number of items skipped as duplicates in the last run.")
	fmt.Fprintln(&buf, "# TYPE rss_crawler_items_skipped_total counter")
	fmt.Fprintf(&buf, "rss_crawler_items_skipped_total %d\n", skipped)
	fmt.Fprintln(&buf, "# HELP rss_crawler_duration_seconds Wall-clock runtime of the last crawler invocation.")
	fmt.Fprintln(&buf, "# TYPE rss_crawler_duration_seconds gauge")
	fmt.Fprintf(&buf, "rss_crawler_duration_seconds %f\n", duration.Seconds())
	fmt.Fprintln(&buf, "# HELP rss_crawler_last_successful_crawl_timestamp Unix timestamp of the last successful crawl completion.")
	fmt.Fprintln(&buf, "# TYPE rss_crawler_last_successful_crawl_timestamp gauge")
	fmt.Fprintf(&buf, "rss_crawler_last_successful_crawl_timestamp %d\n", time.Now().Unix())

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// resolveSourceID queries the ingestion API to look up a source ID by name.
func resolveSourceID(ctx context.Context, sourcesURL, name, apiKey string) (int, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourcesURL+"?name="+name, nil)
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
func postToIngestionAPI(ctx context.Context, url, apiKey string, req feed.IngestRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
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
