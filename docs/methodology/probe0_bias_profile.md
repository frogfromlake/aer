# Probe 0 Bias Profile

This document records the known structural biases of Probe 0 data sources following the WP-003 platform-bias documentation framework. The purpose is transparency: every source entering AER carries documented biases so that downstream consumers can interpret metrics with awareness of selection effects.

WP-003 mandates a "document, don't filter" approach — non-human actors and platform biases are recorded as metadata (`BiasContext`), not used as exclusion criteria.

---

## Platform: RSS (Public Feeds)

All Probe 0 sources are accessed via public RSS feeds. The RSS protocol imposes a uniform bias profile across all sources in this probe.

### BiasContext Values

| Field | Value | Rationale |
|-------|-------|-----------|
| `platform_type` | `rss` | Content delivered via RSS/Atom syndication protocol |
| `access_method` | `public_rss` | No authentication, no paywall, no rate limiting |
| `visibility_mechanism` | `chronological` | Items ordered by publication time; no algorithmic ranking |
| `moderation_context` | `editorial` | Content is editorially curated before publication |
| `engagement_data_available` | `false` | RSS provides no likes, shares, comments, or view counts |
| `account_metadata_available` | `false` | RSS provides no author account age, follower count, or verification status |

### Structural Biases

1. **No Engagement Signal.** RSS feeds carry no engagement metrics. Unlike social media platforms, there is no way to measure audience reception, amplification, or virality. Metrics derived from RSS sources reflect only publication behavior, not consumption or response patterns.

2. **No Algorithmic Amplification.** RSS items appear in chronological order. There is no recommendation algorithm, trending mechanism, or personalization that could selectively amplify certain content. This eliminates one major source of platform-induced bias but also means that publication frequency directly determines content volume without any engagement-based filtering.

3. **Editorial Curation Bias.** All content in Probe 0 feeds has passed through an editorial process. This means the corpus excludes unedited user-generated content, spontaneous discourse, and real-time reactions. The editorial filter systematically selects for institutional perspectives and formal register.

4. **Publication Frequency Bias.** Sources with higher publication rates (e.g., tagesschau.de at approximately 50 articles/day) dominate the corpus volume compared to lower-frequency sources (e.g., bundesregierung.de at approximately 5 articles/day). Aggregate metrics that do not normalize by source will disproportionately reflect high-frequency publishers.

5. **Absence of Deletion Signal.** RSS feeds do not indicate when articles are retracted, corrected, or removed. Once ingested, a document remains in the Bronze layer regardless of its current publication status.

---

## Source: tagesschau.de

**Operator:** ARD (Arbeitsgemeinschaft der offentlich-rechtlichen Rundfunkanstalten der Bundesrepublik Deutschland)
**Funding:** Public broadcasting fee (Rundfunkbeitrag)
**Feed URL:** `https://www.tagesschau.de/index~rss2.xml`

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
**Feed URL:** `https://www.bundesregierung.de/breg-de/feed`

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

These biases are structural and expected. They are documented here so that future analysis layers, visualization tools, and consumers of AER data can account for them. The `BiasContext` metadata attached to every Silver document makes these biases machine-readable alongside this human-readable profile.

---

## Deferred: Authenticity Extractors

WP-003 section 8.2 proposes authenticity extractors (bot detection, coordination detection) for platforms where non-human actors are present. For RSS sources, these are not applicable — RSS feeds are editorially controlled and do not contain user-generated content. Authenticity extractors are deferred to phases that introduce social media or forum adapters, where the `CorpusExtractor` path (R-9) will enable the required cross-document analysis.
