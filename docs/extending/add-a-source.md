# Adding a Source

This guide answers: *"AĒR already supports this kind of platform — I just want to add another source on it."* For a news-website probe, that means: **one YAML entry plus one Postgres seed migration.** No code changes.

This guide is the Layer-4 complement to [add-a-source-type.md](add-a-source-type.md), which covers the *Layer-3* case (a wholly new platform class — Twitter, Reddit, Mastodon, …). If the platform class already has a crawler binary in `crawlers/<platform>-crawler/`, you are in the right place.

---

## When this applies

You are adding **a new source on an existing platform class**. Concretely, post-Phase-122 examples are:

- A new German news website on the existing `web-crawler` (Spiegel, FAZ, Süddeutsche, …) on Probe 0 or a successor probe.
- A new French institutional source on the same `web-crawler` for Probe 1 (Phase 125).
- A new English-language news website for a hypothetical English probe.

In all three cases **the binary is the same** — `crawlers/web-crawler/` (Phase 122 / ADR-028). The only per-source artefacts are configuration and a database row.

If your source is on a platform that has *no* AĒR crawler yet (Twitter, Reddit, Mastodon, Telegram, YouTube, Bluesky, …), see [add-a-source-type.md](add-a-source-type.md) first to write the platform crawler. Once that crawler exists, future sources on it become "add a source" tasks (this guide).

---

## What gets touched

| Artefact | Change | Effort |
| :--- | :--- | :--- |
| `crawlers/web-crawler/probes/<probe-id>/sources.yaml` | One new entry under `sources:` (name, sitemap_urls, optional rss_hint_url, politeness/url_filter/content_filter overrides) | 5 min |
| `infra/postgres/migrations/0000NN_seed_<source>.up.sql` (+ `.down.sql`) | One `INSERT INTO sources` row registering the source name + `type='web'` + initial `documentation_url` | 5 min |
| `infra/postgres/migrations/0000NN_seed_<source>_classification.up.sql` (optional but recommended) | One `INSERT INTO source_classifications` row with `review_status='provisional_engineering'` per WP-001 §4.4 | 5 min |
| Probe Dossier (`docs/probes/<probe-id>/README.md`) | Add the source to the source table; refresh the bias_assessment / temporal_profile prose if the new source materially changes the probe's character | 10–60 min depending on dossier currency |

**Realistic effort: 5–15 minutes engineering. Methodological work (provisional classification, bias notes) is the dominant cost.**

---

## Worked example — adding `bbc.co.uk` to a hypothetical English probe

This is illustrative; no English probe exists at the time of writing.

### 1. Verify robots.txt and sitemap availability

```bash
curl -A 'AerWebCrawler/0.1 (+https://aer.example/about; mailto:contact@example)' \
     https://www.bbc.co.uk/robots.txt | grep -i 'allow\|sitemap'
curl -I https://www.bbc.co.uk/sitemaps/news.xml
```

Confirm the `User-Agent` of the crawler is not `Disallow`-ed for the relevant paths and that the sitemap returns `200 OK`. Document the verification in the probe's dossier (`docs/probes/<probe-id>/temporal_profile.md` under "Discovery surface").

### 2. Add a YAML entry under the probe's source list

Open `crawlers/web-crawler/probes/<probe-id>/sources.yaml` and append:

```yaml
sources:
  # … existing sources …

  - name: bbc-news
    sitemap_urls:
      - https://www.bbc.co.uk/sitemaps/news.xml
    rss_hint_url: https://feeds.bbci.co.uk/news/rss.xml  # optional discovery hint only
    politeness:
      delay_seconds: 1.0
      autothrottle: true
      max_concurrent_per_domain: 2
    url_filter:
      exclude_extensions:
        [jpg, png, gif, svg, webp, mp4, mp3, css, js, pdf, ico, woff, woff2]
      exclude_path_prefixes:
        [/api/, /search/, /privacy, /accessibility]
      require_html_content_type: true
    content_filter:
      min_word_count: 50
      require_extraction_success: true
    custom_extractors: {}  # Tier-E: empty unless a specific analysis demands a bespoke field.
```

