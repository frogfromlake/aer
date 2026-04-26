// Open Research Questions catalog — all questions from WP §7 and §8 sections.
//
// Faithfully transcribed from the Working Paper series. Each entry maps to
// a numbered question (Q1, Q2, …) within a named disciplinary subsection.
// The /reflection/open-questions hub renders these as a structured index
// grouped by source paper and discipline.

export interface OpenQuestion {
  id: string; // 'wp-001-q1'
  sourceWp: string; // 'wp-001'
  sourceSection: string; // '8' — the §N where the questions live
  subsection: string; // '8.1', '7.2', etc.
  disciplinaryScope: string; // "For Cultural Anthropologists and Area Studies Scholars"
  shortLabel: string; // Q1 headline (bold text from WP)
  question: string; // full question text
  deliverable?: string; // "Deliverable: …" from the WP
  pipelineHook?: string; // where external collaboration can contribute in AĒR
}

export const OPEN_QUESTIONS: OpenQuestion[] = [
  // -------------------------------------------------------------------------
  // WP-001 §8 — Probe Catalog and Functional Taxonomy
  // -------------------------------------------------------------------------
  {
    id: 'wp-001-q1',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.1',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'Regional probe mapping: which sources serve each discourse function?',
    question:
      'For each major cultural region, which sources serve each of the four discourse functions? This is the foundational question. AĒR needs area experts to map the discourse landscape of their region of expertise onto the four-function taxonomy.',
    deliverable:
      'Regional Probe Nomination Reports (3–5 pages each) identifying candidate sources for each discourse function, with emic descriptions and etic classifications, for at minimum: Germany, France, United Kingdom, United States, Brazil, Russia, China, Japan, India, Nigeria, Iran, Saudi Arabia, Indonesia, South Africa, Mexico.',
    pipelineHook:
      'Probe Dossier directory; Source Adapter Protocol (ADR-015); SilverMeta emic fields'
  },
  {
    id: 'wp-001-q2',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.1',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'Are four discourse functions sufficient, or does the taxonomy need refinement?',
    question:
      'Do some societies have discourse functions that do not map cleanly to the four categories? Is there a fifth function (e.g., "mediation" — actors that bridge between functions)? Should any function be split (e.g., distinguishing religious epistemic authority from scientific epistemic authority)?',
    deliverable:
      'A critical review of the taxonomy based on empirical analysis of non-Western discourse landscapes.',
    pipelineHook:
      'WP-001 §3 taxonomy; the four-lane structure on Surface II; discourse_function field in aer_gold.metrics'
  },
  {
    id: 'wp-001-q3',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.1',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'How should AĒR handle sources that shift discourse function over time?',
    question:
      'When a formerly independent media outlet is captured by the state, it transitions from epistemic authority (or counter-discourse) to power legitimation. How should this transition be documented and when should the etic tag be updated?',
    deliverable:
      'A protocol for detecting and documenting functional transitions, with criteria for re-classification.',
    pipelineHook: 'Etic tag fields in SilverMeta; Probe Dossier revision history'
  },
  {
    id: 'wp-001-q4',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.2',
    disciplinaryScope: 'For Comparative Political Scientists',
    shortLabel: 'How should AĒR weight probes within a probe constellation?',
    question:
      'The function-stratified approach (§5.2) avoids the weighting problem by refusing to aggregate across functions. But within a function, multiple probes may serve the same function with different reach. How should intra-function weighting be handled?',
    deliverable:
      'A weighting methodology for intra-function probe aggregation, drawing on survey sampling theory and media system analysis.',
    pipelineHook:
      'Probe constellation architecture; function-coverage indicator in the Probe Dossier'
  },
  {
    id: 'wp-001-q5',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.2',
    disciplinaryScope: 'For Comparative Political Scientists',
    shortLabel: "How does AĒR's functional taxonomy relate to existing media system typologies?",
    question:
      "Hallin and Mancini's (2004) three models of media and politics (Liberal, Democratic Corporatist, Polarized Pluralist) is the dominant typology in comparative media studies. How does the functional taxonomy relate to, extend, or challenge these models?",
    deliverable:
      "A mapping between Hallin-Mancini media system types and AĒR's discourse function taxonomy, identifying where the taxonomies align and where they diverge.",
    pipelineHook: 'Probe registration; etic classification in SilverMeta'
  },
  {
    id: 'wp-001-q6',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.3',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'Can discourse function be detected computationally?',
    question:
      'Is there a text-level signal that distinguishes epistemic authority discourse from power legitimation discourse? Could a trained classifier assign discourse function tags automatically, reducing the dependence on area experts?',
    deliverable:
      'A feasibility study on automated discourse function classification, using a labeled corpus of sources with known function assignments.',
    pipelineHook: 'MetricExtractor protocol; discourse_function derivation in processor.py'
  },
  {
    id: 'wp-001-q7',
    sourceWp: 'wp-001',
    sourceSection: '8',
    subsection: '8.3',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'How should AĒR measure inter-functional dynamics?',
    question:
      'The interaction between discourse functions — how counter-discourse responds to power legitimation, how epistemic authority mediates between competing identity narratives — is arguably the most analytically interesting phenomenon AĒR can observe. What metrics capture these dynamics?',
    deliverable:
      'A set of inter-functional metrics (e.g., response latency between functions, topic convergence/divergence across functions, entity co-occurrence across functions), with mathematical definitions and computational feasibility assessment.',
    pipelineHook:
      'EntityCoOccurrenceExtractor; aer_gold.entity_cooccurrences; Network Science view-mode cells'
  },

  // -------------------------------------------------------------------------
  // WP-002 §7 — Metric Validity and Sentiment Calibration
  // -------------------------------------------------------------------------
  {
    id: 'wp-002-q1',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: "Which sentiment method is appropriate for AĒR's constraints?",
    question:
      "Which sentiment method(s) are appropriate for German editorial text from institutional RSS feeds, given AĒR's constraints of determinism and transparency? AĒR's current method (SentiWS lexicon, mean polarity) is a Tier 1 baseline. Is there a validated, deterministic alternative that handles negation and compositionality while remaining auditable? If the answer is 'no deterministic method is adequate,' how should AĒR integrate a Tier 2 method (e.g., a fine-tuned classifier with pinned model version) while preserving traceability?",
    deliverable: 'A recommendation with validation evidence on a comparable corpus.',
    pipelineHook: 'SentimentExtractor; metric_provenance.yaml; Tier 1/2 classification'
  },
  {
    id: 'wp-002-q2',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'What annotation scheme should AĒR use for sentiment validation?',
    question:
      'Is a unidimensional positive-negative scale appropriate, or does AĒR need a multidimensional affect model (valence + arousal, or discrete emotions)? How should annotators handle institutional register (press releases that are deliberately neutral)? Is "neutral" the absence of sentiment, or is it a distinct communicative stance?',
    deliverable: 'An annotation codebook with worked examples from German RSS descriptions.',
    pipelineHook: 'SentimentExtractor; aer_gold.metric_validity table; validation framework in §6'
  },
  {
    id: 'wp-002-q3',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'How should AĒR normalize sentiment scores for cross-source comparison?',
    question:
      'Bundesregierung press releases and Tagesschau articles have structurally different sentiment baselines. How do we compare sentiment change across sources when the absolute levels are not comparable?',
    deliverable:
      'A normalization strategy (z-score per source, percentile ranking, difference-in-differences) with statistical justification.',
    pipelineHook:
      'normalization=zscore parameter in GET /api/v1/metrics; equivalence registry (WP-004 §5.2); normalization_equivalence_missing refusal type'
  },
  {
    id: 'wp-002-q4',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Linguists / NLP Researchers',
    shortLabel: 'How should AĒR handle German compound words in sentiment analysis?',
    question:
      'Should compounds be decomposed before lexicon lookup? If so, which decomposition strategy is appropriate (frequency-based, rule-based, neural)? How does this extend to other agglutinative languages (Turkish, Finnish, Korean) when AĒR expands beyond German?',
    deliverable:
      'An evaluation of compound decomposition strategies on a German news corpus, with impact on sentiment scoring accuracy.',
    pipelineHook: 'SentimentExtractor; SentiWS v2.0 lexicon integration'
  },
  {
    id: 'wp-002-q5',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Linguists / NLP Researchers',
    shortLabel: 'Which NER model and entity linking strategies are appropriate for AĒR?',
    question:
      'The current spaCy de_core_news_lg model is monolingual. AĒR will need to process Arabic, Mandarin, Hindi, Portuguese, Spanish, French, and other languages. Is a single multilingual model (XLM-RoBERTa) preferable to per-language models? How should entity linking handle entities that are salient in non-Western contexts but underrepresented in Wikidata?',
    deliverable:
      'A benchmark comparison of NER/entity linking approaches on a multilingual news corpus, evaluated per language and entity type.',
    pipelineHook: 'NamedEntityExtractor; spaCy model selection; aer_gold.entities'
  },
  {
    id: 'wp-002-q6',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Linguists / NLP Researchers',
    shortLabel: 'How should AĒR detect and handle code-switched texts?',
    question:
      'In multilingual digital spaces, documents frequently mix languages. How should language detection, sentiment scoring, and entity extraction operate on code-switched text?',
    deliverable:
      "A survey of code-switching detection methods with a recommendation for AĒR's per-document processing model.",
    pipelineHook:
      'LanguageDetectionExtractor; langdetect seed; aer_gold.language_detections; SilverCore.language field'
  },
  {
    id: 'wp-002-q7',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'How should AĒR calibrate sentiment baselines across cultures?',
    question:
      'If Japanese editorial prose is systematically more restrained than American journalism, a cross-cultural sentiment comparison requires baseline normalization. But the baseline is not merely a statistical artifact — it is the cultural phenomenon AĒR aims to observe. Is baseline normalization an act of cultural erasure? Should AĒR preserve raw baselines and visualize the difference as a finding?',
    deliverable:
      'A position paper on the epistemological tension between normalization and cultural observation.',
    pipelineHook:
      'z-score normalization gate; Metric Equivalence Registry (WP-004 §5.2); non-prescriptive visualization principle (Design Brief §7.5)'
  },
  {
    id: 'wp-002-q8',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: "How should AĒR's entity ontology handle culturally specific organizational forms?",
    question:
      'WP-001 defines a Dual Tagging System (Etic/Emic layers). How should this extend to entity classification? Should a Japanese keiretsu be tagged as ORG in the Etic layer while preserving its emic specificity? What culturally specific entity types are systematically missed by Western NER models?',
    deliverable:
      "A cross-cultural entity taxonomy that extends spaCy's default labels with culturally informed categories, mapped to the WP-001 Emic Layer.",
    pipelineHook:
      'NamedEntityExtractor; entity_label field in aer_gold.entities; SilverMeta emic tags'
  },
  {
    id: 'wp-002-q9',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Methodologists and Statisticians',
    shortLabel: 'How should AĒR account for the ecological inference problem in aggregation?',
    question:
      'When sentiment metrics are aggregated across documents, sources, and time windows in the Gold layer, how can AĒR detect whether observed trends reflect genuine discourse shifts vs. compositional changes in the corpus?',
    deliverable:
      'A statistical framework for decomposing aggregate metric changes into within-source and between-source components.',
    pipelineHook:
      'GET /api/v1/metrics with source filter; Progressive Disclosure (L5 Evidence Reader); CorpusExtractor protocol'
  },
  {
    id: 'wp-002-q10',
    sourceWp: 'wp-002',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Methodologists and Statisticians',
    shortLabel: 'What temporal aggregation window is appropriate for different metric types?',
    question:
      'Sentiment may be meaningful at daily resolution for breaking news but only at weekly or monthly resolution for cultural drift. Topic prevalence may require different temporal windows than entity co-occurrence networks.',
    deliverable:
      'An empirical analysis of metric stability across temporal resolutions on a pilot corpus.',
    pipelineHook:
      'TemporalDistributionExtractor; 5-minute downsampling in GET /api/v1/metrics; the Aleph/Episteme temporal modes'
  },

  // -------------------------------------------------------------------------
  // WP-003 §7 — Platform Bias and Algorithmic Amplification
  // -------------------------------------------------------------------------
  {
    id: 'wp-003-q1',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Platform Governance and Internet Studies Scholars',
    shortLabel: 'How should AĒR document and account for algorithmic curation effects?',
    question:
      "AĒR's RSS-based Probe 0 avoids algorithmic curation almost entirely. When expanding to social media platforms, what metadata and counterfactual strategies are required to distinguish algorithmic signal from societal signal?",
    deliverable:
      'A framework for documenting algorithmic curation effects per platform, suitable for inclusion in SilverMeta.',
    pipelineHook:
      'BiasContext fields in SilverMeta (WP-003 §8.1); Source Adapter Protocol (ADR-015)'
  },
  {
    id: 'wp-003-q2',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Platform Governance and Internet Studies Scholars',
    shortLabel: 'How should AĒR navigate the post-2023 research API landscape?',
    question:
      'With X, Reddit, and Meta restricting academic API access, what alternative data collection strategies are available, ethical, and legally defensible? Which platforms currently offer robust academic research APIs, and what are their known limitations?',
    deliverable:
      'A platform-by-platform access assessment with legal and ethical annotations, updated annually.',
    pipelineHook: 'RSS Crawler (current); future crawler architecture in crawlers/'
  },
  {
    id: 'wp-003-q3',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Social Scientists and Bot Researchers',
    shortLabel: 'Which bot detection methods are applicable to text-only processing?',
    question:
      "Text-only bot detection is less accurate than account-level detection. Which methods are applicable to AĒR's architecture, given that AĒR processes text and metadata but often lacks full account-level behavioral data? What confidence thresholds are appropriate for a system that flags rather than filters?",
    deliverable:
      'An evaluation of text-level and metadata-level bot detection methods on a multilingual corpus, with false positive/negative rates per method.',
    pipelineHook:
      'Future authenticity extractors (MetricExtractor protocol); human_authorship_confidence metric'
  },
  {
    id: 'wp-003-q4',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Social Scientists and Bot Researchers',
    shortLabel: 'How should AĒR detect and flag AI-generated content?',
    question:
      'Current detection methods degrade rapidly. Is there a sustainable detection strategy, or should AĒR accept AI-generated content as an observable phenomenon and focus on measuring its prevalence rather than filtering it out?',
    deliverable:
      'A position paper on the epistemological status of AI-generated content in discourse observation systems, with practical recommendations.',
    pipelineHook:
      'Future Tier 2/3 authenticity extractors; template_match_score metric; MetricExtractor protocol'
  },
  {
    id: 'wp-003-q5',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Social Scientists and Bot Researchers',
    shortLabel: 'How can AĒR detect coordinated inauthentic behavior using corpus-level analysis?',
    question:
      "CIB detection typically requires network data (follower graphs, repost chains). Can temporal co-occurrence and content similarity in AĒR's Gold layer serve as proxies?",
    deliverable:
      'A method for CIB detection using only the features available in SilverCore and SilverMeta (timestamps, source identifiers, cleaned text), evaluated against known CIB campaigns.',
    pipelineHook:
      'CorpusExtractor protocol; aer_gold.entity_cooccurrences; EntityCoOccurrenceExtractor'
  },
  {
    id: 'wp-003-q6',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Area Studies Scholars and Digital Anthropologists',
    shortLabel: 'Which platforms constitute the minimum viable probe set per cultural region?',
    question:
      "AĒR's WP-001 taxonomy defines four discourse functions. For a given society (e.g., Brazil, Japan, Nigeria, Iran), which platforms map to which functions? Which platforms are technically, legally, and ethically accessible?",
    deliverable:
      'Regional platform maps (1–2 pages each) covering at minimum: East Asia, South Asia, MENA, Sub-Saharan Africa, Latin America, Russia/post-Soviet, Europe, North America.',
    pipelineHook: 'Probe registration; crawler architecture; Source Adapter Protocol'
  },
  {
    id: 'wp-003-q7',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Area Studies Scholars and Digital Anthropologists',
    shortLabel: 'How do platform-specific communicative norms affect metric interpretability?',
    question:
      'A Twitter/X thread, a Reddit comment, a Telegram channel post, and an RSS article about the same event will produce structurally different text. How should AĒR account for platform-specific genre effects when comparing metrics across sources?',
    deliverable:
      'A platform-genre taxonomy that documents the structural, rhetorical, and pragmatic differences between text produced on different platforms.',
    pipelineHook:
      'SilverMeta platform_type field; source_type in SilverCore; discourse_function derivation'
  },
  {
    id: 'wp-003-q8',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Survey Methodologists and Statisticians',
    shortLabel: 'Can survey weighting techniques be adapted for digital discourse data?',
    question:
      'Survey weighting requires known population parameters and known selection probabilities. Neither is available for digital platforms. Under what conditions, if any, can weighting reduce demographic skew in crawled data?',
    deliverable:
      "A methodological assessment of weighting strategies for non-probability digital samples, with recommendations for AĒR's aggregation pipeline.",
    pipelineHook:
      'Gold layer aggregation; Negative Space overlay (Phase 113); demographic skew in WP-003 §6'
  },
  {
    id: 'wp-003-q9',
    sourceWp: 'wp-003',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Survey Methodologists and Statisticians',
    shortLabel: 'How should AĒR model and report uncertainty from platform bias?',
    question:
      'If all metrics carry platform-induced uncertainty, how should this uncertainty be propagated through the aggregation pipeline and visualized in the dashboard?',
    deliverable:
      "An uncertainty quantification framework for platform-mediated discourse metrics, compatible with AĒR's ClickHouse Gold schema.",
    pipelineHook:
      'aer_gold.metrics schema; known_limitations in metric_provenance.yaml; uncertainty visualization in Surface II charts'
  },

  // -------------------------------------------------------------------------
  // WP-004 §7 — Cross-Cultural Comparability
  // -------------------------------------------------------------------------
  {
    id: 'wp-004-q1',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Comparative Social Scientists and Methodologists',
    shortLabel: "Which of AĒR's metrics are candidates for cross-cultural scalar equivalence?",
    question:
      'Sentiment polarity, topic prevalence, entity salience, narrative frames — for each, is there a realistic path to establishing scalar equivalence across languages, or should AĒR accept that these metrics are only meaningful intra-culturally?',
    deliverable:
      'A metric-by-metric assessment of comparability potential, with recommended comparison levels (temporal, deviation, absolute) for each.',
    pipelineHook:
      'aer_gold.metric_equivalence table; normalization=zscore gate; normalization_equivalence_missing refusal type'
  },
  {
    id: 'wp-004-q2',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Comparative Social Scientists and Methodologists',
    shortLabel: 'What validation methodology establishes cross-cultural metric equivalence?',
    question:
      'In survey methodology, measurement invariance is tested via multi-group confirmatory factor analysis (MGCFA). Is there an analogous methodology for computational text metrics?',
    deliverable:
      'A validation protocol for cross-cultural metric equivalence, adapted for computational discourse analysis.',
    pipelineHook: 'aer_gold.metric_equivalence; validation framework in WP-002 §6'
  },
  {
    id: 'wp-004-q3',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Comparative Social Scientists and Methodologists',
    shortLabel:
      'How should AĒR handle metrics that are valid intra-culturally but incommensurable cross-culturally?',
    question:
      'Should such metrics be displayed side-by-side with explicit non-equivalence warnings? Should the dashboard offer culturally contextualized interpretive frames? Should incommensurable metrics be excluded from cross-cultural views entirely?',
    deliverable:
      'Dashboard design guidelines for presenting non-equivalent but co-displayed metrics.',
    pipelineHook:
      'Refusal surface for normalization_equivalence_missing; Progressive Disclosure; Cultural context notes in metric provenance'
  },
  {
    id: 'wp-004-q4',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Linguists',
    shortLabel:
      'What tokenization strategy enables the most meaningful cross-linguistic word-level metrics?',
    question:
      'Should AĒR use language-specific tokenizers (spaCy per language, jieba for Chinese, MeCab for Japanese), a universal subword tokenizer (SentencePiece, BPE), or character-level features?',
    deliverable:
      'A comparative evaluation of tokenization strategies on a multilingual news corpus, with impact on downstream metric comparability.',
    pipelineHook: 'Analysis Worker extractor pipeline; future multilingual extractor support'
  },
  {
    id: 'wp-004-q5',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Computational Linguists',
    shortLabel: 'Can multilingual sentence embeddings serve as a cross-cultural feature space?',
    question:
      'Models like XLM-RoBERTa and LaBSE produce language-independent embeddings. Are these embeddings sufficiently culturally neutral for cross-cultural topic alignment, or do they embed English-centric semantic structures?',
    deliverable:
      'An evaluation of multilingual embedding spaces for cross-cultural topic alignment, with specific attention to non-European languages.',
    pipelineHook: 'Future Tier 2/3 topic extractors; planned LDA/BERTopic CorpusExtractor'
  },
  {
    id: 'wp-004-q6',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'What expressive norms affect text metric baselines across cultural regions?',
    question:
      'AĒR needs documentation of expressive conventions, institutional registers, and communicative norms that affect how text metrics should be interpreted in each cultural context.',
    deliverable:
      'Cultural calibration profiles (2–3 pages each) for the same regions identified in WP-003 Q6, focused on text-level communicative norms rather than platform choice.',
    pipelineHook:
      'Cultural context notes in metric_provenance.yaml; methodology tray cultural context section'
  },
  {
    id: 'wp-004-q7',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Cultural Anthropologists and Area Studies Scholars',
    shortLabel: 'Which concepts and topics should not be aligned cross-culturally?',
    question:
      'Some discourse themes are so deeply embedded in local cultural, historical, and political context that cross-cultural alignment would be misleading. Identifying these "non-comparable" topics is as important as identifying comparable ones.',
    deliverable:
      'A list of culturally bounded discourse concepts per region, with explanations of why cross-cultural alignment is inappropriate.',
    pipelineHook: 'Content catalog refusal entries; non-equivalence warnings in the dashboard'
  },
  {
    id: 'wp-004-q8',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Statisticians and Data Scientists',
    shortLabel: 'What baseline calibration period is required for stable z-score computation?',
    question:
      'How many documents, over how many days, are needed to establish a reliable baseline? Does the required calibration period differ by source type (high-volume news feed vs. low-volume government press releases)?',
    deliverable:
      "A statistical analysis of baseline stability as a function of corpus size and temporal window, using AĒR's Probe 0 data as a pilot.",
    pipelineHook:
      'normalization=zscore in GET /api/v1/metrics; aer_gold.metric_equivalence; baseline calibration metadata'
  },
  {
    id: 'wp-004-q9',
    sourceWp: 'wp-004',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Statisticians and Data Scientists',
    shortLabel:
      'How should AĒR propagate normalization uncertainty through the aggregation pipeline?',
    question:
      'When z-scores are computed from uncertain baselines and aggregated across sources, how should the resulting uncertainty be quantified and displayed?',
    deliverable:
      "An uncertainty propagation framework for normalized discourse metrics, compatible with ClickHouse's aggregation functions.",
    pipelineHook:
      'aer_gold.metrics schema; GET /api/v1/metrics distribution endpoint; uncertainty visualization in Surface II'
  },

  // -------------------------------------------------------------------------
  // WP-005 §7 — Temporal Granularity
  // -------------------------------------------------------------------------
  {
    id: 'wp-005-q1',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Time Series Analysts and Statisticians',
    shortLabel:
      "What temporal decomposition method is most appropriate for AĒR's discourse time series?",
    question:
      "Classical decomposition, STL, wavelet analysis, or a combination? AĒR's time series have irregular sampling (RSS feeds do not publish at fixed intervals), potential structural breaks (algorithm changes, new sources added), and multiple overlapping periodicities (daily, weekly, seasonal).",
    deliverable:
      "A comparative evaluation of decomposition methods on AĒR's Probe 0 data, with recommendations for each temporal scale.",
    pipelineHook:
      'GET /api/v1/metrics with resolution parameter; TemporalDistributionExtractor; the three Pillar temporal modes'
  },
  {
    id: 'wp-005-q2',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Time Series Analysts and Statisticians',
    shortLabel: 'How should AĒR compute change points in discourse metrics?',
    question:
      "Which change point detection algorithm (CUSUM, PELT, Bayesian) is appropriate for AĒR's data characteristics (noisy, irregularly sampled, potentially non-stationary)? How should change point significance be assessed — frequentist (p-values) or Bayesian (posterior probability)?",
    deliverable:
      'A change point detection pipeline evaluated on Probe 0 data with known events as ground truth.',
    pipelineHook: 'Future CorpusExtractor for change-point detection; Episteme pillar temporal view'
  },
  {
    id: 'wp-005-q3',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.1',
    disciplinaryScope: 'For Time Series Analysts and Statisticians',
    shortLabel: 'What is the minimum meaningful aggregation window for each metric-source pair?',
    question:
      'Using Probe 0 data (Tagesschau and Bundesregierung RSS), compute the minimum aggregation window at which sentiment, entity count, and word count metrics stabilize (variance of the mean drops below a threshold).',
    deliverable:
      'Empirical minimum windows for each current metric, with the statistical method documented for replication on future probes.',
    pipelineHook:
      '5-minute downsampling in GET /api/v1/metrics; resolution parameter; TemporalDistributionExtractor'
  },
  {
    id: 'wp-005-q4',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Communication Scientists and Media Scholars',
    shortLabel: 'How do news cycles differ across cultures?',
    question:
      'The 24-hour news cycle is not universal. How do news publication rhythms differ between the media cultures AĒR will observe? What are the key temporal structures (edition cycles, news seasons, attention decay patterns)?',
    deliverable:
      'A comparative media temporality profile for the cultural regions identified in WP-003 Q6.',
    pipelineHook:
      'publication_hour and publication_weekday metrics; TemporalDistributionExtractor; Aleph pillar real-time mode'
  },
  {
    id: 'wp-005-q5',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.2',
    disciplinaryScope: 'For Communication Scientists and Media Scholars',
    shortLabel: 'How should AĒR detect genuine discourse shifts vs. seasonal and cyclical effects?',
    question:
      'A sentiment dip in August may reflect the German Sommerloch, not a genuine discourse shift. How should the system distinguish cyclical effects from structural changes?',
    deliverable:
      'A catalog of culturally specific temporal patterns (holidays, media seasons, political cycles) per region, formatted as machine-readable calendar metadata for integration into the dashboard.',
    pipelineHook:
      'TemporalDistributionExtractor; Episteme pillar long-term view; future calendar metadata in SilverMeta'
  },
  {
    id: 'wp-005-q6',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'At what temporal resolution do topic model outputs become stable?',
    question:
      'LDA and BERTopic are sensitive to corpus size. If AĒR runs topic models on daily corpora, weekly corpora, and monthly corpora from the same data, how do the discovered topics differ? At what corpus size do topics stabilize?',
    deliverable:
      'A corpus-size sensitivity analysis for LDA and BERTopic on German news text, with recommendations for minimum corpus size per temporal window.',
    pipelineHook:
      'CorpusExtractor protocol (anticipated); future topic-extraction corpus batch loop'
  },
  {
    id: 'wp-005-q7',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.3',
    disciplinaryScope: 'For Computational Social Scientists',
    shortLabel: 'How should AĒR model cross-source propagation dynamics?',
    question:
      "Granger causality, transfer entropy, and cross-correlation are standard methods for detecting temporal lead-lag relationships between time series. Which method is appropriate for discourse propagation analysis, given AĒR's data characteristics?",
    deliverable:
      'A method evaluation for cross-source propagation detection, tested on known propagation events in Probe 0 data.',
    pipelineHook:
      'Rhizome pillar; Relational Networks view-mode domain; future cross-source propagation metrics'
  },
  {
    id: 'wp-005-q8',
    sourceWp: 'wp-005',
    sourceSection: '7',
    subsection: '7.4',
    disciplinaryScope: 'For Digital Humanities Scholars',
    shortLabel: "How should AĒR operationalize Foucault's epistemic shifts in temporal metrics?",
    question:
      "The Episteme pillar aspires to measure shifts in the 'boundaries of the expressible.' This is a long-term, qualitative concept. Can it be operationalized as a quantitative temporal metric (vocabulary emergence rate, semantic shift velocity, Overton Window width), or does it fundamentally resist quantification?",
    deliverable:
      'A position paper on the operationalizability of epistemic change, with specific proposals for temporal metrics that approximate the concept without reducing it.',
    pipelineHook:
      'Episteme pillar mode; long-term time range view; future vocabulary-emergence CorpusExtractor'
  },

  // -------------------------------------------------------------------------
  // WP-006 §8 — Observer Effect and Reflexivity
  // -------------------------------------------------------------------------
  {
    id: 'wp-006-q1',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.1',
    disciplinaryScope: 'For STS Scholars and Sociologists of Science',
    shortLabel: "How should AĒR's observer effect be empirically studied?",
    question:
      "Can we design a natural experiment that measures whether AĒR's publication of discourse metrics alters subsequent discourse production? For example: measure discourse patterns in sources that are aware of being observed by AĒR vs. sources that are not.",
    deliverable:
      "A research design for empirically studying AĒR's observer effect, with ethical review considerations.",
    pipelineHook:
      'Reflexive Architecture (ADR-017); Surface III Reflection; methodology tray known-limitations'
  },
  {
    id: 'wp-006-q2',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.1',
    disciplinaryScope: 'For STS Scholars and Sociologists of Science',
    shortLabel:
      'What governance structures are appropriate for an open-source discourse macroscope?',
    question:
      'How should AĒR balance openness (scientific integrity, reproducibility) with responsibility (preventing weaponization, protecting vulnerable communities)?',
    deliverable:
      'A governance model proposal, drawing on precedents from other dual-use research instruments (genomic databases, climate models, election monitoring systems).',
    pipelineHook:
      'Silver-layer eligibility review process (WP-006 §5.2); k-anonymity gate at L5; responsible disclosure policy'
  },
  {
    id: 'wp-006-q3',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.2',
    disciplinaryScope: 'For Ethicists and Political Theorists',
    shortLabel: 'Under what conditions is aggregate discourse observation ethically permissible?',
    question:
      'AĒR processes public data and does not identify individuals. But aggregate observation of communities, cultures, and societies raises collective-level ethical questions that individual-level frameworks (GDPR, informed consent) do not address. What ethical framework is appropriate?',
    deliverable:
      'An ethical assessment framework for collective-level discourse observation, addressing the cases identified in §5.2 (indigenous communities, vulnerable populations, authoritarian contexts).',
    pipelineHook:
      'Silver-layer eligibility review (WP-006 §5.2); probe registration ethical review; k-anonymity gate'
  },
  {
    id: 'wp-006-q4',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.2',
    disciplinaryScope: 'For Ethicists and Political Theorists',
    shortLabel: 'How should AĒR handle findings that could be used to suppress dissent?',
    question:
      "If AĒR's metrics reveal that an oppositional narrative is gaining traction in a specific country, publishing this finding could endanger the people behind the narrative. Should AĒR delay publication? Restrict access? Publish but contextualize?",
    deliverable: 'A responsible disclosure policy for politically sensitive discourse findings.',
    pipelineHook:
      'Silver-layer eligibility gate; k-anonymity threshold at L5; refusal-as-feature architecture (ADR-017)'
  },
  {
    id: 'wp-006-q5',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.3',
    disciplinaryScope: 'For Information Design and Visualization Researchers',
    shortLabel: 'How should AĒR minimize reification and maximize critical engagement?',
    question:
      'What visualization strategies encourage users to question the data rather than accept it uncritically? How can uncertainty, provisionality, and cultural context be communicated visually without overwhelming the user?',
    deliverable:
      'Dashboard design principles and prototype wireframes that implement the non-prescriptive visualization principle (§6.2), evaluated with user studies.',
    pipelineHook:
      'Non-prescriptive visualization principle (Design Brief §7.5); epistemic weight (§7.8); Progressive Semantics'
  },
  {
    id: 'wp-006-q6',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.3',
    disciplinaryScope: 'For Information Design and Visualization Researchers',
    shortLabel: 'How should AĒR visually represent what it cannot observe?',
    question:
      'The Digital Divide (Manifesto §II), the platform inaccessibility (WP-003), and the demographic skew are all forms of systematic absence. How should a dashboard make absence visible — showing not just what the macroscope sees, but what it is blind to?',
    deliverable:
      'Visualization concepts for "negative space" — the representation of observational limitations as an integral part of the dashboard.',
    pipelineHook:
      'Negative Space overlay (Phase 113); absence regions on Surface I globe; Negative Space mode in methodology tray'
  },
  {
    id: 'wp-006-q7',
    sourceWp: 'wp-006',
    sourceSection: '8',
    subsection: '8.4',
    disciplinaryScope: 'For Digital Anthropologists and Area Studies Scholars',
    shortLabel:
      "For specific cultural contexts, what are the likely observer effects of publishing AĒR's metrics?",
    question:
      'In each cultural region (WP-003 Q6, WP-004 Q6), how would public visibility of discourse metrics affect discourse production? Are there contexts where publication would be beneficial (transparency, democratic accountability) and contexts where it would be harmful (enabling repression, amplifying manipulation)?',
    deliverable:
      "Per-region observer effect assessments (1–2 pages each) that inform AĒR's ethical review process for new probes.",
    pipelineHook:
      'Probe registration ethical review; Silver-layer eligibility (WP-006 §5.2); responsible disclosure policy'
  }
];

const BY_ID = new Map(OPEN_QUESTIONS.map((q) => [q.id, q]));

export function getOpenQuestion(id: string): OpenQuestion | null {
  return BY_ID.get(id) ?? null;
}

export function getQuestionsByWp(wpId: string): OpenQuestion[] {
  return OPEN_QUESTIONS.filter((q) => q.sourceWp === wpId);
}

/** All questions grouped by source WP. */
export function questionsByWp(): Map<string, OpenQuestion[]> {
  const map = new Map<string, OpenQuestion[]>();
  for (const q of OPEN_QUESTIONS) {
    const group = map.get(q.sourceWp) ?? [];
    group.push(q);
    map.set(q.sourceWp, group);
  }
  return map;
}
