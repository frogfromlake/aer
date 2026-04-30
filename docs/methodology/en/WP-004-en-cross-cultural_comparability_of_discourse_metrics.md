# WP-004: Cross-Cultural Comparability of Discourse Metrics

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft — open for interdisciplinary review
> **Date:** 2026-04-07
> **Depends on:** WP-001 (Functional Probe Taxonomy), WP-002 (Metric Validity), WP-003 (Platform Bias)
> **Architectural context:** Gold Layer Aggregation (§5.1.4), Multi-Resolution Temporal Framework (§8.13), BFF API (§5.1.3), Manifesto §II–§IV
> **License:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Objective

This working paper addresses the fourth open research question from §13.6: **Can the same metric be meaningfully compared across languages and cultural contexts? What normalization is required?**

AĒR's Aleph principle — aggregating fragmented global data streams into a single coherent view — presupposes that data from different cultural contexts can be placed *alongside* each other in a unified analytical space. But comparison is not concatenation. Placing a German sentiment score next to a Japanese sentiment score in the same ClickHouse column does not make them comparable. It makes them *co-located*.

Comparability is the central methodological challenge of any cross-cultural research instrument. The history of comparative social science — from Durkheim's comparative method through the World Values Survey to the current Computational Social Science landscape — is a history of wrestling with a fundamental paradox: **to compare, you need a common frame of reference; but the imposition of a common frame risks erasing the very differences you seek to observe.** This is the comparability paradox, and it lies at the heart of AĒR's scientific agenda.

WP-001 introduced the Etic/Emic Dual Tagging System as a structural response to this paradox at the probe level. WP-004 extends this logic to the metric level. It asks: for each computational metric AĒR produces, under what conditions is cross-cultural comparison meaningful, what normalization is required, and where does comparison become epistemologically illegitimate?

---

## 2. The Comparability Paradox

### 2.1 Comparing What Cannot Be Equated

The paradox is ancient in comparative methodology. Sartori (1970) formulated it as the "ladder of abstraction": the more abstract a concept (and thus the more widely applicable across contexts), the less it captures the specific reality of any single context. The more concrete and context-sensitive a concept, the less it can be applied beyond its original setting.

Applied to AĒR's metrics:

- A **highly abstract metric** like "word count" is perfectly comparable across languages and cultures. A 200-word German article and a 200-word Japanese article have the same word count (assuming equivalent tokenization — already a non-trivial assumption, as discussed in §3.1). But word count tells us almost nothing about discourse.

- A **moderately abstract metric** like "sentiment polarity" claims to measure something culturally universal — evaluative attitude — but does so with culturally specific instruments (lexicons, models trained on culturally situated data). The same sentiment score produced by different tools in different languages may not measure the same construct (WP-002, §3.4).

- A **culturally specific metric** like "narrative frame" (e.g., "securitization frame," "human rights frame") may capture precisely the discourse phenomenon of interest, but its applicability is bounded by the cultural and political context in which the frame exists. The securitization frame (Buzan et al., 1998) is a product of Western international relations theory; applying it to Chinese foreign policy discourse imposes a theoretical lens that may not fit.

AĒR must operate at all three levels simultaneously. The Aleph vision demands high abstraction (global aggregation); the Episteme vision demands cultural sensitivity (measuring what can be thought and said *within* a specific epistemic frame); the Rhizome vision demands relational analysis (how patterns propagate *across* frames). No single level of abstraction serves all three.

### 2.2 Equivalence as a Research Problem, Not an Assumption

Comparative methodology distinguishes several types of equivalence (van de Vijver & Leung, 1997):

**Construct equivalence.** Does the metric measure the same theoretical construct across contexts? Sentiment polarity may not be the same construct in cultures with different affect systems (WP-002, §3.4). Topic categories derived from LDA on a German corpus may not represent the same thematic structures in Arabic discourse.

