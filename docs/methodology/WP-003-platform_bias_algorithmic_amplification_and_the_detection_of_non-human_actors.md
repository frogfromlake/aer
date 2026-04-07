# WP-003: Platform Bias, Algorithmic Amplification, and the Detection of Non-Human Actors

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft — open for interdisciplinary review
> **Date:** 2026-04-07
> **Depends on:** WP-001 (Functional Probe Taxonomy), WP-002 (Metric Validity)
> **Architectural context:** Crawler Ecosystem (§5.2), Source Adapter Protocol (ADR-015), Manifesto §III

---

## 1. Objective

This working paper addresses the second open research question from §13.6: **How do we measure and correct for platform-specific algorithmic amplification, bot activity, and demographic skew in crawled data?**

AĒR aspires to observe the "Digital Pulse" of connected civilization (Manifesto §I). But the pulse AĒR measures is not an unmediated human signal — it is a signal that has been filtered, amplified, suppressed, and restructured by the platforms that host it. Every digital platform is a sociotechnical system with its own affordances, content moderation policies, recommendation algorithms, economic incentives, and governance structures. These systems do not passively transmit human discourse; they actively shape it.

Additionally, an increasing share of online content is produced not by humans but by automated accounts (bots), coordinated influence operations, and generative AI systems. The Manifesto's commitment to observing "the underlying human impulses" (§III) requires AĒR to distinguish — or at least attempt to distinguish — between organic human discourse and synthetic or automated content.

This paper maps the landscape of platform-induced bias, examines the global diversity of platform ecosystems, analyzes the current state of bot and synthetic content detection research, and formulates concrete research questions for interdisciplinary collaboration. It is concerned with distortions that occur *upstream* of AĒR's pipeline — before a document reaches the Bronze layer — as opposed to the measurement distortions addressed in WP-002, which occur *within* the pipeline during metric extraction.

---

## 2. The Platform as Mediating Infrastructure

### 2.1 Platform Affordances and Discourse Structure

Platforms are not neutral conduits. Each platform imposes structural constraints — affordances — that shape the form and content of discourse (Bucher & Helmond, 2018). These affordances are not merely technical features; they are design decisions with sociological consequences.

**Character limits** shape rhetorical strategy. X/Twitter's historical 140-character limit (now 280, or effectively unlimited for premium users) incentivized compressed, affective, and polarized expression. Facebook's absence of character limits but algorithmic prioritization of engagement incentivizes emotional and divisive content. Reddit's threaded comment structure enables deliberative argumentation that is structurally impossible on X. Telegram's channel model creates one-to-many broadcast dynamics distinct from both social media and traditional media. Each affordance produces a different *kind* of text with different sentiment profiles, rhetorical structures, and informational density.

