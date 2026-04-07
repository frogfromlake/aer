# WP-005: Temporal Granularity of Discourse Shifts

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft — open for interdisciplinary review
> **Date:** 2026-04-07
> **Depends on:** WP-002 (Metric Validity), WP-004 (Cross-Cultural Comparability)
> **Architectural context:** ClickHouse Gold Layer (§5.1.4), BFF API downsampling (§8.6), TTL/ILM (Phase 11), Temporal Extractor (Phase 42)

---

## 1. Objective

This working paper addresses the fifth open research question from §13.6: **At what temporal resolution do discourse shifts become meaningful? Hours (breaking news), days (news cycles), weeks (policy debates), months (cultural shifts)?**

AĒR's Gold layer stores every metric with a per-document timestamp. The BFF API aggregates these into 5-minute intervals for the dashboard (Phase 13 downsampling). But the raw temporal resolution of the data says nothing about the temporal resolution of *meaning*. A sentiment score computed every 5 minutes on a Tagesschau RSS feed produces a jagged time series whose high-frequency oscillations are almost certainly noise — the result of individual articles entering and leaving the observation window — while its low-frequency trends may contain genuine signals of shifting discourse.

The question of temporal granularity is not purely statistical. It is deeply entangled with the *kind* of discourse phenomenon being observed, the *type* of data source, and the *cultural context* of the discourse. Breaking news unfolds in minutes; news cycles operate in hours; policy debates develop over days and weeks; cultural shifts — the Episteme dimension of AĒR's DNA — manifest over months and years. A single temporal resolution cannot serve all these phenomena.

This paper maps the relationship between temporal granularity and discourse phenomena, proposes a multi-scale temporal framework, and identifies the architectural and methodological requirements for implementing it within AĒR's pipeline.

---

## 2. Discourse Phenomena and Their Temporal Signatures

### 2.1 A Taxonomy of Temporal Scales

Discourse phenomena operate across at least five distinct temporal scales. Each scale corresponds to different societal processes, requires different metrics, and demands different analytical methods.

**Scale 1: Event response (minutes to hours).** When a major event occurs — a terrorist attack, a natural disaster, a political resignation, a central bank decision — the digital discourse landscape shifts rapidly. Publication frequency spikes; sentiment polarizes; new entities enter the discourse. The temporal signature of event response is a **sharp impulse** followed by exponential decay. On social media platforms (which AĒR may observe in future probes), this impulse can be measured at minute-level resolution. On RSS feeds (Probe 0), the resolution is coarser — institutional media respond within hours, not minutes.

The analytical question at this scale is *reactivity*: how quickly does the discourse respond? How does response latency vary by source type, by platform, by culture? WP-003 noted that different platforms have different temporal affordances (minute-level on X/Twitter, hour-level on RSS, half-day on traditional print media). Event response analysis must account for these platform-specific latencies.

**Scale 2: News cycle (hours to days).** The news cycle is a culturally constructed temporal unit. In Western 24-hour news cultures, a story rises, peaks, and fades within 24–48 hours. In media cultures with morning/evening edition rhythms (Japan, parts of Europe), the cycle is structured differently. The news cycle determines how long a topic remains salient — and salience is what AĒR's topic metrics attempt to measure.

At this scale, the relevant metrics are **topic prevalence** (what share of discourse is devoted to topic X?), **entity salience** (which actors are mentioned?), and **framing shifts** (does the framing of a topic change over the course of the cycle?). Hourly or half-daily aggregation is appropriate; sub-hourly data is noise.

**Scale 3: Agenda dynamics (days to weeks).** Media agendas and public attention are not driven solely by events — they are shaped by institutional rhythms (parliamentary sessions, quarterly earnings, election calendars), by editorial decisions (investigative series, thematic weeks), and by inter-media agenda setting (Macomber McCombs, 2004). At this scale, the question shifts from "what happened?" to "what is the discourse paying attention to?"

Relevant metrics are **topic persistence** (how many days does a topic remain above a salience threshold?), **agenda diversity** (how concentrated or distributed is attention across topics?), and **cross-source convergence** (do different sources converge on the same agenda?). Daily or multi-day aggregation is appropriate.