**Measurement equivalence.** Even if the construct is the same, does the measurement instrument operate identically? A sentiment lexicon calibrated on German newspaper text produces systematically different scores on German parliamentary debate transcripts — and the divergence is *within* a single language and culture. Across languages, measurement equivalence requires separate validation for each language (WP-002, §6.2).

**Scalar equivalence.** Can scores be compared on the same scale? Does a sentiment score of `0.3` in German and `0.3` in Japanese indicate the same *degree* of positive evaluation? Without scalar equivalence, only rank-order comparisons are valid ("more positive than"), not magnitude comparisons ("equally positive").

**Temporal equivalence.** Does a metric retain the same meaning over time within a single cultural context? Language evolves, media registers shift, political vocabularies transform. A sentiment lexicon validated in 2020 may not produce equivalent results in 2026, even within the same language.

AĒR's scientific integrity demands that **equivalence is treated as a research question to be empirically tested for each metric-context pair, not as an assumption to be silently made.** This has direct architectural consequences: the Gold layer must store sufficient metadata to distinguish between validated and unvalidated cross-cultural comparisons.

---

## 3. Linguistic Comparability Challenges

### 3.1 Tokenization: The First Invisible Divergence

Every text metric begins with tokenization — splitting continuous text into discrete units. This seemingly trivial preprocessing step introduces the first cross-linguistic incomparability.

**Space-delimited languages** (English, German, French, Spanish, Portuguese, Russian, Arabic) use whitespace as a word boundary marker. Tokenization is relatively straightforward, though complications arise from clitics (French *l'homme*), contractions (English *don't*), and compound words (German *Klimaschutzpaket*).

**Non-space-delimited languages** (Chinese, Japanese, Thai, Khmer, Lao, Myanmar) require algorithmic word segmentation. Chinese text is a continuous string of characters with no whitespace between words; the correct segmentation depends on context and is itself an NLP task with non-trivial error rates. The word *"研究生命的起源"* can be segmented as *"研究/生命/的/起源"* (research the origin of life) or *"研究生/命/的/起源"* (a graduate student's destiny's origin). Different segmenters (jieba, pkuseg, LTP) produce different token counts for the same text.

**Agglutinative languages** (Turkish, Finnish, Hungarian, Korean, Swahili, Quechua) encode grammatical relationships through affixation. A single Turkish word like *"evlerinizden"* (from your houses) encodes root, plural, possessive, and ablative case — information that requires five English words. Token-level metrics (word count, type-token ratio, lexicon coverage) are structurally incomparable between agglutinative and analytic languages without morphological normalization.

**Implication for AĒR:** The `word_count` metric in `SilverCore` — currently computed by `len(cleaned_text.split())` — is not cross-linguistically comparable. A 200-word German article and a 200-word Japanese article (after segmentation) do not represent equivalent amounts of information. AĒR must either (a) define language-specific tokenization strategies per source adapter, (b) use language-independent units (character n-grams, byte-pair encoding tokens), or (c) acknowledge that word-level metrics are intra-linguistic only.

### 3.2 Sentiment Across Languages: Different Instruments, Different Constructs

WP-002 (§3.4) documented the cultural challenges of sentiment analysis. From a comparability perspective, the problem is structural:

**No universal sentiment lexicon exists.** SentiWS (German), AFINN (English), NRC Emotion Lexicon (multilingual but English-derived), SentiWordNet (English-derived via WordNet) — each lexicon was developed by different teams, using different annotation methodologies, different corpora, and different theoretical frameworks. Even if two lexicons both claim to measure "sentiment," they operationalize it differently. SentiWS assigns continuous polarity weights; AFINN uses integer scores (-5 to +5); NRC uses binary emotion categories. Placing their outputs on the same dashboard is methodologically indefensible without explicit calibration.

**Translation does not preserve sentiment.** Machine-translating text to a common language (typically English) before applying a single sentiment tool is a common shortcut that introduces systematic bias. Translation neutralizes culturally specific connotations, flattens register differences, and imposes the target language's pragmatic conventions. The Arabic word *"إن شاء الله"* (insha'Allah) is used in contexts ranging from genuine piety to polite refusal to resigned fatalism — it cannot be translated to English and then sentiment-scored without losing its pragmatic function.

