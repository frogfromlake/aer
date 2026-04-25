# Dashboard Reframing Note — Iteration 5 Preparation

**Status:** Draft synthesis. Not authoritative.
**Purpose:** Capture the shift in mental model surfaced by the 2026-04-24 review so that
`design_brief.md`, `design_system.md`, and ADR-020 can be rewritten (Step 2) against
a clear target. This file is a working artifact — it is expected to be merged into the
rewritten brief and deleted once Step 2 completes.
**Scope:** What the dashboard *is* — not how it is built.

---

## 0. Why this note exists

Phase 100a landed Surface I descent (L0–L4 attached to the 3D globe). Reviewing the
implementation against the methodological foundation (WP-001 through WP-006, Manifesto,
Arc42 §1) revealed a gap between the brief's prose and what the implementation actually
delivers. The layers on top of the globe read as extended tooltips. The scientific
surfaces where real analysis should live — Surface II (Function Lanes) and Surface III
(Reflection) — have not been built. The globe is carrying too much conceptual weight
for what it can actually show. Navigation between the parts of the dashboard is
implicit where it should be obvious, and methodological context hides behind
micro-buttons where it should be front and centre.

This note identifies what changes, what stays, and what must be added so that the
subsequent rewrite has a clear target.

---

## 1. What AĒR's dashboard is, restated

The dashboard exists to **present Gold-layer NLP metrics — and, on eligible sources,
Silver-layer unified raw data — through multiple interactive view modes drawn from a
principled analytical catalog, with methodological transparency as a first-class UI
concern, so that scientists and engaged laypeople can explore how digital discourse
behaves and verify every finding down to the individual source document.**

Said differently: AĒR's backend produces a growing, interdisciplinarily-grounded dataset
of metrics and entities over a growing probe set. The dashboard's job is to make that
dataset **legible, navigable, questionable, and verifiable** — with the lens
configuration (WP-001 through WP-006) always accessible alongside the numbers.

The dashboard is not a landing page with drill-downs. It is **an interactive research
instrument** whose landing page is a lightweight overview.

---

## 2. The core shift: demoting the globe

Current brief posture (§3.1, §3, Appendix A): the Atmosphere surface is "the default
entry," "first contact," "the layer where the user breathes." It is presented as
co-equal with Surfaces II and III.

New posture: **the globe is the landing overview.** Its job is narrow and specific:

- Show what AĒR currently observes (active probes) and what it does not (coverage
  gaps) — the Probe Coverage Map from WP-001 §5.3 made visible.
- Show the shape of the dataset: probe count, source count, article count, active
  time range, dataset age. A researcher or visitor should form a correct first
  impression of the instrument's scope within seconds.
- Invite descent into the real scientific surfaces — but not try to host them.

Everything that is not overview-level belongs off the globe. Function-stratified
analysis, cross-source comparison, metric exploration, article-level verification,
methodology reading — all of this happens on dedicated surfaces with real UI space,
not in companion panels that crowd the globe.

Consequence for the descent: on Surface I, L0–L2 are globe-native (Immersion,
Orientation, Exploration of coverage / time range / probe selection). **L3 (Analysis),
L4 (Provenance), and L5 (Evidence) are not globe-native.** Entering them transitions
*off* the globe into the scientific surfaces, rather than opening a side panel that
obscures it. The globe disappears entirely during descent to free the full viewport for
the scientific work; a persistent, visible return path (see §2.1) lets the user come
back to it from anywhere.

This is consistent with WP-005 §6: the Aleph pillar (synchronic, "the weather now") is
globe-dominant. Episteme (diachronic, long trends) and Rhizome (propagation, relational)
are not spatial phenomena and do not belong on a globe.

### 2.1 Navigation is first-class

A related observation from the 2026-04-24 review: in the current implementation,
reaching L4 provenance requires noticing and clicking a small button inside the L3
panel. This is the wrong shape for a research instrument. Methodological context —
the tier, the algorithm, the limitations, the provenance — is not a secondary concern
reachable only by those who already know to look for it. It is **the thing that
distinguishes AĒR from every commercial dashboard.** It must be structurally visible
at all times.

The rewrite must specify a primary navigation that is:

- **Persistent.** The user can see, at every moment and on every surface, where they
  are (Atmosphere / Function Lanes / Reflection) and how to move between them.
- **Obvious.** A research instrument's chrome is not hidden in corners. Navigation is
  a named, first-class UI element — not an icon that users may never notice.
- **Bidirectional.** From any depth the user can jump laterally (to a different
  surface on the same scope) or vertically (to a different layer within the same
  surface). No dead ends.
- **Scope-carrying.** The current probe selection, time range, and viewing mode
  travel with the user through navigation. Switching surfaces does not reset context.
- **Return-aware.** Returning to the Atmosphere overview — to pick a different
  probe, to reset context, to re-orient — is available from every view, not only
  from "home."

This is the interaction-architecture counterpart to §2's globe demotion. Demoting the
globe only works if the path *off* the globe, *between* the scientific surfaces, and
*back* is always visible. The same principle applies to the methodology-data connection
described in §3.3 — the affordance to open methodological context is prominent, not
discoverable only by accident.

