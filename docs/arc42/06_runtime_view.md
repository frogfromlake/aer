# 6. Runtime View

This view describes the deterministic path of data through the AĒR system (Data Pipeline Flow). To preserve scientific integrity, this process is strictly sequential.

## 6.1 Standard Data Flow (Aggregation to Presentation)

Using an emerging geopolitical event (e.g., a newly published article) as an example, the dataset passes through the following stages:

1. **Ingestion (Data Collection - Go):**
   * A Go crawler (e.g., `generic-web-ingester`) polls an external API or data source.
   * The crawler extracts the raw data (title, text, date, author, source) *without* any content alteration.
   * The dataset is written as a "Bronze Record" into the Object Storage (MinIO), while its metadata path is indexed in PostgreSQL.

2. **Processing (Event-Triggered Harmonization - Python):**
   * Upon successful upload to the "Bronze Layer", MinIO natively publishes a notification event to the NATS message broker (Subject: `aer.lake.bronze`).
   * A Python worker, subscribed to the NATS JetStream queue, instantly receives the event payload (containing the file path).
   * The text is fetched, cleaned (HTML removal, character set normalization, UTC timestamp standardization), saved as a "Silver Record", and the event is acknowledged.

3. **Analysis (Deterministic Metrics - Python):**
   * The `analysis-service` applies scientific, transparent models (e.g., keyword extraction, N-gram counting, assignment to predefined theme clusters).
   * Opaque LLMs are *not* used for interpretation.
   * The calculated metrics (e.g., "Theme: Economy", "Frequency of word X: 12") are written as time-series data into the high-performance analytical database (ClickHouse / Gold Layer).

4. **Serving & Presentation (BFF & UI - Go + Frontend):**
   * An end user opens the dashboard. The frontend sends a request to the Backend-for-Frontend (Go).
   * The BFF queries ClickHouse for aggregated metrics over the requested timeframe (e.g., "Show me the trend of the 'Economy' theme over the last 48 hours").
   * The UI visualizes this data interactively and provides a drill-down capability (Progressive Disclosure) back to the original raw dataset.