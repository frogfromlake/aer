# 13. Scientific Research Foundations

> **Status:** Living document — updated as interdisciplinary research progresses.
> **Last updated:** 2026-04-07

This appendix documents the scientific disciplines, methodological frameworks, and institutional partners relevant to AĒR's core mission: the observation of large-scale patterns in global digital discourse. While Chapters 0–12 describe the *instrument* (architecture, constraints, runtime behavior), this chapter describes the *lens configuration* — the theoretical and methodological foundation that determines *what* AĒR observes and *how* it interprets the data.

The technological infrastructure is deliberately decoupled from the analytical methodology (see ADR-002, Chapter 9). New metrics and analytical approaches are implemented as isolated processing steps in the Python analysis worker without affecting the ingestion pipeline or the serving layer. This chapter provides the scientific roadmap for those processing steps.

The **Scientific Methodology Working Paper Series** (WP-001 through WP-006, located in `docs/methodology/`) operationalizes the open research questions identified in §13.6. Each working paper addresses one foundational methodological challenge, proposes frameworks and research questions for interdisciplinary collaboration, and documents architectural implications for the AĒR pipeline. The Working Papers are the bridge between this chapter's scientific roadmap and concrete methodological progress.

---

## 13.1 Disciplinary Landscape

AĒR operates at the intersection of multiple academic fields. The following taxonomy maps each discipline to its role within the AĒR system, ordered by proximity to the implementation.

### 13.1.1 Computational Social Science (CSS)

CSS is AĒR's primary scientific home. The field combines algorithmic methods with social science theory to analyze human behavior from digital traces. CSS provides both the methodological toolkit and the epistemological framework for validating whether digital data can serve as a proxy for societal phenomena.

**Relevance to AĒR:**

- Validates the representativeness of digital traces (the "Digital Divide" parameter from the Manifesto, Chapter 0, Section II).
- Provides established methods for sentiment analysis, stance detection, topic modeling, and narrative frame extraction — all candidate metrics for the Gold layer.
- Offers frameworks for handling selection bias, platform effects, and the non-representativeness of online populations.

**Key subfields:**

| Subfield | AĒR Application | Medallion Layer |
| :--- | :--- | :--- |
| Sentiment Analysis | Lexicon-based polarity scoring of harmonized text | Silver → Gold |
| Stance Detection | Measuring attitudinal positions toward entities or topics | Silver → Gold |
| Topic Modeling (LDA, BERTopic) | Identifying thematic clusters across sources and time | Silver → Gold |
| Discourse Network Analysis | Mapping actor-belief networks from textual data | Silver → Gold |
| Digital Behavioral Data (DBD) | Quality assessment of crawled data as behavioral traces | Bronze → Silver |

### 13.1.2 Computational Linguistics / Natural Language Processing (NLP)

NLP provides the technical methods that the analysis worker applies to raw text. Unlike CSS, which asks *what* to measure, NLP asks *how* to measure it. The field is undergoing rapid transformation through Large Language Models (LLMs), which presents both opportunities and tensions with AĒR's Ockham's Razor principle.

**Relevance to AĒR:**

- Tokenization, lemmatization, and text normalization are prerequisites for all downstream metrics (Silver Contract).
- Named Entity Recognition (NER) and Entity Linking enable the construction of discourse networks.
- Multilingual NLP is essential for AĒR's global ambition — the system must process text across languages without English-centric bias.

**Architectural constraint:** AĒR's commitment to deterministic, transparent algorithms (Quality Goal 1, Chapter 1) requires careful evaluation of NLP methods. Lexicon-based approaches and classical ML models are preferred over opaque neural networks for core metrics. LLM-based extraction may be used for enrichment tasks, but must be flagged as non-deterministic in the Gold layer schema and never applied to core metrics without explicit justification (see Section 13.3.2).

### 13.1.3 Cultural Analytics

Founded by Lev Manovich in 2005, Cultural Analytics applies computational methods to the analysis of massive cultural datasets — images, video, text, and user-generated content. The field asks how quantitative techniques can complement qualitative humanities methods when studying culture at digital scale.

**Relevance to AĒR:**

- Directly aligned with the Aleph principle (Chapter 1, Section 1.2): aggregating fragmented cultural data streams into a single coherent view.
- Provides conceptual frameworks for studying cultural patterns across platforms, geographies, and time.
- Raises critical methodological questions that AĒR must address: What does it mean to represent "culture" as "data"? How do we avoid reducing cultural complexity to averages and outliers?

### 13.1.4 Digital Anthropology

Digital Anthropology interprets the cultural codes, rituals, and meaning-making practices within digital environments. Where CSS quantifies and NLP extracts, Digital Anthropology asks what the patterns *mean* within their cultural context.

**Relevance to AĒR:**

- Essential for the interpretation layer: a spike in a sentiment metric in Japan requires different cultural framing than the same spike in Brazil.
- Provides the theoretical grounding for AĒR's "Probe Principle" (Manifesto, Chapter 0, Section IV) — the selection and weighting of observation points requires anthropological sensitivity.
- Guards against the reductive fallacy of treating all digital discourse as culturally homogeneous.

### 13.1.5 Narrative Economics

Introduced by Robert Shiller, Narrative Economics studies how popular stories spread virally and drive economic behavior. The field operationalizes narratives as measurable phenomena with topic, tone, and temporal structure.

**Relevance to AĒR:**

- Provides a framework for operationalizing the Episteme dimension (Chapter 1, Section 1.2): measuring how the boundaries of the expressible shift over time.
- Offers empirically validated methods for tracking narrative contagion — how stories spread across populations and media.
- Bridges the gap between qualitative discourse analysis and quantitative time-series metrics (Gold layer).

### 13.1.6 Science and Technology Studies (STS)

STS examines the relationship between scientific knowledge, technological systems, and society. For AĒR, STS serves a reflexive function: it forces the system to account for its own observer effect.

**Relevance to AĒR:**