### 2.2 The probe concept is introduced in context, not assumed

Surface I's move from "scientific entry point" to "landing overview" exposes a
related risk: **the word "probe" carries no meaning for a first-time visitor.** A
researcher steeped in WP-001 reads the glyph and understands; anyone else sees a
labelled dot and has no frame for it. If the globe is the first surface a user
meets, the probe concept must be introduced *there*, in context — not left as an
implicit prerequisite.

The mechanism is already in the brief: **Progressive Semantics (§5.7).** Every
probe glyph on the globe carries both registers simultaneously:

- **Semantic register (prominent).** Plain-language identity — *"German
  institutional news: Tagesschau and Bundesregierung."* This is what any
  visitor reads first.
- **Methodological register (on obvious affordance).** The probe's formal
  identity — *"Probe 0 — a constellation of sources covering Epistemic Authority
  and Power Legitimation discourse functions in German institutional discourse
  (WP-001)."* This expands inline, in place, without leaving the globe.

Surface III (Reflection) additionally hosts a **"How to read the globe" primer** —
linked from Surface I's L0/L1 chrome — that introduces the probe concept, the four
discourse functions, and what AĒR is and is not trying to see. A first-visit
overlay is appropriate; Step 2 decides whether to include one in Iteration 5.

The principle: a visitor who has never heard the word *probe* should form a
correct first impression from the globe alone, with an obvious path to the full
methodological frame. Probe fluency is earned by using the dashboard, not
demanded at the door.

---

## 3. What is missing from the current brief

The brief's vision already covers much of what is needed — Surfaces II and III,
Visualization Stack Separation (§5.9) with four rendering domains, Fractal Cultural
Granularity (§4.5), Epistemic Weight (§5.8), Progressive Semantics (§5.7), Extensibility
(§6). These do not need rewriting — they need to be **shifted from aspiration to
implementation priority** and extended in five specific ways.

### 3.1 The Probe Dossier is a real, structured surface — not a fly-out

The brief mentions a "Probe Dossier" as L4 provenance content (§3.1, §4.2). The user's
stated requirement — "I want to see how many articles each source of a probe has" —
demands more: the Probe Dossier is the **source-and-article browser per probe**. It
must show, for a selected probe:

- The probe's emic designation and classification (already in the brief).
- The list of sources that constitute the probe, with per-source counts: total
  articles, articles in the current time range, publication frequency.
- A navigable article list per source, with filters (time range, language, entity
  match, sentiment band, etc.) and the ability to open an individual article at L5
  Evidence.
- The probe's methodology: WP-001 functional classification, WP-003 bias
  documentation, WP-006 observer-effect assessment — not as footnotes but as the
  navigation chrome of the dossier itself.

This makes the Probe Dossier the **bridge** between the Atmosphere overview and the
scientific surfaces. It answers the first question a serious user has: *what is
actually in this probe?*

### 3.2 Surface II (Function Lanes) is the primary scientific surface

The brief already specifies Surface II as the "methodological backbone." What the brief
under-specifies is that **Surface II is where the user spends most of their analytical
time.** It is not a secondary surface to be reached after Atmosphere.

Rewriting Surface II should articulate:

- Each function lane (per WP-001's four functions, with the fifth-lane extensibility
  hook from §6.1 preserved) as a **full working surface**, not a single time-series
  per lane. Within a lane: time series, small multiples by source, entity clouds,
  sentiment ridgeline, publication-volume density, cross-source comparison of
  within-context deviations.
