# Probe 0 — German Institutional Web

**Probe ID:** `probe-0-de-institutional-web` *(historical; collection method migrated to web in Phase 122 — the probe-id string is preserved for backward compatibility with cached dashboard URLs and BFF content keys; the dossier directory was renamed to `probe-0-de-institutional-web/` for accuracy)*
**Cultural region:** Germany (`DE`), German-language (`de`)
**Sources:** `tagesschau.de`, `bundesregierung.de`
**Collection method:** Full-article web crawling (Phase 122 / ADR-028). Migrated from RSS-summary ingestion on 2026-05-08.
**Status:** Engineering calibration probe — *not* scientifically motivated.

> **Migration note (Phase 122):** Probe 0 transitioned from RSS-feed ingestion to a generalised Python web crawler that fetches full article HTML and stores it verbatim in Bronze. The analysis worker's `WebAdapter` runs the trafilatura + extruct + htmldate extraction pipeline at the Silver boundary, producing a five-tier `WebMeta` envelope. Source identity is unchanged — only the collection method.
>
> The rename `probe-0-de-institutional-web/` → `probe-0-de-institutional-web/` is at the dossier-directory level only. Old links of the form `docs/probes/probe-0-de-institutional-web/...` redirect to the renamed directory via the mkdocs nav update.

---

## Purpose

Probe 0 is AĒR's first real data source. Its purpose is **engineering pipeline calibration**, not scientific representativeness or hypothesis testing. The two German institutional sources were chosen because they are public, structured, low-volume, in a single language, and free of authentication, engagement signals, and algorithmic amplification — the simplest possible end-to-end load for the Bronze → Silver → Gold pipeline.

The full rationale, including the explicit acknowledgment that source selection was driven by engineering convenience rather than research design, is documented in **Arc42 §13.10**. The Probe Classification Process (WP-001 §4.4) has *not* been executed for these sources; their `source_classifications` row carries `review_status = 'provisional_engineering'` and `function_weights = NULL`.

## Sources

| Source | Operator | Funding | Discovery surface | Volume |
| :--- | :--- | :--- | :--- | :--- |
| `tagesschau.de` | ARD (public broadcasting) | Rundfunkbeitrag | `https://www.tagesschau.de/index~rss2.xml` (sole channel — `sitemap.xml` returns HTML 404; body fetched from HTML, RSS body never consumed) | ≈ 150–300 articles/day published; ≈ 70-item RSS window |
| `bundesregierung.de` | Federal Press Office | Federal budget | Three `service-sitemap-*-sitemap_index.xml` files declared in robots.txt + `https://www.bundesregierung.de/service/rss/breg-de/1151242/feed.xml` (peer-equal channels since Phase 122e F-A1; sitemap is service-only — actual `/aktuelles/` news content surfaces via RSS) | ≈ 5–15 articles/day |

## Engineering Calibration Status

The following decisions are recorded as **engineering defaults**, not scientific choices. They will be revisited under interdisciplinary CSS collaboration (Arc42 §13.5).

- **Etic classification** assigned by the engineering team without expert nomination or peer review (WP-001 §4.4 Steps 1–2 outstanding).
- **`function_weights = NULL`** — quantification of primary/secondary discourse function shares requires the missing classification process.
- **`BiasContext` values** are derived from the structural properties of the web-crawl protocol (Phase 122; previously the RSS protocol) and from publicly known operator characteristics — not from independent platform audits.
- **`min_meaningful_resolution`** is a heuristic recomputed against the post-Phase-122 publication-rate signal — sitemap-driven discovery cadence supersedes the RSS poll cadence.
- **No validation studies** have been conducted for any extractor in the Probe 0 context. Every metric reports `validation_status = unvalidated`.

## Phase 122 Migration — Empirical Justification

The migration from RSS to full-article crawling produces dramatically richer inputs to every downstream extractor. The before/after distributions below are recorded here as the empirical justification for the Phase-122 cutover; they are populated post-cutover by the operator after the spot-check invariants in `make crawl-probe0` complete (the invariants themselves are documented in ROADMAP §122 / TESTING.md).