- The Manifesto's acknowledgment of the Digital Divide and "Resonance over Truth" (Chapter 0, Sections II–III) is an STS-informed position.
- Platform observability research (how platforms shape the data they generate) directly affects crawler design and data quality assessment.
- Provides the ethical vocabulary for AĒR's commitment to observation over surveillance.

---

## 13.2 Institutional Landscape

The following institutions represent potential collaboration partners, ordered by geographic proximity and thematic alignment. This is not an exhaustive list but a curated starting point for outreach.

### 13.2.1 Germany

| Institution | Location | Relevance | Contact Vector |
| :--- | :--- | :--- | :--- |
| **GESIS — Leibniz Institute for the Social Sciences** | Cologne / Mannheim | CSS department with dedicated "Digital Society Observatory" team. Maintains the GESIS Methods Hub for computational social science. Runs the DD4P (DiscourseData4Policy) project using AI/ML to analyze social online discourses. Closest institutional match to AĒR's mission in Germany. | CSS department, Digital Society Observatory team |
| **WZB Berlin Social Science Center** | Berlin | Europe's largest social science institute. Research area "Digitalization and Societal Transformation." Co-founder of the Weizenbaum Institute. Extensive work on democracy, migration, and political systems — all core AĒR observation domains. | Research area III (Digitalization) |
| **Weizenbaum Institute for the Networked Society** | Berlin | BMBF-funded consortium of WZB, four Berlin universities, and Fraunhofer FOKUS. Interdisciplinary research combining social sciences, economics, law, design, and computer science. Research groups on "Technology, Power and Domination" and "Democracy and Digitization." | Research group applications, workshop participation |
| **Alexander von Humboldt Institute for Internet and Society (HIIG)** | Berlin | Founded by HU Berlin, UdK, and WZB. Member of the Global Network of Internet & Society Research Centers (NoC). Research on platform governance, digital public spheres, and innovation. | Open calls, research associate positions |
| **University of Mannheim / MZES** | Mannheim | Hosts the Computational Social Science Workshop series (next: Vienna, May 2026). Strong tradition in survey methodology and political communication research. | Workshop submissions, MZES working paper series |
| **University of Stuttgart — IMS** | Stuttgart | Institute for Natural Language Processing. Research at the intersection of NLP and political science, including the E-DELIB project on NLP-supported digital deliberation. Focus on computational argumentation and political communication. | Research collaboration, shared NLP tooling |

### 13.2.2 Europe

| Institution | Location | Relevance | Contact Vector |
| :--- | :--- | :--- | :--- |
| **ETH Zürich — COSS** | Zürich, CH | Computational Social Science group. Research on narrative warfare, discourse networks, urban discourse analysis, and cultural pattern extraction. Methodologically rigorous. High alignment with AĒR's Rhizome principle. | Research seminars, IC2S2 conference |
| **Linköping University — SweCSS** | Norrköping, SE | Swedish Excellence Center for CSS. Joint initiative between the Institute for Analytical Sociology (IAS) and Computer Science. Offers the Swedish Interdisciplinary Research School in CSS with doctoral programs. Hosted IC2S2 2025. | Research School applications, SICSS summer institute |
| **Erasmus University Rotterdam / ODISSEI** | Rotterdam, NL | Dutch National Infrastructure for Social Science. Runs SICSS-ODISSEI Summer School focused on enriched commercial data, network analysis, and machine learning. Strong FAIR data principles alignment. | Summer school participation (deadline: Feb 2026 for next cohort) |
| **University of Bologna — CSSC** | Bologna, IT | Computational Social Science Center within Political and Social Sciences. Co-organizer of CS2Italy conference (next: Torino, May 2026). Research areas include big data analysis and LLMs for social science. | CS2Italy conference submissions |

### 13.2.3 International

| Institution | Location | Relevance | Contact Vector |
| :--- | :--- | :--- | :--- |
| **Cultural Analytics Lab (CUNY)** | New York, US | Founded 2007 by Lev Manovich. Pioneer of Cultural Analytics. Studies contemporary culture using data science, visualization, and AI while critically questioning these methods. Open-source publications and datasets. | Publications, open-source tooling, conference overlap |
| **SICSS (Summer Institutes in CSS)** | Global (rotating) | Annual partner locations worldwide (11+ locations in 2026). Brings together doctoral students, postdocs, and junior faculty for intensive CSS study. Ideal venue for presenting AĒR's architecture and methodology to the CSS community. | Application as participant or proposal as host site |

---

## 13.3 Methodological Roadmap for the Analysis Worker

This section maps scientific methods to concrete implementation steps in the Python analysis worker. Each method is evaluated against AĒR's architectural constraints: determinism, transparency, and Ockham's Razor.

**Data Contract:** All Tier 1, Tier 2, and Tier 3 metrics operate on `SilverCore.cleaned_text` — the whitespace-normalized text produced by the source adapter during harmonization. The original `SilverCore.raw_text` is preserved for provenance but is not used as input to metric extraction. `SilverMeta` (source-specific context) is available for source-specific enrichment tasks but is excluded from core metrics. This ensures that metrics are comparable across data sources regardless of source-specific metadata structure. See ADR-015 for the Silver schema evolution strategy.

**Implementation Status (Phase 42):** The extensible extractor pipeline is operational with five registered `MetricExtractor` instances: word count (Phase 41), temporal distribution, language detection, lexicon-based sentiment, and named entity recognition (all Phase 42). Per-document extractors implement the `MetricExtractor` protocol and are registered in `main.py` via dependency injection (see §8.10 in Chapter 8). Corpus-level extractors (TF-IDF, LDA, co-occurrence networks) are architecturally anticipated via the `CorpusExtractor` protocol but not yet implemented — they require a batch scheduling mechanism not yet built (see Chapter 11, R-9). Corpus-level Tier 1 methods (TF-IDF) and all Tier 2 methods will require the `CorpusExtractor` path.

**Provisional Status Warning:** All Phase 42 NLP extractors are explicitly **provisional proof-of-concept implementations**. The specific lexicons, models, and parameters chosen are engineering defaults, not scientifically validated choices. They validate the extractor pipeline architecture with real NLP operations. They will be revisited, replaced, or recalibrated when interdisciplinary collaboration (§13.5) provides methodological grounding. See Chapter 11, R-11.

