# Adding a Language

This guide answers: *"I want AĒR to handle a new language X — where do I touch, what is mandatory, what is optional, and what gets it for free?"*

The architecture is layered. Some layers are language-agnostic (handle X without code change), some are language-routed (extend via configuration), and a few are language-bound (require language-specific tooling or content). The matrix below is the authoritative map.

---

## Touchpoint matrix

| Layer | Concern | Status | What you do for language X | Effort |
| :--- | :--- | :--- | :--- | :--- |
| **Language detection (Phase 116)** | Routing input | ✅ Multilingual-by-construction | Nothing. `lingua-py` covers 75+ languages. | 0 |
| **NER (Phase 116 + 42, manifest-driven Phase 118a)** | Entity extraction | 🔧 Manifest-routed | Add `<lang>_core_news_lg` to `requirements.txt`; add a `languages.<code>.ner` block to `configs/language_capabilities.yaml`. If no spaCy model exists, NER degrades gracefully (Phase 116 absence-not-wrong). | 5 min, or 0 with degraded NER |
| **Wikidata Entity Linking (Phase 118)** | QID disambiguation | ✅ Multilingual-by-construction | Re-build the Wikidata alias index with the new language code in the build-script's language set. Quarterly rebuild anyway. | 0 marginal — fold into next index rebuild |
| **Sentiment Tier 1 (Phase 117 / 118a pattern)** | Deterministic baseline | 🔧 Manifest-routed | If a deterministic lexicon exists for X (FEEL for `fr`, AFINN, custom YAML), add it via the custom-lexicon-hook pattern and add a `languages.<code>.sentiment_tier1` block to `configs/language_capabilities.yaml`. If none exists: skip — the absence of the block is the language guard. | 1–4 h with lexicon, 0 without |
| **Sentiment negation handler (Phase 117 / 118a pattern)** | Tier 1 quality | 🔧 Manifest-routed | Author `languages.<code>.sentiment_tier1.negation` in the manifest: negation particles, clause-coordinating conjunctions, spaCy `neg` dep label, plus the clause-boundary dep set. Compound-split is German-specific and not needed for most other languages. | 1–2 h |
| **Sentiment Tier 2 BERT (Phase 119 pattern)** | Validated transformer baseline | 🔧 Language-routed | Either (a) extend the multilingual BERT extractor (one model handles all languages — see ADR-022 *Multilingual Sentiment Strategy*) or (b) add a new per-language extractor following the Phase 119 dual-extractor pattern. Per-language is higher quality, multilingual is operationally cheaper. | 0 if multilingual default suffices, 0.5–1 day per per-language model |
| **BERTopic (Phase 120)** | Topic modeling | ✅ Multilingual-by-construction | Nothing. `intfloat/multilingual-e5-large` covers 100 languages. Per-language topic partitioning is automatic via Phase 120's WP-004 §3.4 implementation. | 0 |
| **Cultural Calendar** | Temporal context for WP-004 §6.3 Level 1 | 🔧 Per-region content | Author `services/analysis-worker/configs/cultural_calendars/<region>.yaml` with public holidays, election dates, recurring media events. Required *before* a temporal-equivalence grant for any probe-pair involving the new language (Phase 123 / Workflow 6). | 1–4 h |
| **Frontend rendering** | Dashboard | ✅ Language-agnostic | Nothing. View modes render any number of language partitions in parallel (Brief §4.2.2). | 0 |
| **BFF API** | Endpoints | ✅ Language-agnostic | Nothing. `language` parameter is already a query string parameter on relevant endpoints. | 0 |
| **Equivalence registry (Phase 115)** | Cross-frame methodology | ✅ Default-deny | Nothing. Cross-frame `?normalization=zscore` requests against the new language refuse out-of-the-box. Granting equivalence is a Phase-123-style operations workflow, not an extension. | 0 |

---

## The minimum viable language addition

For a probe in language X where you accept *temporal-only* analysis (WP-004 §6.3 Level 1) and *no* sentiment / NER coverage:

1. Add language to NER skip list — done by default if no spaCy model is present.
2. Author the cultural calendar (`<region>.yaml`).
3. Document the gap row in `aer_gold.metric_validity` with `reason='no NER/sentiment coverage for <lang>'`.
4. Honest. Done.

**Effort: 1–2 hours.** The probe operates at WP-004 Level 1 — temporal patterns, publication rhythms, weekly cycles. That is methodologically meaningful even without NER or sentiment, and it is the safest analytical floor for any new cultural context.

## The full Tier-1+Tier-2 language addition

For a probe where you want full analytical parity with German Probe 0:

1. spaCy model in `requirements.txt`.
2. NER language map entry.
3. Tier 1 sentiment lexicon (if available) + custom-lexicon hook.
4. Negation config entry (particles + clause boundaries).
5. Tier 2 BERT model (per-language preferred, multilingual fallback).
6. Cultural calendar.
7. `metric_validity` rows for each new context-key combination.

**Effort: 2–3 days.** This is *pattern application*, not architectural design. The patterns are fixed. The work is configuration + tests + content.

---

## Language Capability Manifest (Phase 118a / ADR-024)

`services/analysis-worker/configs/language_capabilities.yaml` is the single source of truth for per-language capability. It is the executable form of the matrix above.

