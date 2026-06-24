# Workbench "Methodik" — Content Inventory, Quality Standard & Authoring Worklist

**Status:** EXECUTED as Phase 148g (2026-06-24). This file is now the record of what shipped;
the original audit groundwork is preserved below it.
**Produced:** 2026-06-24 from a 4-agent read-only audit (view_modes · metrics · fields · target-rubric).

---

## 0. Phase 148g outcome (2026-06-24) — what shipped

> **Durable records:** the architectural decision is **ADR-045** (`docs/arc42/09_architecture_decisions.md`);
> the how-to-author guide is `docs/extending/add-an-extractor.md` §"Dashboard reading content";
> the Content-Catalog `field` entityType is noted in arc42 §8.13. This file is the **phase outcome
> record + the pending orphaned-pairing decision** — not the authoring guide.


A material scope correction landed during execution. The per-(view×metric) deep prose is only
ever fetched by `MeasureDetail` when the presentation has `usesMetric: true` — i.e. **only
`distribution` and `time_series`**. The other "metric-bearing" views (`topic_distribution`,
`topic_evolution`, `cooccurrence_network`) are `usesMetric: false`, so their existing
`<view>_<metric>.yaml` pairing files are **orphaned** (never surfaced since the 148f
`panelSubjectKind` gating). They do not 404; they are simply dead. **Operator decision pending:**
delete the ~40 orphaned `topic_*`/`cooccurrence_*` pairing files, or re-wire a surface for them.
The co-occurrence substance (what "node colour = sentiment" means) was **rescued** into
`howto_cooccurrence_network.yaml` so it is not lost.

**The bigger correction:** the multivariate / channel-driven views (scatter, correlation,
cross-tab, parallel-coordinates, lead-lag, co-occurrence) bind real metrics AND fields through
channels / metric-sets / field-chains / facets — and NONE of their per-subject methodology was
surfaced (only the view-level howto). Phase 148g added a **per-subject methodology surface**:

- `src/lib/presentations/cell-subjects.ts` — pure `cellSubjects(presentation, panel)` enumerates
  every bound metric/field across all slots (channels x/y/size/colour, metricSet, fieldChain,
  crossMetric, netMetric/netColorMetric, facetField) with a role label.
- `SubjectMethodology.svelte` — renders one methodology block per bound subject (metric: dual
  register + provenance + limitations + pairing; field: `content/field/<field>` dual register).
- `MeasureDetail.svelte` rewritten to render the view howto once + one block per subject.
- Result: "every presentation × every metric × every field" is now literally surfaced.

**Shipped content:**
- **Field methodology = Option A.** New `field` content-entityType (openapi enum + codegen +
  `validEntityTypes` + handler). **44 field YAML** (`content/{en,de}/fields/<field>.yaml`),
  gold-standard, bilingual, all anchors verified.
- **8 P0 pairing files** — `{distribution,time_series}×{paywall_status,image_count}×{en,de}` —
  closes the real 404 set (NOT 20 files; only these two views surface pairings).
- **Howto overhaul** — the 5 weak howtos deepened; all 6 multivariate howtos rewritten to
  explain channel/set/facet metric semantics + quantitative limits; co-occurrence rescue.
- **Re-review** — all 10 metric files + live distribution/time_series pairings swept for
  machine-ids-in-prose, calque German, en/de asymmetry; fixed a copy-paste bug (sentiment-class
  composition note inside non-sentiment metrics) + a doubled-"bert" typo + de "Brief"→"Design-Brief".
- **Root bug fixed** — `reconcilePanelForView` snaps a stranded categorical field in `Panel.metric`
  to a real metric when entering a single-metric view (registry = the clean separator).

**Deferred (operator-approved):** the 3 stub metrics `reading_time_minutes`, `comment_count`,
`external_citation_count` — provenance-only, 0% populated, NOT in `/metrics/available` (so not
selectable, no live surface). Author when the Phase-133 extractors populate them.

---
**Operator bar:** the methodology section must be COMPLETE and TOP QUALITY for every presentation × metric × field, **fully bilingual (en + de)**, scientific **but understandable**, no machine IDs, no stubs, no force-translated German. **Even the current "best" entries are considered _ausbaufähig_** — this inventory grades against an aspirational standard, not the present best. Interdisciplinary refinement comes later; the baseline must already be excellent.

This file is the durable groundwork. Re-run the audit agents if the metric/field set changes.

---

## 1. Target quality standard (the rubric)

Grade each text 1–5 on every dimension; target 4–5. Level 3 is passable-but-_ausbaufähig_.