**Validation Infrastructure (Phase 63):** The `aer_gold.metric_validity` ClickHouse table (Migration 006) stores per-metric validation metadata — Krippendorff's alpha, correlation with reference annotations, sample size, error taxonomy, and validity expiration. The BFF API exposes `validation_status` (`unvalidated`, `validated`, `expired`) per metric via `GET /api/v1/metrics/available`. All current metrics report `unvalidated`. The table will be populated when the five-step validation protocol (WP-002 §4) is executed with interdisciplinary collaborators. Per-metric known limitations live structurally in `services/bff-api/configs/metric_provenance.yaml` and are served via `GET /api/v1/metrics/{metricName}/provenance`. See ADR-016 for the Hybrid Tier Architecture decision.

**Cross-Cultural Comparability Infrastructure (Phase 65):** WP-004 proposes baseline computation and z-score normalization for cross-cultural metric comparison. Two new Gold layer tables support this: `aer_gold.metric_baselines` (Migration 007) stores per-(metric, source, language) mean and standard deviation computed offline by `scripts/compute_baselines.py`; `aer_gold.metric_equivalence` (Migration 008) maps etic constructs to concrete metrics with validated equivalence levels (`temporal`, `deviation`, `absolute`). The BFF API `GET /api/v1/metrics` endpoint accepts `?normalization=zscore` to return z-score normalized values, gated by two preconditions: baselines must exist and at least deviation-level equivalence must be confirmed. The equivalence table is initially empty — entries require interdisciplinary validation establishing that baseline normalization does not constitute cultural erasure (WP-004 §7.3, Q7). `GET /api/v1/metrics/available` includes `eticConstruct` and `equivalenceLevel` per metric from the equivalence registry.

### 13.3.1 Tier 1 — Deterministic Core Metrics

These methods are fully deterministic, transparent, and auditable. They form the foundation of AĒR's Gold layer and should be implemented first.

| Method | Description | Deterministic | Transparency | Implementation Status |
| :--- | :--- | :--- | :--- | :--- |
| **Lexicon-Based Sentiment** | Polarity scoring using SentiWS (Leipzig University, CC-BY-SA). Score = mean of matched word-level polarities. | Yes (given fixed lexicon version) | Full — every score is traceable to individual word matches | **Provisional PoC — Phase 42.** `extractors/sentiment.py`. Produces `sentiment_score` metric. Uses naive whitespace tokenization. Does NOT handle negation, irony, compositionality, or German compound words. Lexicon version pinned to SentiWS v2.0. Lexicon provenance (SHA-256 hash) is recorded in `SilverEnvelope.extraction_provenance` — not as a Gold metric (Phase 46). |
| **Word Frequency / TF-IDF** | Term frequency and inverse document frequency across the corpus. Baseline for all text analytics. | Yes | Full | **Partially implemented.** Word count extractor live (Phase 41). TF-IDF requires `CorpusExtractor` path (batch processing, see R-9). |
| **Named Entity Extraction** | Identification of persons, organizations, locations using spaCy `de_core_news_lg`. | Yes (given fixed model version) | High — entity spans are extractable and verifiable | **Provisional PoC — Phase 42.** `extractors/entities.py`. Produces `entity_count` metric in `aer_gold.metrics` and raw entity spans in `aer_gold.entities` (Migration 003). **Entity linking — Phase 118 (Tier 1.5 heuristic).** Optional `WikidataAliasIndex` resolves NER spans against a SQLite alias index of politicians, sovereign states, sub-national entities, cities ≥ 50k population, IOs, parties, government agencies, central banks, news organisations, broadcasters, and EU institutions. The alias index is built quarterly from a Wikidata RDF dump (Phase 118b — streaming N-Triples parse over `latest-truthy.nt.bz2`, not a live API). This makes AĒR's entity inventory at most one Wikipedia weekly-publication cycle behind Wikidata-live for a given build, and at most one quarter behind for a given deployed AĒR instance. Operational reality, not defect — surfaced for WP-002 §4.2's entity-linking-validity discussion. Model pinned to `de_core_news_lg-3.8.0` in `requirements.txt`. See R-10 for model dependency risk. |
| **Temporal Distribution** | Publication frequency, time-of-day patterns, day-of-week patterns per source. | Yes | Full | **Implemented — Phase 42.** `extractors/temporal.py`. Produces `publication_hour` (0–23 UTC) and `publication_weekday` (0=Mon, 6=Sun). Pure metadata, no NLP. Methodologically stable — not provisional. |
| **Language Detection** | Probabilistic language identification using `langdetect` with fixed seed for determinism. | Yes (given fixed seed) | High — probability scores per language candidate | **Provisional PoC — Phase 42, extended Phase 45.** `extractors/language.py`. Produces `language_confidence` metric (0.0–1.0) and persists detected language codes with ranked candidates in `aer_gold.language_detections` (Migration 004). Implements `LanguageDetectionPersistExtractor` protocol. Accuracy degrades on short texts (<50 chars). Fixed seed ensures reproducibility but not accuracy. May be replaced by `lingua-py` or corpus-level profiling. |

### 13.3.2 Tier 2 — Statistical Methods (Reproducible with Seed)

These methods involve randomness but are reproducible when seeded. They require explicit documentation of parameters and random seeds in the Gold layer.

| Method | Description | Deterministic | Transparency | Implementation Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Topic Modeling (LDA)** | Latent Dirichlet Allocation for discovering latent topics in document collections. | Reproducible with fixed seed, fixed vocabulary | Medium — topic interpretation requires human judgment | Store `random_seed`, `n_topics`, `alpha`, `beta` parameters alongside results. Periodic retraining produces new topic snapshots, not mutations of existing ones. |
| **BERTopic** | Transformer-based topic modeling using embeddings + HDBSCAN clustering. | Reproducible with fixed seed and model version | Medium — embedding model is a soft dependency | Pin sentence-transformer model version. Store model hash alongside topic assignments. Flag as Tier 2 in Gold schema. |
| **Keyword Co-occurrence Networks** | Graph construction from term co-occurrence within sliding windows. Edge weights = co-occurrence frequency. | Yes | Full | Store as adjacency lists in ClickHouse. Enables Rhizome-inspired network analysis. |

