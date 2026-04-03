# 13. Scientific Research Foundations

> **Status:** Living document — updated as interdisciplinary research progresses.
> **Last updated:** 2026-04-03

This appendix documents the scientific disciplines, methodological frameworks, and institutional partners relevant to AĒR's core mission: the observation of large-scale patterns in global digital discourse. While Chapters 0–12 describe the *instrument* (architecture, constraints, runtime behavior), this chapter describes the *lens configuration* — the theoretical and methodological foundation that determines *what* AĒR observes and *how* it interprets the data.

The technological infrastructure is deliberately decoupled from the analytical methodology (see ADR-002, Chapter 9). New metrics and analytical approaches are implemented as isolated processing steps in the Python analysis worker without affecting the ingestion pipeline or the serving layer. This chapter provides the scientific roadmap for those processing steps.

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

### 13.3.1 Tier 1 — Deterministic Core Metrics

These methods are fully deterministic, transparent, and auditable. They form the foundation of AĒR's Gold layer and should be implemented first.

| Method | Description | Deterministic | Transparency | Implementation Notes |
| :--- | :--- | :--- | :--- | :--- |
| **Lexicon-Based Sentiment** | Polarity scoring using established lexicons (VADER, SentiWordNet, NRC Emotion Lexicon). Score = sum of word-level polarities. | Yes (given fixed lexicon version) | Full — every score is traceable to individual word matches | Add `sentiment_score`, `sentiment_method`, and `lexicon_version` columns to ClickHouse Gold schema. Multilingual support via translated lexicons. |
| **Word Frequency / TF-IDF** | Term frequency and inverse document frequency across the corpus. Baseline for all text analytics. | Yes | Full | Already partially implemented (word counters in current PoC). Extend to TF-IDF with corpus-level statistics stored in ClickHouse. |
| **Named Entity Extraction** | Identification of persons, organizations, locations, and events in text using spaCy or Stanza. | Yes (given fixed model version) | High — entity spans are extractable and verifiable | Store extracted entities in a dedicated ClickHouse table (`aer_gold.entities`). Pin spaCy model version in `requirements.txt`. |
| **Temporal Distribution** | Publication frequency, time-of-day patterns, day-of-week patterns per source and topic. | Yes | Full | Pure metadata aggregation — no NLP required. Directly derivable from Bronze timestamps. |

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

**Architectural Decision:** The introduction of Tier 3 methods requires a formal ADR (to be filed as ADR-014) that documents the tradeoff between analytical depth and the Ockham's Razor principle. The decision must specify which Tier 3 outputs are permissible in the Gold layer and under what conditions.

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

The following questions must be answered through interdisciplinary collaboration before the analysis worker can move beyond the current PoC state. They are ordered by dependency — later questions build on answers to earlier ones.

1. **Probe Selection:** Which digital spaces (platforms, media, forums) constitute representative "probes" for observing societal discourse? How do we weight them against each other? (Manifesto, Section IV)
2. **Bias Calibration:** How do we measure and correct for platform-specific algorithmic amplification, bot activity, and demographic skew in crawled data? (Manifesto, Section III)
3. **Metric Validity:** Which computational metrics (sentiment, topic prevalence, narrative frames) have established validity as proxies for societal attitudes? Under what conditions do they fail?
4. **Cross-Cultural Comparability:** Can the same metric be meaningfully compared across languages and cultural contexts? What normalization is required?
5. **Temporal Granularity:** At what temporal resolution do discourse shifts become meaningful? Hours (breaking news), days (news cycles), weeks (policy debates), months (cultural shifts)?
6. **Observer Effect:** Does the act of measuring and visualizing societal discourse alter the discourse itself? How does AĒR account for its own potential impact?

---

## 13.7 References and Further Reading

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