- Multiple **view modes per metric**, organised as a two-axis matrix:
  **analytical disciplines** (what method is applied) × **presentation forms** (how
  it is rendered). The axes are independent — one discipline can be rendered multiple
  ways, one presentation form can serve multiple disciplines. A concrete view mode is
  a cell of this matrix, bound to a metric context.

  **Analytical disciplines** — non-exhaustive, extensible:

  - *Explorative Data Analysis (EDA)* — summarising and pattern-discovery without
    prior hypothesis.
  - *Network Science / Network Analysis* — the structure of relationships between
    entities (authors, topics, sources, timestamps, cross-source co-occurrences).
  - *Force-directed graph layout* — physics-based node positioning for dense
    relational data.
  - *Clustering (unsupervised)* — automated grouping by statistical similarity
    (article clusters, topic clusters, affect clusters).
  - *Metadata mining* — knowledge extracted from the descriptive layer (source,
    timestamp, language, entity labels) rather than the primary text.
  - *Natural Language Processing* — the Gold extractors' native domain: sentiment,
    entities, language detection, and future extractors as they come online.

  **Presentation forms** — non-exhaustive, extensible:

  - *Time-series charts* — lines and areas with uncertainty bands.
  - *Heatmaps* — color-coded density (e.g. day-of-week × hour activity density).
  - *Correlation matrices* — grids showing pairwise strength.
  - *Distributional plots* — ridgeline, violin, density.
  - *Dynamic network graphs* — interactive node-link diagrams with real-time
    manipulation.
  - *Topographic maps / themescapes* — 3D landscapes where topical relevance or
    density becomes elevation.
  - *Small multiples* — per-source or per-context panels on a shared scale.
  - *Parallel-context constellations* — multi-probe composition within a function
    lane (per the brief's existing §1.3 principle).

  The user selects the view; AĒR does not pick one "correct" chart. A metric like
  sentiment is not a time-series — it is a point in a space that can also show it as
  a per-source distribution, as a network of co-occurring entities coloured by
  sentiment, as a clustering of articles in affect space, or as a themescape of
  sentiment density across time.

  **MVP target for the first Iteration 5 release: three view modes per metric,
  chosen from three structurally different cells of the matrix** — not three chart
  variants of the same discipline. For sentiment, a first set might be: a time-series
  (NLP × time-series), a per-source distribution (EDA × ridgeline), and an entity
  co-occurrence network coloured by sentiment (Network Science × force-directed
  graph). The catalog is extensible — new modes are registered as cells in the
  matrix and become available without frontend redesign, per §6 of the brief.

  Bound by what current Gold supports: themescapes and topic clustering depend on
  extractors not yet in the pipeline; the MVP view-mode catalog stays within what
  can be computed from sentiment, entities, language, temporal distribution, and
  word count. The backend implications of broadening this beyond existing Gold are
  enumerated in §7.

- An **exploratory composition mode** inspired by Kriesel-style free-form metadata
  exploration: the user picks two or more metrics/dimensions and asks the instrument
  to show how they relate. Connection cards with physical-string-like links between
  metrics; co-occurrence networks; temporal lead-lag views (WP-005 §6 Rhizome
  propagation). This uses the **Relational Networks** domain from §5.9 which is
  already architecturally reserved but has no design articulation today. Reserved
  and named for a later iteration — flagged as important but not blocking Iteration 5.

- **Within-context comparison is always on.** Cross-context comparison remains
  governed by the equivalence registry (WP-004 §5.2) and the refusal surface (§5.4).

Surface II is the answer to "this looks like extended tooltips." It is the surface that
makes AĒR look like a research instrument rather than a fancy overview.

### 3.3 Surface III (Reflection) is where methodology is lived, not filed

The brief describes Surface III as "the documentary surface" with long-form prose and
inline interactive visualizations (§3.3). What the brief under-specifies is that this
surface is also **where the user comes to understand a finding they just saw on
Surface II** — clicking a metric on Surface II should open the Reflection content for
that metric, scrolled to the section that explains the current limitation.

Equally important: methodology is not hidden behind an unobtrusive icon. **For every
metric shown, the methodological surface is reachable through a visible, obvious
affordance** that makes its existence impossible to miss — even when the full content
is one click deeper. The principle: *a click to expand is acceptable; a click to
discover that methodology exists is not.* This is the operational response to WP-006's
reification risk (§3.2). The exact shape is named in §6.1 (recommended right-edge
docked methodology tray) and finalised in Step 2.

The rewrite should articulate:

- Reflection is not a separate section to be visited rarely. It is **reachable from
  every metric, every badge, every refusal** — the "read more" that is always one
  obvious click away. This is the operational form of the "Methodological
  Transparency" principle from WP-006 §6.1.
- Working Papers (WP-001 through WP-006 and future additions) are rendered natively
  inside Reflection with live interactive illustrations — a Distill.pub-style
  presentation where manipulating a parameter on the page shows its effect on real
  Probe 0 data. The user's Kriesel reference maps directly here: the paper is not
  static, the paper is an interactive argument.
- The **open research questions from WP-001 §8, WP-002 §7, WP-003 §7, WP-004 §7,
  WP-005 §7, WP-006 §8** become first-class entries on a dedicated page within
  Reflection — each with its scope, its relevant pipeline hooks, and an invitation
  to interdisciplinary contribution. Linked from every Working Paper and every
  relevant refusal surface. This operationalizes the Manifesto §V call for dialogue.

Reflection is where AĒR's commitment to "observation over surveillance" is made
legible. A dashboard without it is a surveillance tool; a dashboard where it is
peripheral is a research tool with an alibi. Reflection must be central.

### 3.4 Silver-layer self-service — committed to Iteration 5

The brief is silent on Silver-layer access. The user has stated the capability:
*"it should later also be possible to just explore the silver layer (the uniformed raw
data) to do some Data analysis interactively by yourself."*

A key observation from the 2026-04-24 review reframes this: **the same view-mode
catalog from §3.2 (analytical disciplines × presentation forms) applies equally to
Gold metrics and to Silver raw data.** The difference is the backend data source, not
the user-facing machinery. An EDA × heatmap view on Gold `sentiment_score` and an EDA
× heatmap view on Silver `cleaned_text` token distributions are the same interaction
built over a different query. This reframes Silver-layer exploration from *"a future
fourth surface"* to *"a data source that Surface II can consume."*

**Decision (2026-04-24): Silver-layer access commits to Iteration 5.** The backend
work involved (eligibility flag on sources, Silver query endpoints, enforcement in the
BFF) is accepted scope. The UI work is a modest increment once the Gold view-mode
catalog is built — a data-source selector on Surface II and visibility rules for
non-eligible sources.

Governance is the constraint, not the UI. Silver exposes raw cleaned text. Unlike
Gold, which is already aggregated and k-anonymity-safe by construction, Silver-level
access for a source requires an explicit **eligibility flag** on the source (or probe)
record:

- Probe 0 is auto-flagged (engineering POC, institutional public data, no
  re-identification risk — both sources are government or public-broadcaster RSS,
  per Manifesto §VI and WP-006 §7).
- All other sources require a review process before the flag is set — applying
  WP-006 §5.2 ethical review specifically to the Silver-access question. The
  review outcome, the reviewer, and the date are recorded alongside the flag.
- Sources without the flag do not appear as Silver options on Surface II,
  regardless of their Gold presence. The absence is documented as an explicit
  "not Silver-eligible" state in the UI, not a silent omission.

Backend work implications of this commit are enumerated in §7.

### 3.5 Probes and sources as selection units

The brief's §6.4 establishes *the probe is the unit of granularity*. The Phase
100a implementation, by necessity of Probe 0's shape (a single probe with two
sources), renders emission points on the globe that are **sources**, not probes.
The 2026-04-24 review surfaced that this representation is cognitively confusing:
the user themselves, looking at the globe, begins thinking in sources and loses the
probe frame. Since the probe *is* the unit the science operates on, this is a
structural bug in the interaction model, not a cosmetic one.

Three shapes were considered:

1. **Probe-first (canonical selection only).** Globe emits one point per probe.
   Source-level detail is not visible at Surface I at all. Source filtering and
   inspection happen inside the Probe Dossier on Surface II.
2. **Probe-first with source presentation.** Globe emits one point per probe as the
   canonical, selectable object. Sources are visible as a **read-only
   presentation layer** — satellite glyphs, density rings, or a zoom-reveal that
   shows the sources contributing to a probe without making them selectable targets.
   Clicking a source does not change scope; it opens a source filter inside the
   Probe Dossier. The probe view is canonical; the source view is ancillary.
3. **Dual-mode emission (rejected).** Probes and sources as first-class peer
   selectable points. Rejected because it reintroduces the probe/source cognitive
   slip — the very confusion the reframing aims to fix.

**Decision (2026-04-24): Option 2.** Selection is probe-only; source visibility is
presentational. The probe view is the default and the "correct" view at all times.
Sources surface as informational detail for users curious about a probe's
composition, without becoming a parallel interaction frame. The Phase 100a globe
behaviour — where clicking a source drove the descent — is deprecated; it will be
replaced by probe-level emission with sources shown as visible satellites (or
equivalent presentational form, to be resolved visually in Step 2).

Critical clarification: **source-specific analysis is fully preserved.** Option 2
removes source-as-globe-selection, not source-as-analysis-scope. Inside the Probe
Dossier on Surface II, the user can narrow the active scope to a single source,
and every view mode from §3.2 runs equally at source scope. The backend view-mode
queries (§7.3) accept either a probe scope or a source scope as their parameter.
The user who wants *"sentiment for Tagesschau alone, without Bundesregierung in the
aggregate"* has that view — it just reaches them through the Dossier, not through
a click on the globe.

### 3.6 What a probe is — a terminology note (Path B, deferred reconciliation)

The word *probe* carries a slight mismatch between the scientific foundation
(WP-001) and the current engineering usage. The reframing records this
explicitly so future readers are not blindsided, and defers reconciliation to
a post-Step-2 ADR.

**WP-001 usage.** §3–§5 treat a probe as an *individual observation point* —
1:1 with a source, carrying one etic function tag and one emic designation. The
multi-source grouping that covers multiple discourse functions for a society is
called a *probe constellation* or *Minimum Viable Probe Set (MVPS)* in WP-001
§5.1.

**Current engineering usage.** The system calls the *grouping* a "Probe" (e.g.
"Probe 0 contains two sources: `tagesschau.de` and `bundesregierung.de`"). The
individual observation points are called *sources* and live in the PostgreSQL
`sources` table.

**What a probe (in the current sense) actually is.** A **grouping of sources
defined by a shared cultural and discursive scope**, not by matching etic/emic
tags. Sources *within* a probe typically carry *different* etic function tags
— that is precisely what makes the constellation analytically useful per
WP-001 §5.1 (one probe covers multiple functions for one society). Probe 0's
two sources have different primary etic functions (Epistemic Authority vs.
Power Legitimation) and different emic designations. Two probes in different
countries are not required to have matching tag sets, matching source counts,
or matching function coverage.

A probe's **function coverage** is a derived property: how many of the four
WP-001 discourse functions its sources collectively address. Probe 0 covers
2/4 today. Function coverage is a useful visualisation in the Probe Dossier
and on the Coverage Map; it is not a constraint on what a probe can be.

**Path B decision (2026-04-24): keep current usage for Iteration 5.** The
engineering terminology — "Probe" = grouping, "Source" = individual, matching
the PostgreSQL schema and existing API surface — stays as-is. The rewritten
brief names the WP-001 drift explicitly in a short "Terminology" section so new
readers understand the mismatch, but does not propagate a terminology change
through code, schema, or endpoints. A post-Step-2 ADR revisits reconciliation
once the dashboard is shipping.

Why Path B, not Path A (full reconciliation): terminology surgery mid-rewrite
is high-risk for little Iteration 5 value. The mismatch is a documentation
clarity issue, not a correctness issue — the architecture behind the words is
coherent. Step 2 adds the named Terminology section; a later ADR handles any
rename with full engineering scope.