### 13.3.3 Tier 3 — LLM-Augmented Enrichment (Non-Deterministic)

These methods use Large Language Models and are inherently non-deterministic. They must never replace Tier 1/2 metrics but may augment them. All LLM-derived data must be explicitly flagged in the Gold schema.

| Method | Description | Deterministic | Transparency | Implementation Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Narrative Frame Detection** | LLM-based extraction of narrative frames (e.g., "securitization," "human rights," "economic opportunity") from text. | No — LLM outputs vary across runs | Low — reasoning is opaque | Store `model_id`, `temperature`, `prompt_hash` alongside results. Add `is_deterministic: false` flag to Gold schema. Use only for exploratory analysis, never as primary metric. |
| **Stance Classification** | LLM-based classification of author stance toward specific entities or policies. | No | Low | Same flagging requirements as Narrative Frame Detection. Consider fine-tuned, smaller models for improved consistency. |
| **Cross-Lingual Summary** | LLM-generated summaries enabling cross-cultural comparison of discourse on the same topic. | No | Low | Store in Silver layer as enrichment, not in Gold. Useful for analyst-facing Progressive Disclosure but not for quantitative dashboards. |

**Architectural Decision:** The introduction of Tier 3 methods requires a formal ADR (to be filed as ADR-020) that documents the tradeoff between analytical depth and the Ockham's Razor principle. The decision must specify which Tier 3 outputs are permissible in the Gold layer and under what conditions.

---

## 13.4 Mapping to the AĒR DNA

The following table maps each philosophical pillar (Chapter 1, Section 1.2) to concrete scientific methods and their implementation status.

| Pillar | Concept | Operationalization | Methods | Status |
| :--- | :--- | :--- | :--- | :--- |
| **A — Aleph** | The single point containing all other points | Aggregation of fragmented global data streams into a unified view | Multi-source crawlers, source-agnostic ingestion contract, ClickHouse OLAP aggregation | Architecture complete. Crawler expansion pending. |
| **E — Episteme** | The rules defining what can be thought and said | Measuring shifts in the boundaries of expressible discourse over time | Temporal topic modeling, semantic shift detection, narrative frame tracking, keyword co-occurrence evolution | Research phase. Tier 1/2 methods identified. |
| **R — Rhizome** | Non-linear, decentralized information spread | Modeling how cultural patterns propagate through networks | Discourse network analysis, entity co-occurrence graphs, cross-source narrative diffusion | Research phase. Requires multi-source data. |

---

## 13.5 Recommended Outreach Strategy

### 13.5.1 Working Paper Series (WP-001 through WP-006)

> **Status:** Draft — all six papers complete, open for interdisciplinary review.
> **Date:** 2026-04-07
> **Languages:** English and German (bilingual, content-identical)

The scientific methodology for AĒR's "lens configuration" is documented in a series of six interdisciplinary working papers. Each paper addresses one of the open research questions identified in §13.6 and formulates concrete deliverables for collaborators from specific disciplines. The papers are maintained under `docs/methodology/` in both English (`en/`) and German (`de/`) to support outreach to international and German-speaking research institutions alike.

| Paper | Title | §13.6 Question | Primary Discipline |
| :--- | :--- | :--- | :--- |
| **WP-001** | Functional Probe Taxonomy | 1. Probe Selection | Digital Anthropology, Comparative Politics |
| **WP-002** | Metric Validity & Sentiment Calibration | 3. Metric Validity | CSS, NLP |
| **WP-003** | Platform Bias & Non-Human Actors | 2. Bias Calibration | Internet Studies, CSS |
| **WP-004** | Cross-Cultural Comparability | 4. Cross-Cultural Comparability | Comparative Methodology |
| **WP-005** | Temporal Granularity | 5. Temporal Granularity | Time Series Analysis, Communication Science |
| **WP-006** | Observer Effect & Reflexivity | 6. Observer Effect | STS, Sociology, Ethics |

**Key constructs introduced across the series:**

- **Functional Probe Taxonomy** (WP-001): Four universal discourse functions (Epistemic Authority, Power Legitimation, Cohesion & Identity, Subversion & Friction) as the basis for culturally agnostic probe selection.
- **Etic/Emic Dual Tagging System** (WP-001, extended in WP-004): Architectural mechanism for achieving cross-cultural comparability without cultural erasure, operationalized in `SilverMeta`.
- **Validation Protocol** (WP-002): Five-step framework for validating computational metrics against human judgment, per metric-context pair.
- **Metric Equivalence Registry** (WP-004): A curated Gold-layer table that documents which instrument-specific metrics are validated for cross-cultural comparison.
- **Multi-Scale Temporal Framework** (WP-005): Five temporal scales (event response → cultural drift) with resolution-specific aggregation in ClickHouse.
- **Reflexive Architecture Principles** (WP-006): Five design principles (Methodological Transparency, Non-Prescriptive Visualization, Reflexive Documentation, Governed Openness, Interpretive Humility).

**Consolidated Research Question Index** (from WP-006, Appendix C):

- Computational Social Science: WP-002 Q1–Q3, WP-003 Q3–Q5, WP-005 Q6–Q7
- Computational Linguistics / NLP: WP-002 Q4–Q6, WP-004 Q4–Q5
- Cultural Anthropology / Area Studies: WP-002 Q7–Q8, WP-003 Q6–Q7, WP-004 Q6–Q7, WP-006 Q7
- Methodology / Statistics: WP-002 Q9–Q10, WP-003 Q8–Q9, WP-004 Q1–Q3, Q8–Q9, WP-005 Q1–Q3
- Communication Science / Media Studies: WP-005 Q4–Q5
- STS / Sociology / Ethics: WP-006 Q1–Q4
- Information Design / Visualization: WP-006 Q5–Q6
- Digital Humanities: WP-005 Q8

