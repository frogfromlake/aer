# Scalability Roadmap

Project AĒR is an instrument designed to observe global discourse. Its long-term trajectory targets coverage of dozens to hundreds of cultural-linguistic contexts — not two or three. This document is the honest accounting of where today's architecture supports that trajectory, where it does not, and which open questions need to be answered before they become emergencies.

This is a *living* document. It gets updated when the scaling situation changes — when a new ADR resolves an open question, when a phase ships that turns a "not yet" into a "✅", or when reality reveals a bottleneck this roadmap missed.

---

## What scales today

These layers are multilingual-by-construction or trivially additive. Adding the 50th language or 100th probe touches them at most via configuration:

| Layer | Why it scales | Verified up to |
| :--- | :--- | :--- |
| Phase 116 Language Router | `lingua-py` covers 75+ languages out of the box; routing is pure config | Tested with `de`, `en`; designed for any ISO 639-1 |
| Phase 118 Wikidata Entity Linking | Wikidata pflegt labels in 300+ languages; the type-bucket build is language-parametrised | DE/EN/FR baked in; adding a language = next quarterly rebuild |
| Phase 120 BERTopic with E5-large | `intfloat/multilingual-e5-large` covers 100 languages on the MTEB benchmark; per-language partitioning is automatic | All 100 E5 languages |
| Phase 121 Topic View Modes | Sprachagnostisches frontend rendering | Any number of language partitions in parallel |
| Phase 115 Equivalence Registry | Default-deny — empty table scales trivially; equivalence grants are individual scientific decisions | Up to N×(N−1)/2 probe-pair entries |
| Cultural Calendar | Linear per-region YAML files; trivial to author and version | Verified for `de`; `fr` planned (Phase 125) |
| Phase 114 Multi-Probe Composition | ClickHouse `IN (...)` indexed scan; scales with index, not probe count | Tested with 1 probe; designed for arbitrary N |
| Probe Registry | Pure config in BFF | Linear additive |
| Frontend rendering | Sprachagnostisches; view modes render arbitrary language partitions in parallel | Limited only by visual complexity at high N |

**Engineering verdict:** the architectural substrate is ready for 100+ probes.

---

## What does not yet scale

These are real bottlenecks. They are not blockers today — they will become painful at specific probe-count thresholds. Each entry below names the threshold, the architectural question, the proposed answer, and the realistic timeline.

### NER model availability

**Threshold:** Probe 5–10.

**Problem:** spaCy ships trained NER models for ~25 languages. Many languages of interest (Suaheli, Tagalog, Bengali, Khmer, Amharic, ...) have no off-the-shelf NER model. The Phase 116 absence-not-wrong guarantee handles this: no model → no entity spans → honest gap row in `metric_validity`. But a probe without NER has reduced analytical capability — no entity co-occurrence network, no Wikidata linking.

**Proposed answer:** the Probe Coverage Map (see open phase below) makes the structural asymmetry visible. Probes without NER show explicit "NER coverage: absent" in the dossier and the dashboard. This is honest, not a defect — but it must be visualised, not buried.

**Long-term mitigation:** as multilingual NER transformers mature (e.g. mBERT-NER-tagger, XLM-RoBERTa-NER models on Hugging Face), a Tier-2 multilingual NER fallback can fill gaps where spaCy has no model. Out-of-scope for current iterations; ADR-worthy when it becomes relevant.

### Tier-1 sentiment lexicon coverage

**Threshold:** Probe 3–5.

**Problem:** Deterministic sentiment lexicons (SentiWS for `de`, FEEL for `fr`, VADER for `en`, AFINN multilingual but thin) exist for maybe 15–20 languages. For a probe in a language without a lexicon, Tier 1 sentiment is unavailable. The Phase 116 language guard ensures graceful absence; Phase 117's custom-lexicon-hook accepts community-contributed lexica.

**Proposed answer:** same as NER — the gap is honest, the Coverage Map shows it. The custom-lexicon-hook lets a domain expert contribute a lexicon for a previously unsupported language without code changes.

**Long-term mitigation:** Tier-2 multilingual BERT (see next section) becomes the *primary* sentiment signal for languages without Tier 1 coverage, with the absence of Tier 1 documented.

