# Adding a Language

This guide answers: *"I want AĒR to handle a new language X — where do I touch, what is mandatory, what is optional, and what gets it for free?"*

The architecture is layered. Some layers are language-agnostic (handle X without code change), some are language-routed (extend via configuration), and a few are language-bound (require language-specific tooling or content). The matrix below is the authoritative map.

---

## Touchpoint matrix

| Layer | Concern | Status | What you do for language X | Effort |
| :--- | :--- | :--- | :--- | :--- |
| **Language detection (Phase 116)** | Routing input | ✅ Multilingual-by-construction | Nothing. `lingua-py` covers 75+ languages. | 0 |
| **NER (Phase 116 + 42)** | Entity extraction | 🔧 Language-routed | Add `<lang>_core_news_lg` to `requirements.txt`; one entry to NER language map. If no spaCy model exists, NER degrades gracefully (Phase 116 absence-not-wrong). | 5 min, or 0 with degraded NER |
| **Wikidata Entity Linking (Phase 118)** | QID disambiguation | ✅ Multilingual-by-construction | Re-build the Wikidata alias index with the new language code in the build-script's language set. Quarterly rebuild anyway. | 0 marginal — fold into next index rebuild |
| **Sentiment Tier 1 (Phase 117 pattern)** | Deterministic baseline | 🔧 Language-routed | If a deterministic lexicon exists for X (FEEL for `fr`, AFINN, custom YAML), add it via the custom-lexicon-hook pattern; add language to `SentimentExtractor` language map. If none exists: skip — Phase 116 language guard ensures graceful absence. | 1–4 h with lexicon, 0 without |
| **Sentiment negation handler (Phase 117 pattern)** | Tier 1 quality | 🔧 Language-routed | Add language entry to `extractors/_negation_config.py`: negation particles + clause-coordinating conjunctions + spaCy `neg` dep tag for the language. Compound-split is German-specific and not needed for most other languages. | 1–2 h |
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

## Language Capability Manifest (planned)

A planned addition (see [scalability-roadmap.md](scalability-roadmap.md) item #2) is a single source of truth for per-language capabilities at `services/analysis-worker/configs/language_capabilities.yaml`:

```yaml
# Example sketch — not yet implemented
de:
  ner_model: de_core_news_lg
  ner_model_version: "3.8.0"
  tier1_sentiment_lexicon: sentiws-v2.0
  tier1_sentiment_features: [negation, compound_split, custom_lexicon]
  tier2_sentiment_models:
    - mdraw/german-news-sentiment-bert  # primary, news-domain
    - oliverguhr/german-sentiment-bert  # secondary, review-domain baseline
  cultural_calendar: de.yaml
  multilingual_fallbacks_used: []

fr:
  ner_model: fr_core_news_lg
  ner_model_version: "3.8.0"
  tier1_sentiment_lexicon: feel-v1.0
  tier1_sentiment_features: [negation, custom_lexicon]   # no compound_split
  tier2_sentiment_models:
    - cmarkea/distilcamembert-base-sentiment
  cultural_calendar: fr.yaml
  multilingual_fallbacks_used: []

# ...
```

Once implemented, the manifest drives:

- The NER and Sentiment language routing (replaces hard-coded language maps).
- Auto-generation of `aer_gold.metric_validity` scaffold rows for each `(metric_name, context_key)` pair.
- The Probe Coverage Map (planned phase) showing analytical capability per probe.
- This very matrix — generated from the manifest, not hand-written.

Until the manifest exists, the touchpoint matrix above is the hand-maintained reference. The matrix and the manifest are equivalent in intent — the manifest is the executable form.

---

## Worked example: adding French (Probe 1)

Concrete diff for adding French support, as a reference for future language additions. This is what Phase 122 actually does, condensed.

### `services/analysis-worker/requirements.txt`

```diff
 spacy==3.8.13
 https://github.com/explosion/spacy-models/releases/download/de_core_news_lg-3.8.0/de_core_news_lg-3.8.0-py3-none-any.whl
+https://github.com/explosion/spacy-models/releases/download/fr_core_news_lg-3.8.0/fr_core_news_lg-3.8.0-py3-none-any.whl
```

### NER language map

```diff
 NER_LANGUAGE_MODELS = {
     "de": "de_core_news_lg",
+    "fr": "fr_core_news_lg",
 }
```

### `extractors/_negation_config.py`

```diff
 NEGATION_CONFIG = {
     "de": NegationLanguageConfig(
         particles={"nicht", "kein", "keine", "keiner", "keines", "keinem", "keinen", "niemals", "nie", "nirgends", "kaum"},
         clause_boundaries={"weil", "dass", "obwohl", "während", "nachdem", "bevor"},
         spacy_neg_dep="neg",
     ),
+    "fr": NegationLanguageConfig(
+        particles={"ne", "pas", "non", "jamais", "plus", "rien", "aucun", "aucune", "personne", "nulle"},
+        clause_boundaries={"parce", "que", "lorsque", "quand", "bien"},
+        spacy_neg_dep="neg",
+    ),
 }
```

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
- [Scientific Operations Guide → Workflow 6 (Cultural Calendar)](../scientific_operations_guide.md) — the canonical procedure for the cultural calendar bullet above.
- [scalability-roadmap.md](scalability-roadmap.md) — the long-term plan for language capability scaling and the Capability Manifest.
- [ADR-022 (planned): Multilingual Sentiment Strategy](../arc42/09_architecture_decisions.md) — when authored, will resolve the per-language vs. multilingual Tier 2 trade-off referenced in this matrix.