**Proposed approach: Relative rather than absolute comparison.** Instead of comparing raw sentiment scores cross-linguistically, AĒR should compare *within-context deviations* from an established baseline. If the baseline sentiment of Tagesschau articles is `0.05` and a given week produces `0.15`, the deviation of `+0.10` is a meaningful signal within the German institutional RSS context. If the baseline of NHK (Japan) articles is `0.01` and a given week produces `0.06`, the deviation of `+0.05` is a comparable signal — not because the raw scores are equivalent, but because both represent a shift of similar relative magnitude from their respective baselines.

This approach requires:

1. Establishing per-source sentiment baselines over a calibration period
2. Computing deviations from baseline (z-scores, percentile ranks, or difference-in-differences)
3. Comparing deviations, not raw values, in the dashboard
4. Documenting the baseline period, method, and cultural context as metric metadata

### 3.3 Named Entities Across Ontologies

WP-002 (§4) documented NER challenges. Cross-cultural comparability introduces a deeper issue: **ontological incommensurability**.

The entity categories used by Western NER models (PER, ORG, LOC, GPE, DATE, MONEY) reflect a Western ontological framework that distinguishes persons from organizations, locations from geopolitical entities, and temporal from monetary expressions. These distinctions are not universally salient:

- In many indigenous cultures, the distinction between PER (person) and LOC (location) is blurred by animistic ontologies where rivers, mountains, and forests have personhood.
- The ORG category presupposes a Western model of formal organization that does not map cleanly onto clan structures, caste networks, or informal patronage systems.
- The GPE (geopolitical entity) category encodes contested political boundaries as settled facts.

For AĒR's entity aggregation (`aer_gold.entities`), this means that cross-cultural entity counts and entity co-occurrence networks are only comparable within the ontological framework of the NER model used. Comparing "top entities in German discourse" with "top entities in Chinese discourse" using the same entity taxonomy produces results that are linguistically valid but ontologically flawed.

WP-001's Emic Layer addresses this at the probe level. The analogous mechanism at the metric level would be a **per-language entity ontology mapping** that preserves local entity categories alongside the etic NER labels.

### 3.4 Topic Models Across Cultural Boundaries

Topic modeling (LDA, BERTopic) discovers latent thematic structures in document collections. These structures are corpus-dependent — the topics discovered in a German corpus are not the same as those discovered in a Japanese corpus, even if both corpora cover "the news."

**Language-specific topic spaces.** A German LDA model might discover topics like "Energiewende," "Leitkultur," or "Schuldenbremse" — concepts deeply embedded in German political discourse with no direct equivalents in other languages. A Japanese model might discover "働き方改革" (work style reform) or "少子高齢化" (declining birthrate and aging population). These are not different labels for the same topics — they are different topics that reflect different societal preoccupations.

**Cross-lingual topic alignment.** Recent research on multilingual topic models (Hao & Paul, 2018; Bianchi et al., 2021) attempts to discover shared topic spaces across languages using multilingual embeddings. These models can identify thematically related clusters across languages — but "related" is not "equivalent." The German "Energiewende" topic and the Japanese "原発再稼働" (nuclear restart) topic may cluster together because both involve energy policy, but they carry fundamentally different political valences, historical contexts, and societal meanings.

**Proposed approach: Parallel topic discovery with human-validated alignment.** Rather than imposing a single cross-lingual topic model, AĒR should:

1. Run language-specific topic models per linguistic corpus (intra-cultural topic discovery)
2. Use multilingual embeddings to suggest cross-linguistic topic alignments
3. Require human validation by area experts for each proposed alignment
4. Store topic alignments as a separate mapping table, not as a property of the topics themselves

This preserves the cultural specificity of each topic while enabling validated cross-cultural comparison — a direct application of the Etic/Emic principle from WP-001.

---

## 4. Cultural Comparability: Beyond Language