### Tier-2 sentiment strategy ⚠️ **architecturally unresolved**

**Threshold:** Probe 3 (i.e., now — Probe 1 already triggers this).

**Problem:** Phase 119 ships `mdraw/german-news-sentiment-bert` as the German Tier 2 extractor. Phase 125's NLP section sketches `cmarkea/distilcamembert-base-sentiment` for French following the same pattern. **At 50 languages, maintaining 50 separate per-language BERT extractors is operationally untenable**: 50 pinned model revisions, 50 determinism CI gates, exploding Docker image size, 50 memory footprints.

The methodological tension is real: per-language models trained on the news domain (like `mdraw`) offer better in-domain accuracy than multilingual models, but that quality advantage scales as N, not log(N).

**Open architectural question:** when and how should a multilingual Tier 2 sentiment model become the default, with per-language models as optional higher-quality refinements?

**Proposed answer (planned ADR-022 *Multilingual Sentiment Strategy*):**

- Default Tier 2 sentiment: `cardiffnlp/twitter-xlm-roberta-base-sentiment` (or equivalent multilingual model — final selection part of the ADR). Metric name `sentiment_score_bert_multilingual`. Covers all languages the model handles, in one extractor instance.
- Optional Tier 2.5 per-language refinement: `mdraw/german-news-sentiment-bert`, `cmarkea/distilcamembert-...`, etc. Metric name `sentiment_score_bert_<lang>_<domain>`. Available where higher-quality per-language models exist on Hugging Face Hub.
- Both metrics ship in parallel where both apply. Dashboard renders both with Epistemic Weight per Brief §7.8. The gap between them is itself a methodological observation (domain transfer signal per WP-002 §3.2).

**Why this matters now:** Phase 125 will ship Probe 1 with French sentiment. The decision *which* Tier 2 model to use for French, and how it relates to a future multilingual default, should be captured in an ADR before that pattern hardens. **Recommendation:** author ADR-022 before Phase 119 implementation begins for the second language.

### Cultural Calendar composition

**Threshold:** Probe 10–20.

**Problem:** at N = 30 probes in islamic-majority countries, Ramadan, Eid al-Fitr, Eid al-Adha appear duplicated in 30 YAML files. Same for Christian holidays across European probes, Lunar New Year across East Asian probes. The current per-region single-file model is linear in N; at scale this becomes maintenance friction.

**Proposed answer:** calendar inheritance. `services/analysis-worker/configs/cultural_calendars/_shared/christian-western.yaml` defines the shared base; per-region calendars `extends:` the base and add regional specifics:

```yaml
# Sketch — not yet implemented
# fr.yaml
extends: _shared/christian-western.yaml
events:
  - name: "Bastille Day"
    date: "07-14"
    category: national_holiday
  - name: "Toussaint"
    date: "11-01"
    category: religious_holiday
    # already in christian-western.yaml — this is just a note for clarity
```

**When to implement:** at N = 10, not before. Premature now. Designed-but-unimplemented is sufficient documentation; `scalability-roadmap.md` is the design note.

### Probe Coverage Map ⚠️ **roadmap gap**

**Threshold:** Probe 3.

**Problem:** WP-001 §5.3 explicitly mandates a Probe Coverage Map: a visualisation showing, for each cultural region, which discourse functions are covered by active probes and which remain unobserved. **The current ROADMAP has no phase for this.** Phase 110 (Surface I globe refinement) has a probe-glow indicator, but no structural coverage view.

At N = 3 probes, the lack of a coverage map becomes acutely felt: which language has Tier 1 sentiment? Which has NER? Which discourse functions are covered for Germany? For France? Today the answer requires reading three dossier README files; it should be one visual.

**Proposed answer:** a new ROADMAP phase, sequenced after Phase 125 (Probe 1 lands → coverage map becomes meaningful) and before Phase 127 (Composition Mode → coverage map is its prerequisite navigational element). Sketch:

- Backend: `GET /api/v1/coverage/map` returning per-region per-discourse-function coverage status, plus per-probe analytical capability (NER tier, sentiment tier, calendar coverage).
- Frontend: a D3-based world map module on Surface I, layered as a togglable view alongside the probe glow. Negative-space markings for unobserved regions.
- Doc: WP-001 §5.3 cross-link marked as operationalised.