**Scale 4: Policy and public discourse (weeks to months).** Policy debates, social movements, and public controversies unfold over weeks and months. The German *Energiewende* debate, the American healthcare reform discourse, the French retirement reform protests — these are sustained discourse phenomena with complex internal dynamics: phases of argumentation, counter-argumentation, framing contests, and resolution or exhaustion.

At this scale, the relevant metrics are **sentiment trends** (is evaluation of a policy becoming more positive or negative over weeks?), **narrative evolution** (how do the dominant frames shift?), and **polarization dynamics** (is discourse converging or diverging?). Weekly aggregation with moving averages is appropriate; daily fluctuations are typically noise at this scale.

**Scale 5: Cultural drift (months to years).** The Episteme dimension — the boundaries of what can be thought and said — shifts slowly. The normalization of certain vocabulary, the emergence of new political categories, the gradual reframing of social issues — these are multi-year phenomena that require longitudinal observation. Foucault's epistemic shifts operate on generational timescales; Shiller's narrative epidemics operate on multi-year cycles.

At this scale, the relevant metrics are **vocabulary evolution** (which terms enter and exit the active lexicon?), **semantic drift** (do existing terms change their connotative meaning over time?), **baseline shifts** (does the "normal" sentiment level change?), and **structural changes** (do new topic clusters emerge or old ones dissolve?). Monthly or quarterly aggregation is appropriate; any higher resolution is overwhelmed by cyclical noise (weekday patterns, seasonal effects, holiday dips).

### 2.2 The Multi-Scale Hypothesis

The central hypothesis of this paper is that **no single temporal resolution is appropriate for all discourse metrics, and the optimal resolution depends on the interaction between the metric type, the source type, and the discourse phenomenon under observation.**

This is not merely a data engineering concern (how to aggregate efficiently in ClickHouse) — it is a scientific concern (at what resolution does signal emerge from noise?). Presenting high-frequency data for a slow phenomenon creates the illusion of volatility where there is stability. Presenting low-frequency data for a fast phenomenon masks the dynamics that constitute the phenomenon.

---

## 3. Signal, Noise, and the Resolution Problem

### 3.1 Noise Sources in AĒR's Time Series

AĒR's Gold layer time series contain noise at multiple frequencies:

**Document-level noise.** Individual articles have idiosyncratic sentiment scores, entity compositions, and word counts. When the observation window contains only a few documents (as in low-volume RSS feeds), a single article can dominate the aggregate metric. This is a small-sample problem — the signal-to-noise ratio improves with document volume.

**Publication schedule noise.** RSS feeds publish at irregular intervals determined by editorial rhythms, not by discourse dynamics. A gap between publications is not a gap in discourse — it is a gap in observation. The temporal distribution extractor (Phase 42) captures publication patterns, but the *absence* of data at a given time point is ambiguous: it may mean "nothing happened" or "nothing was published."

**Metric extraction noise.** Provisional extractors (WP-002) introduce measurement noise that is superimposed on genuine signal. SentiWS sentiment scores have unknown error distributions; spaCy NER has variable precision per entity type. This noise is constant at all temporal scales but becomes proportionally larger as aggregation windows shrink (fewer documents to average over).

**Cyclical noise.** Weekday/weekend patterns, time-of-day patterns, and seasonal patterns (holiday periods, parliamentary recesses, summer news holes) create periodic fluctuations that are predictable but must be filtered or modeled before trend analysis. AĒR's temporal extractor produces `publication_hour` and `publication_weekday` — these can serve as features for cyclical decomposition.

### 3.2 Temporal Decomposition

Time series analysis offers established methods for separating signal from noise at different scales. The most relevant for AĒR:

**Classical decomposition.** Decompose the time series into trend (T), seasonal (S), and residual (R) components: Y = T + S + R. The trend captures Scale 4–5 phenomena; the seasonal component captures cyclical patterns (Scale 2–3); the residual captures event responses (Scale 1) and noise.

**STL decomposition (Seasonal and Trend decomposition using Loess).** A robust nonparametric method (Cleveland et al., 1990) that handles irregular seasonality and is resistant to outliers. Well-suited for AĒR's data, which may have irregular publication schedules and event-driven spikes.