### 4.1 Expressive Norms and Baseline Calibration

Cross-cultural comparability is not merely a linguistic problem — it is a cultural one. Even if two languages had identical sentiment lexicons (they do not), the *baseline expressiveness* of discourse differs across cultures in ways that affect metric interpretation.

**High-context vs. low-context communication.** Hall's (1976) distinction between high-context cultures (where meaning is embedded in context, relationships, and nonverbal cues — Japan, China, Arab cultures, much of Africa) and low-context cultures (where meaning is explicit in the text — Germany, Scandinavia, the United States) has direct implications for text-based metrics. In high-context communication, the most important information is what is *not* said. A text metric that measures only explicit content systematically underrepresents the communicative richness of high-context cultures.

**Emotional display rules.** Cultures differ in norms about which emotions can be publicly expressed and with what intensity (Matsumoto, 1990). Japanese media discourse follows *honne/tatemae* (本音/建前) norms that distinguish private opinion from public expression. Arabic media discourse operates within *wajh* (وجه, "face") norms that govern the public expression of criticism and dissent. American media discourse follows norms of affective intensification that make "amazing," "incredible," and "devastating" register-neutral descriptors. These are not noise — they are cultural data that AĒR should observe, not normalize away.

**Institutional register conventions.** Government press releases — a primary data source for AĒR's early probes — follow culturally specific institutional registers (WP-002, §3.4). The *Beamtendeutsch* of the Bundesregierung, the *guānfāng yǔyán* (官方语言) of the Chinese State Council, the British Civil Service style, and the American executive prose style all produce systematically different sentiment profiles that reflect institutional culture, not public sentiment. Comparing sentiment across these registers without accounting for register-specific baselines would produce artifacts, not findings.

### 4.2 The Normalization Dilemma

WP-002 (§7.3, Q7) previewed a fundamental epistemological tension: if AĒR normalizes metrics to account for cross-cultural baseline differences, it may erase the very phenomena it seeks to observe.

**Case study: Restraint as cultural data.** If Japanese institutional discourse is systematically more restrained than American institutional discourse (lower absolute sentiment scores, fewer superlatives, more hedging), this difference is itself a finding — it reflects different communicative norms, different institutional cultures, and different relationships between state and public. Normalizing both to the same baseline erases this finding.

**Case study: Polarization as relative phenomenon.** If German political discourse shows a sentiment range of [-0.3, +0.3] while American political discourse shows [-0.8, +0.8], the wider American range may indicate higher polarization — or it may reflect different expressive norms. Only with cultural expertise can we distinguish these explanations.

**The normalization spectrum:**

| Strategy | What It Compares | What It Preserves | What It Erases |
| :--- | :--- | :--- | :--- |
| **Raw values** | Nothing (values are incommensurable) | Everything | Nothing |
| **Z-score per source** | Within-source deviations from baseline | Temporal dynamics within each source | Absolute level differences between sources |
| **Percentile rank per language** | Relative position within linguistic corpus | Rank ordering within language | Scale differences between languages |
| **Deviation from discourse-function baseline** | Shifts relative to same-function sources | Functional equivalence (WP-001) | Within-function cultural variation |
| **Universal normalization** | Raw positions on a single global scale | Nothing culture-specific | Everything culture-specific |

AĒR should support **multiple normalization views simultaneously**, selectable by the analyst. The dashboard should never present a single "correct" comparison — it should expose the raw data, the normalization method, and the cultural context, enabling the analyst to select the comparison that is appropriate for their research question.

### 4.3 Temporal Rhythms Across Cultures

Temporal metrics — publication frequency, time-of-day distribution, day-of-week patterns — are among AĒR's most robust and language-independent metrics. Yet even these carry cultural specificity.

