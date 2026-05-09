# Probe 0 — Platform Bias Assessment (WP-003)

This document records the known structural biases of Probe 0 data sources following the WP-003 platform-bias documentation framework. The purpose is transparency: every source entering AĒR carries documented biases so that downstream consumers can interpret metrics with awareness of selection effects.

WP-003 mandates a "document, don't filter" approach — non-human actors and platform biases are recorded as metadata (`BiasContext`), not used as exclusion criteria.

> **Cross-reference.** As of Phase 122 (ADR-028), the structured `BiasContext` values below are emitted into every Silver record by `WebAdapter` (`services/analysis-worker/internal/adapters/web.py`). The pre-Phase-122 `RssAdapter` (`services/analysis-worker/internal/adapters/rss.py`) emitted an `rss`-flavoured `BiasContext` and remains registered for backward compatibility while the 90-day Bronze TTL window ages out residual `rss/...` keys. [Scientific Operations Guide → **Workflow 5: Assessing Bias for a Data Source**](../../operations/scientific_operations_guide.md#workflow-5-assessing-bias-for-a-data-source) describes how these values are produced for new sources.

---

## Platform: Public Web (Phase 122)

Probe 0 sources are accessed via full-article web crawling against their public sitemaps. The collection method imposes a uniform bias profile across all sources in this probe.

### BiasContext Values (post-Phase-122)

| Field | Value | Rationale |
|-------|-------|-----------|
| `platform_type` | `web` | Content fetched as raw HTML from public web pages (Phase 122 / ADR-028) |
| `access_method` | `public_web` | No authentication, no paywall on the indexed surface, polite-defaults crawl honouring robots.txt |
| `visibility_mechanism` | `editorial_homepage_and_sitemap` | Articles surface via editor-curated homepage routing AND via the sitemap index — both are editorial decisions; sitemap inclusion is not algorithmic ranking |
| `moderation_context` | `editorial` | Content is editorially curated before publication |
| `engagement_data_available` | `false` | The web HTML carries no likes, shares, or view counts that the crawler ingests |
| `account_metadata_available` | `false` | The web HTML carries no author-account follower count or verification status |

### Structural Biases

1. **No Engagement Signal.** The crawled HTML carries no engagement metrics that AĒR ingests. Unlike social media platforms, there is no way to measure audience reception, amplification, or virality. Metrics derived from these sources reflect only publication behavior, not consumption or response patterns.

2. **No Algorithmic Amplification.** Discovery is sitemap-driven and chronological — every article surfaced by the sitemap is ingested unless a technical filter (asset extension, search/legal-page prefix) excludes it. There is no recommendation algorithm, trending mechanism, or personalization that selectively amplifies certain content. Publication frequency directly determines content volume.

3. **Editorial Curation Bias.** All content in Probe 0 has passed through an editorial process. The corpus excludes unedited user-generated content, spontaneous discourse, and real-time reactions. The editorial filter systematically selects for institutional perspectives and formal register. **(Removed by Phase 122):** the structural bias of "RSS-only summary visibility" — analysis is no longer constrained to RSS title-plus-description snippets.

4. **Depth-of-Article-Body Access.** **(New, Phase 122 / WP-003 §3.2.)** Article-body access depends on paywall handling. Probe 0's two sources are currently free, but as the system extends to commercial publishers (a future probe), the same crawl semantics will encounter paywalls — yielding either a Tier-A-only DLQ rejection or a partial-body Silver record depending on how the source serves anonymous traffic. This is a structural bias dimension that did not exist on the RSS-summary collection method (RSS exposes the description regardless of paywall).

5. **Metadata-Richness Asymmetry Between Sources.** **(New, Phase 122.)** Sources differ in how completely they emit Schema.org / OpenGraph metadata. tagesschau.de typically populates JSON-LD `NewsArticle` blocks fully; bundesregierung.de often relies on OpenGraph + heuristics. The `WebMeta.extraction_methods` provenance markers expose this asymmetry per field; the Coverage Map (Phase 125a) will surface it system-wide. Cross-source aggregations on Tier-C fields must be filtered by `extraction_method` to avoid conflating "field absent because the source does not emit it" with "field absent because the article omitted it".

6. **Publication Frequency Bias.** Sources with higher publication rates (e.g., tagesschau.de at approximately 50 articles/day) dominate the corpus volume compared to lower-frequency sources (e.g., bundesregierung.de at approximately 5 articles/day). Aggregate metrics that do not normalize by source will disproportionately reflect high-frequency publishers.

7. **Absence of Deletion Signal.** The crawl does not indicate when articles are retracted, corrected, or removed at the source. Once ingested, a document remains in the Bronze layer regardless of its current publication status. The Tier-C `correction_notice` field, when populated, surfaces source-side correction signals; absence of the field is not evidence of absence of correction.

---

## Source: tagesschau.de

**Operator:** ARD (Arbeitsgemeinschaft der offentlich-rechtlichen Rundfunkanstalten der Bundesrepublik Deutschland)
**Funding:** Public broadcasting fee (Rundfunkbeitrag)
**Discovery URL:** `https://www.tagesschau.de/sitemap.xml` (primary, Phase 122); `https://www.tagesschau.de/index~rss2.xml` (RSS hint only — body fetched from HTML)

### Known Biases

1. **State-Funding Bias.** As a publicly funded broadcaster, tagesschau.de operates under a legal mandate for balanced reporting (Staatsvertrag). However, the funding structure creates an institutional dependency on political consensus around the broadcasting fee. This does not imply partisan bias but does create an incentive structure favoring institutional stability narratives.

2. **Editorial Bias Toward Institutional Sources.** tagesschau.de relies heavily on official sources (government press releases, wire agencies, institutional spokespersons). This is standard for public broadcasters but systematically underrepresents grassroots, activist, or marginalized perspectives.

3. **German-Language Monolingualism.** The feed is exclusively German-language. International events are reported through a German editorial lens, including translation choices, framing, and source selection.

4. **High Publication Frequency.** Approximately 50 articles per day across all topic areas. This makes tagesschau.de the dominant volume contributor in Probe 0, which must be accounted for in any cross-source comparison.

5. **News Focus.** The feed covers current affairs, politics, economy, sports, science, and culture. It does not cover opinion pieces, reader letters, or user-submitted content. The discourse function is primarily `epistemic_authority` (WP-001 classification).

---

## Source: bundesregierung.de

**Operator:** Presse- und Informationsamt der Bundesregierung (Federal Press Office)
**Funding:** Federal government budget
**Discovery URL:** `https://www.bundesregierung.de/sitemap.xml` (primary, Phase 122); `https://www.bundesregierung.de/service/rss/breg-de/1151242/feed.xml` (RSS hint only)

### Known Biases

1. **Government Communication Bias.** bundesregierung.de is the official communication channel of the German federal government. Content is authored or approved by the Federal Press Office. By definition, this source presents the government's perspective and framing of events.

2. **Power Legitimation Function.** Unlike tagesschau.de, which has a journalistic mandate, bundesregierung.de explicitly serves institutional agenda-setting. Its discourse function is `power_legitimation` (WP-001 classification). Metrics derived from this source measure government communication strategy, not independent reporting.

3. **Low Publication Frequency.** Approximately 5 articles per day. This source is significantly underrepresented in volume-based aggregate metrics compared to tagesschau.de.

4. **Formal Register.** Content uses formal, institutional language with standardized phrasing. Sentiment metrics derived from this source are expected to show low variance and neutral-to-positive polarity, reflecting the communicative norms of government press offices rather than the underlying discourse dynamics.

5. **Selective Topic Coverage.** The feed covers government decisions, policy announcements, chancellor statements, and ministerial activities. Topics not on the government agenda are absent. This creates a systematic gap in issue coverage compared to journalistic sources.

---

## Implications for Metric Interpretation

- **Sentiment scores** from Probe 0 reflect editorial and institutional communication norms, not public opinion or organic discourse sentiment.
- **Entity counts** are biased toward institutional actors (politicians, ministries, organizations) due to the editorial focus of both sources.
- **Word counts** reflect editorial production volume, not discourse engagement or reach.
- **Temporal patterns** reflect editorial publishing schedules (working hours, weekday concentration), not organic discourse rhythms.
- **Cross-source comparisons** must normalize for publication frequency; raw aggregate metrics disproportionately reflect tagesschau.de.

These biases are structural and expected. They are documented here so that future analysis layers, visualization tools, and consumers of AĒR data can account for them. The `BiasContext` metadata attached to every Silver document makes these biases machine-readable alongside this human-readable profile.

---

## Deferred: Authenticity Extractors

WP-003 section 8.2 proposes authenticity extractors (bot detection, coordination detection) for platforms where non-human actors are present. For RSS sources, these are not applicable — RSS feeds are editorially controlled and do not contain user-generated content. Authenticity extractors are deferred to phases that introduce social media or forum adapters, where the `CorpusExtractor` path (R-9) will enable the required cross-document analysis.
