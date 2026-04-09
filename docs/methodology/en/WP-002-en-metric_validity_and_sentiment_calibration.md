# WP-002: Metric Validity and Sentiment Calibration

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft — open for interdisciplinary review
> **Date:** 2026-04-07
> **Depends on:** WP-001 (Functional Probe Taxonomy), Arc42 Chapter 13 (Scientific Foundations)
> **Architectural context:** Analysis Worker Extractor Pipeline (§8.10), Tier 1–3 Metric Classification (§13.3)
> **License:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Objective

This working paper addresses a foundational question for the AĒR project: **Under what conditions can computational text metrics serve as valid proxies for societal attitudes, and what calibration is required before they can be meaningfully compared across languages, cultures, and media types?**

AĒR's engineering pipeline (Phases 41–46) has established a functional extractor architecture with provisional proof-of-concept implementations for sentiment scoring, named entity recognition, language detection, and temporal distribution. These extractors validate the pipeline's technical correctness — they do not validate the *scientific meaning* of the numbers they produce. A sentiment score of `0.34` extracted from a Tagesschau RSS description using SentiWS tells us that certain words in the text carry positive polarity weights in a Leipzig University lexicon. It does not tell us whether the article expresses a positive attitude, whether the journalist intended to convey optimism, or whether a reader in Munich, Nairobi, or São Paulo would perceive the text as positive.

This paper maps the gap between computational output and sociological interpretation, identifies the specific scientific questions that must be answered through interdisciplinary collaboration, and proposes a validation framework compatible with AĒR's architectural constraints: determinism, transparency, and Ockham's Razor.

---

## 2. The Validity Problem: From Numbers to Meaning

### 2.1 Construct Validity in Computational Social Science

The central epistemological challenge is one of **construct validity**: does the computational metric measure the theoretical construct it claims to represent? This problem is well-established in survey methodology (Cronbach & Meehl, 1955) but takes a distinct form in computational text analysis.

In traditional social science, a researcher designs a survey instrument, administers it to a sample population, and validates the instrument against external criteria. The researcher controls the measurement process. In computational discourse analysis, the "instrument" is an algorithm applied to text that was produced for purposes entirely unrelated to the measurement. The text is not a response to a structured prompt — it is a cultural artifact embedded in a specific communicative context (editorial policy, platform affordances, audience expectations, genre conventions).

Grimmer, Roberts & Stewart (2022) formalize this tension: computational text analysis methods are powerful tools for *discovery* and *measurement*, but they require explicit validation against human judgment for every new application domain. There is no universal sentiment algorithm that "works" across all text types, languages, and cultural contexts. Validity is always *contextual*.

**Implication for AĒR:** Every metric extractor must be validated not once, but for each combination of:

- **Language** (German editorial prose ≠ Arabic Twitter discourse ≠ Japanese blog culture)
- **Source type** (RSS feed descriptions ≠ full articles ≠ social media posts ≠ forum threads)
- **Discourse function** (WP-001 taxonomy: epistemic authority ≠ counter-discourse ≠ identity formation)
- **Temporal context** (crisis periods may shift baseline sentiment distributions)

### 2.2 The Aggregation Fallacy

AĒR's Gold layer stores metrics as time-series data points aggregated across documents and sources. This aggregation is the system's core value proposition — it enables macroscopic observation. But aggregation introduces a critical risk: **ecological inference fallacy** (Robinson, 1950). Patterns observed at the aggregate level (e.g., "average sentiment toward immigration declined in Q3 2026") may not reflect patterns at the individual document level. They may instead reflect changes in the *composition* of the corpus (e.g., a new source with systematically lower sentiment was added, or a high-volume source increased its publication frequency).

This is not merely a statistical concern — it is a sociological one. If AĒR's dashboard shows a decline in "positive discourse" in a given region, is this because:

1. Individual authors are expressing more negative views? (Genuine attitude shift)
2. The platform's recommendation algorithm is amplifying negative content? (Algorithmic artifact)
3. A new, structurally negative source (e.g., investigative journalism) was added to the probe set? (Sampling artifact)
4. The sentiment lexicon systematically underscores the language variety used in that region? (Measurement artifact)

Without validation, these explanations are indistinguishable in the aggregate metric. AĒR's Progressive Disclosure principle (ADR-003) — the ability to drill from Gold aggregates down to individual Bronze documents — is the architectural safeguard. But Progressive Disclosure enables *human* disambiguation; it does not *automate* it. The scientific question is how to design metrics and metadata that make this disambiguation tractable.