---

## 4. What changes in the brief — a punch list

This is the concrete rewrite scope for Step 2, not the rewrite itself.

1. **§1.1 and §1.3:** Keep Atmospheric Aesthetics and Composition-not-comparison. They
   still apply — but scoped more tightly to Surface I's overview role.
2. **§3, §3.1:** Rewrite to demote the globe to a landing overview. Make Surface II
   the primary scientific surface in language and priority, Surface III the primary
   reflective surface, Surface I the primary orientation surface.
3. **§3.1 specifically:** Remove or scope down "the layer where the user breathes."
   The user does not breathe on the globe — they land there, get oriented, and leave
   for Surface II. The globe is the *welcome mat*, not the *living room*.
4. **New § (placed before the surface articulation): Navigation is first-class.**
   Persistent, obvious, bidirectional, scope-carrying, return-aware. Addresses the
   "L4 hidden behind a button" failure mode observed in Phase 100a. See §2.1 above.
   The Step 2 rewrite names a concrete chrome shape — recommended left side rail +
   thin scope bar hybrid (see §6.1).
5. **New §: Probe concept introduced in-context.** Per §2.2 above. Every probe
   glyph on the globe carries Progressive Semantics (semantic register prominent,
   methodological register on obvious affordance). Surface III hosts a "How to
   read the globe" primer linked from Surface I's chrome. Probe fluency is not
   assumed.
6. **§3.1 (globe behaviour):** Probe-first emission; sources as a read-only
   presentation layer, never a peer selection target. Phase 100a's source-click
   behaviour is deprecated. See §3.5 above.
7. **§3.2:** Expand Surface II's articulation to include the **analytical disciplines
   × presentation forms** view-mode matrix (§3.2 above), the Probe Dossier as the
   source-and-article browser, and the reserved-but-named exploratory composition
   mode. Fix the MVP at three view modes per metric, chosen from structurally
   different cells of the matrix. Note the Gold-boundedness caveat and cross-link
   to §7.
8. **§3.3:** Expand Surface III's articulation to make it reachable from every metric
   and every refusal, and to make the methodological surface **accessible through a
   visible, obvious affordance** rather than a hidden button — the content itself
   may open elsewhere (recommended right-edge docked tray, see §6.1) rather than
   crowding Surface II. Name the Distill-style interactive-paper pattern explicitly.
   Add the dedicated open-research-questions page.
9. **§4 and §4.2 matrix:** Redistribute L3/L4/L5 across surfaces. L0–L2 live on
   whichever surface the user is on. L3 (Analysis) lives natively on Surfaces II
   and III, not as a "companion panel" on Surface I. L4 (Provenance) is the
   dedicated role of the methodology surface (right-edge tray or equivalent),
   reachable from any surface. L5 (Evidence) is a reader-pane overlay reachable
   from any surface. The matrix must reflect this.
10. **§4.5 Fractal Cultural Granularity:** Stays. The probe-defined granularity
    principle is exactly right.
11. **§5.8 Epistemic Weight:** Stays. Already implemented in `design_system.md` §4.
12. **§5.9 Visualization Stack Separation:** Stays — and gains a fifth design
    articulation for what the **Relational Networks** domain actually *shows*
    (Kriesel-style exploratory composition). Today this domain is reserved but
    undesigned.
13. **§6 Extensibility:** Stays. Confirm the "probe is the granularity unit"
    principle (§6.4) — and add the concrete interaction-level resolution from §3.5
    (probe-first selection, sources as presentation). Confirm that the view-mode
    catalog from §3.2 is itself extensible by the same principle: new disciplines
    and new presentation forms register as matrix cells without layout redesign.
14. **New §: Terminology note on "probe" vs. WP-001 usage.** Per §3.6 above.
    Record the mismatch, record the Path B decision (keep current usage for
    Iteration 5), defer reconciliation to a post-Step-2 ADR. Short section, not a
    restructure.
15. **New §: Silver-layer access as a data-source toggle with eligibility flag.**
    Per §3.4. Committed to Iteration 5. The eligibility flag is a backend concern
    (source schema addition + review metadata); the UI is a Surface II toggle.