| Distribution | RSS-era baseline | Web-era target | Status |
| :--- | :--- | :--- | :--- |
| Median `word_count` per article | ≈ 50 (RSS title + description) | ≥ 500 (full article body) | populated by operator post-cutover |
| Median `SilverCore.timestamp` spread across the corpus | single cluster around the crawl-invocation moment (legacy RSS-adapter `timestamp = event_time` bug) | spans the actual publication-date range of the corpus | populated by operator post-cutover |
| Tier-B `published_date` extraction-method coverage | n/a (RSS pubDate only) | ≥ 80 % `extraction_method ∈ {'json_ld', 'open_graph', 'microdata'}` | populated by operator post-cutover |
| Tier-D `structured_data` coverage | n/a | ≥ 80 % non-empty | populated by operator post-cutover |

### Per-tier field-population statistics

`WebMeta` carries five tiers; metadata richness varies per source. The table below is updated by the operator after each significant crawl window so the dossier reflects the empirical population rate, not aspirational targets.

| Tier | Field | tagesschau.de population rate | bundesregierung.de population rate | Notes |
| :--- | :--- | :--- | :--- | :--- |
| A | canonical_url, original_url, fetch_at, http_status, html_lang, title | 100 % | 100 % | Tier-A is mandatory; missing → DLQ. |
| B | published_date | populated post-cutover | populated post-cutover | JSON-LD path expected on both sources. |
| B | author | populated post-cutover | populated post-cutover | OpenGraph fallback covers tagesschau on opinion pieces. |
| C | editor | populated post-cutover | populated post-cutover | Tier-C captures only when the source emits the field. |
| C | paywall_status | populated post-cutover | populated post-cutover | Both sources currently free; field populated as `false`. |
| D | structured_data | populated post-cutover | populated post-cutover | Verbatim extruct dump; sized 2–5 KB / article typical. |

## Exit Criteria

Probe 0 graduates from `provisional_engineering` to `pending` (and ultimately `reviewed`) only when:

1. WP-001 §4.4 Steps 1–2 are completed for both sources (area expert nomination + peer review).
2. `function_weights` are quantified and inserted as a new `source_classifications` row (the composite primary key allows temporal tracking; old rows are not overwritten).
3. At least one validation study has been recorded in `aer_gold.metric_validity` for the `de:rss:epistemic_authority` and `de:rss:power_legitimation` context keys.
4. An ethical review has been completed against `docs/templates/observer_effect_assessment.yaml` (see `observer_effect.md`).

Until then, all metrics derived from Probe 0 must be presented to consumers as **provisional** via the `validation_status` field in `GET /api/v1/metrics/available` and the `/provenance` endpoint.

---

## WP Coverage Matrix

The Probe Dossier Pattern documents per-probe scientific context along the six Working Paper axes. Per-metric methodological provenance is system-wide (`metric_provenance.yaml`) and per-validation context is system-wide (`metric_validity`, `metric_baselines`, `metric_equivalence`) — those are referenced from this dossier rather than duplicated inside it.

| Working Paper | Topic | Probe-specific location |
| :--- | :--- | :--- |
| **WP-001** | Functional Probe Taxonomy | [`classification.md`](classification.md) |
| **WP-002** | Metric Validity & Sentiment Calibration | System-wide: `aer_gold.metric_validity` (per `(metric_name, context_key)` pair). No probe-specific file — validation studies are scoped to context keys, not probes. |
| **WP-003** | Platform Bias & Non-Human Actors | [`bias_assessment.md`](bias_assessment.md) |
| **WP-004** | Cross-Cultural Comparability | System-wide: `aer_gold.metric_baselines` and `aer_gold.metric_equivalence`. Cross-probe by definition — a single probe cannot establish equivalence. |
| **WP-005** | Temporal Granularity | [`temporal_profile.md`](temporal_profile.md) |
| **WP-006** | Observer Effect & Reflexivity | [`observer_effect.md`](observer_effect.md) |

---

## See Also

- Arc42 §13.10 — full rationale for Probe 0 source selection.
- Arc42 §8.15 — Probe Dossier Pattern as a cross-cutting concept.
- Operations Playbook → "Probe Dossier" section — how to author a new dossier.
- [Scientific Operations Guide](../../operations/scientific_operations_guide.md) — workflows that produce the data referenced from this dossier; see in particular the [Probe 0 End-to-End Walkthrough](../../operations/scientific_operations_guide.md#probe-0-end-to-end-walkthrough).