The working papers are designed to be shared with potential collaborators as self-contained documents. Each paper identifies the relevant discipline, the specific gap in AĒR's current implementation, and the form of contribution that would be most valuable. The German translations ensure accessibility for German-speaking researchers at institutions such as GESIS, WZB, Weizenbaum Institute, and University of Stuttgart IMS (see §13.2.1).

### 13.5.2 Manual Scientific Workflows

Phases 62–68 introduced scientific infrastructure tables, config files, and templates whose contents are authored by researchers, not by the pipeline. These *manual workflows* live at the boundary between developer operations (the Operations Playbook — "what to type") and methodological reasoning (the Working Papers — "why this approach"). The canonical bridge document is the **[Scientific Operations Guide](../operations/scientific_operations_guide.md)** (`docs/scientific_operations_guide.md`, Phase 71). Each workflow in that guide names its trigger, performer, Working Paper rationale, Playbook command sequence, Phase 68 template, destination table/config, and a concrete Probe 0 walkthrough.

Workflows covered by the Scientific Operations Guide:

1. **Classifying a new probe** (WP-001 §4.4) → `source_classifications` PostgreSQL table.
2. **Validating a metric** (WP-002 §6.2) → `aer_gold.metric_validity`.
3. **Establishing metric equivalence** (WP-004 §5.2) → `aer_gold.metric_equivalence`.
4. **Computing and updating baselines** (WP-004 §6.1) → `aer_gold.metric_baselines`.
5. **Assessing bias for a data source** (WP-003 §8.1) → `BiasContext` values in the adapter + prose under `docs/methodology/`.
6. **Updating the Cultural Calendar** (WP-005 §4.3) → `configs/cultural_calendars/<region>.yaml`.

The Scientific Operations Guide is now published. Until the workflows it describes have actually been executed by interdisciplinary collaborators, each of these tables and files remains populated only with provisional engineering defaults — see Chapter 11, R-13 for the architectural risk this creates.

### 13.5.3 Outreach Roadmap

### Phase 1 — Literature and Community (Immediate)

1. Attend the **Computational Social Science Workshop** (Vienna, May 2026) as an observer or with a position paper describing AĒR's architecture.
2. Submit an abstract to **CS2Italy** (Torino, May 2026) presenting AĒR as a methodological contribution — an open, auditable pipeline for CSS research.
3. Review GESIS publications on the DD4P project and the Digital Society Observatory for methodological alignment.
4. Study the HYBRIDS project deliverables (EU-funded) on NLP applied to discourse analysis for state-of-the-art method surveys.

### Phase 2 — Institutional Contact (Q3 2026)

1. Contact the **GESIS CSS department** with a collaboration proposal: AĒR as an open infrastructure for testing CSS methods on real-time discourse data.
2. Explore **Weizenbaum Institute** workshop participation (Berlin) — their interdisciplinary format (social science + computer science) matches AĒR's DNA.
3. Apply to **SICSS 2027** as a participant or propose a partner location focused on open discourse analysis infrastructure.

### Phase 3 — Formal Collaboration (2027+)

1. Seek a research partnership with an established CSS group (GESIS, ETH COSS, or SweCSS) for methodological validation of AĒR's metric pipeline.
2. Publish AĒR's architecture and methodology as an open-source reference implementation for CSS infrastructure.
3. Explore DFG (German Research Foundation) or EU Horizon funding for a collaborative project combining AĒR's technical infrastructure with social science expertise.

---

## 13.6 Open Research Questions

The following questions must be answered through interdisciplinary collaboration before the analysis worker can move beyond the current PoC state. They are ordered by dependency — later questions build on answers to earlier ones. **Each question is now addressed in detail by the corresponding working paper** (see §13.5.1); the papers formulate concrete sub-questions, identify target disciplines, and specify deliverables.

1. **Probe Selection:** Which digital spaces (platforms, media, forums) constitute representative "probes" for observing societal discourse? How do we weight them against each other? (Manifesto, Section IV) — **Addressed in [WP-001](../methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md)**: proposes a functional taxonomy of four universal discourse functions (Epistemic Authority, Power Legitimation, Cohesion & Identity, Subversion & Friction) and an Etic/Emic Dual Tagging System that enables cross-cultural probe selection without imposing Western institutional categories.
2. **Bias Calibration:** How do we measure and correct for platform-specific algorithmic amplification, bot activity, and demographic skew in crawled data? (Manifesto, Section III) — **Addressed in [WP-003](../methodology/en/WP-003-en-platform_bias_algorithmic_amplification_and_the_detection_of_non-human_actors.md)**: maps the global platform landscape, analyzes algorithmic amplification effects, examines bot/synthetic content detection, documents the Digital Divide, and proposes a "document, don't filter" approach to non-human actors.
3. **Metric Validity:** Which computational metrics (sentiment, topic prevalence, narrative frames) have established validity as proxies for societal attitudes? Under what conditions do they fail? — **Addressed in [WP-002](../methodology/en/WP-002-en-metric_validity_and_sentiment_calibration.md)**: analyzes the construct validity problem, documents limitations of SentiWS and NER implementations, proposes a five-step validation protocol (annotation study, baseline comparison, error taxonomy, cross-context transfer, longitudinal stability), and formulates concrete research questions per discipline.
4. **Cross-Cultural Comparability:** Can the same metric be meaningfully compared across languages and cultural contexts? What normalization is required? — **Addressed in [WP-004](../methodology/en/WP-004-en-cross-cultural_comparability_of_discourse_metrics.md)**: introduces the comparability paradox, defines three levels of cross-cultural comparison (temporal patterns, baseline-normalized deviations, instrument-harmonized absolutes), extends the Etic/Emic framework to metrics via a proposed Metric Equivalence Registry, and addresses the ethical dimension of comparison.
5. **Temporal Granularity:** At what temporal resolution do discourse shifts become meaningful? Hours (breaking news), days (news cycles), weeks (policy debates), months (cultural shifts)? — **Addressed in [WP-005](../methodology/en/WP-005-en-temporal_granularity_of_discourse_shifts.md)**: defines five temporal scales of discourse phenomena, proposes multi-scale temporal decomposition, analyzes cultural temporalities (calendars, news cycles, attention rhythms), and maps each AĒR pillar to its temporal signature (Aleph → synchronic, Episteme → diachronic, Rhizome → propagation dynamics).
6. **Observer Effect:** Does the act of measuring and visualizing societal discourse alter the discourse itself? How does AĒR account for its own potential impact? — **Addressed in [WP-006](../methodology/en/WP-006-en-observer_effect_reflexivity_and_the_ethics_of_discourse_measurement.md)**: examines performativity and Goodhart's Law, analyzes risks of metric gaming, reification, and weaponization, proposes five design principles for reflexive architecture (methodological transparency, non-prescriptive visualization, reflexive documentation, governed openness, interpretive humility), and addresses the ethics of making discourse visible.