16. **§11 open questions:** Prune questions answered by the reframing; keep
    questions that remain live; add new questions surfaced by the reframing (e.g.
    "does the user successfully leave the globe, or do they bounce back to it?",
    "is the always-co-present methodology tray readable or crowding?", "does the
    in-context probe introduction succeed — does a first-time visitor form a
    correct first impression?").

ADR-020 does not need a full rewrite. The technology choices (SvelteKit, three.js,
MapLibre+deck.gl, Observable Plot+uPlot+D3, D3-force) are validated by the reframing —
if anything the reframing leans *harder* on the non-globe visualization domains. What
ADR-020 needs is a **"Compliance check"** update to reflect the rebalanced surface
priority, the navigation-primacy requirement, the methodology tray, and the
probe-first selection model; **an "Implementation outline" reorder** so Surface II and
Surface III precede further Surface I deepening; and a **new backend-work section**
(or updates to the existing Content Catalog section) covering the Silver-layer
endpoints, the eligibility flag, the Probe Dossier expansion, the article browser, and
the view-mode query endpoints listed in §7.

`design_system.md` does not need rewriting at all — the tokens, typography, color,
Epistemic Weight classes, and base components all stand. The new surfaces will consume
them. It will gain one additional section for navigation-chrome primitives (side rail,
scope bar, methodology tray), depending on the shape chosen in Step 2.

---

## 5. What Phase 100a's output becomes

Phase 100a built a Surface I descent with L1–L4 companion panels. Under the reframed
design, this output is **partially preserved and partially deprecated:**

- **Preserved:** the 3D engine itself, the globe rendering, terminator, reach auras,
  absence regions, time scrubber, URL state, OTel wiring. This is all globe-native
  work that the new design still wants for L0–L2.
- **Deprecated / redesigned:** the source-as-emission-point behaviour (replaced by
  probe-first emission per §3.5), and the L3/L4 "companion panels" on Surface I
  (replaced by transition to Surface II scoped to the selected probe, with the
  methodology tray available from there). The panels' *content* is not lost — it
  moves to its natural home, and methodology is elevated to a persistent,
  obviously-reachable tray per §3.3.

Phase 100a stays committed; it is a reference point, not a commitment to its
interaction model. Phase 100b is deferred entirely until the rewritten brief exists.

---

## 6. Decisions resolved in the 2026-04-24 review

The following were resolved during the review and are fixed inputs to Step 2:

1. **Globe stickiness during descent.** The globe disappears entirely during descent
   to free the full viewport for scientific work. A persistent, visible return path
   is available from every view (per §2.1).
2. **Surface II default landing.** When the user selects a probe from the globe,
   Surface II opens on the **probe overview (Dossier)**. Meta first, then specific.
3. **View modes per metric at MVP.** Three per metric, **chosen from structurally
   different cells of the analytical-disciplines × presentation-forms matrix** —
   e.g. one chart, one network, one heatmap — not three variants of the same
   discipline. The catalog is extensible (§3.2).
4. **Exploratory composition mode scope.** Reserved and named for a later iteration.
   Important, not blocking (§3.2).
5. **Silver-layer self-service.** **Committed to Iteration 5.** Surface II gains a
   data-source toggle; the backend gains a source-eligibility flag and Silver query
   endpoints. Probe 0 is auto-flagged; other sources require WP-006 §5.2 review
   (§3.4 and §7).
6. **Probe vs. source emission on the globe.** **Option 2 (probe-first with source
   presentation).** Selection is probe-only. Sources are a read-only presentation
   layer visible around probe glyphs on the globe and inside the Probe Dossier —
   never a peer selection target. Phase 100a's source-click behaviour is
   deprecated (§3.5).
7. **Open-research-questions hub.** Dedicated page within Reflection, linked from
   every Working Paper and every relevant refusal.
8. **Probe concept introduction.** The globe introduces probes in-context via
   Progressive Semantics on every glyph, plus a "How to read the globe" primer in
   Reflection linked from Surface I's chrome. Probe fluency is not assumed (§2.2).
9. **Probe terminology — Path B.** Keep current engineering usage for Iteration 5
   ("Probe" = grouping, "Source" = individual, matching the PostgreSQL `sources`
   schema and existing API surface). Name the WP-001 drift in a short Terminology
   section of the rewritten brief. Defer full reconciliation to a post-Step-2
   ADR (§3.6).

### 6.1 Design recommendations with Step 2 final-call

Two design decisions were scoped during the review with explicit recommendations,
but the final shape is Step 2's to settle once the rewrite is underway.

#### 6.1.1 Methodology surface shape — recommended: right-edge docked tray

Requirement from the review: the methodology surface must be **visible and easily
reachable without consuming Surface II's primary space**. Expandable, connected,
elegant, obvious-that-it-exists, toggleable.

Recommendation: **a right-edge docked tray.** Rationale:

- **Closed state.** A narrow vertical tab anchored to the right edge, always
  visible across Surface II and parts of Surface III. The tab carries the active
  metric's tier/validation badge and a clear "Methodology" label. The presence of
  the tab makes it impossible to overlook that methodology exists for the thing
  you are looking at.
- **Open state.** Slides out to roughly 35–40% viewport width as a full-height
  panel. Push-mode (Surface II content compresses) or overlay-mode (floats over)
  can both be tried; push-mode reads as more "connected," overlay-mode preserves
  chart layout better. Step 2 picks.
- **Content.** The metric's current provenance, tier classification, known
  limitations, and a clear link into the relevant Working Paper rendered on
  Surface III. Dual-Register content (§5.7 of the brief) lives naturally here.
- **Binding to the active metric.** The tray's content follows whatever metric
  is currently in focus on Surface II. Changing the metric updates the tray;
  no separate interaction is required.

Alternative considered: **bottom drawer.** Good for short inline explanations, bad
for long-form prose (methodology content reads better at narrow full-height than at
wide short-height). Fallback for short-form affordances inside individual charts.

Rejected: inline sidebar permanently co-present. Takes too much from Surface II.

#### 6.1.2 Navigation-chrome shape — recommended: left side rail + thin scope bar

Requirement from the review: obvious, elegant, beautiful, integrated ("baked into
the rest of the design"), hybrid, leaning side rail.

Recommendation: **a hybrid of a left side rail (primary) and a thin top scope bar
(secondary).** Rationale:

- **Left side rail.** A narrow permanent vertical element. Three surface anchors —
  **Atmosphere** (with a planet/globe glyph that also serves as the "return to
  overview" anchor), **Function Lanes**, **Reflection** — as visibly primary
  navigation. The rail also carries a compact scope indicator: the current probe
  name, the current time range, and the current viewing mode. Clicking a surface
  anchor switches surface while preserving scope. The rail is visually integrated
  (not floating, not over-styled) and reads as part of the page's structure.
- **Thin top scope bar.** A subdued horizontal strip above the active surface's
  content. Carries the within-surface navigation — lane selection on Surface II,
  section anchor on Surface III. Keeps the side rail uncluttered by surface-local
  controls.
- **Methodology tray tab** (from §6.1.1) docks at the right edge, visually
  balancing the left side rail. The three elements — left rail, top bar, right
  tray — form a persistent frame around the active content, always visible,
  always obvious.
- **Return-to-Atmosphere** is a single click on the planet glyph at the top of
  the left rail from any surface or depth.

Alternative considered: top-only navigation with breadcrumbs. Rejected because it
reads as ordinary web-app chrome and loses the "instrument" character the brief
commits to.

Rejected: floating nav overlays, hamburger-hidden menus. Explicitly forbidden by
§2.1's "navigation is a named, first-class UI element — not an icon that users may
never notice."

---

## 7. Backend work implied by the reframing

The reframing pressures the BFF (and adjacent backend layers) in several specific
ways. This section enumerates the scope deltas that Step 2's ADR-020 rewrite must
address, and flags the one open question that the reframing itself does not resolve.

The BFF is the correct layer for most of this — it is the query-shaping and
authorization surface between the dashboard and Gold/Silver storage. A few items
touch PostgreSQL (schema migrations), ClickHouse (query endpoints / views), or the
analysis worker (corpus-level aggregation).

### 7.1 Endpoints that expand beyond what ADR-020 currently anticipates

- **Probe Dossier data.** Per-probe: source list, per-source article counts in
  arbitrary time windows, per-source publication frequency, emic content, probe
  classification. ADR-020's content catalog endpoint is insufficient — this is
  aggregate data, not static content. Likely shape: `GET /api/v1/probes/{id}/dossier`
  returning the composite payload, or a set of narrow endpoints (`/sources`,
  `/counts`).
- **Article browsing.** Paginated article list per source with filters (time
  range, language, entity match, sentiment band). Likely shape:
  `GET /api/v1/sources/{id}/articles?...` — returns article IDs with light
  metadata for listing.
- **Article detail (L5 Evidence).** Individual article with full cleaned text and
  provenance. Reads from MinIO Bronze (raw) and ClickHouse/Silver (metadata and
  extractor outputs). Likely shape: `GET /api/v1/articles/{id}`. Subject to
  k-anonymity gates per WP-006 §7 — the gate itself needs BFF-side enforcement
  against the relevant aggregate floor.

### 7.2 Endpoints new because of the Silver-layer commit (§3.4)

- **Source eligibility flag** on PostgreSQL `sources` — a schema migration to add
  `silver_eligible` (boolean) plus review metadata (reviewer, date, rationale,
  review reference). Probe 0's two sources are seeded as eligible in the same
  migration.
- **Eligible-sources query.** A BFF endpoint that returns only Silver-eligible
  sources for a given scope — or an `?includeSilverOnly=true` filter on
  `/api/v1/sources`.
- **Silver document browsing.** Paginated list and detail endpoints against
  Silver storage (MinIO + ClickHouse `aer_silver` if any) for eligible sources.
  The data model here needs a design decision in Step 2: does the BFF expose
  Silver `SilverCore` records directly, or a normalised projection?
- **Silver aggregation queries** for the view-mode catalog — analogous to the
  Gold `/metrics`, `/entities`, `/languages` endpoints but over Silver data.

### 7.3 Queries new for the view-mode catalog (§3.2)

Each view mode is a cell in discipline × presentation. Most map to queries the BFF
can compose from existing Gold data; a few require new aggregations.

**Scope parameter on every view-mode query.** All view-mode endpoints accept
either a probe scope (`probeId`) or a source scope (`sourceId`) as their
parameter, with probe scope as the default. This preserves source-specific
analysis as a first-class mode per §3.5, without duplicating the endpoint
surface.

- **Distributional queries** (EDA × ridgeline/violin/density). Per-source
  distributions of metric values within a time window. Probably a new
  `/api/v1/metrics/{metricName}/distribution` endpoint returning histogram or
  raw-value arrays.
- **2D binning for heatmaps.** Day-of-week × hour, source × entity-label, etc.
  Can be a `resolution` and `dimension` parameter on `/api/v1/metrics` or a new
  `/api/v1/metrics/{metricName}/heatmap` endpoint.
- **Correlation matrices** (metadata mining × correlation matrix). Pairwise
  correlation of metrics across sources or time windows. New endpoint:
  `/api/v1/metrics/correlation?metrics=a,b,c&...`.
- **Entity co-occurrence** (Network Science × force-directed graph).
  Pair-counts of entities appearing together in the same document or time
  window. This is the open question in §7.5: BFF-computed from `aer_gold.entities`
  vs. precomputed by a new CorpusExtractor.
- **Small multiples**, **parallel-context constellations**. These are frontend
  compositions of per-source or per-probe queries; no new BFF surface needed
  if the per-source queries exist.

### 7.4 Content catalog expansion

The reframing increases the volume of Dual-Register content substantially. The
content catalog (ADR-020 Layer 4) must cover:

- A Dual-Register pair (semantic / methodological) per metric, per view mode,
  per refusal type, per probe, per discourse function.
- Open-research-question entries for each WP §8 / §7 entry.
- Cross-links from content entries to Working Paper anchors on Surface III.

No new endpoint beyond `GET /api/v1/content/{entityType}/{entityId}?locale=...`
that ADR-020 already anticipates; the scope of the content itself grows.

### 7.5 The one open scope question: corpus-level extractors

The reframing asks for view modes that are conceptually aggregate: entity
co-occurrence, article clustering, topic-level themescapes. The CLAUDE.md
architecture already reserves a `CorpusExtractor` protocol (aggregate batch
extraction over cores in a time window) for exactly this purpose — "for future
TF-IDF, LDA, co-occurrence."

The non-goal §8 "No new NLP extractors" was written tightly. Step 2 (or a
pre-Step-2 scope session) must resolve whether that non-goal holds strictly, or
relaxes to include the first CorpusExtractor implementations needed by the MVP
view-mode catalog. Three positions are available:

1. **Strict non-goal.** MVP view-mode catalog limited to what per-document
   extractors already produce. No CorpusExtractor in Iteration 5. Entity
   co-occurrence and clustering views are deferred until the analysis worker
   gains the relevant extractors. Effect: the "three structurally different
   cells" MVP narrows — may end up as time-series + distribution + simple
   heatmap. Loses some analytical richness.
2. **Scoped relaxation.** Allow one CorpusExtractor — most useful is probably
   entity co-occurrence, because it unblocks the Network Science view-mode
   family on existing Gold entity data. Everything else (topic modelling,
   article clustering, themescapes) stays deferred.
3. **Broader relaxation.** Implement CorpusExtractor plus one clustering
   method (e.g. sentiment-space k-means) to unblock the Clustering discipline.
   Largest scope bump; largest analytical value.

**Recommendation: option 2.** Entity co-occurrence is the single highest-value
unlock for the MVP view-mode catalog — it enables the Network Science discipline
on data the backend already produces (`aer_gold.entities`), which is the single
most visually distinctive new view mode in the Step 2 rewrite. Topic modelling
and article clustering depend on infrastructure (LDA pipelines, embedding
stores) that is not a modest increment and should stay deferred.

This is a scope choice, not a design choice. Step 2 may proceed on option 2 by
default; the user can override before the rewrite starts.

### 7.6 What the reframing does *not* add to the backend

For clarity on what stays out:

- No changes to Bronze ingestion or to the Silver harmonisation path.
- No changes to ADR-015 SilverCore contract.
- No changes to per-document extractors already in the pipeline (sentiment,
  entities, language, temporal, word count).
- No changes to NATS JetStream, MinIO provisioning, or the Medallion architecture.
- No changes to authentication (Traefik + static API key per ADR-011/ADR-018).
- No real-time streaming requirements; the dashboard remains pull-based.

---

## 8. Non-goals for this iteration

Stated explicitly so Step 2 does not drift:

- **3D engine internals.** Out of scope. Globe rendering, shaders, camera control
  stay as Phase 100a left them.
- **New per-document NLP extractors.** Out of scope. The per-document extractor set
  (sentiment, entities, language, temporal, word count) is fixed for Iteration 5.
  Corpus-level extractors are governed by §7.5 and are a separate scope question.
- **User authentication changes.** ADR-020's API-key-at-Traefik remains.
- **Mobile-first design.** Still desktop-first with Low-Fidelity mode per the
  brief's §5.6.
- **Real-time streaming.** The dashboard remains a pull-based BFF consumer.
- **Multi-tenant or per-user state.** No per-user accounts, no saved dashboards,
  no personal annotations in Iteration 5.

---

## 9. Checkpoint for the user

If the shape of this reframing — including the backend-work enumeration in §7 and
the open scope question in §7.5 — matches what you meant in the 2026-04-24 review
and in the follow-up resolutions, the next move is Step 2: I rewrite
`design_brief.md` against §4's punch list, apply the compliance-check,
implementation-outline, and backend-work updates to ADR-020, and leave
`design_system.md` alone (pending the navigation-chrome primitive decision from
§6.1). If any of §2 (globe demotion), §2.1 (navigation primacy), §3 (the five gaps),
§4 (punch list), §6 (resolved decisions), §6.1 (design recommendations), or §7
(backend work) is off, redline before Step 2 starts.

The single pre-Step-2 decision still needed is §7.5 — strict non-goal vs. option 2
(one CorpusExtractor for entity co-occurrence) vs. option 3 (broader relaxation).
My recommendation is option 2; confirm or override.