**When to plan:** insert as a P2 phase right after Phase 125 in the next ROADMAP revision.

### Equivalence Registry UX scaling

**Threshold:** Probe 20+.

**Problem:** at N = 100 probes, there are N×(N−1)/2 = 4,950 possible probe pairs. Even with default-deny (most pairs have no equivalence entry), the Probe Dossier "valid comparisons" panel needs UX scaling — a flat list of 99 pairs is not navigable.

**Proposed answer:** UI pagination on "valid comparisons", with relevance ranking (probes that share language → higher; probes in same WP-001 functional taxonomy → higher; recently composed in user's session → higher). Backend changes minimal; this is frontend UX work.

**When to implement:** at N = 10, not before. Ignore for now.

### Probe Dossier authoring as the human bottleneck ⚠️ **methodological, not architectural**

**Threshold:** Probe 3 (i.e., immediately).

**Problem:** each probe requires five Markdown files of substantive content (purpose, sources, calibration status, exit criteria, WP coverage matrix, classification, bias assessment, temporal profile, observer effect). Authentic content is irreplaceable — neither AI-generated content nor templated content can substitute.

For the solo developer this means **an honest 3–5 probe ceiling on `provisional_engineering` status alone**, similar to Probe 0. Beyond that, the "100+ probes" trajectory requires interdisciplinary contributors — domain experts, ethical reviewers, peer reviewers per WP-001 §4.4.

**Architecture cannot solve this.** This is the methodological reality of the project. The architecture supports it (the `provisional_engineering` mechanism, the Probe Dossier Pattern, the Workflow 1 → 5 sequence in the Scientific Operations Guide), but the work itself is human and scientific.

**What `scalability-roadmap.md` should say plainly:** *engineering scales to 100+ probes with current architecture; methodologically valid coverage scales with the size of the interdisciplinary contributor network*. The two grow on different curves.

---

## Open architectural decisions

In rough priority order — these are the ADRs that should land before they become emergencies:

| Priority | ADR | Trigger | Owner | Effort |
| :--- | :--- | :--- | :--- | :--- |
| **High** | ADR-022: Multilingual Sentiment Strategy | Before Phase 119 implementation for the second language | Engineering Lead | 1 afternoon |
| **High** | Language Capability Manifest | Before Phase 125 — drives `metric_validity` scaffolds, Coverage Map, [add-a-language.md](add-a-language.md) | Engineering Lead | 1 day |
| **Medium** | Probe Coverage Map (ROADMAP phase, not ADR) | Before Phase 127 — Coverage Map is a prerequisite navigation element | Engineering Lead | 2–3 days for the phase itself |
| **Medium** | ADR (sketch only): Cultural Calendar Composition | At N = 10 probes | Engineering Lead | Sketch now (this doc), full ADR later |
| **Low** | ADR (sketch only): Multilingual NER Fallback | At N = 5 probes without spaCy NER coverage | Engineering Lead | Out-of-band |

---

## What this document does *not* do

- It does not commit to specific probe counts at specific dates. AĒR is a research instrument, not a product roadmap with quotas.
- It does not promise that hundreds of probes will exist soon. The methodological work scales with contributor capacity, not engineering velocity.
- It does not propose premature architectural complexity. Several open questions above are explicitly tagged as "implement at threshold N=10" or "implement when relevant" — premature implementation is forbidden by Occam's Razor at the architectural level.

The document's job is **knowing what we know we don't know**, so the project does not get blindsided when probe number 3, 10, or 30 reveals a bottleneck that should have been anticipated.

---

## Cross-references

- [Adding a Language](add-a-language.md) — the per-language extension matrix.
- [Adding a Probe](add-a-probe.md) — the per-probe Solo-developer Quickstart.
- [WP-001 §5.3 Probe Coverage Map](../methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md) — the methodological mandate for the Coverage Map.
- [Arc42 §11 Risks and Technical Debts](../arc42/11_risks_and_technical_debts.md) — running register of known risks; the items above feed into this register as they mature.
- [ROADMAP](https://github.com/frogfromlake/aer/blob/main/ROADMAP.md) — the time-axis view; this document is the orthogonal scaling-axis view.