---

## 13.7 Scientific Methodology Working Paper Series

The Working Paper series constitutes AĒR's methodological research program. The papers are designed as interdisciplinary dialogue documents — they map the gap between AĒR's engineering pipeline and scientifically validated discourse observation, formulate concrete research questions per target discipline, and propose frameworks that respect AĒR's architectural constraints (determinism, transparency, Ockham's Razor).

| Paper | Title | Core Question | Primary Disciplines | §13.6 Question |
| :--- | :--- | :--- | :--- | :--- |
| **[WP-001](../methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md)** | Toward a Culturally Agnostic Probe Catalog | How to select observation points without cultural bias? | Digital Anthropology, Comparative Political Science | 1. Probe Selection |
| **[WP-002](../methodology/en/WP-002-en-metric_validity_and_sentiment_calibration.md)** | Metric Validity and Sentiment Calibration | When are computational metrics valid proxies for societal attitudes? | CSS, NLP, Cultural Anthropology | 3. Metric Validity |
| **[WP-003](../methodology/en/WP-003-en-platform_bias_algorithmic_amplification_and_the_detection_of_non-human_actors.md)** | Platform Bias, Algorithmic Amplification, and Non-Human Actors | How to account for platform distortion and synthetic content? | Internet Studies, CSS, Area Studies | 2. Bias Calibration |
| **[WP-004](../methodology/en/WP-004-en-cross-cultural_comparability_of_discourse_metrics.md)** | Cross-Cultural Comparability of Discourse Metrics | Can metrics be compared across cultures? What normalization is required? | Comparative Methodology, Computational Linguistics | 4. Cross-Cultural Comparability |
| **[WP-005](../methodology/en/WP-005-en-temporal_granularity_of_discourse_shifts.md)** | Temporal Granularity of Discourse Shifts | At what time scale do discourse shifts become meaningful? | Time Series Analysis, Communication Science | 5. Temporal Granularity |
| **[WP-006](../methodology/en/WP-006-en-observer_effect_reflexivity_and_the_ethics_of_discourse_measurement.md)** | Observer Effect, Reflexivity, and the Ethics of Discourse Measurement | What happens because we observe? | STS, Sociology, Ethics, Information Design | 6. Observer Effect |

### Key Contributions Across the Series

**WP-001** introduces the **Functional Probe Taxonomy** — four universal discourse functions (Epistemic Authority, Power Legitimation, Cohesion & Identity, Subversion & Friction) grounded in Foucault, Gramsci, Anderson, and Scott. It operationalizes cross-cultural comparison via the **Etic/Emic Dual Tagging System** embedded in `SilverMeta`, defines the **Minimum Viable Probe Set** concept, and proposes a formalized five-step probe classification process requiring area studies expertise. Probe 0 is formally classified as covering only Functions 1–2 (Epistemic Authority + Power Legitimation) in the German institutional sphere.

**WP-002** maps the construct validity gap between computational metrics and sociological interpretation. It documents the known limitations of all Phase 42 extractors (SentiWS negation blindness, compound word failure, spaCy NER entity linking absence, language detection short-text degradation), proposes a **five-step validation protocol** (annotation study → baseline comparison → error taxonomy → cross-context transfer → longitudinal stability), and recommends a **hybrid tier architecture** (Option C: Tier 1 as immutable baseline, Tier 2/3 as validated enrichments via Progressive Disclosure). Proposes `aer_gold.metric_validity` table for storing validation results.

**WP-003** analyzes upstream distortions — platform affordances, algorithmic amplification, and non-human actors — that affect data *before* it enters AĒR's pipeline. It maps the **global platform landscape** beyond the Western stack (East Asia, Russia, South Asia, MENA, Sub-Saharan Africa, Latin America), proposes **counterfactual strategies** for detecting algorithmic effects (cross-platform triangulation, temporal discontinuity detection), advocates a **"document, don't filter"** approach to bot/synthetic content (compute authenticity indicators as Gold metrics, expose via Progressive Disclosure), and proposes `BiasContext` fields for standardized bias documentation in `SilverMeta`.

**WP-004** addresses the **comparability paradox**: comparison requires a common frame, but imposing a common frame risks erasing the differences being observed. It defines four types of equivalence (construct, measurement, scalar, temporal), proposes **three levels of cross-cultural comparison** (temporal patterns, baseline-normalized deviations, instrument-harmonized absolutes), extends the Etic/Emic framework to metrics via a proposed **Metric Equivalence Registry** (`aer_gold.metric_equivalence`), and proposes Gold layer extensions for baseline computation and z-score normalization.

**WP-005** defines **five temporal scales** of discourse phenomena (event response, news cycle, agenda dynamics, policy discourse, cultural drift), maps each AĒR pillar to its temporal signature, analyzes cultural temporalities (religious calendars, political cycles, media rhythms), proposes multi-resolution ClickHouse aggregation and BFF API extensions, and introduces a tiered retention strategy for long-term cultural drift analysis.