**Wavelet decomposition.** Multi-resolution analysis that decomposes a time series into components at different frequency bands simultaneously. This is the most theoretically aligned with the multi-scale hypothesis — it produces a *resolution pyramid* where each level captures phenomena at a different temporal scale.

**Change point detection.** Statistical methods (CUSUM, PELT, Bayesian change point detection) that identify moments where the statistical properties of the time series change abruptly. These are directly relevant for detecting discourse shifts — moments where the trend changes slope, the variance changes, or the mean shifts. Change points at different scales correspond to different types of discourse phenomena.

### 3.3 The Minimum Meaningful Aggregation Window

For each metric-source pair, there exists a **minimum meaningful aggregation window** — the smallest time window for which the aggregate metric is statistically distinguishable from noise. Below this window, the aggregate is dominated by document-level variation and publication schedule effects.

The minimum window depends on:

- **Document volume per window.** A source publishing 20 articles per day can support daily aggregation for most metrics. A source publishing 5 articles per day needs at least 2–3 days for stable sentiment averages. The relationship is approximately: minimum window ≈ k / publication_rate, where k is the number of documents needed for a stable estimate (k ≈ 30 for means, following the central limit theorem heuristic).

- **Metric variance.** High-variance metrics (sentiment, where individual documents vary widely) need larger windows than low-variance metrics (language detection confidence, which is stable per source).

- **Effect size of interest.** If the analyst is looking for large shifts (crisis response), smaller windows are sufficient because the signal is strong. If the analyst is looking for subtle drift (Episteme-level changes), larger windows are required because the signal is weak relative to noise.

AĒR should compute and expose minimum meaningful aggregation windows as metric metadata, enabling the dashboard to warn analysts when they select a temporal resolution that is below the meaningful threshold for a given metric-source pair.

---

## 4. Cultural Temporalities

### 4.1 Time Is Not Culturally Neutral

WP-004 (§4.3) introduced the concept of cultural temporal rhythms. WP-005 deepens this analysis.

The Gregorian calendar and the 24-hour UTC clock that AĒR uses as its temporal reference frame are not universal — they are Western instruments that impose a specific temporal ontology. Other temporal frameworks coexist:

**Religious calendars.** The Islamic calendar (lunar, 354–355 days) structures social rhythms in Muslim-majority societies. Ramadan shifts by approximately 10 days annually relative to the Gregorian calendar, creating a moving temporal pattern in publication frequency and discourse topics. The Hebrew calendar, the Hindu calendar (with its regional variants), the Chinese agricultural calendar, and the Buddhist calendar all structure social temporality differently.

**Political calendars.** Election cycles, legislative sessions, fiscal years, and national commemorations create temporal structures that vary by country. The American political calendar (midterms every 2 years, presidential elections every 4) creates discourse rhythms that differ from the German Bundestag cycle (4 years, no midterms) or the Japanese Diet session schedule (ordinary session January–June, extraordinary sessions as needed).