**Weekly rhythms.** The "weekend" is culturally defined. In most Western countries, Saturday and Sunday are rest days. In Israel, the weekend is Friday–Saturday. In many Muslim-majority countries, Friday is the primary rest day; some observe Friday–Saturday, others Thursday–Friday. AĒR's `publication_weekday` metric (0=Monday, 6=Sunday) does not encode this cultural knowledge. A dip in publication frequency on Friday in Saudi Arabian sources and on Sunday in German sources reflect the same social phenomenon — the weekly rest cycle — but are encoded as different data points.

**News cycles.** The rhythm of news publication varies by media culture. American news cycles are 24-hour with peaks driven by East Coast business hours. Japanese news cycles follow *asa-kan/yū-kan* (朝刊/夕刊, morning/evening edition) rhythms inherited from print journalism. European news cycles vary by country. Breaking news dynamics differ by platform: Twitter/X operates on minute-level cycles; RSS on hour-level; traditional media on half-day cycles.

**Calendar effects.** Cultural and religious calendars affect publication patterns in ways that are not captured by the Gregorian calendar. Ramadan, Chinese New Year, Diwali, Obon, and national independence days all create temporal patterns in discourse that require cultural context to interpret. A drop in publication volume during Chinese New Year is not a decrease in discourse — it is a cultural rhythm.

---

## 5. Extending the Etic/Emic Framework to Metrics

### 5.1 From Probes to Metrics

WP-001 established the Etic/Emic Dual Tagging System for probe classification:

- **Etic Layer:** Abstract, functional classification enabling cross-cultural aggregation
- **Emic Layer:** Local, context-specific metadata preserving cultural reality

The same logic applies to metrics. Each metric in the Gold layer should carry both:

- **Etic metric identity:** What abstract construct does this metric measure? (e.g., "evaluative polarity," "entity salience," "thematic focus")
- **Emic metric context:** What culturally specific instrument produced this measurement? (e.g., "SentiWS v2.0 on German text," "ASARI on Japanese text," "BERT-base-arabic-sentiment on Arabic text")

This enables the BFF API to expose two views:

1. **Etic aggregation:** "Show me evaluative polarity across all sources" — aggregates metrics from different instruments that measure the same construct, with explicit equivalence metadata
2. **Emic detail:** "Show me SentiWS sentiment for Tagesschau" — shows the raw output of a specific instrument on a specific source, without cross-cultural claims

### 5.2 The Metric Equivalence Registry

To operationalize etic/emic metric classification, AĒR needs a **Metric Equivalence Registry** — a curated mapping that defines which instrument-specific metrics are considered equivalent for cross-cultural comparison, under what conditions, and with what confidence.

```sql
CREATE TABLE aer_gold.metric_equivalence (
    etic_construct     String,         -- e.g., "evaluative_polarity"
    metric_name        String,         -- e.g., "sentiment_score_sentiws"
    language           String,         -- e.g., "de"
    instrument_version String,         -- e.g., "SentiWS_v2.0"
    equivalence_type   String,         -- "construct", "measurement", "scalar"
    validation_status  String,         -- "validated", "assumed", "contested"
    validation_ref     Nullable(String), -- reference to validation study
    valid_from         DateTime,
    valid_until        Nullable(DateTime)
) ENGINE = MergeTree()
ORDER BY (etic_construct, language, metric_name);
```

This table is *not* populated automatically — it is curated by researchers through the validation process described in WP-002 (§6.2). Each entry represents a scholarly judgment: "SentiWS sentiment on German RSS and ASARI sentiment on Japanese RSS are considered measurement-equivalent for the construct 'evaluative polarity' based on [validation study]." Without an entry in this registry, cross-cultural aggregation of a metric pair should be flagged as unvalidated in the dashboard.

### 5.3 Three Levels of Cross-Cultural Comparison

Based on the equivalence framework, AĒR can support three levels of cross-cultural comparison, each with different requirements and confidence levels:

**Level 1: Temporal pattern comparison (language-independent).** Comparing *when* discourse happens across cultures: publication frequency, time-of-day distributions, weekly rhythms, event-response latencies. These metrics are structurally comparable because they measure time, not text. Cultural calendar knowledge is required for interpretation but not for computation. Confidence: High.