**WP-006** addresses reflexivity — the fact that AĒR is part of the system it observes. Drawing on Giddens' double hermeneutic, Bourdieu's epistemic reflexivity, and Callon/MacKenzie's performativity theory, it analyzes four concrete risks (metric gaming, reification, weaponization, normative visualization) and proposes five **design principles for reflexive architecture**: methodological transparency, non-prescriptive visualization, reflexive documentation, governed openness, and interpretive humility.

### Interdisciplinary Research Questions

The Working Papers collectively formulate **30+ concrete research questions** organized by target discipline, each specifying the gap in AĒR's current implementation and the form of contribution that would be most valuable. See WP-006 Appendix C for the consolidated index. The primary target disciplines are:

- **Computational Social Science:** metric validation, bot detection, topic model stability, propagation dynamics
- **Computational Linguistics / NLP:** compound word handling, multilingual NER/entity linking, code-switching, tokenization strategies
- **Cultural Anthropology / Area Studies:** regional probe nominations, expressive norm calibration, entity ontology, observer effect assessments
- **Methodology / Statistics:** baseline calibration, uncertainty propagation, temporal decomposition, change point detection
- **Communication Science / Media Studies:** cross-cultural news cycles, seasonal pattern catalogs
- **STS / Sociology / Ethics:** observer effect empirics, governance models, responsible disclosure
- **Information Design:** non-prescriptive visualization, negative space representation

---

## 13.8 References and Further Reading

### Foundational Texts

- Manovich, L. (2020). *Cultural Analytics*. MIT Press.
- Shiller, R. J. (2020). *Narrative Economics: How Stories Go Viral and Drive Major Economic Events*. Princeton University Press.
- Foucault, M. (1966). *Les Mots et les Choses* (The Order of Things). Gallimard.
- Deleuze, G. & Guattari, F. (1980). *Mille Plateaux* (A Thousand Plateaus). Les Éditions de Minuit.
- Borges, J. L. (1945). "El Aleph." *Sur*, No. 131.

### Computational Social Science

- Lazer, D. et al. (2020). "Computational Social Science: Obstacles and Opportunities." *Science*, 369(6507), 1060–1062.
- Grimmer, J., Roberts, M. E., & Stewart, B. M. (2022). *Text as Data: A New Framework for Machine Learning and the Social Sciences*. Princeton University Press.
- Salganik, M. J. (2018). *Bit by Bit: Social Research in the Digital Age*. Princeton University Press.

### NLP and Discourse Analysis

- Jurafsky, D. & Martin, J. H. (2024). *Speech and Language Processing* (3rd edition). Draft available online.
- Card, D. et al. (2022). "Computational Analysis of 140 Years of US Political Speeches." *Proceedings of the National Academy of Sciences*.

### Methodology

- King, G., Lam, P., & Roberts, M. E. (2017). "Computer-Assisted Keyword and Document Set Discovery from Unstructured Text." *American Journal of Political Science*, 61(4), 971–988.
- Blei, D. M., Ng, A. Y., & Jordan, M. I. (2003). "Latent Dirichlet Allocation." *Journal of Machine Learning Research*, 3, 993–1022.
- Grootendorst, M. (2022). "BERTopic: Neural Topic Modeling with a Class-Based TF-IDF Procedure." arXiv:2203.05794.

### Comparative Methodology and Reflexivity

- Sartori, G. (1970). "Concept Misformation in Comparative Politics." *American Political Science Review*, 64(4), 1033–1053.
- Santos, B. de S. (2014). *Epistemologies of the South: Justice Against Epistemicide*. Routledge.
- Hallin, D. C. & Mancini, P. (2004). *Comparing Media Systems: Three Models of Media and Politics*. Cambridge University Press.
- Scott, J. C. (1990). *Domination and the Arts of Resistance: Hidden Transcripts*. Yale University Press.
- Giddens, A. (1984). *The Constitution of Society: Outline of the Theory of Structuration*. Polity.
- MacKenzie, D. (2006). *An Engine, Not a Camera: How Financial Models Shape Markets*. MIT Press.
- van de Vijver, F. J. R. & Leung, K. (1997). *Methods and Data Analysis for Cross-Cultural Research*. SAGE.

### AĒR Scientific Methodology Working Papers (Internal)

- WP-001: Toward a Culturally Agnostic Probe Catalog — A Functional Taxonomy for Global Discourse Observation (2026-04-07)
- WP-002: Metric Validity and Sentiment Calibration (2026-04-07)
- WP-003: Platform Bias, Algorithmic Amplification, and the Detection of Non-Human Actors (2026-04-07)
- WP-004: Cross-Cultural Comparability of Discourse Metrics (2026-04-07)
- WP-005: Temporal Granularity of Discourse Shifts (2026-04-07)
- WP-006: Observer Effect, Reflexivity, and the Ethics of Discourse Measurement (2026-04-07)

---

## 13.9 Data Protection Architecture

AĒR's privacy commitment (Manifesto §VI) is operationalized through architectural constraints at each layer of the Medallion Architecture, not through post-hoc policy compliance. The system is designed to make re-identification technically infeasible.

### Anonymization by Layer

| Layer Boundary | Technique | Implementation |
| :--- | :--- | :--- |
| Bronze → Silver | Irreversible identifier stripping, SHA-256 hash pseudonymization with rotated salt, temporal truncation to hour-level for social media sources | Source Adapter protocol (`adapters/base.py`). Each adapter's `harmonize()` method is responsible for stripping identifiers before constructing `SilverCore`. |
| Silver → Gold | k-Anonymity (k ≥ 10), l-Diversity (l ≥ 3), minimum aggregation windows per source type, private-person entity suppression | Extractor pipeline, BFF API query layer. |
| Gold → API/Dashboard | Server-side downsampling, hard row limits (ADR-003), no raw-text endpoints, no drill-down to individual social-media documents | BFF API configuration. |

### Explicit Data Exclusions

AĒR does not collect, regardless of technical feasibility: behavioral metadata (clicks, likes, engagement), social graph topology, device/session fingerprints, or content from private/semi-private digital spaces.