---

## 3. Sentiment Analysis: State of the Art and Limitations

### 3.1 AĒR's Current Implementation (Provisional PoC)

The `SentimentExtractor` (Phase 42) uses SentiWS v2.0, a German-language word-level polarity lexicon published by Leipzig University (Remus, Quasthoff & Heyer, 2010). The algorithm is deliberately minimal:

1. Tokenize `cleaned_text` by whitespace.
2. Lowercase each token and strip boundary punctuation.
3. Look up each token in the SentiWS lexicon (~3,500 base forms + inflections).
4. Score = arithmetic mean of matched token polarities.
5. Clamp to [-1.0, 1.0].

This approach was chosen because it is fully deterministic, auditable (every score is traceable to individual word matches), and requires no external API calls or model inference. It satisfies AĒR's Tier 1 criteria (§13.3.1). It is also, by any standard of modern NLP, severely limited.

### 3.2 Known Limitations of Lexicon-Based Sentiment

The following limitations are documented in the extractor's source code and in §13.3.1. They are restated here in scientific context:

**Negation blindness.** The sentence *"Das ist nicht gut"* ("This is not good") receives a positive score because *"gut"* carries a positive polarity weight and the negation particle *"nicht"* is not in the lexicon. Negation handling is a solved problem in rule-based NLP (e.g., NegEx, Wiegand et al., 2010), but any rule-based negation scope detection must be validated per language. German clause structure, with its verb-final subordinate clauses, creates negation scopes that differ fundamentally from English.

**Compositionality and compound words.** German is an agglutinative language. The compound *"Klimaschutzpaket"* (climate protection package) is a single orthographic token that carries latent sentiment through its components — but the compound itself is not in any sentiment lexicon. Decomposition strategies (empirical compound splitting, e.g., Koehn & Knight, 2003) introduce their own error sources. Other agglutinative languages — Turkish, Finnish, Hungarian, Korean, Japanese — present analogous challenges with distinct morphological rules.

**Irony and sarcasm.** Lexicon-based methods are structurally incapable of detecting irony, where the surface-level polarity is inverted. Irony detection remains an open research problem even for transformer-based models (Van Hee et al., 2018), and irony conventions are deeply culture-specific. What reads as biting sarcasm in British English may be interpreted literally in many East Asian communicative contexts, where indirect communication follows different pragmatic conventions (Hall, 1976; Hofstede, 2001).

**Domain dependency.** The word *"Krise"* (crisis) carries a negative polarity weight in SentiWS. In economic journalism, *"Krise"* is a descriptive term with no inherent evaluative intent — it describes a state of affairs. In political commentary, the same word may carry strong negative affect. Lexicon scores are context-blind; the same word receives the same weight regardless of genre, register, or communicative intent.

**Lexicon coverage and currency.** SentiWS v2.0 was published in 2010. The German language has evolved — new evaluative terms have entered the discourse (*"Wutbürger,"* *"Querdenker,"* *"toxisch"* in its metaphorical social sense), and existing terms have shifted polarity (*"liberal"* carries different connotations in German vs. American English discourse). A fixed lexicon is a snapshot of evaluative language at a point in time, not a living instrument.

### 3.3 Beyond Lexicons: The Spectrum of Sentiment Methods

The research landscape offers a spectrum of approaches with different tradeoffs against AĒR's architectural principles:

**Rule-based systems (VADER, TextBlob, SentiStrength).** These extend lexicon approaches with heuristic rules for negation, intensification, and punctuation. VADER (Hutto & Gilbert, 2014) was calibrated on English social media text and performs poorly on formal editorial prose and non-English languages. SentiStrength (Thelwall et al., 2010) supports multiple languages but was developed for short informal text. None of these systems have been systematically validated on German institutional RSS content.

**Supervised machine learning (SVM, Naive Bayes with TF-IDF features).** These require labeled training data — a corpus of texts annotated for sentiment by human coders. The annotation process itself is culturally situated: what counts as "positive" or "negative" depends on the annotators' cultural background, linguistic competence, and interpretive framework. Inter-annotator agreement on sentiment is notoriously low for nuanced texts (Rosenthal et al., 2017). For AĒR, this approach introduces a circular dependency: we need validated sentiment labels to train the model, but generating those labels *is* the validation problem.