The default `url_filter` and `content_filter` values are universal — copy them verbatim. **Do not add section-level editorial filters** (no `/sport/` exclusions, no `/opinion/` exclusions) per WP-006 §3 / ADR-028. Per-article discourse-function imprecision is addressed in Phase 126b, not at the crawler.

### 3. Add a Postgres seed migration

Pick the next free index in `infra/postgres/migrations/` (e.g. `000018`):

```sql
-- 000018_seed_bbc_news.up.sql
INSERT INTO sources (name, type, url)
VALUES ('bbc-news', 'web', 'https://www.bbc.co.uk/news')
ON CONFLICT DO NOTHING;

UPDATE sources
   SET documentation_url = 'http://localhost:8000/probes/<probe-id>/'
 WHERE name = 'bbc-news';
```

Mirror it with a `.down.sql`:

```sql
-- 000018_seed_bbc_news.down.sql
DELETE FROM sources WHERE name = 'bbc-news';
```

If you want the `discourse_function` column populated in Gold from day one, add a parallel `INSERT INTO source_classifications` migration with `review_status='provisional_engineering'`, `function_weights=NULL`, `primary_function='epistemic_authority'` (or whichever WP-001 §6 function applies as your provisional engineering judgement). The migration carries an explicit comment that `function_weights` are NULL because WP-001 §4.4 Steps 1–2 are outstanding — this is the architecture's scientific-honesty contract; do not invent reviewer names.

### 4. Run the crawl

```bash
make crawl-<probe-id>
```

The crawler resolves `bbc-news` → `source_id` via `GET /api/v1/sources?name=bbc-news`, discovers URLs from the sitemap (and the optional RSS hint), fetches each new article (conditional GETs against `crawler_state`), and POSTs the raw HTML + fetch envelope to `POST /api/v1/ingest`. The analysis worker's `WebAdapter` (Phase 122) takes it from there.

### 5. Inspect spot-check invariants

After the crawl completes, verify the source surfaces in the BFF:

```bash
curl -H "X-API-Key: $BFF_API_KEY" \
     'http://localhost:8080/api/v1/sources?silverOnly=false' | jq '.[] | select(.name == "bbc-news")'
```

If the source is visible and `silverEligible=false`, run the WP-006 §5.2 review and grant eligibility out-of-band per the canonical procedure.

---

## What you do *not* touch

* **No new crawler code.** The web-crawler binary is identical across sources.
* **No new SilverMeta subclass.** All web sources share the five-tier `WebMeta` envelope.
* **No new extractor.** Word count, language detection, sentiment, NER, topic modelling, entity linking — every extractor consumes `SilverCore.cleaned_text` and is source-agnostic.
* **No `go.work` / Makefile edit.** The crawler-related Makefile targets are probe-scoped (`make crawl-<probe-id>`), not source-scoped.

This is the architectural payoff documented in [add-a-source-type.md](add-a-source-type.md): write the platform-class crawler once, then YAML for each source on that platform.

---

## Cross-references

- ADR-028 — Web Crawling Architecture (the rationale for the single-binary, technical-only-filtering, Bronze-as-raw-HTML decisions).
- [add-a-probe.md](add-a-probe.md) — when you are adding a *new probe*, not just a new source on an existing one.
- [add-a-source-type.md](add-a-source-type.md) — when the new source needs a *new platform-class crawler* (Twitter/Reddit/Mastodon/…).
- WP-006 §3 — the "document, don't filter" principle that motivates the technical-only URL filtering.
- WP-006 §5.2 — Silver-eligibility review.
- [Scientific Operations Guide → Workflow 1 (Probe Registration)](../operations/scientific_operations_guide.md#workflow-1-probe-registration) — the canonical procedure for the methodological side of source addition.
