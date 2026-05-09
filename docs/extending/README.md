# Extending AĒR

This section answers a single question: **"I want to extend AĒR with X — where do I start, what do I touch, and what does it cost?"**

Each guide in this section is a *sequencing index*, not a substitute for the canonical procedures. The canonical procedures live in:

- [Scientific Operations Guide](../operations/scientific_operations_guide.md) — *why* this methodology
- [Operations Playbook](../operations/operations_playbook.md) — *what to type*
- [Working Papers](../methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md) — *the scientific foundation*

The guides here orchestrate those documents for specific extension scenarios.

---

## Quick reference

| What you want to do | Read this | Touched files | Realistic effort |
| :--- | :--- | :--- | :--- |
| **Add a new source on an existing platform** (e.g. another news website to the web-crawler) | [add-a-source.md](add-a-source.md) | 2 (YAML entry + Postgres seed migration) | ~5 minutes |
| **Add a new probe** (cultural context with 1+ sources) | [add-a-probe.md](add-a-probe.md) | ~10 (3 migrations, crawler config, dossier, content) | 0.5–2 days engineering + content writing |
| **Add a new language** (NER, sentiment, calendar coverage) | [add-a-language.md](add-a-language.md) | 1–8 (varies with optional tiers) | 5 min minimum, ~3 days for full Tier 1+2 |
| **Add a new source type / platform class** (Twitter, Reddit, Mastodon, …) | [add-a-source-type.md](add-a-source-type.md) | 5–8 (crawler binary, adapter, registry, SilverMeta subclass, tests) | 1–3 days |
| **Add a new extractor** (new metric, new algorithm) | [add-an-extractor.md](add-an-extractor.md) | 4–8 (extractor, registration, tests, provenance, validity scaffold) | 0.5–2 days for Tier 1, longer for Tier 2 |
| **Understand long-term scaling** (path to 100+ probes) | [scalability-roadmap.md](scalability-roadmap.md) | — | 15 min reading |

---

## The four-layer architecture (Phase 122)

AĒR's extension cost is bounded by a four-layer model. Each layer answers a different question — *"how often must I write code at this layer?"* — and the answer is "less often than you'd think":

1. **Layer 1 — Universal core** (`pkg/`, `services/ingestion-api/`, `services/bff-api/`, the analysis-worker core, the Silver/Gold schemas). Written once, ages well. Adding a new probe, source, language, platform, or extractor **does not touch Layer 1**.
2. **Layer 2 — Source-agnostic harmonisation** (the `SourceAdapter` protocol, the `MetricExtractor` pipeline, the Capability Manifest, the BiasContext / DiscourseContext envelopes). Mostly done. New extensions occasionally amend it (e.g. a new manifest section for a new metric tier), but adding a probe / source / language **does not touch Layer 2** as code.
3. **Layer 3 — Collection-method-specific** (one crawler per platform class — `crawlers/web-crawler/` for HTML news sites; future `crawlers/web-crawler-js/` for JS-rendered SPAs; future per-platform crawlers for Twitter, Reddit, Mastodon, Telegram, YouTube, Bluesky). Written **once per platform class**, then every source on that platform is YAML-only.
4. **Layer 4 — Per-source configuration** (a YAML entry plus a Postgres seed migration). Takes minutes per source.

| Platform class | Crawler binary | Status |
| :--- | :--- | :--- |
| HTML news (server-rendered) | `crawlers/web-crawler/` | Phase 122 (live) |
| HTML news (JS-rendered) | `crawlers/web-crawler-js/` (Playwright) | Deferred — when first probe needs it |
| Twitter / X | `crawlers/twitter-crawler/` | Future |
| Reddit | `crawlers/reddit-crawler/` | Future |
| Mastodon (Fediverse) | `crawlers/mastodon-crawler/` | Future |
| Telegram | `crawlers/telegram-crawler/` | Future |
| YouTube | `crawlers/youtube-crawler/` | Future |
| Bluesky | `crawlers/bluesky-crawler/` | Future |
| RSS-only legacy | *(retired Phase 122; see `crawlers/_archived/rss-crawler/MIGRATED.md`)* | Retired |

**The answer to *"how often do I write a new crawler?"* is "once per platform class, then YAML for each source on that platform."** This is the architectural payoff of the Source Adapter pattern (ADR-002 / ADR-028) for Layer-3 collection methods.

---

## Extension philosophy

Three principles govern AĒR's extension model. They appear repeatedly in the guides below; this is the short version.

**1. Substrate vs. content.** AĒR distinguishes architectural substrate (multilingual routing, equivalence registry, probe registry, extractor protocol) from content (a specific German sentiment lexicon, a specific French BERT model, the Probe 0 dossier). Substrate changes are rare and ADR-worthy. Content changes are frequent and follow patterns. Most extensions are content changes, even though they may feel architectural at first glance.

**2. Multilingual-by-construction where possible, language-routed otherwise.** Phase 116 made the analysis worker route by `detected_language`. Components that are language-agnostic (Wikidata entity linking, BERTopic with multilingual embeddings, the topic view modes, the BFF API) need no per-language work. Components that are language-bound (NER models, deterministic sentiment lexica, dependency-parse-based negation handlers) extend per language via configuration, not per-language code branches.

**3. `provisional_engineering` is honest.** Solo-developer probe addition cannot perform WP-001 §4.4 Steps 1–2 (Area Expert Nomination + Peer Review). The architecture handles this with `review_status='provisional_engineering'` and `validation_status='unvalidated'` — every consumer sees the limitation. Do not paper over this with fabricated reviewer names or invented function weights.

---

## What is *not* in this section

- **How to add a new methodology layer** (e.g. a new working paper). That is a scientific decision documented in `docs/methodology/en/`, not an extension procedure.
- **How to add a new dashboard surface or view mode.** That is design work driven by the Design Brief and visualization guidelines.
- **How to onboard interdisciplinary contributors.** That is a project-governance concern, currently out-of-band.

---

## Cross-references

- [Arc42 §8.15 Probe Dossier Pattern](../arc42/08_concepts.md) — the canonical concept that several guides operationalise.
- [ROADMAP](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the time-axis view of when extensions land; this section is the orthogonal extension-axis view.