**Pre-trained transformer models (BERT, XLM-RoBERTa, language-specific variants).** German-specific models like *german-sentiment-bert* (Guhr et al., 2020) achieve state-of-the-art performance on benchmark datasets. However, they violate AĒR's Tier 1 determinism requirement — transformer inference is sensitive to floating-point precision, hardware, and library versions. They are opaque (the model's internal reasoning is not auditable), and they embed training biases that are difficult to characterize. Under AĒR's tier system, transformer-based sentiment would be classified as Tier 2 (reproducible with pinned model version and fixed seed) or Tier 3 (non-deterministic).

**LLM-based sentiment (GPT-4, Claude, open-source LLMs).** The most capable but least transparent option. LLM outputs are non-deterministic, non-reproducible, and economically costly at scale. Under AĒR's framework, LLM-derived sentiment is strictly Tier 3 — permissible only as exploratory augmentation, never as a primary metric (§13.3.3).

### 3.4 The Cross-Cultural Sentiment Challenge

Sentiment is not a universal construct. The assumption that "positive" and "negative" are culturally invariant emotional poles is a Western psychological framework (Russell, 1980) that does not map cleanly onto all human affective systems.

**Linguistic relativity of affect.** Languages encode affect differently. The Japanese concept of *amae* (甘え) — a pleasant feeling of dependence on another person — has no English equivalent and no polarity mapping in any Western sentiment lexicon. The German *Schadenfreude* is lexically negative (it contains *Schaden*, damage) but psychologically complex. Arabic emotional vocabulary distinguishes states of the heart (*qalb*) that do not align with the positive-negative axis. A sentiment system that reduces all affect to a single scalar value [-1.0, 1.0] imposes a dimensional model of emotion that is culturally specific.

**Pragmatic conventions.** The same utterance carries different sentiment depending on communicative norms. In many East Asian media cultures, restraint and understatement are the default register — a mildly positive statement may represent strong endorsement. In American media culture, hyperbolic positivity is the baseline — "amazing," "incredible," "game-changing" are register-neutral in tech journalism. A cross-cultural sentiment comparison must account for these **baseline calibration differences** in expressive conventions.

**Political and institutional register.** Government press releases — the core of AĒR's Probe 0 — employ a deliberately neutral institutional register across cultures. The *Bundesregierung* communicates in Beamtendeutsch; the White House uses diplomatic prose; the Chinese State Council uses a distinct official register (官方语言). These registers are designed to suppress overt sentiment. A lexicon-based score of `0.0` on a government press release does not mean "neutral attitude" — it means "institutional register functioning as designed." This distinction is critical for cross-cultural comparison.

**Code-switching and multilingual discourse.** In many digital spaces — particularly in postcolonial contexts (India, Philippines, Nigeria, North Africa) — users routinely switch between languages within a single text. A tweet mixing Hindi and English (*Hinglish*), or a Facebook post mixing French and Wolof, cannot be meaningfully scored by any monolingual sentiment system. AĒR's current language detection extractor assigns a single primary language per document — this is insufficient for code-switched texts.

---

## 4. Named Entity Recognition: Beyond Extraction

### 4.1 AĒR's Current Implementation

The `NamedEntityExtractor` (Phase 42) uses spaCy's `de_core_news_lg` model (v3.8.0) with all pipeline components disabled except NER. It extracts entity spans classified as PER (person), ORG (organization), LOC (location), and MISC (miscellaneous). Raw spans are stored in `aer_gold.entities`; an aggregate `entity_count` is stored as a metric.

### 4.2 The Entity Linking Problem

Raw entity extraction without **entity linking** (also called entity disambiguation or entity resolution) produces data that is difficult to aggregate meaningfully. The string *"Merkel"* in one document and *"Angela Merkel"* in another are stored as separate entities. *"CDU,"* *"die Union,"* and *"Christdemokraten"* refer to the same political entity but are not linked. This fragmentation becomes catastrophic at scale: a dashboard showing "top entities" would list the same real-world entity multiple times under different surface forms.

Entity linking requires a **knowledge base** (e.g., Wikidata, DBpedia) that maps surface forms to canonical identifiers. The choice of knowledge base is itself culturally significant: Wikidata's ontology reflects the editorial decisions of its contributor community, which skews toward Western, English-speaking perspectives. An entity that is highly salient in Brazilian political discourse may have a sparse or absent Wikidata entry. The knowledge base is not neutral ground — it is a cultural artifact.

