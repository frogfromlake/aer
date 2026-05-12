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

A web crawler can only find articles through what the publisher **exposes** for machine consumption. Three discovery surfaces are universally usable, in this order of preference:

| Surface | When the publisher exposes it | What we configure | Coverage |
| :--- | :--- | :--- | :--- |
| **(A) XML sitemap** | `Sitemap:` directive in `robots.txt` (preferred), OR a `/sitemap.xml` / `/sitemap_index.xml` that returns `200 OK` | `sitemap_urls: [...]` (one or more roots) | Full archive — typically every URL the publisher considers canonical |
| **(B) RSS / Atom feed** | A feed URL the publisher advertises, usually at `/rss` or as a `<link rel="alternate" type="application/rss+xml">` on the homepage | `rss_hint_url: ...` (single URL) | Sliding window of the most recent ≈ 50–200 items. **Not** an archive |
| **(C) Date-indexed HTML archive** | A page parameterised by date — e.g. `/archiv?datum=YYYY-MM-DD` — that the publisher uses for navigation | `archive_index: { url_template, date_format, article_url_pattern }` | Per-day list going back as far as the publisher chooses to expose. Used **only when** A is absent (no sitemap and no `Sitemap:` directive) |

Methodologically — every one of these is *publisher curation*, not researcher curation: the publisher built the surface, the publisher decides what's on it, we ingest the listed URLs verbatim. **Do not** hand-pick a subset of feeds (e.g. only `politik` + `wirtschaft`) — that is researcher selection bias per WP-006 §3 / ADR-028 and inverts the Manifesto's "unaltered mirror" principle. If the publisher advertises N RSS feeds, configure all N (or none); never pick "the relevant ones."

A source can use multiple surfaces — configurations are additive. URL collisions deduplicate on the consumer side, with sitemap entries winning over RSS/archive entries (the sitemap entry carries the canonical `lastmod` and `sitemap_section` context).

**Rule of thumb**: prefer A > B > C. Use B as a freshness-sort cue alongside A. Use C when the publisher exposes neither A nor a comprehensive RSS — Probe 0's `tagesschau.de` is the canonical example: no XML sitemap, only a 2-feed RSS catalogue (top stories in two formats), and a deep date-indexed archive at `/archiv?datum=`.

## Worked example — adding `bbc.co.uk` to a hypothetical English probe

This is illustrative; no English probe exists at the time of writing.

### 1. Verify which discovery surfaces the publisher exposes

```bash
# Surface A — XML sitemap.
curl -A 'AerWebCrawler/0.1 (+https://aer.example/about; mailto:contact@example)' \
     https://www.bbc.co.uk/robots.txt | grep -iE 'allow|disallow|sitemap'
curl -I https://www.bbc.co.uk/sitemaps/news.xml

# Surface B — RSS / Atom catalogue. Look for an RSS index page or
# `<link rel="alternate" type="application/rss+xml">` on the homepage.
curl -sL https://www.bbc.co.uk/ \
  | grep -oE 'href="[^"]+(rss|atom)[^"]*"' | sort -u

# Surface C — date-indexed archive. Try common patterns:
#   /archive, /archiv, /archive?date=, /sitemap?date=, /YYYY/MM/DD/
# If the publisher exposes one, sample two distant dates and confirm
# they return distinct article lists.
```

Confirm:
- our `User-Agent` is not in any `Disallow` block;
- at least one surface returns `200 OK` and exposes article URLs;
- if Surface C is in play, sample-fetch two dates one year apart and verify both return populated article lists (i.e. it is an archive, not a homepage snapshot).

Document the verification — including which surface(s) we settled on, and *why* — in the probe's dossier (`docs/probes/<probe-id>/temporal_profile.md` under "Discovery surface"). The asymmetry between sources' discovery surfaces is a recorded structural bias (see Probe 0's `bias_assessment.md` Structural Bias #8).

### 2. Add a YAML entry under the probe's source list

Open `crawlers/web-crawler/probes/<probe-id>/sources.yaml` and append:

```yaml
sources:
  # … existing sources …

  - name: bbc-news
    # Surface A — XML sitemap (preferred).
    sitemap_urls:
      - https://www.bbc.co.uk/sitemaps/news.xml
    # Surface B — RSS feed (freshness-sort hint).
    rss_hint_url: https://feeds.bbci.co.uk/news/rss.xml
    # Surface C — date-indexed HTML archive. Configure ONLY if Surface A
    # is absent. Do NOT configure all three "to be safe" — the surfaces
    # already overlap and Surface C costs ~ 1 HTTP fetch per day in the
    # window. Example shape (BBC has no archive of this form, so this
    # block stays absent for bbc-news in production):
    #
    # archive_index:
    #   url_template: 'https://www.example.com/archive?date={date}'
    #   date_format: '%Y-%m-%d'
    #   article_url_pattern: '^https://www\.example\.com/.+\.html$'
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