### Privacy Risk Classification by Probe Type

| Probe Type (WP-001) | Privacy Risk | Required Anonymization |
| :--- | :--- | :--- |
| Institutional RSS/API (Probe 0) | Low — public press releases, no personal data | Identifier stripping only (standard adapter behavior). |
| Public news/media | Low — editorial content, named public figures | Standard. Public-figure entities are not anonymized. |
| Public social media | High — user-generated content, quasi-identifiers | Full anonymization: identifier removal, temporal truncation, k-anonymity, entity suppression for private persons, stylometric risk assessment. |
| Forum/community archives | Medium–High — pseudonymous but potentially identifiable | Hash pseudonymization, temporal truncation, k-anonymity. |

The detailed anonymization framework, including formal privacy models (k-Anonymity, l-Diversity, t-Closeness) and open research questions for privacy researchers, is documented in WP-006 §7.

---

## 13.10 Probe 0: Pipeline Calibration (German Institutional RSS)

> **Status:** Operational — pipeline validated end-to-end (Phase 43). Formally classified under the Functional Probe Taxonomy (Phase 62).
> **Date:** 2026-04-11

This section documents AĒR's first real data source. The source selection is explicitly **provisional** — it is driven by pragmatic engineering criteria, not by scientific probe methodology. The Manifesto's Probe Principle (§IV) requires interdisciplinary dialogue for valid probe selection; this dialogue has not yet occurred. The RSS feeds selected here serve as **calibration data** for the pipeline, not as a scientifically representative sample of German discourse. WP-001 (§6) formally classifies Probe 0 under the Functional Probe Taxonomy: bundesregierung.de serves the **Power Legitimation** function (primary) and tagesschau.de serves the **Epistemic Authority** function (primary). Functions 3 (Cohesion & Identity) and 4 (Subversion & Friction) are entirely unrepresented.

**Formal Classification (Phase 62):** Probe 0 sources are now classified in the `source_classifications` PostgreSQL table with `review_status = 'provisional_engineering'`. The `DiscourseContext` (primary function, secondary function, emic designation) propagates through `RssMeta` into the Gold layer as the `discourse_function` column in `aer_gold.metrics` and `aer_gold.entities`. Classification `function_weights` are intentionally NULL — quantification requires the WP-001 §4.4 classification process (area expert nomination and peer review), which has not yet occurred. See `docs/templates/probe_registration_template.yaml` for the registration form.

### Purpose

Engineering calibration of the AĒR pipeline. This probe validates:

- End-to-end data flow from external source through Bronze → Silver → Gold.
- The Silver Contract evolution (SilverCore + SilverMeta, ADR-015) with real-world data.
- Source Adapter pattern correctness (RSSAdapter producing valid SilverCore records).
- Metric extraction on non-synthetic text (German-language editorial content).
- BFF API serving of multi-source aggregated data (metrics, entities, and metric discovery endpoints — validated in Phase 43 E2E test).

### Source Selection Criteria (Engineering, Not Scientific)

The following criteria are purely pragmatic — they optimize for pipeline validation, not for societal representativeness:

- **Publicly available:** No authentication, no API keys, no paid subscriptions.
- **Structured format:** RSS/Atom feeds with predictable XML structure. Parseable via standard libraries (`gofeed`).
- **No Terms of Service restrictions:** Government and public broadcasting feeds are explicitly intended for public consumption.
- **No personal data:** Editorial content only — no user profiles, comments, or engagement data.
- **Predictable document volume:** Institutional feeds publish 5–30 items per day, enabling controlled pipeline load testing.
- **German-language:** Provides a homogeneous linguistic corpus for validating NLP model behavior (tokenization, whitespace normalization, character encoding) before introducing multilingual complexity.

### Milieu Bias Acknowledgment (Per Manifesto §III)

This probe captures exclusively **institutional and editorial voice**. It does not represent:

- "The German public" or public opinion in any statistical sense.
- Grassroots discourse, citizen journalism, or independent media.
- Social media dynamics, virality, or engagement patterns.
- Any specific demographic, age group, or socioeconomic milieu.
- Regional variation within Germany (federal government perspective is structurally national).

This bias is a **documented parameter of the observation**, not a defect. Every dataset carries selection bias — the scientific integrity lies in documenting it explicitly rather than pretending it doesn't exist.

### Selected Sources (Provisional, Subject to Change Without ADR)

| Source | Feed URL | Type | Expected Volume |
| :--- | :--- | :--- | :--- |
| **bundesregierung.de** | `https://www.bundesregierung.de/breg-de/aktuelles.rss` | Government press releases | ~5–15 items/day |
| **tagesschau.de** | `https://www.tagesschau.de/index~rss2.xml` | Public broadcasting news (ARD) | ~20–40 items/day |

Additional quality press feeds may be added as the pipeline matures. Each addition requires only a new entry in `feeds.yaml` and a PostgreSQL seed migration — no code changes to any AĒR service.

### Limitations

- **Editorial content only.** No user-generated content, no comments, no forum threads.
- **No engagement metrics.** RSS feeds do not expose view counts, shares, or reactions.
- **No threading or reply structure.** Each item is an independent document with no relational context.
- **Limited to German language.** Multilingual processing is deferred until cross-cultural comparability methodology is established (§13.6, Question 4).
- **RSS feeds may be incomplete.** Many feeds provide truncated descriptions rather than full article text. The `raw_text` field may contain summaries, not complete articles. This limitation is inherent to the RSS format and must be documented in any analysis derived from this probe.
- **Feed URLs may change without notice.** Government and public broadcasting feed endpoints are not contractually stable. The crawler must handle HTTP 301/404 gracefully.

### Exit Criteria

This probe is **superseded** — not retired — when a scientifically motivated probe selection is made through the research process (§13.5). The RSS crawler remains operational as one data source among many. The engineering calibration data it has collected retains its value for pipeline regression testing and baseline comparisons, even after scientifically selected probes are introduced.