1. **Scientific accuracy & WP anchoring** — claims grounded in `docs/methodology/en/` WPs + ADRs, cited to the **section** (`WP-002 §3.2`, not `WP-002`). 5 extends into cross-metric signal + exact governing ADR.
2. **Honest provisional/uncertainty disclosure** — Tier classification + named specific limitations + _why each matters_ (e.g. "Twitter-trained, institutionally applied → the gap is a research signal, not a defect"). Never present a provisional metric as validated.
3. **Plain-language jargon explanation** — every technical term glossed once at first use; abstract concepts grounded with a concrete image. Accessible to a researcher who is not an NLP specialist.
4. **Bilingual symmetry (en ↔ de)** — both fully fluent + semantically equivalent. Keep technical terms (z-Score, BERT, SentiWS, Pearson r, Lead-Lag, Bucket, Provenienz); everything else idiomatic German, **no calques**. `„German curly quotes"`. **"Sonde"** for probe (never "Probe"). Symmetric disclosure of every caveat.
5. **Register structure** — `semantic` (what it means for a reader) vs `methodological` (algorithm/Tier/anchor) genuinely differentiated, not duplicated; `short` is a 10-second aperture, `long` expands.
6. **Length & composability** — semantic ≈100–150 w, methodological ≈150–250 w; scannable; composition (merged/split) in its own sub-paragraph.

**Auto-disqualifiers:** machine IDs / registry paths / unglossed entityIds in body text · stub/placeholder text · calque German · asymmetric en/de uncertainty disclosure.

---

## 2. Authoring templates (YAML skeletons)

The gold-standard schema (from the strongest existing entries — `sentiment_score_sentiws`, `sentiment_score_bert_multilingual`, `howto_topic_distribution`, `howto_revision_edit_clusters`):

```yaml
entityId: <id>            # machine id — NEVER surfaced in body prose
entityType: metric | view_mode
locale: en | de
# displayLabel only on metric entries
registers:
  semantic:        { short: "<≤20 w aperture>", long: "<150–200 w, reader-facing>" }
  methodological:  { short: "<≤20 w: algo · version · Tier · key caveat · anchor>", long: "<200–300 w: method → hardening → limitations → composition>" }
contentVersion: "vYYYY-MM-a"
lastReviewedBy: "<name/team>"
lastReviewedDate: "YYYY-MM-DD"
workingPaperAnchors: ["WP-XXX §Y", "ADR-NNN"]
```

Three shapes: **metric** (`content/<loc>/metrics/<metric>.yaml`), **howto_<presentation>** (`content/<loc>/view_modes/`), **<presentation>_<metric>** (`content/<loc>/view_modes/`). Full per-shape skeletons + voice notes are in the audit appendix (Agent D) — reproduce them here verbatim before authoring.

**Voice:** methodologist to peer, honest about uncertainty (a feature). Semantic opens with the 10-second answer; methodological leads with the operational fact, then its limits.

---

## 3. Inventory matrix

### 3a. Presentations — `howto_<presentation>` (per-view methodology)

**17/17 exist, en+de, all 4 registers.** WP anchors 1–5 (median 2). Agent grades (against their own bar; treat as RELATIVE, re-grade against §1):

- **Strongest (de-facto reference):** `howto_topic_distribution`, `howto_revision_discourse_shift`, `howto_revision_edit_clusters`, `howto_cross_probe_lead_lag`.
- **Weakest → overhaul first:** `howto_sankey`, `howto_metric_scatter`, `howto_revision_activity`, `howto_distribution`, `howto_revision_timeline` — thin methodological register, only 1 WP anchor, missing quantitative limits (bin clamp, point cap, capture-frequency variance, truncation-disclosure rationale).

### 3b. Per-(view × metric) deep prose — `<presentation>_<metric>`

Only **single-metric-binding views** carry these. Current coverage (rows = view, cols = the 8 documented metrics):

| view | entity_count | language_confidence | publication_hour | publication_weekday | sentiment_sentiws | sentiment_bert_multi | sentiment_bert_de | word_count |
|---|:--:|:--:|:--:|:--:|:--:|:--:|:--:|:--:|
| time_series | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| distribution | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| topic_distribution | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| topic_evolution | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| cooccurrence_network | ✓ | ✓ | — | — | ✓ | — | — | ✓ |

**GAP (P1):** these views carry per-metric prose for **only 8 metrics** — the two metadata-promoted metrics **`paywall_status` and `image_count` are MISSING** for every view (e.g. `distribution_image_count.yaml` does not exist → `hasCellMethodologyContent` fetch 404s for those pairings). When the 3 stub metrics ship (below), they too will need per-(view×metric) coverage.
The other 9 presentations are **metric-less by design** (field-driven / metric-set / self-sourced) and correctly have no per-metric files — they rely on `howto_<presentation>` alone (now surfaced everywhere by the 148f fix).

### 3c. Metrics — content (`metrics/<m>.yaml`) + provenance

