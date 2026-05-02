# Extending AĒR

This section answers a single question: **"I want to extend AĒR with X — where do I start, what do I touch, and what does it cost?"**

Each guide in this section is a *sequencing index*, not a substitute for the canonical procedures. The canonical procedures live in:

- [Scientific Operations Guide](../scientific_operations_guide.md) — *why* this methodology
- [Operations Playbook](../operations_playbook.md) — *what to type*
- [Working Papers](../methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md) — *the scientific foundation*

The guides here orchestrate those documents for specific extension scenarios.

---

## Quick reference

| What you want to do | Read this | Touched files | Realistic effort |
| :--- | :--- | :--- | :--- |
| **Add a new probe** (cultural context with 1+ sources) | [add-a-probe.md](add-a-probe.md) | ~10 (3 migrations, crawler, dossier, content) | 0.5–2 days engineering + content writing |
| **Add a new language** (NER, sentiment, calendar coverage) | [add-a-language.md](add-a-language.md) | 1–8 (varies with optional tiers) | 5 min minimum, ~3 days for full Tier 1+2 |
| **Add a new source type** (Forum, social, API beyond RSS) | [add-a-source-type.md](add-a-source-type.md) | 3–6 (adapter, registry, optional SilverMeta subclass, tests) | 1–3 days |
| **Add a new extractor** (new metric, new algorithm) | [add-an-extractor.md](add-an-extractor.md) | 4–8 (extractor, registration, tests, provenance, validity scaffold) | 0.5–2 days for Tier 1, longer for Tier 2 |
| **Understand long-term scaling** (path to 100+ probes) | [scalability-roadmap.md](scalability-roadmap.md) | — | 15 min reading |

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