```yaml
manifest_version: 1

languages:
  de:
    iso_code: de
    display_name: German
    ner:
      tier: 1.5
      model: de_core_news_lg
      model_version: "3.8.0"
    sentiment_tier1:
      tier: 1
      method: lexicon
      lexicon: sentiws_v2.0
      features: [negation_dependency, compound_split, custom_lexicon]
      negation:
        particles: [nicht, kein, keine, ...]
        clause_boundaries: [weil, dass, obwohl, ...]
        spacy_neg_dep: neg
        spacy_neg_deps_extra: [ng]
        clause_boundary_deps: [cc, mark, cp, oc, re, cj, cd]
      metric_name: sentiment_score_sentiws
    cultural_calendar:
      region_default: de
      file: cultural_calendars/de.yaml

shared: {}   # populated by Phase 119 with shared.multilingual_bert
```

Consumers:

- `NamedEntityExtractor` reads `languages.<code>.ner.model` for routing.
- `SentimentExtractor` reads `languages.<code>.sentiment_tier1` for the lexicon, feature flags, and negation cues.
- `scripts/generate_metric_validity_scaffold.py` (run via `make scaffold-metric-validity`) emits one block per `(language, metric_name, tier)` triple into `infra/clickhouse/seed/metric_validity_scaffold_generated.sql`. Drift is a CI failure.
- The BFF reads the same YAML at startup and gates every endpoint that accepts `?language=` against the manifest's keys; unknown values produce a structured `gate=invalid_language` refusal payload.

The matrix above remains hand-maintained for now. Auto-generation of the matrix from the manifest is slated for Phase 122a.

---

## Worked example: adding French (Probe 1)

Concrete diff for adding French support, as a reference for future language additions. This is what Phase 122 actually does, condensed.

### `services/analysis-worker/requirements.txt`

```diff
 spacy==3.8.13
 https://github.com/explosion/spacy-models/releases/download/de_core_news_lg-3.8.0/de_core_news_lg-3.8.0-py3-none-any.whl
+https://github.com/explosion/spacy-models/releases/download/fr_core_news_lg-3.8.0/fr_core_news_lg-3.8.0-py3-none-any.whl
```

### `configs/language_capabilities.yaml`

A single manifest edit replaces what used to be a NER map entry, a negation-config entry, and a metric_validity scaffold edit (Phase 118a / ADR-024):

```diff
 languages:
   de:
     iso_code: de
     ...
+  fr:
+    iso_code: fr
+    display_name: French
+    ner:
+      tier: 1.5
+      model: fr_core_news_lg
+      model_version: "3.8.0"
+    sentiment_tier1:
+      tier: 1
+      method: lexicon
+      lexicon: feel_v1.0
+      features: [negation_dependency, custom_lexicon]   # no compound_split
+      negation:
+        particles: [ne, pas, non, jamais, plus, rien, aucun, aucune, personne, nulle]
+        clause_boundaries: [parce, que, lorsque, quand, bien]
+        spacy_neg_dep: neg
+        clause_boundary_deps: [cc, mark]
+      metric_name: sentiment_score_feel
+    cultural_calendar:
+      region_default: fr
+      file: cultural_calendars/fr.yaml
```

Then run `make scaffold-metric-validity` to regenerate `infra/clickhouse/seed/metric_validity_scaffold_generated.sql` and commit the diff.

### `data/sentiment_lexica/`

New file: `feel.csv` from the FEEL French Expanded Emotion Lexicon (CC-licensed). Sentiment extractor language map gets `'fr': 'feel'`.

### `configs/cultural_calendars/fr.yaml`

New file. Initial content per Phase 122: French federal public holidays, presidential/legislative election dates, Bastille Day, August holiday rhythm, Easter-based movable feasts.

### Tier 2 BERT

If using ADR-022's multilingual default: nothing further. If adding a per-language French extractor: new file `extractors/sentiment_bert_fr_news.py` cloned from the German Phase 119 pattern, swapping `mdraw/german-news-sentiment-bert` for `cmarkea/distilcamembert-base-sentiment`.

### `aer_gold.metric_validity` scaffold

Append rows for each `(metric_name, context_key)` covering French. All `validation_status='unvalidated'`, `alpha_score=null`, `correlation=null`. Honest until annotation studies for French sources arrive.

### What you do *not* change

Confirm by absence: no BFF endpoint changes, no frontend changes, no schema migrations beyond the `metric_validity` seed inserts, no BERTopic changes, no Wikidata index work beyond the next quarterly rebuild.

---

## Cross-references

- [Adding a Probe](add-a-probe.md) — the per-cultural-context guide; nearly always paired with adding a language for non-German contexts.
- [Scientific Operations Guide → Workflow 6 (Cultural Calendar)](../operations/scientific_operations_guide.md) — the canonical procedure for the cultural calendar bullet above.
- [scalability-roadmap.md](scalability-roadmap.md) — the long-term plan for language capability scaling and the Capability Manifest.
- [ADR-022 (planned): Multilingual Sentiment Strategy](../arc42/09_architecture_decisions.md) — when authored, will resolve the per-language vs. multilingual Tier 2 trade-off referenced in this matrix.