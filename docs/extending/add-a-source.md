# Adding a Source

This guide answers: *"AĒR already supports this kind of platform — I just want to add another source on it."* For a news-website probe, that means: **one YAML entry plus one Postgres seed migration.** No code changes.

This guide is the Layer-4 complement to [add-a-source-type.md](add-a-source-type.md), which covers the *Layer-3* case (a wholly new platform class — Twitter, Reddit, Mastodon, …). If the platform class already has a crawler binary in `crawlers/<platform>-crawler/`, you are in the right place.

---

## When this applies

You are adding **a new source on an existing platform class**. Concretely, post-Phase-122 examples are:

- A new German news website on the existing `web-crawler` (Spiegel, FAZ, Süddeutsche, …) on Probe 0 or a successor probe.
- A new French institutional source on the same `web-crawler` for Probe 1 (Phase 123).
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

## Discovery surfaces — how to pick one (or several)

A web crawler can only find articles through what the publisher **exposes** for machine consumption. Four discovery surfaces are universally usable. The four-channel model is the Phase 122g `DiscoveryProtocol` contract — see ADR-031:

| Channel | When the publisher exposes it | What we configure | Coverage |
| :--- | :--- | :--- | :--- |
| **(A) XML sitemap** | `Sitemap:` directive in `robots.txt`, OR a `/sitemap.xml` / `/sitemap_index.xml` that returns `200 OK` | `discovery.sitemap_urls: [...]` (one or more roots) | Full archive — typically every URL the publisher considers canonical |
| **(B) RSS / Atom feeds (plural)** | Feed URLs the publisher advertises, often catalogued on a `/service/rss` or `/newsletter` page (Phase 122g — multiple feeds per source supported) | `discovery.rss_hint_urls: [...]` (list of URLs) | Sliding window of the most recent ≈ 50–200 items per feed. **Not** an archive |
| **(C) HTML sitemap page** | A publisher-built navigation page that surfaces the current article set in HTML at a non-standard URL (e.g. tagesschau's `/infoservices/startseite-sitemap-102.html`). Operator-discoverable, NOT standardly auto-discoverable | `discovery.html_sitemap_urls: [{ url, article_url_pattern }]` | Daily-refreshed snapshot of the publisher's current top stories. ≈ 50–100 article links per page |
| **(D) Date-indexed HTML archive** | A page parameterised by date — e.g. `/archiv?datum=YYYY-MM-DD` — that the publisher uses for navigation | `discovery.archive_index: { url_template, date_format, granularity, article_url_pattern }` | Per-day list going back as far as the publisher chooses to expose |

Methodologically — every one of these is *publisher curation*, not researcher curation: the publisher built the surface, the publisher decides what's on it, we ingest the listed URLs verbatim. **Do not** hand-pick a subset of feeds (e.g. only `politik` + `wirtschaft`) — that is researcher selection bias per WP-006 §3 / ADR-028 / ADR-031 and inverts the Manifesto's "unaltered mirror" principle. If the publisher advertises N RSS feeds in a catalogue page, configure all N (or none); never pick "the relevant ones."

Channels are additive. URL collisions deduplicate on the consumer side with channel-order precedence (sitemap > rss > html_sitemap > archive_index — the sitemap entry carries the canonical `lastmod` and `sitemap_section` context). Phase 122g per-channel telemetry attributes `urls_after_dedup` to whichever channel got the first-yield credit, so the dashboard's `DiscoveryCoveragePanel` reports each channel's actual contribution.

**Rule of thumb**: declare every channel the publisher actually exposes. Probe-0's `tagesschau.de` is the canonical multi-channel example: no XML sitemap, one main RSS feed, one daily-refreshed HTML sitemap, one date-indexed archive walker — all four channel slots used (one empty, three populated). Probe-0's `bundesregierung.de` is the canonical multi-RSS-feed example: three service-only sitemap_index URLs plus four publisher-curated RSS feeds (`Bundesregierung kompakt`, `Pressemitteilungen`, `Artikel`, `Bulletin`).

## Worked example — adding `bbc.co.uk` to a hypothetical English probe

This is illustrative; no English probe exists at the time of writing.

### 0. Run the audit CLI (Phase 122g)

The `audit-source-discovery` CLI probes a candidate source's homepage and reports the discovery channels the publisher exposes — RSS feeds via `trafilatura.feeds.find_feed_urls`, sitemaps via `trafilatura.sitemaps.sitemap_search`, plus per-source-class probes for common HTML-sitemap and date-indexed archive URL patterns. Cross-reference its output against Mediacloud's open registry (https://search.mediacloud.org) — if your candidate already exists there, compare their curated feed list against the audit.

```bash
python crawlers/web-crawler/audit_source.py https://www.bbc.co.uk
# … or via the installed entry point:
aer-audit-source https://www.bbc.co.uk
```

The CLI emits a YAML-shaped `discovery:` block ready for pasting into `sources.yaml`, with `<edit-me>` placeholders for the publisher-specific `article_url_pattern` regex(es) you must derive from sample article URLs.

### 1. Verify the audit findings manually

Cross-check the audit output by curl. In particular:

- our `User-Agent` is not in any `Disallow` block in `robots.txt`;
- the suggested feeds + sitemaps actually return HTTP 200;
- if the audit flagged an `archive_index` candidate, sample two distant dates and confirm they return distinct article lists (so the endpoint really is a date-walker, not a homepage snapshot). Note the granularity — daily or monthly. Phase 122e A20's investigation of tagesschau's archive is the worked methodology.
- if the publisher exposes an RSS *catalogue page* (often at `/service/rss`, `/newsletter`, or similar — bundesregierung's `/breg-de/service/newsletter-und-abos/rss-newsfeed` is the canonical example), visit it manually. Trafilatura returns the feeds advertised via `<link rel="alternate">` only — many publishers organise additional feeds on a separate catalogue page that auto-discovery misses.

Document the verification — including which channels we settled on and *why* — in the probe's dossier (`docs/probes/<probe-id>/temporal_profile.md` under "Discovery surface"). The asymmetry between sources' discovery surfaces is a recorded structural bias dimension (Probe-0 `bias_assessment.md` Structural Bias #8); new asymmetries will be caught at runtime by Phase 122g's per-channel coverage telemetry (`GET /api/v1/sources/{id}/discovery-coverage`).

### 2. Add a YAML entry under the probe's source list

Open `crawlers/web-crawler/probes/<probe-id>/sources.yaml` and append:

```yaml
sources:
  # … existing sources …

  - name: bbc-news
    # Phase 122g — per-source `discovery:` block (ADR-031). Declare
    # every channel the publisher actually exposes (verify via the
    # audit CLI above + manual cross-check). The four-channel model
    # is platform-agnostic — future Twitter / Reddit / Mastodon
    # crawlers contribute their own channel names under the same block.
    discovery:
      # Channel A — XML sitemap (cheapest, highest-coverage when present).
      sitemap_urls:
        - https://www.bbc.co.uk/sitemaps/news.xml
      # Channel B — RSS / Atom feeds. PLURAL since Phase 122g. If the
      # publisher catalogues multiple feeds, declare all of them; the
      # publisher curated the set.
      rss_hint_urls:
        - https://feeds.bbci.co.uk/news/rss.xml
        # - https://feeds.bbci.co.uk/news/world/rss.xml  # if exposed
        # - https://feeds.bbci.co.uk/news/business/rss.xml  # if exposed
      # Channel C — HTML sitemap page (operator-discoverable; usually
      # absent on publishers that ship a proper XML sitemap).
      # html_sitemap_urls: []
      # Channel D — date-indexed archive walker. Configure when the
      # publisher exposes `?date=...`-style date-indexed navigation.
      # archive_index:
      #   url_template: 'https://www.example.com/archive?date={date}'
      #   date_format: '%Y-%m-%d'
      #   granularity: daily   # operator: verify by sampling two distant dates
      #   article_url_pattern: '^https://www\.example\.com/.+\.html$'
      # Phase 122g — minimum URLs per discovery run for the underflow
      # alert (two-consecutive-runs gate). Set conservatively after the
      # first crawl establishes the empirical baseline.
      expected_floor_per_run: 50
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

The default `url_filter` and `content_filter` values are universal — copy them verbatim. **Do not add section-level editorial filters** (no `/sport/` exclusions, no `/opinion/` exclusions) per WP-006 §3 / ADR-028. Per-article discourse-function imprecision is addressed in Phase 122a, not at the crawler.

**Discovery cost ladder.** Surface A (XML sitemap) is the cheapest — one or a few HTTP fetches yield the full URL universe. Surface B (RSS) is similarly cheap. Surface C (date-indexed archive) is the expensive one: one HTTP fetch per day in the window. At a 1 s polite delay and a 5-year window, Surface C alone is ≈ 30 minutes of discovery before the article-fetch stage even begins. Only configure Surface C when Surfaces A/B do not expose the historical depth the probe requires; never as redundancy.

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