**Visibility mechanisms** determine which content is observed. On algorithmically curated platforms (Facebook, YouTube, TikTok, Instagram), the content a crawler can access depends on what the algorithm surfaces. Even when accessing public feeds or APIs, the selection of *which* content appears is shaped by engagement-optimizing algorithms whose parameters are proprietary, opaque, and subject to change without notice. On chronologically ordered platforms (RSS, Mastodon's default, many forums), the selection bias is different but still present: recency bias, publication frequency asymmetries, and editorial gatekeeping replace algorithmic curation.

**Moderation regimes** filter content before observation. Every platform removes content according to its community standards — but these standards vary dramatically across platforms, jurisdictions, and cultural contexts. Content that is permitted on Telegram may be removed on Facebook. Content moderated in one jurisdiction (Germany's NetzDG) may remain visible in another. AĒR does not observe the discourse that platforms suppress, and this suppression is itself a form of bias: the *absence* of certain speech patterns in crawled data may reflect platform policy rather than societal attitudes.

**API access as a research choke point.** Since 2023, major platforms have systematically restricted or priced out research API access (X's API pricing restructuring, Reddit's API changes, Meta's CrowdTangle deprecation). AĒR's architectural decision to use standalone crawlers per source (§5.2) means each data source's accessibility is an independent constraint. RSS feeds — AĒR's Probe 0 data source — sidestep this problem entirely because they are designed for public consumption. But RSS represents only a narrow slice of the digital discourse landscape (institutional, editorial, one-directional). Expanding to richer discourse spaces will inevitably encounter API restrictions.

### 2.2 The Asymmetry of Observable Discourse

Not all digital discourse is equally observable. The observability of a discourse space depends on three orthogonal dimensions:

**Technical accessibility.** Does the platform offer public endpoints (RSS, public APIs, scrapeable web pages), or is content locked behind authentication, app-only access, or end-to-end encryption? WhatsApp, Signal, and Telegram private groups — major vehicles for discourse in many societies — are technically unobservable without participant access, which raises ethical concerns that AĒR explicitly avoids (no user-level data, no covert observation).

**Legal accessibility.** Is crawling permitted under the platform's Terms of Service, under the applicable data protection regime (GDPR, CCPA, LGPD, PIPA), and under the relevant jurisdiction's computer access laws? Legal constraints are not uniform — they vary by country, by platform, and by the nature of the data collected. AĒR's Probe 0 source selection criteria (§13.8) explicitly prioritize sources without ToS restrictions, but this limits the observable space to institutional, government, and public media sources.

**Ethical accessibility.** Even where technically and legally possible, some discourse spaces should not be observed without informed consent. Private forums, support groups, minority community spaces, and platforms used by vulnerable populations require ethical review that goes beyond legal compliance. AĒR's Manifesto commits to observation over surveillance — but the boundary between the two is contextually defined and requires ongoing ethical judgment.

The intersection of these three dimensions creates a **bias topology**: the discourse that AĒR *can* observe is a non-random subset of the discourse that *exists*. This subset systematically overrepresents institutional, public, English-language, Western-platform-hosted content and underrepresents private, encrypted, non-English, and platform-locked discourse. This is not a bug — it is a structural property of any digital discourse observatory. The scientific obligation is to document this topology explicitly, not to pretend it does not exist.

---

## 3. The Global Platform Landscape: Beyond the Western Stack

### 3.1 Platform Hegemony and Digital Geopolitics

The default mental model of "the internet" in Western research is a landscape dominated by a handful of American platforms: Google, Meta (Facebook/Instagram/WhatsApp), X/Twitter, Reddit, YouTube, TikTok (Chinese-owned but globally distributed). This model is not merely incomplete — it is actively misleading for a project that aspires to observe global discourse.

The global platform landscape is fragmented along linguistic, cultural, political, and regulatory lines. AĒR's expansion beyond Probe 0 must account for this fragmentation. The following mapping is illustrative, not exhaustive:

**East Asia.** China's internet is a distinct ecosystem behind the Great Firewall: WeChat (微信) serves as super-app combining messaging, social media, payments, and public information; Weibo (微博) functions as a micro-blogging platform with distinct censorship dynamics; Douyin (抖音, the Chinese TikTok) shapes visual discourse; Baidu Tieba (百度贴吧) provides forum-based discussion. Japan's digital discourse landscape includes LINE (messaging with public accounts), Yahoo! Japan News comments, 2channel/5channel (anonymous forums with profound cultural influence), and Hatena (blogging). South Korea's landscape centers on Naver (portal and news), KakaoTalk (messaging), and community platforms like DC Inside and theqoo. Each of these platforms has distinct affordances, moderation regimes, and user demographics that produce structurally different discourse.

**Russia and the post-Soviet space.** VKontakte (VK) and Odnoklassniki remain dominant social platforms. Telegram serves a dual function as both messaging platform and de facto public media infrastructure, particularly since 2022. RuNet governance involves state-directed content regulation (Roskomnadzor) that shapes observable discourse in ways fundamentally different from Western content moderation.

**South and Southeast Asia.** India's digital discourse is shaped by WhatsApp (end-to-end encrypted, largely unobservable), ShareChat (Hindi-language social platform), Koo (Indian micro-blogging, now defunct as of 2024 — illustrating platform ephemerality), and a vibrant vernacular blog sphere across dozens of languages. Indonesia's discourse landscape includes distinct uses of Facebook, X, and TikTok, alongside local forums like Kaskus. The Philippines' political discourse is heavily shaped by Facebook, which functions as the de facto internet for many users (Meta's Free Basics program).

**Middle East and North Africa.** Platform usage patterns are shaped by state surveillance concerns, leading to high adoption of encrypted messaging (Telegram, Signal) alongside public platforms (X/Twitter remains significant for political discourse in the Gulf states and Iran). Al Jazeera, BBC Arabic, and national media RSS feeds represent the institutional layer, while Telegram channels and X accounts represent the counter-discourse layer. The Arabic-speaking world is not monolithic — Gulf states, the Levant, the Maghreb, and Egypt each have distinct digital cultures.

**Sub-Saharan Africa.** Mobile-first internet access shapes platform adoption: WhatsApp and Facebook dominate in many markets. Nigeria, Kenya, and South Africa have distinct digital public spheres. Local-language radio station Facebook pages function as community discourse platforms. The absence of dominant local platforms means discourse is hosted on global platforms but shaped by local communicative norms — a form of digital code-switching that complicates analysis.

**Latin America.** WhatsApp is the dominant communication platform across the region. X/Twitter serves political discourse functions in Brazil, Argentina, and Mexico. Telegram adoption has grown, particularly in Brazil. Regional media ecosystems (Folha de São Paulo, El País América, Clarín) provide RSS-accessible institutional discourse, but the majority of discourse occurs on global platforms used in locally specific ways.

### 3.2 Implications for AĒR's Crawler Architecture

AĒR's architectural pattern — standalone crawlers per source type, submitting to a unified Ingestion API (§5.2) — is well-suited to this heterogeneous landscape. Each platform requires its own crawler with platform-specific logic for authentication, pagination, rate limiting, and data extraction. The Source Adapter Protocol (ADR-015) ensures that platform-specific data is harmonized into `SilverCore` without contaminating the core schema.

However, the *selection* of which platforms to crawl is not a technical decision — it is a scientific one, governed by WP-001's Functional Probe Taxonomy. A Telegram channel operated by an Iranian dissident group and a Bundesregierung RSS feed may both serve the discourse function of "Subversion & Friction" and "Resource & Power Legitimation" respectively, but they require fundamentally different crawlers, different ethical review, and different bias documentation.

The `SilverMeta` layer must capture platform-specific metadata that enables bias analysis:

- `platform_type`: The hosting platform (e.g., `rss`, `twitter`, `telegram`, `weibo`)
- `access_method`: How the data was obtained (e.g., `public_api`, `rss_feed`, `web_scrape`, `academic_api`)
- `visibility_mechanism`: How the platform surfaced this content (e.g., `chronological`, `algorithmic`, `editorial_curation`)
- `moderation_regime`: Known moderation context (e.g., `netzdg_applicable`, `great_firewall`, `unmoderated`)

These fields do not claim to *correct* for platform bias — they document the conditions under which the data was produced, enabling downstream analysis to account for them.

---

## 4. Algorithmic Amplification: The Invisible Hand in the Data

### 4.1 What Algorithmic Amplification Means for AĒR

When AĒR observes discourse from an algorithmically curated platform, it does not observe "what people are saying" — it observes "what the algorithm chose to surface from what people are saying." This distinction is critical.

Recommendation algorithms optimize for engagement metrics (clicks, shares, comments, watch time). A substantial body of research demonstrates that engagement-optimizing algorithms systematically amplify content that is emotionally arousing, polarizing, outrage-inducing, or novelty-seeking (Brady et al., 2017; Bail et al., 2018; Huszár et al., 2022). This means that algorithmically curated data sources carry a systematic **negativity and extremity bias** — not because the population is negative or extreme, but because the algorithm selects for content that provokes reaction.

For AĒR, this creates a concrete measurement problem: if the system measures rising negativity in discourse from an algorithmically curated source, it cannot determine whether:

1. The population is genuinely expressing more negative sentiment (societal signal)
2. The algorithm has been updated to amplify more negative content (platform signal)
3. The engagement dynamics have shifted such that negative content gets more interaction and therefore more visibility (behavioral feedback loop)

These three explanations produce identical data in the Gold layer. Distinguishing between them requires either platform-internal data (which AĒR does not have) or carefully designed counterfactual comparison strategies.

### 4.2 Counterfactual Strategies

**Cross-platform triangulation.** If the same topic produces different sentiment distributions on algorithmically curated platforms (Facebook, YouTube) versus chronologically ordered platforms (RSS, Mastodon, forums), the divergence may indicate algorithmic amplification. This requires AĒR to maintain parallel probes across platform types for the same discourse function and cultural context — a direct application of WP-001's taxonomy.

**Temporal discontinuity detection.** Algorithmic changes are often deployed abruptly. If sentiment or topic distributions shift suddenly without a corresponding real-world event, the shift may reflect a platform algorithm change rather than a discourse change. The Gold layer's temporal metrics (publication frequency, temporal distribution) can serve as anomaly detectors for this purpose.

**Volume-sentiment decoupling.** On algorithmically curated platforms, high-volume content is not a random sample — it is the algorithm's selection. If sentiment and volume are correlated (negative content is higher volume), this correlation may be algorithmic rather than organic. Decomposing trends into volume-weighted and unweighted sentiment can reveal this effect.

**Source diversity monitoring.** If a small number of accounts or sources dominate the observed corpus from a platform, this may indicate algorithmic concentration rather than discourse consensus. Tracking source diversity metrics (number of unique authors, Gini coefficient of publication volume per source) in the Gold layer can flag this pattern.

### 4.3 The Special Case of RSS and Editorial Platforms

AĒR's Probe 0 — German institutional RSS — is notably free from algorithmic amplification bias. RSS feeds are chronologically ordered, editorially curated (by human journalists, not algorithms), and do not contain engagement metrics. This makes RSS an excellent calibration baseline precisely because it lacks the amplification effects present on social media.

However, editorial curation is itself a bias: the editorial decisions of Tagesschau journalists about which stories to cover, how to frame them, and which voices to include reflect institutional priorities, journalistic norms, and the cultural milieu of German public broadcasting. This is WP-001's "Resource & Power Legitimation" function — a specific discourse function, not a neutral baseline.

---

## 5. Non-Human Actors: Bots, Coordinated Operations, and Synthetic Content

### 5.1 The Scale of the Problem

The Manifesto's aspiration to observe "the underlying human impulses" (§III) presupposes that the observed content is produced by humans. This assumption is increasingly unreliable.

**Social bots.** Automated accounts that mimic human behavior have been documented on every major social platform. Estimates of bot prevalence vary widely — Varol et al. (2017) estimated 9–15% of active Twitter accounts were bots; more recent estimates for X in 2024–2025 range higher. Bot prevalence varies by platform, language, and topic: political discourse, cryptocurrency, and health misinformation attract disproportionate bot activity. Bots produce content that enters AĒR's pipeline indistinguishably from human-authored content if the crawler does not perform bot detection upstream.

**Coordinated inauthentic behavior (CIB).** State-sponsored and commercially motivated influence operations use networks of accounts (not all automated) to amplify specific narratives. The Stanford Internet Observatory, Graphika, and the DFRLab have documented hundreds of CIB campaigns across platforms. CIB is particularly challenging for AĒR because the content itself may be linguistically indistinguishable from organic human discourse — the inauthenticity lies in the coordination pattern, not the text.

**AI-generated content.** Since 2023, large language models have enabled the production of synthetic text at unprecedented scale and quality. AI-generated news articles, blog posts, social media content, and forum comments are increasingly difficult to distinguish from human-authored text using surface-level linguistic features alone. For AĒR, this represents an existential challenge: if a growing share of the observed "discourse" is machine-generated, the system's claim to observe human societal patterns is undermined. The rate of AI content proliferation varies by platform and language — English-language content on platforms with low moderation barriers is likely most affected.

### 5.2 Detection Approaches

Bot and synthetic content detection is an active research field with no settled solutions. The approaches can be mapped to AĒR's architectural constraints:

**Account-level behavioral features.** Traditional bot detection relies on metadata about the posting account: account age, posting frequency, follower/following ratios, profile completeness (Cresci, 2020). These features are not available to AĒR in all contexts — RSS feeds have no account concept; API-restricted platforms may not expose account metadata. Where available, account metadata belongs in `SilverMeta` as source-specific context, not in `SilverCore`.

**Network-level coordination detection.** CIB detection relies on identifying coordinated posting patterns: synchronized timing, shared content, network structure analysis (Pacheco et al., 2021). This requires corpus-level analysis (the `CorpusExtractor` path anticipated in §13.3) and cross-document metadata — specifically, temporal co-occurrence of content from different sources. AĒR's Gold layer already stores temporal distribution metrics; extending this to coordination detection is architecturally feasible but methodologically demanding.

**Linguistic features of synthetic text.** AI-generated text exhibits statistical regularities that differ from human text: lower perplexity, more uniform token distributions, characteristic phrase patterns, and reduced lexical diversity (Gehrmann et al., 2019; Mitchell et al., 2023). However, these features are rapidly becoming less reliable as language models improve. Detection methods that work against GPT-3 may fail against GPT-5 or Claude. This is an arms race with no stable equilibrium.

**Watermarking and provenance.** Some AI providers embed statistical watermarks in generated text (Kirchenbauer et al., 2023). These are not universally adopted, can be removed by paraphrasing, and are only detectable if the watermarking scheme is known. AĒR cannot rely on watermarking as a primary detection strategy, but should monitor the development of provenance standards (C2PA, Content Credentials) that may provide metadata-level authenticity signals in the future.

### 5.3 AĒR's Position: Document, Don't Filter

A critical design decision for AĒR is whether to *filter out* suspected bot or synthetic content or to *flag* it as metadata. The Manifesto's commitment to unaltered observation ("unaltered mirror of humanity") suggests the latter: AĒR should not suppress data based on inferred authenticity, because the inference itself may be wrong (false positives), and because the presence, prevalence, and dynamics of non-human actors are themselves phenomena worth observing.

The proposed approach:

1. **Compute authenticity indicators as Gold metrics.** Implement per-document extractors that produce probabilistic authenticity scores (e.g., `human_authorship_confidence: 0.87`). These are Tier 2 or Tier 3 metrics — reproducible with fixed model version but not deterministic across model versions.

2. **Compute coordination indicators as corpus-level metrics.** Implement `CorpusExtractor` instances that detect temporal and content-based coordination patterns across documents within a time window. Flag coordinated clusters rather than individual documents.

3. **Expose indicators via Progressive Disclosure.** The dashboard shows aggregate metrics with and without suspected non-human content. The analyst can drill down to inspect flagged documents and make their own judgment. The system does not make the filtering decision — the human does.

4. **Document the arms race.** The detection models will degrade over time as synthetic content improves. AĒR must version-pin detection models and document their known limitations, just as WP-002 requires for sentiment and NER models.

---

## 6. Demographic Skew and the Digital Divide

### 6.1 Who Is Missing from the Data?

The Manifesto acknowledges the Digital Divide as "a defining parameter of the observation" (§II). This section makes the divide concrete by mapping who is systematically underrepresented in the digital discourse AĒR can observe.

**Age.** Internet usage varies dramatically by age cohort, but the pattern is not uniform across cultures. In many Western societies, older adults (65+) are underrepresented on social media but present in news comment sections and forums. In sub-Saharan Africa, where median age is 19, the "older generation" underrepresentation threshold is much lower. In Japan, an aging society with high digital literacy, older adults are present on LINE and Yahoo! News comments but absent from TikTok and Instagram.

**Socioeconomic status.** Access to the internet correlates with income, education, and urbanization. Rural populations in India, Indonesia, and Sub-Saharan Africa have different internet access patterns (mobile-only, intermittent connectivity, data-cost-constrained usage) than urban populations. These access patterns determine which platforms are used and how — data-light applications (WhatsApp, Telegram) may be preferred over data-heavy ones (YouTube, TikTok).

**Gender.** Gender gaps in internet access persist in many regions. The GSMA reports that women in low- and middle-income countries are 16% less likely to use mobile internet than men (2023). Even where access is equal, platform choice and usage patterns may differ by gender in culturally specific ways. Anonymous platforms may overrepresent male participation in some cultures while providing protective spaces for women's voices in others.

**Language.** The internet is dominated by English-language content (approximately 55–60% of web content by most estimates), followed by Spanish, Chinese, Arabic, and Portuguese. Speakers of less-digitized languages — thousands of languages with active speaker communities but minimal digital presence — are effectively invisible to any text-based observation system. AĒR's Ockham's Razor principle demands acknowledging this limit rather than pretending universal coverage.

**Political environment.** Citizens of authoritarian regimes may self-censor on observable platforms and reserve genuine political discourse for encrypted or offline channels. The public discourse AĒR observes in such contexts may represent the *permissible* rather than the *authentic* — a distinction that has profound implications for interpreting sentiment and stance metrics.

### 6.2 From Acknowledging to Documenting

AĒR cannot solve the Digital Divide. It can, however, document it systematically for each probe. The proposed approach:

**Per-probe demographic profile.** For each data source registered in the PostgreSQL `sources` table, document the known demographic skew: which populations are overrepresented, which are underrepresented, and what evidence supports these assessments. This documentation belongs in `SilverMeta` as source-level metadata, not per-document metadata.

**Coverage gap visualization.** The dashboard should visualize not only what AĒR observes but what it *cannot* observe — a map of known blind spots. This is the macroscope's equivalent of a telescope's light pollution map: an explicit acknowledgment of the instrument's limitations as a feature of the instrument itself.

**Weighting strategies (research question).** Can demographic weighting — a standard technique in survey methodology — be applied to digital discourse data? If AĒR knows that a platform overrepresents urban, educated, 25–35-year-old men, can it weight the data to approximate population representativeness? This is an open research question with significant methodological challenges: the demographic composition of platform users is itself uncertain, and the relationship between demographics and discourse patterns is complex and culturally variable.

---

## 7. Open Questions for Interdisciplinary Collaborators

### 7.1 For Platform Governance and Internet Studies Scholars

**Q1: How should AĒR document and account for algorithmic curation effects when observing discourse from engagement-optimized platforms?**

- AĒR's RSS-based Probe 0 avoids this problem entirely. When expanding to social media platforms, what metadata and counterfactual strategies are required to distinguish algorithmic signal from societal signal?
- Deliverable: A framework for documenting algorithmic curation effects per platform, suitable for inclusion in `SilverMeta`.

**Q2: How should AĒR navigate the post-2023 research API landscape?**

- With X, Reddit, and Meta restricting academic API access, what alternative data collection strategies are available, ethical, and legally defensible?
- Which platforms currently offer robust academic research APIs, and what are their known limitations?
- Deliverable: A platform-by-platform access assessment with legal and ethical annotations, updated annually.

### 7.2 For Computational Social Scientists and Bot Researchers

**Q3: Which bot detection methods are applicable to AĒR's architecture, given that AĒR processes text and metadata but often lacks full account-level behavioral data?**

- Text-only bot detection is less accurate than account-level detection. What confidence thresholds are appropriate for a system that flags rather than filters?
- Deliverable: An evaluation of text-level and metadata-level bot detection methods on a multilingual corpus, with false positive/negative rates per method.

**Q4: How should AĒR detect and flag AI-generated content in its pipeline?**

- Current detection methods degrade rapidly. Is there a sustainable detection strategy, or should AĒR accept AI-generated content as an observable phenomenon and focus on *measuring its prevalence* rather than *filtering it out*?
- Deliverable: A position paper on the epistemological status of AI-generated content in discourse observation systems, with practical recommendations for AĒR's extractor pipeline.

**Q5: How can AĒR detect coordinated inauthentic behavior using corpus-level analysis?**

- CIB detection typically requires network data (follower graphs, repost chains). Can temporal co-occurrence and content similarity in AĒR's Gold layer serve as proxies?
- Deliverable: A method for CIB detection using only the features available in `SilverCore` and `SilverMeta` (timestamps, source identifiers, cleaned text), evaluated against known CIB campaigns.

### 7.3 For Area Studies Scholars and Digital Anthropologists

**Q6: For each major cultural region, which platforms constitute the minimum viable probe set for discourse observation?**

- AĒR's WP-001 taxonomy defines four discourse functions. For a given society (e.g., Brazil, Japan, Nigeria, Iran), which platforms map to which functions? Which platforms are technically, legally, and ethically accessible?
- Deliverable: Regional platform maps (1–2 pages each) covering at minimum: East Asia, South Asia, MENA, Sub-Saharan Africa, Latin America, Russia/post-Soviet, Europe, North America. Each map should identify platforms, their discourse function, accessibility, and known demographic skew.

**Q7: How do platform-specific communicative norms affect the interpretability of text metrics across platforms?**

- A Twitter/X thread, a Reddit comment, a Telegram channel post, and an RSS article about the same event will produce structurally different text. How should AĒR account for platform-specific genre effects when comparing metrics across sources?
- Deliverable: A platform-genre taxonomy that documents the structural, rhetorical, and pragmatic differences between text produced on different platforms.

### 7.4 For Survey Methodologists and Statisticians

**Q8: Can demographic weighting techniques from survey methodology be adapted for digital discourse data?**

- Survey weighting requires known population parameters and known selection probabilities. Neither is available for digital platforms. Under what conditions, if any, can weighting reduce demographic skew in crawled data?
- Deliverable: A methodological assessment of weighting strategies for non-probability digital samples, with recommendations for AĒR's aggregation pipeline.

**Q9: How should AĒR model and report the uncertainty introduced by platform bias?**

- If all metrics carry platform-induced uncertainty, how should this uncertainty be propagated through the aggregation pipeline and visualized in the dashboard?
- Deliverable: An uncertainty quantification framework for platform-mediated discourse metrics, compatible with AĒR's ClickHouse Gold schema.

---

## 8. Architectural Implications

### 8.1 SilverMeta Extension for Bias Documentation

The Source Adapter Protocol (ADR-015) already supports source-specific `SilverMeta` models. WP-003 proposes standardizing a set of bias-relevant metadata fields across all future source adapters:

```python
class BiasContext(BaseModel):
    """Standardized bias documentation fields for SilverMeta."""
    platform_type: str           # e.g., "rss", "microblog", "forum", "messenger_channel"
    access_method: str           # e.g., "public_rss", "academic_api", "public_web"
    visibility_mechanism: str    # e.g., "chronological", "algorithmic", "editorial"
    moderation_context: str      # e.g., "netzdg", "unmoderated", "state_censored"
    engagement_data_available: bool  # whether likes/shares/comments are captured
    account_metadata_available: bool # whether author/account features are captured
```

These fields enable downstream analysis to stratify metrics by platform characteristics — for example, comparing sentiment distributions across algorithmically curated vs. chronologically ordered sources.

### 8.2 New Extractor Types

WP-003 anticipates two new extractor categories beyond WP-002's metric extractors:

**Authenticity extractors (per-document, Tier 2/3).** Compute probabilistic scores for human authorship, AI-generation likelihood, and template-based content detection. These would implement the existing `MetricExtractor` protocol, producing metrics like `human_authorship_confidence` and `template_match_score`.

**Coordination extractors (corpus-level, Tier 2).** Detect temporal and content-based coordination patterns across documents within a time window. These require the `CorpusExtractor` protocol (anticipated in §13.3, blocked by R-9) and produce corpus-level metrics like `coordination_score` for document clusters.

### 8.3 Impact on the Gold Layer

No schema changes to `aer_gold.metrics` are required — authenticity and coordination scores are numeric metrics that fit the existing `(timestamp, value, source, metric_name)` schema. However, corpus-level coordination detection may produce *cluster-level* outputs (groups of documents flagged as coordinated) that do not map cleanly to the per-document metric model. This may require a new Gold table:

```sql
CREATE TABLE aer_gold.coordination_clusters (
    cluster_id       String,
    detection_date   DateTime,
    detection_method String,
    method_version   String,
    document_ids     Array(String),
    coordination_type String,    -- e.g., "temporal_sync", "content_duplicate", "cross_platform"
    confidence       Float32
) ENGINE = MergeTree()
ORDER BY (detection_date, cluster_id)
TTL detection_date + INTERVAL 365 DAY;
```

This table would be queried by the BFF API to annotate individual documents with their coordination cluster membership — enabling Progressive Disclosure from aggregate trends to individual flagged clusters.

---

## 9. The Observer's Responsibility

WP-003 intersects with a deeper ethical question that will be fully addressed in WP-006 (Observer Effect): AĒR's act of observing discourse is not neutral. By selecting certain platforms, applying certain detection methods, and flagging certain content as "inauthentic," AĒR makes judgments that carry political weight. Labeling a Telegram channel as "bot-driven" or a news source as "state-controlled" is an analytical act with consequences for how the data is interpreted.

AĒR's architectural commitment to Progressive Disclosure — showing the analyst the raw data alongside the computed metrics — is the primary safeguard against black-box bias judgments. But the design of the dashboard, the default views, the choice of which metrics to highlight and which to hide — these are all design decisions that carry epistemological weight. They should be made transparently, with input from the interdisciplinary collaborators this paper seeks to engage.

---

## 10. References

- Bail, C. A., Argyle, L. P., Brown, T. W., et al. (2018). "Exposure to Opposing Views on Social Media Can Increase Political Polarization." *Proceedings of the National Academy of Sciences*, 115(37), 9216–9221.
- Brady, W. J., Wills, J. A., Jost, J. T., Tucker, J. A. & Van Bavel, J. J. (2017). "Emotion Shapes the Diffusion of Moralized Content in Social Networks." *Proceedings of the National Academy of Sciences*, 114(28), 7313–7318.
- Bucher, T. & Helmond, A. (2018). "The Affordances of Social Media Platforms." In Burgess, J., Marwick, A. & Poell, T. (Eds.), *The SAGE Handbook of Social Media*, 233–253. SAGE.
- Cresci, S. (2020). "A Decade of Social Bot Detection." *Communications of the ACM*, 63(10), 72–83.
- Gehrmann, S., Strobelt, H. & Rush, A. M. (2019). "GLTR: Statistical Detection and Visualization of Generated Text." *Proceedings of ACL 2019 — System Demonstrations*.
- GSMA (2023). *The Mobile Gender Gap Report 2023*. GSMA Connected Women.
- Huszár, F., Ktena, S. I., O'Brien, C., et al. (2022). "Algorithmic Amplification of Politics on Twitter." *Proceedings of the National Academy of Sciences*, 119(1), e2025334119.
- Kirchenbauer, J., Geiping, J., Wen, Y., et al. (2023). "A Watermark for Large Language Models." *Proceedings of ICML 2023*.
- Mitchell, E., Lee, Y., Khazatsky, A., Manning, C. D. & Finn, C. (2023). "DetectGPT: Zero-Shot Machine-Generated Text Detection using Probability Curvature." *Proceedings of ICML 2023*.
- Pacheco, D., Hui, P.-M., Torres-Lugo, C., et al. (2021). "Uncovering Coordinated Networks on Social Media: Methods and Case Studies." *Proceedings of ICWSM 2021*.
- Varol, O., Ferrara, E., Davis, C. A., Menczer, F. & Flammini, A. (2017). "Online Human-Bot Interactions: Detection, Estimation, and Characterization." *Proceedings of ICWSM 2017*.

---

## Appendix A: Mapping to AĒR Open Research Questions (§13.6)

| §13.6 Question | WP-003 Section | Status |
| :--- | :--- | :--- |
| 2. Bias Calibration | §2–§6 | Addressed — framework and research questions proposed |
| 1. Probe Selection | §3 (platform landscape), §6 (demographic skew) | Partially addressed — complements WP-001 |
| 3. Metric Validity | §4.1 (amplification affects metric interpretation) | Cross-reference to WP-002 |
| 6. Observer Effect | §9 | Previewed — deferred to WP-006 |

## Appendix B: Mapping to WP-001 Functional Taxonomy

| WP-001 Discourse Function | Platform Bias Considerations |
| :--- | :--- |
| Epistemic Authority | Institutional RSS/websites — low algorithmic bias, high editorial bias. Government platforms may involve state censorship. |
| Resource & Power Legitimation | Official channels — structurally controlled messaging. Bot amplification is common for state narratives. |
| Cohesion & Identity Formation | Social media, influencer platforms — high algorithmic amplification, engagement-optimized, significant bot/CIB presence. |
| Subversion & Friction | Encrypted messaging, anonymous forums, alternative platforms — low observability, high authenticity uncertainty, platform ephemerality risk. |