### 4.3 Cross-Cultural Entity Challenges

**Name conventions.** NER models trained on Western European text expect given-name-family-name ordering. East Asian naming conventions (family name first), patronymic systems (Icelandic, Arabic, Russian), mononymous names (common in Indonesian culture), and honorific-embedded names (Japanese keigo) all violate these expectations. A spaCy model trained on German news text will not correctly parse *"習近平"* or *"محمد بن سلمان"*.

**Organizational boundaries.** The concept of "organization" varies culturally. In the Japanese *keiretsu* system or Korean *chaebol* structure, the boundary between distinct entities is blurred. In many African contexts, tribal or clan structures function as organizational entities but are not recognized by NER models trained on Western corporate and governmental ontologies. WP-001's Functional Probe Taxonomy explicitly addresses this through the Emic Layer — but NER models operate on the Etic Layer and must be selected or trained accordingly.

**Geopolitical sensitivity.** Location entity extraction involves implicit geopolitical assumptions. Whether *"Taiwan"* is tagged as a country or a region, whether *"Palestine"* receives a LOC or GPE label, whether *"Kashmir"* is associated with India or Pakistan — these are not technical decisions. They are political positions embedded in the training data of the NER model. AĒR must document these embedded assumptions and, where possible, defer geopolitical classification to the analyst layer (Progressive Disclosure) rather than baking it into the extractor.

---

## 5. Language Detection: The Illusion of Monolingualism

### 5.1 AĒR's Current Implementation

The `LanguageDetectionExtractor` (Phase 42, extended Phase 45) uses `langdetect` with a fixed random seed. It assigns a primary language code and a confidence score (0.0–1.0) per document. Ranked candidates are persisted in `aer_gold.language_detections`.

### 5.2 Limitations in a Multilingual World

**Short text degradation.** RSS feed descriptions are typically 50–200 characters. Language detection accuracy degrades significantly below 100 characters (Jauhiainen et al., 2017). A German headline containing an English loanword (*"Homeoffice-Pflicht"*) or a French term (*"Ménage-à-trois der Parteien"*) may produce an ambiguous or incorrect language classification.

**Dialect and variety detection.** `langdetect` distinguishes between language codes (e.g., `de`, `en`, `fr`) but not between language varieties. Formal Standard German (*Hochdeutsch*), Swiss German (*Schweizerdeutsch*), and Austrian German are classified identically. This collapses meaningful sociolinguistic variation. For a system that aspires to observe global discourse, the distinction between Simplified and Traditional Chinese, between Brazilian and European Portuguese, between Latin American and Castilian Spanish is analytically relevant.

**Script detection vs. language detection.** Languages sharing a script (e.g., Serbian in Latin vs. Cyrillic, Bosnian/Croatian/Serbian as a pluricentric language) present detection challenges that are irreducible to statistical character n-gram models. The choice of writing system is itself a cultural and political statement.

---

## 6. Toward a Validation Framework

### 6.1 Design Principles

Any validation framework for AĒR's metrics must respect the project's architectural constraints:

1. **Transparency over performance.** A validated metric with known limitations and documented failure modes is preferable to a high-performing metric whose error characteristics are opaque. AĒR is a macroscope, not a microscope — it sacrifices individual-document precision for aggregate-level observability, but it must *know* what it sacrifices.

2. **Provenance as first-class data.** Every metric must carry its own provenance: the algorithm version, the lexicon/model hash, the parameters used, and a confidence qualifier. This is already architecturally anticipated in the `SilverEnvelope.extraction_provenance` field (Phase 46) and the `version_hash` property on each extractor.

3. **Validation is per-context, not universal.** A sentiment method validated for German editorial RSS cannot be assumed valid for Arabic social media or Japanese blog posts. Validation results are bound to the specific combination of language, source type, and discourse function (WP-001 taxonomy).