**Level 2: Relative deviation comparison (baseline-normalized).** Comparing *how much discourse changes* across cultures: z-score deviations from established baselines, within-source trend directions, volatility measures. These comparisons are meaningful because they measure change relative to a culturally calibrated reference point. They do not claim that the absolute values are equivalent — only that the magnitude and direction of change are comparable. Confidence: Medium, requires per-source baseline calibration.

**Level 3: Absolute value comparison (instrument-harmonized).** Comparing *what discourse says* across cultures: sentiment levels, topic prevalences, entity saliences. This is the most ambitious and most fragile comparison level. It requires validated scalar equivalence between instruments, normalization for expressive norms, and expert judgment on construct equivalence. Confidence: Low without extensive validation; should never be the default dashboard view.

---

## 6. Architectural Implications

> **Implementation status (Phase 65 + Phase 115).** The schema and the
> raw / zscore normalization parameter (§6.1, §6.2) shipped in Phase 65;
> the existence-gate that refuses unvalidated normalized requests
> shipped at the same time. Phase 115 sharpens that into the cross-frame
> equivalence gate, adds `?normalization=percentile` as a third Level-2
> view backed by ClickHouse window functions, adds the `notes` column
> on `aer_gold.metric_equivalence` and the corresponding Postgres
> `equivalence_reviews` workflow table, automates baseline maintenance
> via the in-process `MetricBaselineExtractor` (the standalone
> `scripts/compute_baselines.py` is retained for first-run / ad-hoc
> operations and shares the canonical computation function), and
> implements the §6.3 dashboard treatment — the LensBar normalization
> control, the deviation byline on Level-2 views, the Probe Dossier
> "valid comparisons" panel, and the cross-frame refusal surface.

### 6.1 Gold Layer Extensions

The current `aer_gold.metrics` schema stores `(timestamp, value, source, metric_name)`. To support cross-cultural comparability, the following extensions are proposed:

**Baseline table.** Stores computed baselines per source and metric for deviation calculations:

```sql
CREATE TABLE aer_gold.metric_baselines (
    metric_name    String,
    source         String,
    language       String,
    baseline_value Float64,
    baseline_std   Float64,
    window_start   DateTime,
    window_end     DateTime,
    n_documents    UInt32,
    compute_date   DateTime
) ENGINE = ReplacingMergeTree(compute_date)
ORDER BY (metric_name, source, language);
```

**Deviation view.** A ClickHouse materialized view or query-time computation that produces z-scores:

```sql
SELECT
    m.timestamp,
    m.source,
    m.metric_name,
    m.value AS raw_value,
    (m.value - b.baseline_value) / nullIf(b.baseline_std, 0) AS z_score
FROM aer_gold.metrics m
JOIN aer_gold.metric_baselines b
    ON m.metric_name = b.metric_name AND m.source = b.source
```

This enables the BFF API to serve both raw values and normalized deviations, letting the frontend choose the appropriate comparison level.

### 6.2 BFF API Extensions

The BFF API (`/api/v1/metrics`) should support a `normalization` query parameter:

- `?normalization=raw` — return raw metric values (default, current behavior)
- `?normalization=zscore` — return z-score deviations from source baselines
- `?normalization=percentile` — return within-language percentile ranks

The `/api/v1/metrics/available` endpoint should expose equivalence metadata: which metrics are validated for cross-cultural comparison and at which equivalence level.

### 6.3 Dashboard Design Implications

The dashboard must never silently place non-equivalent metrics on the same chart. When displaying cross-cultural comparisons:

1. **Default to Level 1 (temporal patterns).** Show publication frequency, temporal distribution, and event-response timing across cultures. These are safe comparisons.

2. **Enable Level 2 (deviations) with labeling.** Show z-score deviations with clear labeling: "This chart shows deviation from source baseline, not absolute sentiment." Include the baseline period and method in tooltips.

