# Extractor Limitations (Phase 42 Provisional Implementations)

This document records the known limitations of AĒR's Phase 42 metric extractors as identified in WP-002 (Metric Validity and Sentiment Calibration, §3). All Phase 42 NLP extractors are **provisional proof-of-concept implementations** — the specific lexicons, models, and parameters are engineering defaults, not scientifically validated choices.

This file is the human-readable complement to the `aer_gold.metric_validity` ClickHouse table, which will store machine-readable validation metadata once interdisciplinary validation studies are conducted.

---

## SentiWS Sentiment Extractor (`sentiment_score`)

**Extractor:** `extractors/sentiment.py`
**Resource:** SentiWS v2.0 (Leipzig University, CC-BY-SA 4.0)
**Provenance:** Lexicon SHA-256 hash recorded in `SilverEnvelope.extraction_provenance`.

### Known Limitations

1. **Negation Blindness.** The extractor performs naive whitespace tokenization and lexicon lookup. It does not detect negation particles (e.g., "nicht gut" scores as positive because "gut" matches the lexicon). This systematically inflates positive sentiment scores when negation is present.

2. **Compound Word Failure.** German compound nouns (e.g., "Umweltkatastrophe") are not decomposed before lexicon lookup. The lexicon contains base forms ("Katastrophe") but not their compounds, causing systematic under-detection of sentiment-bearing words in German text.

3. **No Irony or Sarcasm Detection.** The lexicon-based approach assigns literal polarity scores. Ironic or sarcastic usage is scored at face value, producing inverted sentiment measurements.

4. **No Compositionality.** Multi-word expressions, intensifiers ("sehr gut"), and diminishers ("kaum relevant") are not handled. Each word is scored independently.

5. **Language Scope.** SentiWS is a German-only lexicon. Non-German text receives no sentiment score (graceful degradation), but mixed-language documents may produce partial, misleading scores.

---

## spaCy Named Entity Extractor (`entity_count`, `aer_gold.entities`)

**Extractor:** `extractors/entities.py`
**Resource:** spaCy `de_core_news_lg-3.8.0`
**Labels:** PER (person), ORG (organization), LOC (location), MISC (miscellaneous)

### Known Limitations

1. **No Entity Linking.** The extractor identifies entity mentions (surface forms) but does not resolve them to canonical entities. "Angela Merkel," "Merkel," and "die Bundeskanzlerin" are treated as three distinct entities. This inflates entity counts and prevents entity-level aggregation.

2. **Western Bias in Entity Ontology.** The spaCy model was trained predominantly on Western European news corpora. Entity recognition accuracy degrades for non-Western names, organizations, and locations. This introduces systematic measurement bias when applied to sources from non-Western contexts.

3. **Model Version Coupling.** Entity extraction results are tied to the specific spaCy model version (`de_core_news_lg-3.8.0`). Model updates may produce different entity boundaries and labels for the same input text, breaking longitudinal comparability. See Chapter 11, R-10.

4. **No Coreference Resolution.** Pronouns and anaphoric references are not resolved to their antecedents. Entity frequency counts reflect only explicit named mentions, not the full discourse prominence of an entity.

---

## Language Detection Extractor (`language_confidence`, `aer_gold.language_detections`)

**Extractor:** `extractors/language.py`
**Resource:** `langdetect` library with fixed seed for determinism

### Known Limitations

1. **Short-Text Degradation.** Detection accuracy degrades significantly for texts shorter than approximately 50 characters. Headlines, captions, and metadata fields may receive incorrect or low-confidence language classifications.

2. **Fixed Seed Determinism vs. Accuracy.** The fixed seed ensures reproducibility across runs but does not improve accuracy. The same incorrect classification is reproducibly returned for ambiguous inputs.

3. **Script and Code-Switching.** The detector assumes monolingual input. Documents containing code-switched text (e.g., German text with embedded English quotations) may be classified as either language depending on the relative proportion of each.

4. **Potential Replacement.** `langdetect` may be replaced by `lingua-py` or corpus-level language profiling in future phases, pending methodological review.

---

## Word Count Extractor (`word_count`)

**Extractor:** `extractors/word_count.py`

### Known Limitations

Word count is a deterministic, transparent metric with minimal limitations. The only known issue is that whitespace tokenization (splitting on whitespace boundaries) does not account for language-specific word boundary rules in agglutinative or logographic languages. For the current Probe 0 (German institutional RSS), this is not a concern.

---

## Temporal Distribution Extractor (`publication_hour`, `publication_weekday`)

**Extractor:** `extractors/temporal.py`

### Known Limitations

Temporal distribution metrics are pure metadata extraction (UTC hour and weekday from the document timestamp). They are deterministic and methodologically stable. No known limitations for the current use case. These extractors are **not provisional** — they do not require validation studies.

---

## Cross-Cutting Concerns

- **Graceful Degradation:** A failing extractor is logged and skipped — other extractors continue processing. No documents are sent to the DLQ for extractor failures. This means partial metric sets are possible for any given document.
- **Immutable Input:** All extractors receive an immutable `SilverCore`. No extractor can modify the input data.
- **Validation Required:** None of these extractors have undergone the five-step validation protocol proposed in WP-002 §4. Results must not be interpreted as validated measurements until validation studies are conducted and recorded in `aer_gold.metric_validity`.
