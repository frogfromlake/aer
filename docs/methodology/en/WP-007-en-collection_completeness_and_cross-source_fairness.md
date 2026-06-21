# WP-007: Collection Completeness and Cross-Source Fairness

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft — open for interdisciplinary review
> **Date:** 2026-06-22
> **Depends on:** WP-001 (functional probe catalog), WP-004 (cross-cultural comparability), WP-006 (reflexivity & ethics)
> **Architectural context:** ADR-028 (single configurable web-crawler), ADR-031 (DiscoveryProtocol for multi-channel source discovery), ADR-039 (Negative Space), Manifesto §"unaltered mirror", Quality Goal 1 (Scientific Integrity)
> **License:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Objective

AĒR compares discourse *across* sources, languages, and cultures. Every such comparison rests on a silent precondition: that the corpus AĒR holds for each source is a **faithful and comparable sample of what that source actually published**. This working paper makes that precondition explicit, shows why it is fragile, and defines the model — completeness measurement, fairness auditing, and disclosure — that keeps it honest as the system scales from two probes to hundreds.

The motivating observation is concrete. In a controlled single-window crawl (2026-06-21), the French public broadcaster `franceinfo` yielded ≈ 1.35× the document volume of the German flagship `tagesschau`. That ratio is either a **real fact about the discourse landscape** (a large national multimedia newsroom genuinely publishes more than a focused flagship brand) or an **artifact of how AĒR crawls** (we collect one source more completely than the other, or over-collect non-articles from one). The two are indistinguishable from the number alone — and they have opposite epistemic status. The first is signal worth surfacing; the second is bias that corrupts every downstream comparison while *looking* like signal.

The question this paper answers is therefore not "how do we crawl everything?" but: **how does AĒR prove that a measured difference between sources is the world, not the instrument?**

---

## 2. The Validity Problem: Throughput as Signal or Artifact

A corpus-size or throughput difference between two sources can arise from four distinct causes, only one of which is a valid discourse signal:

1. **Genuine publication rate** — the source really does publish more. *Valid signal.*
2. **Under-collection of the quieter source** — a broken or undeclared discovery channel, a pagination cutoff, an over-aggressive URL/content filter, a deduplication false-positive, or robots/throttle losses make a source look smaller than it is. *Artifact.*
3. **Over-collection of the louder source** — an under-aggressive filter, non-article pages (live-blogs, photo galleries, media stubs, listing pages), locale-mirror leakage, or duplicate counting inflate a source. *Artifact.*
4. **Asymmetric collection method** — one source exposes a deep archive walker, another only a shallow news feed; the difference is real but reflects *publisher infrastructure*, not discourse volume. *Partially valid, must be disclosed as a parameter.*

Causes 2 and 3 are the dangerous pair: they are **silent** (nothing errors), they **masquerade as signal** (a smaller corpus reads as a quieter discourse), and they **compound at scale** (every probe added without a completeness check adds another potential asymmetry). They are the collection-side analogue of the selection bias WP-006 §3 and WP-001 warn about, but operating one layer earlier — not "which sources did we pick?" but "did we faithfully capture the sources we picked?"

The franceinfo/tagesschau case is instructive precisely because the gap *narrowed* under controlled conditions (an earlier backfill suggested ≈ 2.3:1; a fair single-window crawl showed ≈ 1.35:1). The difference between those two ratios was not discourse — it was accumulation asymmetry in the collection process. Without a completeness model, that artifactual 0.95× would have been read as signal.

---

## 3. The Impossibility of Literal Completeness — and What Replaces It

It is tempting to set the goal as "crawl 100 % of every source." This goal is **incoherent**, and pursuing it directly produces worse science, for three reasons:

1. **No ground truth exists.** A publisher does not expose a verifiable count of "every article we have ever published." Sitemaps are partial and lag; RSS feeds are windowed; archives paginate inconsistently. There is no denominator against which 100 % could be proven.
2. **Completeness is time-bounded and channel-bounded.** "Complete" only has meaning relative to a period (the rolling window) and the channels a publisher actually exposes. A source that publishes only via a feed we cannot reach is not "incompletely crawled" — it is *structurally unobservable*, which is a different (and disclosable) fact.
3. **Chasing 100 % invites adversarial crawling.** Circumventing bot-walls, ignoring robots.txt, or scraping rendered SPAs to "get everything" violates the Manifesto's non-adversarial, polite-by-default posture (ADR-028). Completeness must never be bought with politeness.

AĒR therefore replaces *literal* completeness with **measured, reconciled, disclosed completeness**:

> A completeness claim is a **ratio with a denominator**, monitored for **drift**, and **disclosed** wherever the data is shown. A source is never asserted to be "fully crawled"; it is reported as "N % complete for this period against its declared channels," with the un-captured remainder named as Negative Space.

This reframing converts an unprovable absolute into a measurable, falsifiable, honest quantity — exactly the move WP-002 makes for metric validity and WP-006 makes for reflexive provenance.

---