**14 in `metric_provenance.yaml`.** Reconciliation (extractors ∩ provenance ∩ `/metrics/available`):

| metric | extractor | provenance | content en+de | grade | note |
|---|:--:|:--:|:--:|:--:|---|
| sentiment_score_sentiws | ✓ | ✓ | ✓ | 5 | reference quality |
| sentiment_score_bert_multilingual | ✓ | ✓ | ✓ | 5 | reference quality |
| sentiment_score_bert_de_news | ✓ | ✓ | ✓ | 5 | reference quality |
| publication_hour | ✓ | ✓ | ✓ | 4 | UTC caveat exemplary |
| publication_weekday | ✓ | ✓ | ✓ | 4 | |
| word_count | ✓ | ✓ | ✓ | 4 | |
| entity_count | ✓ | ✓ | ✓ | 4 | |
| language_confidence | ✓ | ✓ | ✓ | 4 | |
| paywall_status | — (metadata, Phase 133) | ✓ | ✓ | 4 | **add schema.org `isAccessibleForFree` example; "Brief" untranslated in de** |
| image_count | — (metadata, Phase 133) | ✓ | ✓ | 4 | **add JSON-LD `image[]` example** |
| revision_count | — (corpus-level) | ✓ | ✗ | n/a | deliberately no content (article_revisions table, not in `/metrics/available`) |
| reading_time_minutes | ✗ (Phase 133 pending) | ✓ | ✗ | **stub** | provenance-only; 0% populated |
| comment_count | ✗ (Phase 133 pending) | ✓ | ✗ | **stub** | provenance-only; 0% populated |
| external_citation_count | ✗ (Phase 133 pending) | ✓ | ✗ | **stub** | provenance-only; 0% populated |

No en/de asymmetries in existing content. No orphaned metrics (every extractor metric has provenance).

### 3d. Fields — 22, all populated, **no deep methodology**

All 22 are crawler-populated, in `FIELD_LABELS` + `metadata_field_desc_*` (en+de) + `/metadata/available`. **There is NO `field` content-entityType.** Descriptions are one-liners grading **2–3/5**: they say WHAT a field is but rarely HOW it is extracted (priority chain), what its ABSENCE means structurally (WP-003 §3.2 authorship model), or its ANALYTICAL use. Weakest: `author`, `image_url`, `word_count`, `comment_url`, `paywall_status`, `social_share_counts`.

---

## 4. Decision needed — field methodology mechanism

To bring fields to the gold standard, the audit recommends **Option A: add a `field` content-entityType** (mirror `metric`): 44 YAML files (`content/{en,de}/fields/<field>.yaml`), touch `internal/config/content_catalog.go` (whitelist `field`), `internal/handler/content_handler.go`, regen. Gains dual-register + WP anchors + CI parity + versioning; `MeasureDetail` then fetches `content/field/<field>` for field subjects. Option B (enrich Paraglide `metadata_field_desc_*`) is lighter but misses WP anchoring/registers/governance. **Operator must choose A vs B before field authoring.**

---

## 5. Prioritized worklist

- **P0 — coverage holes that 404 or read as broken:**
  - Add per-(view×metric) prose for `paywall_status` + `image_count` across the 4 (+cooccurrence) metric-bearing views (§3b gap).
  - Field methodology mechanism (§4 decision) → then author all 22 (or enrich descriptions).
- **P1 — quality overhaul against §1 (everything is ausbaufähig):**
  - The 5 weak `howto_*` (sankey, metric_scatter, revision_activity, distribution, revision_timeline): deepen methodological register, add quantitative limits, +WP anchors.
  - `image_count` / `paywall_status` metric content: schema.org examples; fix de "Brief" untranslated.
  - Re-review ALL existing metric + howto + pairing content against the rubric (even the 5-graded ones) for the higher bar.
- **P2 — decide & document the stub metrics:** `reading_time_minutes`, `comment_count`, `external_citation_count` — author content now (prophylactic) or defer until the Phase-133 extractors actually populate them (they currently produce no data, so no live methodology surface).
- **Cross-cutting:** the **field-as-metric root bug** (scatter→distribution keeps a categorical field as `Panel.metric`) — make `metricSupportsPresentation` refuse a pure categorical field as a scalar-metric dimension (the `/metrics/available` registry is the clean separator; mind the word_count/comment_count field∩metric overlap). 148f already no-ops the fetch (`isRegisteredMetric`), so this is correctness, not a 404.

---

## 6. After every authoring change

`pnpm exec svelte-check --threshold error` · `eslint` · `prettier --check` · `pnpm exec vitest run` · de=en i18n parity. **Restart the BFF** (it loads this content into its map at startup). Never auto-commit — manual one-line command for the operator. Answer in German when the operator writes German.