3. **Gate Level 3 (absolute values) behind validation status.** Only display absolute cross-cultural metric comparisons for metric pairs that have entries in the Metric Equivalence Registry with `validation_status = 'validated'`. For unvalidated pairs, show a warning and offer the deviation view instead.

---

## 7. Open Questions for Interdisciplinary Collaborators

### 7.1 For Comparative Social Scientists and Methodologists

**Q1: Which of AĒR's planned metrics are candidates for cross-cultural scalar equivalence, and which are inherently incommensurable?**

- Sentiment polarity, topic prevalence, entity salience, narrative frames — for each, is there a realistic path to establishing scalar equivalence across languages, or should AĒR accept that these metrics are only meaningful intra-culturally?
- Deliverable: A metric-by-metric assessment of comparability potential, with recommended comparison levels (temporal, deviation, absolute) for each.

**Q2: What validation methodology is appropriate for establishing cross-cultural metric equivalence?**

- In survey methodology, measurement invariance is tested via multi-group confirmatory factor analysis (MGCFA). Is there an analogous methodology for computational text metrics?
- Deliverable: A validation protocol for cross-cultural metric equivalence, adapted for computational discourse analysis.

**Q3: How should AĒR handle metrics that are valid intra-culturally but incommensurable cross-culturally?**

- Should such metrics be displayed side-by-side with explicit non-equivalence warnings? Should the dashboard offer culturally contextualized interpretive frames? Should incommensurable metrics be excluded from cross-cultural views entirely?
- Deliverable: Dashboard design guidelines for presenting non-equivalent but co-displayed metrics.

### 7.2 For Computational Linguists

**Q4: What tokenization strategy enables the most meaningful cross-linguistic word-level metrics?**

- Should AĒR use language-specific tokenizers (spaCy per language, jieba for Chinese, MeCab for Japanese), a universal subword tokenizer (SentencePiece, BPE), or character-level features?
- Deliverable: A comparative evaluation of tokenization strategies on a multilingual news corpus, with impact on downstream metric comparability.

**Q5: Can multilingual sentence embeddings serve as a language-independent feature space for cross-cultural topic comparison?**

- Models like XLM-RoBERTa and LaBSE produce language-independent embeddings. Are these embeddings sufficiently culturally neutral for cross-cultural topic alignment, or do they embed English-centric semantic structures?
- Deliverable: An evaluation of multilingual embedding spaces for cross-cultural topic alignment, with specific attention to non-European languages.

### 7.3 For Cultural Anthropologists and Area Studies Scholars

**Q6: For each major cultural region, what are the key expressive norms that affect text metric baselines?**

- AĒR needs documentation of expressive conventions, institutional registers, and communicative norms that affect how text metrics should be interpreted in each cultural context.
- Deliverable: Cultural calibration profiles (2–3 pages each) for the same regions identified in WP-003 Q6, focused on text-level communicative norms rather than platform choice.

**Q7: Which concepts and topics are culturally specific and should *not* be aligned cross-culturally?**

- Some discourse themes are so deeply embedded in local cultural, historical, and political context that cross-cultural alignment would be misleading. Identifying these "non-comparable" topics is as important as identifying comparable ones.
- Deliverable: A list of culturally bounded discourse concepts per region, with explanations of why cross-cultural alignment is inappropriate.

### 7.4 For Statisticians and Data Scientists

**Q8: What baseline calibration period is required for stable z-score computation per source?**

- How many documents, over how many days, are needed to establish a reliable baseline? Does the required calibration period differ by source type (high-volume news feed vs. low-volume government press releases)?
- Deliverable: A statistical analysis of baseline stability as a function of corpus size and temporal window, using AĒR's Probe 0 data as a pilot.

**Q9: How should AĒR propagate normalization uncertainty through the aggregation pipeline?**

- When z-scores are computed from uncertain baselines and aggregated across sources, how should the resulting uncertainty be quantified and displayed?
- Deliverable: An uncertainty propagation framework for normalized discourse metrics, compatible with ClickHouse's aggregation functions.

---

## 8. The Ethical Dimension of Comparison