## 4. A Four-Layer Model

Completeness and fairness are not one question but four, operating at different layers. Conflating them is the source of most confusion (e.g. the false intuition that "fair" means "equal corpus size").

### 4.1 Layer 1 — Completeness (vertical: did we capture all of *this* source's output?)

The vertical question, per source, per channel, per period: `completeness = collected / declared`.

The **declared denominator** is the publisher's own statement of what it offered in the period — counted *before* AĒR's filters: the number of sitemap entries whose `<lastmod>` falls in the window, the number of RSS items in the window, the pagination depth of an archive index. This is distinct from what AĒR currently records. Today the crawler logs `urls_discovered` (the count *after* the temporal-window filter is applied inside each channel) and `urls_after_dedup`, and compares them against a **hand-set `expected_floor_per_run`** heuristic (ADR-031, `crawler_discovery_runs` / `crawler_discovery_alerts`). That heuristic catches gross underflow but cannot express a completeness *ratio*, because it has no measured denominator — only a guess.

The fix is to **measure the declared denominator** as a first-class telemetry field, turning the existing two-consecutive-underflow alarm into a drift detector against a real number rather than an estimate.

### 4.2 Layer 2 — Fairness (horizontal: are the *rules* symmetric across sources?)

Fairness is the most misunderstood layer. **Fairness is not equal corpus size.** Forcing two sources to equal volume would *falsify reality* — it would erase the genuine-publication-rate signal (cause 1) that AĒR exists to capture. That is the opposite of an unaltered mirror.

Fairness is **symmetry of method**: every source is subject to the same *classes* of inclusion rule (the same technical-only filtering per WP-006 §3, the same completeness standard, the same disclosure obligation), and any **asymmetry of collection infrastructure** (Layer-1 cause 4) is documented and assessed for its bias direction rather than hidden. tagesschau's date-indexed archive walker, franceinfo's news sitemap, and the Élysée's RSS-only surface are not a fairness violation — but the asymmetry is a *recorded structural-bias parameter* (already partially captured in the Probe-0 `bias_assessment.md` Structural Bias #8), not an invisible one. A fairness audit asks one question: **does any source silently receive a stricter or looser rule than its peers?**

### 4.3 Layer 3 — Over-collection (purity: is every collected document a real article?)