**Media calendars.** Publishing rhythms vary by media culture. Some media cultures have distinct "news seasons" (the German *Sommerloch* — summer news hole — when reduced political activity leads to light news). Some cultures have media blackouts around elections (France's 24-hour pre-election silence). Some cultures have censorship-driven temporal patterns (increased content removal before politically sensitive anniversaries in China).

### 4.2 Event-Response Dynamics Across Cultures

How quickly and how intensely different cultures respond to events is itself a cultural variable. Comparative event-response analysis requires controlling for:

**Platform-mediated response latency.** Social media enables immediate response; RSS reflects editorial processing time. Comparing response latency across cultures requires comparing like-for-like platform types (WP-003).

**Institutional response norms.** German institutional sources tend toward measured, delayed response (official statements take time). American institutional sources often respond rapidly with preliminary statements. This difference reflects institutional culture, not the speed of public reaction.

**Attention duration.** How long does a society maintain attention on an event? The concept of *nichibu* (日没, "sunset" — a topic fading from attention) in Japanese media studies suggests culturally specific attention decay curves. American media studies document increasingly short attention cycles driven by 24-hour cable news competition. Whether these differences are cultural or platform-driven (or both) is an open research question.

### 4.3 Implications for AĒR's Temporal Framework

AĒR's temporal framework must be:

1. **Multi-scale by default.** The dashboard should never force a single temporal resolution. Instead, it should offer a resolution selector that corresponds to the discourse phenomenon taxonomy (§2.1): event response (hours), news cycle (days), agenda dynamics (weeks), policy discourse (months), cultural drift (quarters/years).

2. **Culturally annotated.** Temporal anomalies (publication frequency dips, sentiment shifts) should be interpretable against cultural calendars. This requires a cultural calendar metadata service or annotation layer — a lookup table that maps dates to culturally significant events per region.

3. **Source-adaptive.** The minimum meaningful aggregation window (§3.3) should be computed per source and exposed to the dashboard, preventing analysts from over-interpreting high-frequency noise.

---

## 5. Architectural Implications

### 5.1 ClickHouse Multi-Resolution Aggregation

AĒR's current BFF API uses a single aggregation strategy: `toStartOfFiveMinute()` with `avg()` (Phase 13). This is appropriate for real-time monitoring but insufficient for multi-scale analysis.

The proposed extension introduces resolution-specific aggregation:

```sql
-- Scale 1: Event response (hourly)
SELECT toStartOfHour(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Scale 3: Agenda dynamics (daily)
SELECT toStartOfDay(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Scale 4: Policy discourse (weekly)
SELECT toStartOfWeek(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Scale 5: Cultural drift (monthly)
SELECT toStartOfMonth(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;
```

ClickHouse's columnar storage and vectorized execution make these aggregations efficient even on large datasets. No materialized views are required for the initial implementation — query-time aggregation is sufficient. Materialized views can be introduced later for performance optimization if query latency becomes a concern.

### 5.2 BFF API Extension

The BFF API's `/api/v1/metrics` endpoint should accept a `resolution` query parameter:

- `?resolution=5min` — current behavior (default for real-time monitoring)
- `?resolution=hourly` — Scale 1 aggregation
- `?resolution=daily` — Scale 2–3 aggregation
- `?resolution=weekly` — Scale 4 aggregation
- `?resolution=monthly` — Scale 5 aggregation

The `rowLimit` OOM protection (Phase 13) should be adjusted per resolution: wider aggregation windows produce fewer rows, so the limit can be relaxed for lower resolutions.

### 5.3 Trend and Anomaly Metadata

To support multi-scale analysis in the dashboard, the BFF API should expose computed trend and anomaly metadata alongside raw metrics:

**Moving averages.** Compute exponentially weighted moving averages (EWMA) at multiple window sizes. ClickHouse supports window functions that can compute these efficiently.

**Change point indicators.** Pre-compute change points using CUSUM or similar methods on the server side, returning them as metadata alongside the time series. This avoids pushing complex statistical computation to the frontend.

**Minimum meaningful window.** Return the computed minimum meaningful aggregation window per metric-source pair as part of the `/api/v1/metrics/available` response, enabling the frontend to disable temporal resolutions that are below the threshold.

### 5.4 Retention and Resolution Tiers

AĒR's current TTL policy is 365 days at full resolution. For long-term cultural drift analysis (Scale 5), data must be retained for years — but storing per-document metrics at full resolution for years is neither necessary nor economical.

A tiered retention strategy:

| Age | Resolution | Storage |
| :--- | :--- | :--- |
| 0–30 days | Full (per-document) | `aer_gold.metrics` (current) |
| 30–365 days | Hourly aggregates | `aer_gold.metrics_hourly` (materialized view) |
| 1–5 years | Daily aggregates | `aer_gold.metrics_daily` (materialized view) |
| 5+ years | Monthly aggregates | `aer_gold.metrics_monthly` (materialized view) |

ClickHouse materialized views can automate this tiering. The BFF API selects the appropriate tier based on the requested time range and resolution, transparently serving data from the highest-resolution table that covers the requested period.

---

## 6. The Three Pillars and Their Temporal Signatures

Each of AĒR's philosophical pillars (Chapter 1, §1.2) corresponds to a distinct temporal analysis mode:

### 6.1 Aleph: Synchronic Aggregation

The Aleph principle — the single point containing all other points — is fundamentally **synchronic**: it asks what the world looks like *right now*. The Aleph dashboard is a real-time cross-sectional snapshot: what is the discourse about today? Which entities are salient? What is the emotional tone?

Temporal requirement: near-real-time to hourly aggregation. The Aleph view is the "weather report" of global discourse — it shows current conditions, not historical trends.

### 6.2 Episteme: Diachronic Trend Analysis

The Episteme principle — the boundaries of the expressible — is fundamentally **diachronic**: it asks how the discourse *changes over time*. Detecting shifts in what can be thought and said requires long-term temporal analysis — weeks, months, years.

Temporal requirement: weekly to monthly aggregation with trend detection. The Episteme view is the "climate record" of global discourse — it shows slow structural changes that are invisible at daily resolution.

Specific Episteme-relevant analyses:

- **Vocabulary emergence.** Track the first appearance and adoption curve of new terms (e.g., *"Wutbürger"* entering German political vocabulary, *"incel"* entering English discourse, *"内卷"* (involution) entering Chinese social vocabulary).
- **Semantic shift.** Track changes in the co-occurrence context of existing terms over months/years using temporal word embeddings or diachronic distributional semantics (Hamilton et al., 2016).
- **Overton Window dynamics.** Track which topics and positions move from fringe to mainstream discourse (or vice versa) over multi-month periods.

### 6.3 Rhizome: Propagation Dynamics

The Rhizome principle — non-linear, decentralized information spread — is fundamentally **relational-temporal**: it asks how patterns propagate *through* networks *over* time. This is the most temporally complex analysis mode because it requires tracking the movement of discourse elements (topics, frames, entities) across sources, platforms, and cultures.

Temporal requirement: high-resolution (hourly) within event-response windows, combined with cross-source lag analysis. The Rhizome view is the "epidemiology" of discourse — it tracks how narratives spread from source to source, the temporal lag between first appearance and widespread adoption, and the structural pathways of information flow.

Specific Rhizome-relevant analyses:

- **Cross-source propagation.** When a topic first appears in Source A at time t₁ and in Source B at time t₂, the lag (t₂ - t₁) and the direction of propagation carry information about the information ecosystem's structure. Institutional sources (RSS) may lag social media; but in some cases, institutional sources break stories that propagate to social media. The direction and latency of propagation are themselves findings.
- **Narrative contagion curves.** Shiller's narrative economics framework tracks the adoption of narratives as epidemic curves (S-shaped adoption, exponential decay). Fitting these curves requires high-resolution temporal data during the growth phase and lower-resolution data for the steady-state and decay phases.

---

## 7. Open Questions for Interdisciplinary Collaborators

### 7.1 For Time Series Analysts and Statisticians

**Q1: What temporal decomposition method is most appropriate for AĒR's discourse time series?**

- Classical decomposition, STL, wavelet analysis, or a combination? AĒR's time series have irregular sampling (RSS feeds do not publish at fixed intervals), potential structural breaks (algorithm changes, new sources added), and multiple overlapping periodicities (daily, weekly, seasonal).
- Deliverable: A comparative evaluation of decomposition methods on AĒR's Probe 0 data, with recommendations for each temporal scale.

**Q2: How should AĒR compute change points in discourse metrics?**

- Which change point detection algorithm (CUSUM, PELT, Bayesian) is appropriate for AĒR's data characteristics (noisy, irregularly sampled, potentially non-stationary)? How should change point significance be assessed — frequentist (p-values) or Bayesian (posterior probability)?
- Deliverable: A change point detection pipeline evaluated on Probe 0 data with known events as ground truth.

**Q3: What is the minimum meaningful aggregation window for each metric-source pair in AĒR's current pipeline?**

- Using Probe 0 data (Tagesschau and Bundesregierung RSS), compute the minimum aggregation window at which sentiment, entity count, and word count metrics stabilize (variance of the mean drops below a threshold).
- Deliverable: Empirical minimum windows for each current metric, with the statistical method documented for replication on future probes.

### 7.2 For Communication Scientists and Media Scholars

**Q4: How do news cycles differ across cultures, and how should AĒR account for these differences?**

- The 24-hour news cycle is not universal. How do news publication rhythms differ between the media cultures AĒR will observe? What are the key temporal structures (edition cycles, news seasons, attention decay patterns)?
- Deliverable: A comparative media temporality profile for the cultural regions identified in WP-003 Q6.

**Q5: How should AĒR detect and distinguish genuine discourse shifts from seasonal and cyclical effects?**

- A sentiment dip in August may reflect the German *Sommerloch*, not a genuine discourse shift. How should the system distinguish cyclical effects from structural changes?
- Deliverable: A catalog of culturally specific temporal patterns (holidays, media seasons, political cycles) per region, formatted as machine-readable calendar metadata for integration into the dashboard.

### 7.3 For Computational Social Scientists

**Q6: At what temporal resolution do topic model outputs become stable?**

- LDA and BERTopic are sensitive to corpus size. If AĒR runs topic models on daily corpora, weekly corpora, and monthly corpora from the same data, how do the discovered topics differ? At what corpus size do topics stabilize?
- Deliverable: A corpus-size sensitivity analysis for LDA and BERTopic on German news text, with recommendations for minimum corpus size per temporal window.

**Q7: How should AĒR model cross-source propagation dynamics?**

- Granger causality, transfer entropy, and cross-correlation are standard methods for detecting temporal lead-lag relationships between time series. Which method is appropriate for discourse propagation analysis, given AĒR's data characteristics?
- Deliverable: A method evaluation for cross-source propagation detection, tested on known propagation events in Probe 0 data.

### 7.4 For Digital Humanities Scholars

**Q8: How should AĒR operationalize Foucault's epistemic shifts in temporal metrics?**

- The Episteme pillar aspires to measure shifts in the "boundaries of the expressible." This is a long-term, qualitative concept. Can it be operationalized as a quantitative temporal metric (vocabulary emergence rate, semantic shift velocity, Overton Window width), or does it fundamentally resist quantification?
- Deliverable: A position paper on the operationalizability of epistemic change, with specific proposals for temporal metrics that approximate the concept without reducing it.

---

## 8. References

- Cleveland, R. B., Cleveland, W. S., McRae, J. E. & Terpenning, I. (1990). "STL: A Seasonal-Trend Decomposition Procedure Based on Loess." *Journal of Official Statistics*, 6(1), 3–73.
- Hamilton, W. L., Leskovec, J. & Jurafsky, D. (2016). "Diachronic Word Embeddings Reveal Statistical Laws of Semantic Change." *Proceedings of ACL 2016*.
- McCombs, M. (2004). *Setting the Agenda: The Mass Media and Public Opinion*. Polity.
- Shiller, R. J. (2020). *Narrative Economics: How Stories Go Viral and Drive Major Economic Events*. Princeton University Press.

---

## Appendix A: Mapping to AĒR Open Research Questions (§13.6)

| §13.6 Question | WP-005 Section | Status |
| :--- | :--- | :--- |
| 5. Temporal Granularity | §2–§6 (full treatment) | Addressed — multi-scale framework proposed |
| 4. Cross-Cultural Comparability | §4 (cultural temporalities) | Cross-reference to WP-004 |
| 3. Metric Validity | §3 (noise and signal separation) | Cross-reference to WP-002 |

## Appendix B: Temporal Scale Reference

| Scale | Phenomena | Resolution | AĒR Pillar | Example Metric |
| :--- | :--- | :--- | :--- | :--- |
| 1 (min–hours) | Event response | 5min–hourly | Rhizome | Publication frequency spike, entity emergence |
| 2 (hours–days) | News cycle | Hourly–daily | Aleph | Topic prevalence, entity salience |
| 3 (days–weeks) | Agenda dynamics | Daily–weekly | Aleph / Episteme | Topic persistence, agenda diversity |
| 4 (weeks–months) | Policy discourse | Weekly–monthly | Episteme | Sentiment trends, narrative frame evolution |
| 5 (months–years) | Cultural drift | Monthly–quarterly | Episteme | Vocabulary emergence, semantic shift, baseline change |
| Cross-scale | Propagation | Hourly (within event) + weekly (structural) | Rhizome | Cross-source lag, narrative contagion curves |