Cross-cultural comparison is not merely a technical challenge — it is an ethical one. The history of comparative social science includes episodes where quantitative comparisons across cultures served to rank societies on a presumed scale of "development" or "modernity" — a legacy of colonial epistemology that AĒR must actively resist.

**The ranking trap.** A dashboard that shows "Country A has higher positive sentiment than Country B" invites ranking. But ranking presupposes a normative framework in which higher positive sentiment is "better." AĒR's Manifesto commits to observation over evaluation — to documenting resonance, not judging it. The dashboard design must resist the temptation to present comparison as ranking.

**The universalism trap.** A cross-cultural metric comparison implicitly claims that the measured construct is universal — that "sentiment" is the same thing in all cultures. This claim is empirically contestable and must be treated as a hypothesis, not an assumption. The Metric Equivalence Registry (§5.2) is the architectural mechanism for making this claim explicit and falsifiable.

**The deficit trap.** Comparing metrics across cultures risks framing difference as deficit. If a culture's digital discourse shows lower "diversity" (however measured), this could reflect a cohesive public sphere, a restricted media environment, or a measurement artifact — these explanations carry vastly different normative valences. AĒR's Progressive Disclosure principle serves as a safeguard: the analyst can always drill down from the aggregate comparison to the cultural context.

The fundamental ethical commitment is: **AĒR compares to understand, not to rank.** The architecture must enforce this commitment through design — by making normalization explicit, equivalence claims falsifiable, and cultural context always accessible.

---

## 9. References

- Bianchi, F., Terragni, S. & Hovy, D. (2021). "Cross-lingual Contextualized Topic Models with Zero-shot Learning." *Proceedings of EACL 2021*.
- Buzan, B., Wæver, O. & de Wilde, J. (1998). *Security: A New Framework for Analysis*. Lynne Rienner.
- Hall, E. T. (1976). *Beyond Culture*. Anchor Books.
- Hao, S. & Paul, M. J. (2018). "Lessons from the Bible on Modern Topics: Low-Resource Multilingual Topic Model Evaluation." *Proceedings of NAACL 2018*.
- Matsumoto, D. (1990). "Cultural Similarities and Differences in Display Rules." *Motivation and Emotion*, 14(3), 195–214.
- Sartori, G. (1970). "Concept Misformation in Comparative Politics." *American Political Science Review*, 64(4), 1033–1053.
- van de Vijver, F. J. R. & Leung, K. (1997). *Methods and Data Analysis for Cross-Cultural Research*. SAGE.

---

## Appendix A: Mapping to AĒR Open Research Questions (§13.6)

| §13.6 Question | WP-004 Section | Status |
| :--- | :--- | :--- |
| 4. Cross-Cultural Comparability | §2–§6 (full treatment) | Addressed — framework, normalization strategies, and research questions proposed |
| 3. Metric Validity | §3.2 (cross-lingual sentiment), §3.3 (NER ontologies) | Cross-reference to WP-002 |
| 5. Temporal Granularity | §4.3 (temporal rhythms across cultures) | Partially addressed — deferred to WP-005 |
| 1. Probe Selection | §5 (etic/emic metrics extending WP-001) | Cross-reference to WP-001 |

## Appendix B: Comparison Level Decision Matrix

| Research Question Type | Recommended Level | Normalization | Requirements |
| :--- | :--- | :--- | :--- |
| "When does discourse peak across cultures?" | Level 1 (temporal) | None needed | Cultural calendar knowledge |
| "Did sentiment shift in both DE and JP after event X?" | Level 2 (deviation) | Z-score per source | Per-source baseline calibration |
| "Is German discourse more polarized than Japanese?" | Level 3 (absolute) | Instrument harmonization | Validated scalar equivalence, expert review |
| "What topics dominate in each culture?" | Intra-cultural only | N/A | Per-language topic models, human alignment |
| "Do the same entities appear across cultures?" | Level 1 (co-occurrence) | Entity linking to shared KB | Multilingual entity linking, ontology alignment |