The mirror image of Layer 1. A source whose volume is inflated by non-articles — live-blog stubs, photo/video pages, listing/index pages, locale-mirror duplicates — is as biased as one with gaps, in the opposite direction. The signal is the per-source **extraction-success rate** and **non-article rate**. (Concretely: the 2026-06-21 validation found tagesschau pages reaching Gold with a body `word_count` of 4–40, because the crawler's `min_word_count` filter operates on *raw-HTML* word count — a 404/stub guard — while no article-body length floor exists at the Silver boundary.)

The governing invariant is **DISCLOSE-NEVER-COERCE** (ADR-039): thin or non-article content is either filtered *with* a disclosed rule or surfaced as Negative Space — it is **never silently dropped** (which would hide a collection gap) and **never silently counted** (which would inflate volume).

### 4.4 Layer 4 — Comparison-layer robustness (already built — named here for completeness)

Even a perfectly measured, genuinely real size difference must never be compared naively. AĒR already enforces this at the analysis layer: z-score / percentile normalisation, the cross-cultural equivalence gate (WP-004), the merged-vs-split composition refusal for scaled/intensive metrics, and k-anonymity. Layers 1–3 *feed* Layer 4: a completeness ratio tells the comparison layer **how much to trust each corpus**. A cross-source overlay drawn over two corpora of 60 % and 98 % completeness is making a claim its data cannot support — and the completeness number is what lets a reader (or a future guard) see that.

---

## 5. The Reconciliation Mechanism

The operational core is a per-source, per-channel, per-run **funnel**, every stage of which is counted and attributable:

```
declared (publisher inventory in window)
   └─▶ discovered (channel surfaced it, in window)
          └─▶ after_dedup (unique across channels)
                 └─▶ passed_filters (url_filter + content_filter)
                        └─▶ fetched (HTTP success, not 304-unchanged)
                               └─▶ extracted (trafilatura body ≥ floor)
                                      └─▶ Gold (reached the analytical store)
```

`completeness = collected / declared` is the top-to-bottom ratio; the *drop at each stage* is the diagnostic. The 2026-06-21 funnel asymmetry — franceinfo `discovered 526 → collected 395` (75 %) versus tagesschau `284 → 282` (99 %) — is exactly the kind of signal this makes legible: the franceinfo drop is *hypothesised* to be legitimate media-filtering (`/replay-radio/`, `/replay-jt/`) but **must be confirmed channel-by-channel**, because an unexplained 24-point conversion gap between two peer EA sources is indistinguishable from a filtering bias until it is attributed.

Two design constraints follow from the rest of the AĒR architecture:

- **The denominator is measured politely or not at all.** Counting declared inventory uses the channels the publisher already exposes (sitemap entry counts, feed lengths) — never additional adversarial probing. Where a true denominator is unobtainable, completeness is reported as *indeterminate* (Negative Space), not assumed to be 100 %.
- **Drift, not absolute thresholds, triggers alarms.** A source's completeness is compared against its own rolling baseline (the existing two-consecutive-runs semantics in `crawler_discovery_alerts`), so a publisher's genuine quiet week does not fire a false alarm, but a channel that breaks does.

---

## 6. The Onboarding Contract (Extensibility)

The single greatest risk is not the two current probes — it is the *hundredth* source added by a future operator who does not hold the whole bias model in their head. Completeness and fairness must therefore be **structural properties of the act of adding a source**, not a manual audit performed once and forgotten. This is the load-bearing requirement of this paper.

Every new source and probe carries a **completeness contract** from day one:

1. **Audit captures the denominator.** `aer-audit-source` (ADR-031) already inventories a publisher's discovery channels; it is extended to also record the **declared-inventory baseline** per channel, so the per-source floor is a *measured* starting point rather than a hand-typed `expected_floor_per_run` guess.
2. **`add-a-source.md` gains a completeness step.** The onboarding checklist requires: declare the channels the publisher exposes; record the declared-inventory baseline; set the drift threshold; and **document any collection-method asymmetry versus the probe's existing peer sources** (Layer 2) in the probe dossier.
3. **`add-a-probe.md` honesty pass checks symmetry.** Step G ("Honesty pass") gains an explicit completeness/fairness review: are this probe's sources collected under symmetric rules, and are their asymmetries documented and assessed?

The cost discipline of WP-006 §3 still governs: the contract adds *measurement and disclosure*, never editorial selection. It does not tell an operator which sources to pick — it ensures that whatever they pick is collected faithfully and comparably.

---

## 7. Disclosure as Negative Space

Completeness is only meaningful if it travels *with* the data into every view (ADR-039: "what AĒR does not see is a first-class surface"). The disclosure extends existing surfaces rather than inventing new ones:

- **The per-source coverage panel** (`DiscoveryCoveragePanel`, already consuming `GET /api/v1/sources/{id}/discovery-coverage`) gains the completeness ratio and the funnel breakdown alongside today's observed-vs-floor view.
- **A new Negative-Space class — `collection-completeness`** — joins the six existing classes in the taxonomy, surfaced by the single `NegativeSpaceBadge` token: "this source's corpus is N % complete for the selected period." Because the badge rides with the data, a reader cannot place two corpora side by side without seeing the completeness of each — which is the entire point.

This closes the reflexive loop WP-006 demands: AĒR does not merely *try* to be complete; it **measures and shows how complete it actually is**, and names the remainder it cannot see.

---

## 8. Non-Goals (What This Paper Explicitly Rejects)

- **Equalising corpus sizes.** Rejected — it falsifies the genuine-publication-rate signal (§4.2).
- **Pursuing literal 100 % completeness.** Rejected as incoherent and adversarial-inviting (§3).
- **Editorial gating to "balance" sources.** Rejected per WP-006 §3 — completeness adds measurement, never selection.
- **Silently dropping thin/non-article content.** Rejected per DISCLOSE-NEVER-COERCE — it must be disclosed or surfaced (§4.3).
- **Treating a structurally-unobservable source as "0 % crawled".** Rejected — unobservability (e.g. a bot-walled portal, ADR-028) is a distinct, disclosed fact, not a completeness failure.

---

## 9. Open Questions for Interdisciplinary Collaborators

1. **For survey methodologists and statisticians.** Is there a principled way to estimate a declared denominator's own incompleteness (the publisher's sitemap is itself a partial frame)? What is the right confidence representation for a completeness ratio whose denominator is uncertain?
2. **For computational social scientists.** At what completeness asymmetry does a cross-source comparison become invalid rather than merely caveated? Is there a defensible threshold that should *block* (not just annotate) a comparison?
3. **For STS scholars (continuing WP-006).** Does publishing per-source completeness create a new reactivity — could a publisher game its "completeness score" by manipulating its sitemap? Is completeness disclosure itself performative?
4. **For information-design researchers.** How is a per-source completeness figure shown so it informs interpretation without becoming a spurious "data-quality leaderboard" that re-introduces the ranking bias AĒR avoids elsewhere?

---

## 10. Implementation

The engineering that operationalises this paper — crawler declared-denominator capture and reconciliation, the BFF completeness field, the dashboard panel and `collection-completeness` Negative-Space class, and the onboarding-contract changes to `add-a-source.md` / `add-a-probe.md` / `aer-audit-source` — is tracked as a dedicated ROADMAP phase. Most of it is **pre-deployment** work: it is a data-quality foundation, and because it shapes how every future source is added, it must exist *before* the probe catalogue scales, not after.

---

*WP-007 extends the AĒR methodology series from "how should we observe" (WP-001–005) and "what happens because we observe" (WP-006) to a third reflexive question: **how faithfully did we actually capture what we set out to observe — and how do we prove it?***