4. **Human judgment as ground truth.** For subjective constructs like sentiment and stance, there is no external ground truth — only human judgment. Validation requires annotation studies with culturally competent coders, following established intercoder reliability protocols (Krippendorff's Alpha ≥ 0.667 as minimum threshold, Krippendorff, 2004).

### 6.2 Proposed Validation Protocol

For each metric-context pair (e.g., "SentiWS sentiment on German institutional RSS"), the following protocol is proposed:

**Step 1 — Annotation study.** Sample *n* documents from the probe (stratified by source, temporal distribution, and topic). Recruit ≥ 3 annotators with native-speaker competence and domain knowledge. Define a clear annotation scheme (e.g., 5-point Likert scale for sentiment, or categorical labels). Compute intercoder reliability (Krippendorff's Alpha). If Alpha < 0.667, the annotation scheme is too ambiguous — revise before proceeding.

**Step 2 — Baseline comparison.** Run the computational metric on the annotated sample. Compute correlation (Pearson or Spearman) between algorithmic scores and averaged human judgments. Report precision, recall, and F1 for categorical classifications. Document systematic error patterns (e.g., "SentiWS overscores negation-containing sentences by 0.3 on average").

**Step 3 — Error taxonomy.** Classify disagreements between human and algorithmic judgments into a structured taxonomy: negation error, irony error, domain-specific term error, compositionality error, cultural register error. This taxonomy becomes part of the metric's documentation and informs the design of improved extractors.

**Step 4 — Cross-context transfer test.** Apply the metric to a structurally different context (e.g., from German RSS to German Twitter, or from German RSS to French RSS). Measure performance degradation. Document the *transfer boundary* — the point at which the metric's validity breaks down.

**Step 5 — Longitudinal stability test.** Run the metric on a time-series sample spanning ≥ 6 months. Check for temporal drift: does the metric's relationship to human judgment change over time? If so, is the drift driven by language change (new terms entering the lexicon), source behavior change (editorial policy shifts), or metric decay (the model's training data becoming stale)?

### 6.3 Architectural Integration

Validation results should be stored as **metric metadata** in the Gold layer, enabling the BFF API to expose confidence qualifiers alongside raw metric values. A proposed schema extension:

```
aer_gold.metric_validity (
    metric_name       String,
    context_key       String,    -- e.g., "de:rss:epistemic_authority"
    validation_date   DateTime,
    alpha_score       Float32,   -- intercoder reliability
    correlation       Float32,   -- human-algorithmic correlation
    n_annotated       UInt32,
    error_taxonomy    String,    -- JSON blob: error type → frequency
    valid_until       DateTime   -- expiration of validity claim
)
```

This table enables downstream consumers (dashboards, research APIs) to distinguish between validated and unvalidated metrics — a critical transparency requirement for scientific use.

---

## 7. Open Questions for Interdisciplinary Collaborators

The following questions are formulated as concrete research problems that require expertise beyond software engineering. Each question identifies the relevant discipline, the specific gap in AĒR's current implementation, and the form of contribution that would be most valuable.

### 7.1 For Computational Social Scientists

**Q1: Which sentiment method(s) are appropriate for German editorial text from institutional RSS feeds, given AĒR's constraints of determinism and transparency?**

- AĒR's current method (SentiWS lexicon, mean polarity) is a Tier 1 baseline. Is there a validated, deterministic alternative that handles negation and compositionality while remaining auditable?
- If the answer is "no deterministic method is adequate," how should AĒR integrate a Tier 2 method (e.g., a fine-tuned classifier with pinned model version) while preserving traceability?
- Deliverable: A recommendation with validation evidence on a comparable corpus.

**Q2: What annotation scheme should AĒR use for sentiment validation studies?**

- Is a unidimensional positive-negative scale appropriate, or does AĒR need a multidimensional affect model (valence + arousal, or discrete emotions)?
- How should annotators handle institutional register (press releases that are deliberately neutral)? Is "neutral" the absence of sentiment, or is it a distinct communicative stance?
- Deliverable: An annotation codebook with worked examples from German RSS descriptions.

**Q3: How should AĒR normalize sentiment scores for cross-source comparison?**

- Bundesregierung press releases and Tagesschau articles have structurally different sentiment baselines. How do we compare sentiment *change* across sources when the absolute levels are not comparable?
- Deliverable: A normalization strategy (z-score per source, percentile ranking, difference-in-differences) with statistical justification.

### 7.2 For Computational Linguists / NLP Researchers

**Q4: How should AĒR handle German compound words in lexicon-based sentiment analysis?**

- Should compounds be decomposed before lexicon lookup? If so, which decomposition strategy is appropriate (frequency-based, rule-based, neural)?
- How does this extend to other agglutinative languages (Turkish, Finnish, Korean) when AĒR expands beyond German?
- Deliverable: An evaluation of compound decomposition strategies on a German news corpus, with impact on sentiment scoring accuracy.

**Q5: Which NER model(s) and entity linking strategies are appropriate for AĒR's multilingual, multi-source context?**

- The current spaCy `de_core_news_lg` model is monolingual. AĒR will need to process Arabic, Mandarin, Hindi, Portuguese, Spanish, French, and other languages. Is a single multilingual model (XLM-RoBERTa) preferable to per-language models?
- How should entity linking handle entities that are salient in non-Western contexts but underrepresented in Wikidata?
- Deliverable: A benchmark comparison of NER/entity linking approaches on a multilingual news corpus, evaluated per language and entity type.

**Q6: How should AĒR detect and handle code-switched texts?**

- In multilingual digital spaces, documents frequently mix languages. How should language detection, sentiment scoring, and entity extraction operate on code-switched text?
- Deliverable: A survey of code-switching detection methods with a recommendation for AĒR's per-document processing model.

### 7.3 For Cultural Anthropologists and Area Studies Scholars

**Q7: How should AĒR calibrate sentiment baselines across cultures with different expressive norms?**

- If Japanese editorial prose is systematically more restrained than American journalism, a cross-cultural sentiment comparison requires baseline normalization. But the baseline is not merely a statistical artifact — it *is* the cultural phenomenon AĒR aims to observe (epistemic boundaries, per the Manifesto's Episteme pillar).
- Is baseline normalization an act of cultural erasure? Should AĒR preserve raw baselines and visualize the *difference* as a finding?
- Deliverable: A position paper on the epistemological tension between normalization and cultural observation.

**Q8: How should AĒR's entity ontology handle culturally specific organizational forms?**

- WP-001 defines a Dual Tagging System (Etic/Emic layers). How should this extend to entity classification? Should a Japanese *keiretsu* be tagged as ORG in the Etic layer while preserving its emic specificity?
- What culturally specific entity types are systematically missed by Western NER models, and how should AĒR's entity taxonomy accommodate them?
- Deliverable: A cross-cultural entity taxonomy that extends spaCy's default labels with culturally informed categories, mapped to the WP-001 Emic Layer.

### 7.4 For Methodologists and Statisticians

**Q9: How should AĒR's aggregation pipeline account for the ecological inference problem?**

- When sentiment metrics are aggregated across documents, sources, and time windows in the Gold layer, how can AĒR detect whether observed trends reflect genuine discourse shifts vs. compositional changes in the corpus?
- Deliverable: A statistical framework for decomposing aggregate metric changes into within-source and between-source components.

**Q10: What temporal aggregation window is appropriate for different metric types?**

- Sentiment may be meaningful at daily resolution for breaking news but only at weekly or monthly resolution for cultural drift. Topic prevalence may require different temporal windows than entity co-occurrence networks.
- Deliverable: An empirical analysis of metric stability across temporal resolutions on a pilot corpus.

---

## 8. Mapping to AĒR's Architectural Constraints

### 8.1 Tier Classification Revisited

The validation framework proposed in §6 may reveal that no Tier 1 (fully deterministic) sentiment method is adequate for AĒR's analytical goals. This would force a strategic decision:

- **Option A: Accept Tier 1 limitations.** Use lexicon-based sentiment as a transparent but coarse signal. Acknowledge in the dashboard that sentiment scores are *lexical polarity indicators*, not measures of evaluative attitude. This is honest and architecturally simple, but analytically weak.

- **Option B: Promote validated Tier 2 methods.** Adopt a supervised classifier or fine-tuned transformer with pinned model version and fixed seed. Achieve higher accuracy while maintaining reproducibility. This requires validation infrastructure (annotation studies, benchmarking pipelines) and introduces model dependency risks (R-10).

- **Option C: Hybrid architecture.** Use Tier 1 as the immutable baseline. Layer Tier 2/3 enrichments on top, explicitly flagged. The dashboard always shows the Tier 1 score; Tier 2/3 scores are available via Progressive Disclosure. This preserves AĒR's transparency guarantee while enabling analytical depth.

Option C aligns most closely with AĒR's existing Tier architecture (§13.3) and the Ockham's Razor principle — it adds complexity only where the simpler method is *demonstrated* to be insufficient, and it never hides the simple method behind the complex one.

### 8.2 Impact on the Extractor Pipeline

The validation framework requires no changes to the extractor pipeline's technical architecture. The `MetricExtractor` protocol and dependency injection pattern (§8.10) already support multiple extractors producing metrics with the same semantic intent but different methods. A validated Tier 2 sentiment extractor would be registered alongside (not replacing) the existing SentiWS extractor, producing a separate `metric_name` (e.g., `sentiment_score_bert` vs. `sentiment_score_sentiws`). The BFF API's `/metrics/available` endpoint would expose both; the dashboard would select based on the metric's validation status.

### 8.3 Impact on the Data Contract

The `SilverCore` record remains unchanged — extractors receive immutable Silver data and produce Gold metrics. The proposed `aer_gold.metric_validity` table (§6.3) is a new Gold-layer construct that does not affect the Silver Contract (ADR-002, ADR-015).

---

## 9. References

- Cronbach, L. J. & Meehl, P. E. (1955). "Construct Validity in Psychological Tests." *Psychological Bulletin*, 52(4), 281–302.
- Grimmer, J., Roberts, M. E. & Stewart, B. M. (2022). *Text as Data: A New Framework for Machine Learning and the Social Sciences*. Princeton University Press.
- Guhr, O., Schumann, A.-K., Baber, F. & Buettner, A. (2020). "Training a Broad-Coverage German Sentiment Classification Model for Dialog Systems." *Proceedings of LREC 2020*.
- Hall, E. T. (1976). *Beyond Culture*. Anchor Books.
- Hofstede, G. (2001). *Culture's Consequences: Comparing Values, Behaviors, Institutions and Organizations Across Nations*. Sage.
- Hutto, C. J. & Gilbert, E. (2014). "VADER: A Parsimonious Rule-based Model for Sentiment Analysis of Social Media Text." *Proceedings of ICWSM 2014*.
- Jauhiainen, T., Lui, M., Zampieri, M., Baldwin, T. & Lindén, K. (2017). "Automatic Language Identification in Texts: A Survey." *Journal of Artificial Intelligence Research*, 65, 675–782.
- Koehn, P. & Knight, K. (2003). "Empirical Methods for Compound Splitting." *Proceedings of EACL 2003*.
- Krippendorff, K. (2004). *Content Analysis: An Introduction to Its Methodology* (2nd ed.). Sage.
- Remus, R., Quasthoff, U. & Heyer, G. (2010). "SentiWS — A Publicly Available German-Language Resource for Sentiment Analysis." *Proceedings of LREC 2010*.
- Robinson, W. S. (1950). "Ecological Correlations and the Behavior of Individuals." *American Sociological Review*, 15(3), 351–357.
- Rosenthal, S., Farra, N. & Nakov, P. (2017). "SemEval-2017 Task 4: Sentiment Analysis in Twitter." *Proceedings of SemEval-2017*.
- Russell, J. A. (1980). "A Circumplex Model of Affect." *Journal of Personality and Social Psychology*, 39(6), 1161–1178.
- Thelwall, M., Buckley, K., Paltoglou, G., Cai, D. & Kappas, A. (2010). "Sentiment Strength Detection in Short Informal Text." *Journal of the American Society for Information Science and Technology*, 61(12), 2544–2558.
- Van Hee, C., Lefever, E. & Hoste, V. (2018). "SemEval-2018 Task 3: Irony Detection in English Tweets." *Proceedings of SemEval-2018*.
- Wiegand, M., Balahur, A., Roth, B., Klakow, D. & Montoyo, A. (2010). "A Survey on the Role of Negation in Sentiment Analysis." *Proceedings of the Workshop on Negation and Speculation in Natural Language Processing*.

---

## Appendix A: Mapping to AĒR Open Research Questions (§13.6)

| §13.6 Question | WP-002 Section | Status |
| :--- | :--- | :--- |
| 3. Metric Validity | §2–§3, §6 | Addressed — validation framework proposed |
| 4. Cross-Cultural Comparability | §3.4, §4.3, §7.3 | Addressed — research questions formulated |
| 5. Temporal Granularity | §7.4 (Q10) | Partially addressed — deferred to WP-005 |
| 1. Probe Selection | — | Addressed in WP-001 |
| 2. Bias Calibration | — | Deferred to WP-003 |
| 6. Observer Effect | — | Deferred to WP-006 |