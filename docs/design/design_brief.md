# Design Brief — The AĒR Frontend

**Status:** Iteration 5 — open for review.
**Authority:** Derived from Manifesto (§I–§V), Arc42 §1.1–§1.4, ADR-003, ADR-016, ADR-017, ADR-020, WP-001 through WP-006, `docs/design/visualization_guidelines.md`, and the 2026-04-24 Reframing Note (merged into this iteration).
**Audience:** Sole author (for now); future contributors; interdisciplinary reviewers from WP-006 §8.3.
**Relationship to existing documents:** This brief sits *above* `visualization_guidelines.md` — the guidelines constrain individual rendering decisions; this brief defines what the dashboard *is*. It sits *alongside* ADR-020 (Frontend Technology Stack), which realizes this brief technically; ADR-020's compliance check audits the brief clause by clause.

---

## 0. What this document is, and is not

This is a **design brief**, not an architecture decision record. It defines the visual identity, interaction architecture, and extensibility commitments of the AĒR dashboard so that any subsequent technical decision can be evaluated against a shared standard.

The brief is a contract with the project's future: any dashboard implementation — in-repo or third-party — that claims to represent AĒR must satisfy the principles recorded here. A dashboard that is fast but prescriptive, beautiful but closed to extension, or sophisticated but methodologically opaque fails this contract.

Iteration 5 (this iteration) reflects a structural reframing of the dashboard surfaced by the 2026-04-24 review of the Phase 100a implementation. The single most important shift: **the 3D globe is the landing overview, not the dashboard's primary working surface.** Real scientific work happens on Surface II (Function Lanes) and Surface III (Reflection), both of which must be obviously reachable from the globe and from each other at all times. Navigation is a first-class concern of the brief (§3).

---

## 1. The Design Concept: Atmospheric Aesthetics

The name AĒR comes from ancient Greek ἀήρ — the lower atmosphere, the surrounding climate (Arc42 §1.1). The Manifesto extends this into the governing metaphor: an orbital sensor that captures atmospheric currents rather than individual fates (§I).

The frontend takes this literally on Surface I (the globe) and extends its underlying values — observation without prescription, stillness with motion beneath, the observer at the edge — across all three surfaces.

### 1.1 What "atmospheric" means as a design principle

**Layering.** The atmosphere is not flat. It is stratified — troposphere, stratosphere, mesosphere — each layer with its own physics, yet part of a single continuum. The dashboard mirrors this: content is accessed by altitude, not by navigation alone. The user does not "switch tabs" — they descend through layers toward evidence, or rise back toward contemplation. This is the structural form of Progressive Disclosure (ADR-003), made visual.

**Currents over points.** Atmospheric phenomena are flows, not dots. Publication volume is not a bar chart — it is an aggregate density. Sentiment is not a single value — it is a pressure gradient with uncertainty at its edges. Entity salience is not a ranking — it is turbulence. This directly operationalizes `visualization_guidelines.md` §1 (Viridis, no discrete valence) and §3 (uncertainty alongside estimates).

**Stillness with motion beneath.** The atmosphere appears macroscopically still and is microscopically chaotic. The dashboard's default state is contemplative; activity reveals itself only to the attentive eye. This is the aesthetic opposite of a status board.

**The observer at the edge.** We live within the atmosphere, but we can only *see* it from outside (orbital sensors) or through its effects (clouds, wind). This is Borges' Aleph paradox — to be simultaneously inside and outside the observed. The dashboard encodes this position spatially: the default viewpoint on Surface I is external (orbital), and descent into Surfaces II and III brings the observer progressively closer to the subject, but never fully in.

**No faces, no names.** The atmosphere shows systems, not individuals. Consistent with Manifesto §I ("atmospheric currents rather than individual fates"), the frontend never displays persons, avatars, or "top influencers" by name as primary content. Individuals appear only at Layer 5 (Evidence), as named entities within a retrieved source document — never as aggregated protagonists.

### 1.2 Where the atmospheric metaphor is literal, and where it is structural

On Surface I, the metaphor is literal: a rotating Earth, a terminator, probe glyphs as luminous points, atmospheric scattering in the halo. On Surfaces II and III, the metaphor is structural, not literal: the aesthetic values (stillness, observer-at-edge, no faces) carry through as restraint in typography, neutrality in color, and discipline in motion — but the surfaces themselves do not render a sky.

This distinction is a correction from earlier iterations. Attempting to maintain atmospheric visuals *everywhere* crowded the scientific surfaces with decoration that carried no data. The reframing preserves atmospheric *values* across the dashboard while confining atmospheric *visuals* to Surface I, where they carry informational weight (terminator = UTC time; probe glow = activity density; absence region = coverage gap).

### 1.3 Composition, not comparison

AĒR observes global discourse through multiple probes across multiple cultural contexts. The vernacular temptation is to call this "comparison" — but WP-004 §8 identifies three traps that the word "comparison" silently opens:

- **The Ranking Trap.** "Culture A has more X than Culture B" invites a scale of better/worse.
- **The Universalism Trap.** "X in culture A equals X in culture B" presumes the construct is universal.
- **The Deficit Trap.** Differences get framed as deficiencies.

The Manifesto's own language resolves this: *"observation over evaluation — to documenting resonance, not judging it."* **Resonance** is the operative word. Multiple cultural contexts, placed together in the frontend, produce a **composition** — a whole that yields more understanding than any single part — not a comparison that yields a ranking.

This is not mere word-hygiene. It is the direct application of the Aleph principle (Arc42 §1.2): *"the single point in space that contains all other points."* An Aleph *contains* its points together; it does not line them up against each other.

Throughout this brief, where multi-context displays are described, the language is deliberate: **composition**, **constellation**, **co-presence**, **resonance**. The word "comparison" is reserved for within-context analysis (this week against last week, this source against the baseline of the same source) where the reference point is native to the context. Cross-context "comparison" as a user-facing frame is not a feature of AĒR.

---

## 2. Binding the Concept to AĒR's Quality Goals

Design cannot be self-justified. Every principle above is anchored to the five quality goals from Arc42 §1.3. A design that violates a quality goal is not "pretty but unimplementable" — it is *wrong*.

| Quality Goal (Arc42 §1.3) | Design Operationalization |
|---|---|
| **1. Scientific Integrity & Transparency** | Navigation (§3) makes methodological context — the tier, the algorithm, the limitations, the provenance — structurally visible at all times, not hidden behind a micro-button. Atmospheric currents *never* smooth over structural limits. Validation status, tier classification, and known limitations are rendered at the layer where they are needed. The dashboard refuses to display what the backend refuses to compute (ADR-017 Principle 5). Ockham's Razor in visual form: no chart is more ornate than the data supports. Epistemic Weight (§7.8) ensures that visual prominence scales with methodological backing. |
| **2. Extensibility (Modularity)** | The design is **open-ended by construction** — see §8. New discourse functions, new pillars, new probes with their own cultural granularity structures, new metrics, new temporal scales, new analytical disciplines, new presentation forms all slot into the existing structure without redesign. A dashboard that requires a refactor when Probe 1 arrives — or when a new view mode is added to the catalog — is broken. |
| **3. Scalability & Performance** | The atmospheric metaphor rewards performance design. Stillness is cheap; motion is expensive — so motion is reserved for where it carries information. The default layer is nearly static. Descent into analysis layers progressively loads detail. Hard frame budgets (§10) ensure the dashboard remains fluid on modest hardware, and a Low-Fidelity mode (§7.6) preserves scientific depth on weak hardware. |
| **4. Maintainability** | The design specifies contracts, not implementations. The three surfaces, the five layers, the chrome (navigation + methodology tray), and the interaction grammar are stable; the code behind them can evolve. A contributor in 2028 should still recognize the dashboard even if every line of code has been rewritten. |
| **5. Security** | No design principle here weakens the Zero-Trust posture from ADR-013. The dashboard is a client of the BFF; it stores no credentials in the browser (see the API-key note in §11). Refusal-as-feature (§7.4) means the dashboard never reveals what authentication could not authorize. Evidence-layer access (§5.5) respects k-anonymity gates per WP-006 §7. Silver-layer access (§9) is gated by an explicit per-source eligibility flag with ethical-review provenance. |

### 2.1 Binding to the AĒR DNA (Arc42 §1.2)

The three philosophical pillars — Aleph, Episteme, Rhizome — correspond to three distinct viewing modes, each with its own visual grammar. The mapping is made explicit in WP-005 §6:

| Pillar | Temporal mode | Atmospheric metaphor | Visual grammar |
|---|---|---|---|
| **A — Aleph** | Synchronic | The weather *now* | Real-time state, spatial distribution, quiet density |
| **E — Episteme** | Diachronic | The climate record | Long trends, slow shifts, multi-year horizons |
| **R — Rhizome** | Relational-temporal | The currents | Propagation between contexts, lag, directionality |

These are not three separate dashboards. They are three **modes of viewing** the same dataset, toggled from within any layer. A user looking at Probe 0's sentiment trajectory can switch between "weather now" (the current week), "climate record" (the last year), and "currents" (when multi-probe data enables propagation observation) without leaving their context. Their relationship to cultural granularity is itself fractal — see §5.5.

The pillars are most legible on different surfaces: Aleph dominates Surface I (the landing overview); Episteme and Rhizome are most at home on Surfaces II and III (the scientific and methodological surfaces). The pillar toggle is available at all depths; the default emphasis follows the surface and the layer.

---

## 3. Navigation is First-Class

Before the surfaces, before the layers, before the visual principles, the brief commits to navigation as a first-class architectural concern. This is a response to a specific failure mode observed in the Phase 100a review: L4 provenance hid behind a micro-button inside an L3 side panel, making the most distinctive feature of AĒR — its methodological transparency — discoverable only by accident.

A research instrument's chrome is not hidden in corners. Navigation in AĒR is a named, first-class UI element with a prescribed shape.

### 3.1 Five properties of navigation

**Persistent.** The user can see, at every moment and on every surface, where they are (Atmosphere / Function Lanes / Reflection) and how to move between them. The navigation chrome never collapses into a hamburger, never disappears during descent, never requires a hover to reveal.

**Obvious.** Primary navigation elements are visually prominent with clear labels. Where iconography is used, it is paired with text. A user who has never seen AĒR before should identify the three surfaces and the return-to-overview path within seconds of first contact.

**Bidirectional.** From any depth, the user can jump laterally (to a different surface at the same scope) or vertically (to a different layer within the same surface). No dead ends. The five layers of descent (§5) are not a one-way trip.

**Scope-carrying.** The current probe selection, time range, viewing mode, and normalization choice travel with the user through navigation. Switching from Surface II to Surface III does not reset scope; it presents the same scope in a different register.

**Return-aware.** Returning to the Atmosphere overview — to pick a different probe, to reset context, to re-orient — is available from every view, not only from a "home" button. The planet glyph at the top of the left rail is the canonical return affordance.

### 3.2 The chrome shape

AĒR's navigation chrome is a three-part persistent frame around the active content. The three parts are always present; what changes across surfaces is what they carry.

**Left side rail (primary navigation).** A narrow permanent vertical element hosting:

- Three surface anchors — **Atmosphere** (with a planet glyph that doubles as the return-to-overview), **Function Lanes**, and **Reflection** — visually primary.
- A compact scope indicator that shows the current probe, the current time range, and the current viewing mode.
- A pillar-mode toggle (Aleph / Episteme / Rhizome) subdued beneath the scope indicator.

Clicking a surface anchor switches surface while preserving scope. The rail is visually integrated (part of the page structure, not a floating overlay) and restrained in styling; it reads as the instrument's spine, not a decorative panel.

**Top scope bar (surface-local navigation).** A thin horizontal strip above the active surface's content. Carries the within-surface navigation:

- On Surface I: time scrubber, resolution switch, probe search/filter.
- On Surface II: function-lane selector, metric selector for the active lane, view-mode switcher.
- On Surface III: section anchor within the active Working Paper or document.

The top bar is secondary to the left rail; it is subdued in visual weight and local in scope. Breadcrumb behavior is implicit in the combination of left rail (where) + top bar (what within where).

**Right methodology tray (L4 Provenance, always reachable).** A right-edge docked element that carries methodological context for whatever metric or finding is currently in focus. See §3.3 for its detailed specification. The tray is the architectural answer to "L4 must not hide."

This three-part frame — left rail, top bar, right tray — is always present on Surfaces II and III, and present in a simplified form on Surface I (the top bar hosts the time scrubber; the right tray minimizes until a probe is selected). The frame is the dashboard's home; the surfaces are rooms within it.

### 3.3 The methodology tray in detail

The right-edge methodology tray is the architectural answer to "L4 provenance must not hide." Its closed-state visibility makes it impossible to miss; its open-state content connects the finding to its methodology without navigating away.

**Closed state (always visible).** A narrow vertical tab anchored to the right edge, approximately 48px wide. It carries:

- The label *"Methodology"* in the instrument typography, rotated or split across the tab.
- The active metric's tier badge (Tier 1 unvalidated / Tier 1 validated / Tier 2 validated / Tier 3 / expired) per the Epistemic Weight principle (§7.8).
- A subtle indicator — a small dot or numeric — when there are known limitations specific to the current view that the user has not yet expanded.

In the closed state, the tray does not consume primary workspace. It is a visible, labeled edge that makes its own existence impossible to overlook.

**Open state (push-mode by default).** Clicking the tab slides the tray out to approximately 35–40% of the viewport width as a full-height panel. **Expansion is push-mode by default**: Surface II content compresses to make room. Push-mode reads as "connected" to the finding, not as an overlay on top of it — the metric and its methodology are co-visible. At narrow viewports or when push-mode would compress the surface below usability, the tray falls back to overlay-mode; the trigger threshold is an implementation detail for `design_system.md`.

**Open state content.** A full-height panel with:

- The metric's provenance: algorithm, lexicon hash, extractor version, pipeline stage.
- Its tier classification and validation status, with expiration date if applicable.
- Known limitations in plain language (negation blindness, compound-word failure, dialect limitations, entity-linking gaps, etc.) drawn from the content catalog.
- Equivalence status for cross-context comparison per WP-004 §5.2 and §7.7 Dual-Register.
- Cross-links to the relevant Working Paper sections on Surface III ("Read the full argument").
- Dual-Register content: the semantic-register summary followed by the methodological-register prose (§7.7).

**Binding to the active metric.** The tray's content follows whatever metric or finding is currently in focus. Hovering or selecting a different metric updates the tray in place; no separate interaction is required. When the user selects a specific time point, source, or entity, the tray scopes to that choice and exposes only the provenance relevant to it.

**Reflection linkage.** Every tray view includes a prominent "Read the full Working Paper" anchor that opens the corresponding Surface III content at the relevant section. The tray is the summary; Reflection is the argument.

**What the tray is not.** The tray is not a settings panel, not a preferences drawer, not a chart-configuration surface. Its sole purpose is methodological context for the currently-observed data. Interactive chart controls live on the chart; the tray explains.

---

## 4. Three Surfaces and the Everywhere-Layer

The dashboard has **three surfaces** — not pages, not routes, but three fundamental ways to encounter AĒR — and one persistent overlay available on all of them.

The three surfaces are not co-equal in the user's time budget. **Surface II (Function Lanes) is where the user spends most of their analytical time.** Surface I (Atmosphere) is the landing overview — the welcome mat, not the living room. Surface III (Reflection) is where methodology becomes legible and where findings can be traced back to their scientific foundation.

Cross-context composition is not a fourth surface. As more probes are added, each of the three surfaces becomes richer: the globe shows more luminous points, the function lanes carry more contexts simultaneously, and Reflection gains more emic content to articulate. The Aleph principle forbids a dedicated "comparison surface" — multiple contexts belong together within the same atmosphere.

### 4.1 Surface I: Atmosphere (the landing overview)

The entry surface. A three-dimensional Earth, slowly rotating, against deep black. Ocean is a deep, almost-black blue; landmasses are restrained, slightly lighter blue silhouettes — rendered from vector polygons rather than satellite imagery, so the surface stays crisp at any zoom and never competes with the discourse signal it carries. No political borders, no country labels at the highest altitude. An optional borders layer (default off) can be revealed at Layer 2 once the user has descended below the atmospheric register.

**The role of Surface I is narrow and specific.** It answers three questions and then invites descent:

1. *What is AĒR currently observing?* Active probes are visible as luminous points.
2. *What does AĒR not observe?* Coverage gaps are positively marked as unmonitored (not merely dark).
3. *What is the shape of the dataset?* Probe count, source count, article count, active time range, dataset age — readable at a glance as compact stats woven into the chrome.

A researcher or curious visitor should form a correct first impression of the instrument's scope within seconds of landing. They should not feel that the globe is *the* work surface; they should feel it is the welcome mat leading them to the work.

**Probes as the emission unit; sources as presentation.** Consistent with the "probe is the unit of granularity" principle (§8.4), luminous points on the globe correspond to **probes**, not sources. A probe is selectable; its constituent sources appear as **visible satellites or read-only presentation around the probe glyph** — informational detail that shows the probe's composition without becoming a parallel selection frame. Clicking a satellite does not re-scope the descent; it opens the Probe Dossier (Surface II) with the source pre-filtered. See §6 for the terminology and §4.2 for the Dossier.

This deprecates the Phase 100a behavior where clicking a source glyph drove the descent. The new model is **probe-first selection with source presentation.** The probe view is canonical; the source view is ancillary.

**On each probe glyph:**

- **Core brightness** reflects publication volume in the selected time window. Pulse rate carries current activity density — a probe publishing 40 documents per hour pulses perceptibly faster than one publishing 5. The fastest pulse remains slow by human standards: this is still "stillness with motion beneath" (§1.1). Pulse rate is bounded.
- **Reach aura** surrounds each probe — a soft, translucent field indicating *where the probe observes*, not its impact. The aura is read from the Probe Dossier and may be a simple territorial region or a non-contiguous pattern of cultural affinity (diasporic probes, trans-national subcultures). Multiple overlapping auras compose additively.
- **Source satellites** appear around the probe glyph as read-only presentation — subtle markers indicating how many sources constitute the probe. Their visual style is muted relative to the probe glyph; they are informational, not interactive targets for scope selection.
- **Progressive Semantics** on every probe glyph carries both registers simultaneously (§7.7). The semantic register is prominent on hover: *"German institutional news: Tagesschau and Bundesregierung."* The methodological register expands on obvious affordance: *"Probe 0 — a constellation of sources covering Epistemic Authority and Power Legitimation discourse functions in German institutional discourse (WP-001)."* A visitor who has never heard the word *probe* can form a correct first impression.

**On the globe as a whole:**

- **The terminator** (day/night boundary) is real and current. Publication rhythms follow it. The terminator moves at Earth rotation speed — visually almost static, cosmologically alive.
- **Absence regions** — every area where AĒR has no active probe — are marked as *explicitly unmonitored*. Not dim, not greyed, but positively tagged. This is the Coverage Map from WP-001 §5.3 integrated into the first impression.
- **Propagation arcs (latent).** The engine reserves a rendering slot for Rhizome propagation — arcs tracing how discourse events move between probes. These arcs remain invisible until multi-probe data produces measurable cross-context propagation. When they appear, their motion is on the scale of minutes, not seconds. Until then, the slot is inert and carries no visual cost.

**Descent from Surface I.** Clicking a probe glyph (or using the keyboard shortcut) transitions off the globe into Surface II, scoped to that probe, opening the Probe Dossier as the default landing view within Surface II. The globe disappears entirely during descent to free the full viewport for scientific work; the return-to-Atmosphere affordance (planet glyph at the top of the left rail, §3.2) is available from every view thereafter.

The Atmosphere surface is kept minimal in cognitive weight, not in informational depth. A single glance shows: which cultural contexts are observed, how active each is now, and — negatively — which contexts are absent. The user sees a *constellation*, not a scoreboard.

### 4.2 Surface II: Function Lanes (the primary scientific surface)

The methodological backbone and the surface on which the user spends most of their analytical time. Not a secondary surface reached after Atmosphere; the primary place to do science with AĒR's data.

The structure of Surface II is **the four discourse functions from WP-001** (Epistemic Authority, Power Legitimation, Cohesion & Identity, Subversion & Friction) as horizontal lanes, each with its own time series, entity cloud, uncertainty band, and per-function methodology. The fifth-lane extensibility hook (§8.1) ensures that if WP-001 §8 Q2 produces a refined taxonomy, new lanes absorb into Surface II without layout redesign.

#### 4.2.1 The Probe Dossier — the bridge from Atmosphere

When the user selects a probe on Surface I, Surface II opens on the **Probe Dossier** as the default landing view — not on a random lane, not on an arbitrary default metric. The Dossier is the meta view; the user drills deeper from there.

The Probe Dossier is a **real, structured surface** — not a fly-out, not a tooltip. For the selected probe, it presents:

- The probe's **emic designation and classification.** The untranslated local name (WP-001 §4.2), the cultural context description, the etic functional classification (primary and secondary), and — critically — a short plain-language sentence explaining what a probe *is* in this context (§5 Probe Concept Introduction).
- The probe's **source composition.** The list of sources that constitute the probe, each with:
  - Per-source article counts (total, in current time range, publication frequency).
  - Per-source etic function classification (which function each source primarily serves).
  - Per-source emic designation and cultural context.
  - The source's accessibility metadata (platform type, access method, visibility mechanism, moderation context — per WP-003 §3.2).
- A **navigable article list per source**, with filters for time range, language, entity match, sentiment band, and temporal range. Opening an individual article enters L5 Evidence (§5).
- The probe's **methodology chrome**: WP-001 functional classification, WP-003 bias documentation, WP-006 observer-effect assessment. These are not footnotes; they are the navigation structure of the Dossier itself.
- A **function coverage indicator**: how many of the four discourse functions this probe's sources collectively address (Probe 0 covers 2/4 today — see §6). The coverage is a property of the probe, not a constraint on it.

The Dossier answers the first question a serious user has: *what is actually in this probe?* It is the connective tissue between the overview (Surface I) and the analytical work (function lanes, §4.2.2).

#### 4.2.2 The function lanes — a full working surface

From the Dossier, the user enters the function lanes proper — the per-function analytical space. Within each lane, for the selected probe's sources that primarily serve that function:

- **Time series** with uncertainty bands, at selectable temporal resolution (§7.5 WP-005 temporal scales).
- **Small multiples by source** — one panel per source within the function, on shared axes where within-context comparison is valid.
- **Entity clouds** — the NER output for that function's sources, with Epistemic Weight applied (§7.8).
- **Publication-volume density** — when publishing happens, not just what.
- **Within-context deviation views** — z-scores against the source's own baseline (per WP-004 §5.2 Level 2 comparability).

When multiple probes are active, each lane carries **parallel context streams** — the institutional German probe beside, say, a diasporic Telegram probe, each with its own baseline, its own within-context deviation signal, its own emic content. They appear together (composition) without being placed on a single common scale (comparison).

Empty lanes — function lanes for which the current probe(s) have no source coverage — are **not hidden and not greyed out.** They are present as empty lanes with the Dual-Register empty-lane invitation (§7.7). The empty lane *is* the question, operationalizing WP-006 §3.4.

#### 4.2.3 View modes — the analytical disciplines × presentation forms matrix

Within a lane, the user chooses **how** to see a given metric. View modes are organised as a two-axis matrix: **analytical disciplines** (what method is applied) × **presentation forms** (how it is rendered). The axes are independent — one discipline can be rendered in multiple presentation forms; one presentation form can serve multiple disciplines. A concrete view mode is a cell of this matrix, bound to a metric context.

**Analytical disciplines (non-exhaustive, extensible):**

- *Explorative Data Analysis (EDA)* — summarising and pattern-discovery without prior hypothesis.
- *Network Science / Network Analysis* — the structure of relationships between entities (authors, topics, sources, timestamps, cross-source co-occurrences).
- *Force-directed graph layout* — physics-based node positioning for dense relational data.
- *Clustering (unsupervised)* — automated grouping by statistical similarity (article clusters, topic clusters, affect clusters).
- *Metadata mining* — knowledge extracted from the descriptive layer (source, timestamp, language, entity labels) rather than the primary text.
- *Natural Language Processing* — the Gold extractors' native domain: sentiment, entities, language detection, and future extractors as they come online.

**Presentation forms (non-exhaustive, extensible):**

- *Time-series charts* — lines and areas with uncertainty bands.
- *Heatmaps* — color-coded density (e.g. day-of-week × hour, source × entity-label).
- *Correlation matrices* — grids showing pairwise strength.
- *Distributional plots* — ridgeline, violin, density.
- *Dynamic network graphs* — interactive node-link diagrams with real-time manipulation.
- *Topographic maps / themescapes* — 3D landscapes where topical relevance or density becomes elevation.
- *Small multiples* — per-source or per-context panels on a shared scale.
- *Parallel-context constellations* — multi-probe composition within a function lane (per §1.3).

The user selects the view; AĒR does not pick one "correct" chart. A metric like sentiment is not a time-series — it is a point in a space that can also show it as a per-source distribution, as a network of co-occurring entities coloured by sentiment, as a clustering of articles in affect space, or as a themescape of sentiment density across time.

**MVP target for Iteration 5: three view modes per metric, chosen from three structurally different cells of the matrix** — not three chart variants of the same discipline. For sentiment, a first set might be: a time-series (NLP × time-series), a per-source distribution (EDA × ridgeline), and an entity co-occurrence network coloured by sentiment (Network Science × force-directed graph). Three structurally different views, not three chart variants.

The catalog is extensible by the same principle as §8 (Extensibility). New disciplines and new presentation forms register as matrix cells without frontend redesign; the view-mode registry is the frontend's read of a backend-served catalog, not a hardcoded enum.

**Gold-boundedness caveat.** The MVP view-mode catalog is bounded by what current Gold data supports. Topic modelling and themescapes depend on extractors not yet in the pipeline; article clustering depends on affect-space or embedding-space infrastructure that Iteration 5 does not introduce. The MVP stays within what can be computed from sentiment, entities, language, temporal distribution, and word count — plus one corpus-level aggregation (entity co-occurrence) introduced specifically to unblock Network Science view modes (see ADR-020 on the CorpusExtractor scope).

#### 4.2.4 Scope parameter — probe and source are both first-class

View modes accept either a **probe scope** or a **source scope** as their parameter. Probe scope is the default (aggregating across the probe's sources); source scope is available from the Probe Dossier when the user narrows to a single source. Every view mode in §4.2.3 runs equally at either scope. The backend view-mode queries (ADR-020 backend-work section) accept either `probeId` or `sourceId`; the frontend's scope indicator (§3.2 left rail) shows which scope is active.

The user who wants *"sentiment for Tagesschau alone, without Bundesregierung in the aggregate"* has that view — it just reaches them through the Dossier's source-narrowing, not through a click on the globe.

#### 4.2.5 Exploratory composition mode (reserved and named)

A Kriesel-style free-form metadata exploration — where the user picks two or more metrics or dimensions and asks the instrument to show how they relate, via physical-string-like links between connection cards, co-occurrence networks, and temporal lead-lag views (WP-005 §6 Rhizome propagation) — is **reserved and named** for a later iteration. It uses the Relational Networks domain from §7.9 (Visualization Stack Separation), which is already architecturally reserved.

It is flagged as important but not blocking Iteration 5. The Function Lanes with multiple view modes per metric — §4.2.3 — is itself a full iteration's worth of work. Exploratory composition is the natural next step once that foundation is shipped.

#### 4.2.6 Within-context comparison is always on

Within-context comparison — this week vs. last week, this source against its own baseline — is supported without ceremony. Cross-context comparison remains governed by the equivalence registry (WP-004 §5.2) and the refusal surface (§7.4). When the user requests a cross-context normalization without established equivalence, the refusal surface explains why and offers alternatives.

### 4.3 Surface III: Reflection (the primary methodological surface)

The documentary surface, and — under the reframing — a central surface, not a peripheral one. Where prose is dominant. Where every metric's provenance, every probe's dossier, every known limitation, every observer-effect assessment, and every Working Paper is rendered as long-form, typographically disciplined content — not as tooltip content, not as modal fragments.

Reflection is where AĒR's commitment to "observation over surveillance" (Manifesto §I) is made legible. A dashboard without it is a surveillance tool; a dashboard where it is peripheral is a research tool with an alibi. Reflection must be central — reachable from every metric, every badge, every refusal — and it must reward its visit.

#### 4.3.1 Structure

Reflection hosts four kinds of content, each with its own navigation structure:

- **Working Papers** (WP-001 through WP-006 and future additions) rendered natively — not as PDF links, not as external references. Long-form prose with inline interactive illustrations (Distill.pub style) where manipulating a parameter on the page shows its effect on real Probe 0 data. The paper is not static; the paper is an interactive argument.
- **Probe Dossiers (methodology view)** — the methodological side of what Surface II shows structurally. The Probe 0 dossier in Reflection reads like a case study: why these sources, what they represent, what they miss, how they relate to the functional taxonomy.
- **Metric provenance pages** — one per metric in the pipeline. Algorithm, lexicon, training data, validation status, known limitations, cross-references to Working Papers. This is what the methodology tray (§3.3) summarises in its open state; Reflection is where the long form lives.
- **The Open Research Questions hub** — a dedicated page listing every open research question from WP-001 §8, WP-002 §7, WP-003 §7, WP-004 §7, WP-005 §7, and WP-006 §8. Each entry has its scope, its relevant pipeline hooks, and an invitation to interdisciplinary contribution — operationalizing Manifesto §V.
- **The "How to read the globe" primer** (see §4.5) — a short onboarding document introducing the probe concept, the four discourse functions, and what AĒR is and is not trying to see.

#### 4.3.2 Reachability

Reflection is **reachable from every metric, every badge, every refusal, and every Working Paper reference** as an obvious one-click link — the "read more" that is always visible.

- The methodology tray (§3.3) on Surfaces I and II includes a prominent "Read the full Working Paper" anchor that opens the corresponding Reflection content scrolled to the relevant section.
- Every refusal surface (§7.4) includes a "Why is this refused?" link that opens the relevant methodological discussion (e.g. WP-004 §5.2 for cross-context equivalence refusals).
- Every empty-lane invitation on Surface II links to WP-001's functional taxonomy.
- Every Epistemic Weight badge (§7.8) links to WP-002's validation framework.

Reflection is not a destination the user visits once. It is the methodological index of the dashboard, entered frequently, by trajectory rather than by intention.

#### 4.3.3 Interactive arguments

Reflection's distinguishing feature — beyond long-form prose — is **inline interactivity**. A Working Paper about sentiment calibration (WP-002) embeds real Probe 0 data as the argument's demonstration: the reader manipulates a parameter, the embedded chart updates, the argument's claim is tested on real data rather than illustrated abstractly.

This is the Distill.pub pattern explicitly. It requires the visualization-stack separation of §7.9 to work — the same Observable Plot, uPlot, D3, and MapLibre+deck.gl modules that power Surface II also power the inline illustrations in Surface III's prose. The paper and the dashboard share a vocabulary.

#### 4.3.4 Why Reflection being central is not optional

Three of AĒR's commitments make Reflection central by construction:

- **Scientific Integrity & Transparency** (Quality Goal 1). A metric without accessible methodology is a black box. AĒR cannot be a black box.
- **Methodological Transparency** (Manifesto §VI). "Disclosed openly through the Working Paper series, the Arc42 documentation, and the codebase itself." The dashboard is the interface to that disclosure for non-engineers.
- **Reflexive Architecture** (ADR-017 / WP-006). The instrument that knows it is an instrument must expose its own limitations. Reflection is where those limitations are read.

A Reflection that is unused is a failed dashboard, regardless of how good the other surfaces look.

### 4.4 The Everywhere-Layer: Negative Space

Not a surface. A persistent, user-toggled overlay available on every view. Named — for now — **"What AĒR doesn't see"**.

When activated on the Atmosphere surface, unmonitored regions become more prominent, not less. On the Function Lanes, empty lanes gain explicit demographic-skew annotations (WP-003 §6). On Reflection, known limitations scroll into the margin.

**Demographic opacity.** The overlay is also the visual home of demographic absence. WP-003 §6.1 documents what is systematically missing from digital discourse — older cohorts, non-dominant languages, authoritarian-context voices, populations behind access barriers. These absences intensify as the descent goes deeper: at L0 (Immersion) the overlay shows regional coverage gaps; at L3 (Analysis) it adds demographic-skew annotations on the active context; at L4 (Provenance, the methodology tray) it foregrounds the full WP-003 bias profile. The absence is visible at the level of detail where it matters for that view.

This is WP-006 §8.3 Q6 operationalized. Absence is not a failure state to be hidden; it is a first-class feature of the instrument. The toggle's existence answers the question WP-006 asks of every dashboard: *can the user see what the instrument is blind to?*

### 4.5 Introducing the probe concept in context

Surface I's demotion from "scientific entry point" to "landing overview" exposes a related risk: **the word "probe" carries no meaning for a first-time visitor.** A researcher steeped in WP-001 reads the glyph and understands; anyone else sees a labeled dot with no frame for it. The brief commits to introducing the probe concept *in context*, not assuming it.

The mechanism is already a principle of the brief: **Progressive Semantics** (§7.7). Every probe glyph on Surface I carries both registers simultaneously:

- **Semantic register (prominent).** Plain-language identity — *"German institutional news: Tagesschau and Bundesregierung."* This is what any visitor reads first.
- **Methodological register (on obvious affordance).** The probe's formal identity — *"Probe 0 — a constellation of sources covering Epistemic Authority and Power Legitimation discourse functions in German institutional discourse (WP-001)."* This expands inline, in place, without leaving the globe.

Surface III hosts a **"How to read the globe" primer** — linked from Surface I's L0/L1 chrome — that introduces the probe concept, the four discourse functions, and what AĒR is and is not trying to see. First-visit onboarding (an optional overlay) is available for visitors entirely new to AĒR.

The principle: a visitor who has never heard the word *probe* should form a correct first impression from the globe alone, with an obvious path to the full methodological frame. **Probe fluency is earned by using the dashboard, not demanded at the door.**

---

## 5. The Five Layers of Progressive Descent

Each surface is navigable by depth. The five layers are the governing interaction architecture of the dashboard and are uniform across all three surfaces. The reframing redistributes where each layer primarily lives.

| Layer | Name | User stance | Primary home | What is absent |
|---|---|---|---|---|
| **0** | **Immersion** | Watching | The active surface in its quietest form | All UI chrome beyond the left rail |
| **1** | **Orientation** | First engagement | The active surface with a soft context overlay | Controls beyond "where am I" |
| **2** | **Exploration** | Steering | The active surface with filters, toggles, zoom | Synthesis the backend would refuse |
| **3** | **Analysis** | Measuring | **Surfaces II and III natively.** Not Surface I. | Composite "health scores"; cross-context without equivalence |
| **4** | **Provenance** | Auditing | **The methodology tray (§3.3), reachable from any surface.** | Reassurance language |
| **5** | **Evidence** | Verifying | **A reader-pane overlay, reachable from any surface.** | Interpretation |

**The redistribution (key reframing change).** In prior iterations, L3 Analysis, L4 Provenance, and L5 Evidence were described as companion panels on Surface I (the globe). The reframing moves them off the globe:

- **L3 (Analysis)** lives natively on Surface II (within a function lane, with the view-mode matrix) and on Surface III (within an interactive Working Paper illustration). Surface I does not host L3.
- **L4 (Provenance)** is the dedicated role of the methodology tray (§3.3). The tray is reachable from every surface and every layer. There is no "Provenance panel"; there is "the methodology tray, opened."
- **L5 (Evidence)** is a reader-pane overlay that surfaces the individual Bronze document behind a metric point. The overlay is reachable from Surface II (click a chart point), Surface III (inline source citations), or the Probe Dossier's article list. Not from Surface I.

### 5.1 Governing rules of the descent

1. **Each layer must be reachable from the one above it in one interaction.** No hidden navigation, no deep menus. The user is always one click, tap, or keypress from going deeper or returning.
2. **Each layer adds; no layer replaces.** A user who reaches Layer 3 does not lose the context of Layer 0 — the left rail, top scope bar, and methodology tray remain visible.
3. **A user can stop at any layer.** Layer 0 is a complete, valid experience. A contemplative viewer need never leave it. A forensic analyst who descends to Layer 5 does not feel the upper layers were wasted.
4. **Refusal is visible at the appropriate layer.** If the BFF returns HTTP 400 on `?normalization=zscore` (ADR-016), the refusal manifests at Layer 2 (where the user requested it), with the explanation surfaced in the methodology tray (Layer 4). See §7.4.
5. **The descent is reversible in both directions.** Deep-linking into Layer 4 or Layer 3 is a legitimate entry point (e.g. someone sharing a methodology reference). The ascent from Layer 4 to Layer 0 must remain intuitive — and the left rail makes it so.

### 5.2 The Surface × Layer matrix

Surfaces and Layers are orthogonal. Every surface has L0–L2 natively; L3–L5 live per the redistribution above. The matrix below makes this concrete.

| | **Surface I: Atmosphere** | **Surface II: Function Lanes** | **Surface III: Reflection** |
|---|---|---|---|
| **L0 — Immersion** | Rotating globe. Probe points as luminous dots with source satellites. Terminator live. Minimal chrome (left rail only). | Landing on the Probe Dossier in its quietest form. No filters expanded. Four lanes visible at baseline density. | A single long-form essay at reading width. Minimal chrome. |
| **L1 — Orientation** | Soft overlay in the top scope bar: current time range, active probe count, normalization mode, active contexts. Region label under cursor. | Overlay in the top scope bar: active lane, current time window, equivalence status. Empty-lane captions appear. Probe Dossier navigation visible. | Breadcrumb in the top scope bar: which Working Paper, which section, which WP reference. Reading progress indicator. |
| **L2 — Exploration** | Time-range scrubber. Resolution switch. Region zoom. Pillar mode toggle. Probe selection. | Lane selection. View-mode switcher. Source-scope narrowing from the Dossier. Cross-lane overlay toggle. Raw/z-score switch (refused where appropriate — §7.4). | Linked interactive elements within the prose become manipulable. Chart parameters are adjustable inline. |
| **L3 — Analysis** | *Not the native home of L3.* Selecting a probe transitions to Surface II L3 with the Probe Dossier. | **Native home.** Within a lane: time series, small multiples, entity clouds, per-source distributions, view-mode matrix selection, within-context deviations. Per-source analysis from Dossier narrowing. | **Native home.** Embedded interactive visualizations gain full chart controls. Reading remains primary; analysis supports the text. |
| **L4 — Provenance** | *Not the native home of L4.* The methodology tray (right edge, §3.3) is always accessible; for Surface I it is minimized until a probe is selected. | **The methodology tray is open here.** Per-metric provenance, tier, known limitations, equivalence level, cultural context notes, observer-effect status, linked Working Paper anchors. | **The methodology tray is open here, containing the metric's summary.** The prose itself is the long-form provenance. |
| **L5 — Evidence** | *Not the native home of L5.* | Click a chart point → the reader-pane overlay appears, scoped to that lane's source. The Bronze document is shown with trace metadata. | Inline "example source" affordances in the prose open the same reader-pane overlay. |

**How to read the matrix.** Each cell describes *what the user sees at this depth on this surface*. The matrix is the architecture — not the implementation. Cells marked "not the native home" mean the layer does not originate here; the user who wants L3/L4/L5 transitions to the natural host surface. This is not a restriction on the user — it is a discipline on the dashboard.

### 5.3 A narrative walkthrough: from breath to evidence

To make the matrix tangible, one complete descent as a single user journey. The researcher has never used AĒR before.

**Arrival.** She opens the dashboard. The globe appears — Europe turning into view, dawn terminator crossing the Atlantic. One luminous point over Germany, with two small satellites close by. Black everywhere else on the continent. She watches for a few seconds. The left rail is visible — three surface anchors with clear labels, a compact scope indicator showing *"No probe selected · last 24h."* A narrow right-edge tab reads *"Methodology"*. [Surface I, Layer 0]

**Orientation.** She moves the mouse. The top scope bar softens into view: *"Probe 0 · 1 active probe · 2 sources · 47 documents · last 24 hours."* Her cursor crosses the Atlantic — no label. The absence is honest. [Surface I, Layer 1]

**First exploration.** She hovers the luminous point. A soft caption unfolds: *"German institutional news: Tagesschau and Bundesregierung."* An obvious affordance invites expansion. She clicks it. The caption grows: *"Probe 0 — a constellation of sources covering Epistemic Authority and Power Legitimation discourse functions in German institutional discourse (WP-001)."* She has just met the concept of a probe in context. [Surface I, Layer 1 + §7.7]

**Descent to the Probe Dossier.** She clicks the probe glyph. The globe slides away; Surface II opens on the Probe Dossier. The left rail now highlights Function Lanes; the scope indicator carries *"Probe 0."* The dossier shows two source cards (Tagesschau, Bundesregierung), per-source article counts, per-source etic classification, function coverage (2/4). Each source card is expandable. [Surface II, Layer 1]

**Into a lane.** She clicks "Epistemic Authority." The Dossier compresses; the Epistemic Authority lane expands with a default view: Tagesschau's sentiment time series, publication volume density, entity cloud. A view-mode switcher in the top scope bar reads *"Time series · Distribution · Network."* [Surface II, Layer 3]

**View-mode exploration.** She switches to *Distribution*. The time series is replaced by a ridgeline plot showing sentiment density per week over the last six months. She switches to *Network*. A force-directed graph appears showing entity co-occurrences, coloured by the sentiment of the articles they appeared in. Three views, three structurally different lenses on the same metric. [Surface II, Layer 3 + §4.2.3]

**Provenance encounter.** The right-edge methodology tab has been showing a subtle indicator — *Tier 1 unvalidated*. She clicks it. The tray slides out, Surface II compresses. The tray reads: *"Sentiment (SentiWS v2.0 lexicon) · Tier 1 · Unvalidated · Score is the arithmetic mean of matched token polarities. Known limitations: negation blindness, compound-word failure, register-neutral institutional language tends to produce scores close to zero. Use with care. Read the full Working Paper →"* [Surface II, Layer 4 via methodology tray]

**Reading turn.** She clicks the "Read the full Working Paper" link. Surface III opens to WP-002, scrolled to §3 (State of the Art). The methodology tray remains open on the right, showing a short summary of what she is reading. An interactive chart in the prose lets her adjust a parameter and see the effect on real Probe 0 data. She reads for ten minutes. [Surface III, Layer 3]

**Refusal encounter.** Back on Surface II, she tries to toggle raw/z-score, hoping to see the data on a different scale. The toggle dims. A soft inline refusal appears — the Dual-Register refusal from §7.4. The semantic register reads *"This transformation requires established equivalence across contexts — not yet available for sentiment on a single probe."* She expands the methodological register, which names the missing equivalence-registry row and links to WP-004 §5.2 in Reflection. The refusal is not a dead-end; it is a turn in the road. [Surface II, Layer 2 + §7.4]

**Evidence verification.** A sentiment dip on 2026-03-14 catches her eye. She clicks the point. A reader-pane overlay slides up, scoped to that document: the raw cleaned text, the source, the timestamp, the trace ID. No interpretation, no labels. She reads it herself. The pattern was a budget announcement; the lexicon picked up on several negation-sensitive phrases — exactly the limitation the methodology tray had named. [Surface II, Layer 5]

**Return.** She closes the reader pane. She clicks the planet glyph at the top of the left rail. Surface I returns — globe, probe point, the same dawn terminator, the same stats. Her scope is preserved. She is back where she began, having left and returned with something she did not have before: not an answer, but a better-shaped question.

### 5.4 The Negative Space overlay across the matrix

The Negative Space overlay is a perceptual modifier — it does not add content, it re-weights it. Activating the overlay (one keyboard shortcut, one toggle) changes what the user sees at every layer on every surface:

| | **With overlay off** | **With overlay on** |
|---|---|---|
| **Atmosphere L0** | Probe points luminous; absence is dark. | Absence regions become positively marked — a subtle atmospheric field indicates "no observation here." |
| **Atmosphere L1** | Active probe count. | Active + *missing* probe-function count per region. Coverage Map (WP-001 §5.3) becomes legible. |
| **Function Lanes L0–L2** | Empty lanes present but quiet. | Empty lanes gain prominence: what a probe *would* need to contribute, which contexts are unaddressed. |
| **Function Lanes L3** | Charts of available data. | Charts include explicit demographic-skew annotations (WP-003 §6.1) in the margin. |
| **Reflection L0–L3** | Prose about what AĒR observes. | Prose about what AĒR cannot observe scrolls into the margin alongside the primary text. |
| **Methodology tray (L4)** | Known limitations listed as methodological context. | Known limitations *lead* the tray display — the first thing shown, not the last. |

The overlay is not a "dark mode." It is a **stance mode** — the user declares "I want to see what is missing," and the interface re-organizes its emphasis without changing its structure.

### 5.5 Fractal Cultural Granularity

The descent through layers is simultaneously a **hermeneutic deepening** (from contemplation toward evidence) and a **cultural narrowing** (from broad co-presence toward a single voice). These two motions are not separate axes the user controls — they are **one continuous movement** with both properties.

**The motion is fluid, not stepped.** There are no discrete cultural zoom levels — no "regional → national → institutional → milieu" ladder. Such a ladder would reproduce precisely the Western administrative taxonomy that WP-001 §2 warns against. Cultural granularity is instead **probe-defined**: each probe carries its own emic structure (WP-001 §4.2), and what counts as "the cultural context" at any descent depth is whatever the active probe declares it to be.

**Different probes have different granularity shapes.** A probe on "institutional epistemic authority in Germany" has a relatively flat cultural structure — the relevant emic categories are institutional. A probe on "diasporic identity-formation across Arabic-speaking Telegram channels in Europe" has a layered structure — national diasporas, generational cohorts, linguistic registers, religious affiliations. A probe on "subversion and friction in Russian post-2022 exile media" has yet another shape. There is no single hierarchy that contains all three. This is Foucault's episteme taken seriously.

**The Pillars are fractal.** Aleph, Episteme, and Rhizome are not tied to specific descent depths. They act at every depth — but with the cultural frame appropriate to that depth:

- **Aleph** is strongest at L0–L1: the *co-presence of multiple contexts* as the first impression.
- **Episteme** is strongest at L2–L3: as the user narrows to a probe and a time range, the *boundaries of the expressible in that specific discursive formation* become the subject.
- **Rhizome** is strongest at the *transitions* between depths and between probes once a constellation is active: how a topic moves from one context to another.

A user on Surface I at L0 is in an Aleph-dominant mode. A user on Surface II at L3, focused on one probe in one lane, is in an Episteme-dominant mode. A user who moves between probes, or follows a topic across them, is activating Rhizome.

**Practical consequence.** The dashboard must never render a cultural-granularity control as a fixed selector ("Region → Country → Institution"). Where granularity appears as a control, it reads from the active probe's emic structure, which may have two levels, or seven, or none beyond the probe itself. The probe is the unit of granularity; the probe defines its own depths.

**Relationship to k-anonymity.** At L5 (Evidence), where the cultural reach narrows to a single voice, WP-006 §7 gates apply: a retrieved document must be part of an aggregation of at least *k* = 10 source documents for the metric being viewed. Probes that cannot meet this threshold (very small, very specific) cannot descend to L5 through the dashboard — the descent is refused at the moment of request with an explanation. This is not a limitation of the design; it is the design respecting the ethical commitment.

---

## 6. Terminology: Probes, Sources, and the WP-001 Alignment (Path B)

The word *probe* carries a slight mismatch between the scientific foundation (WP-001) and the current engineering usage. This section records the mismatch explicitly so future readers are not blindsided, and defers reconciliation to a post-Step-2 ADR.

**WP-001 usage.** §3–§5 treat a *probe* as an individual observation point — 1:1 with a source, carrying one etic function tag and one emic designation. The multi-source grouping that covers multiple discourse functions for a society is called a *probe constellation* or *Minimum Viable Probe Set (MVPS)* in WP-001 §5.1.

**Current engineering usage.** The system calls the grouping a *Probe* (e.g. "Probe 0 contains two sources: `tagesschau.de` and `bundesregierung.de`"). The individual observation points are called *sources* and live in the PostgreSQL `sources` table.

**What a probe (in the current sense) actually is.** A **grouping of sources defined by a shared cultural and discursive scope**, not by matching etic/emic tags. Sources within a probe typically carry *different* etic function tags — that is precisely what makes the constellation analytically useful per WP-001 §5.1 (one probe covers multiple functions for one society). Probe 0's two sources have different primary etic functions (Epistemic Authority vs. Power Legitimation) and different emic designations. Two probes in different countries are not required to have matching tag sets, matching source counts, or matching function coverage.

A probe's **function coverage** is a derived property: how many of the four WP-001 discourse functions the probe's sources collectively address. Probe 0 covers 2/4 today. Function coverage is a useful visualisation in the Probe Dossier and on the Coverage Map; it is not a constraint on what a probe can be.

**Path B decision (2026-04-24): keep current usage for Iteration 5.** The engineering terminology — "Probe" = grouping, "Source" = individual, matching the PostgreSQL schema and existing API surface — stays as-is. The brief names the drift here so new readers understand the mismatch but does not propagate a terminology change through code, schema, or endpoints. A post-Step-2 ADR revisits reconciliation once the dashboard is shipping.

Why Path B, not full reconciliation: terminology surgery mid-rewrite is high-risk for little Iteration 5 value. The mismatch is a documentation clarity issue, not a correctness issue — the architecture behind the words is coherent. A later ADR handles any rename with full engineering scope.

**Consequence for the brief.** Wherever this brief uses "probe," it means the grouping in the current engineering sense (= a WP-001 probe constellation). Wherever it uses "source," it means the individual observation point (= a WP-001 probe). Readers coming to the brief directly from WP-001 should mentally translate: "probe" in the brief = "probe constellation" in WP-001; "source" in the brief = "probe" in WP-001.

---

## 7. Design Principles

These are the rules any screen, component, or interaction is checked against.

### 7.1 Ockham's Razor is visible

Arc42 §1.1 commits AĒR to Ockham's Razor. The dashboard extends this commitment to its own visual grammar. Every chart junk element is a small violation. Every decorative gradient without informational content is a small violation. Ornamentation that does not encode data is forbidden at Layers 2–5; at Layer 0 (Immersion), ornamentation is permitted only if it carries atmospheric meaning (e.g., the terminator on the globe is ornamental *and* informational).

### 7.2 No valence, ever

No red-green axis. No "warm" palettes on one extreme. No normative labels ("high", "concerning", "healthy"). The `visualization_guidelines.md` §1–§2 constraints are not negotiable and are not to be softened for "legibility" or "intuitiveness."

A sentiment score of −0.4 appears as its numeric value with its uncertainty band, colored on a perceptually uniform Viridis scale. It does not appear as an orange bar trending downward with a concerned-face icon. The dashboard's job is to make the user *ask* whether −0.4 is a meaningful shift, not to tell them.

### 7.3 The Question-First Frontend, with Answers Where Tenable

WP-006 §3.4 states the principle: *the dashboard invites questions, not conclusions*. This translates into concrete interaction design:

- Every primary number is paired with its uncertainty (shading, bands, status badges).
- Every chart has a visible "why this shape?" affordance at Layer 3 that opens the methodology tray at Layer 4.
- Empty states are never apologetic ("no data available"); they are explanatory.
- Within-context trends (this week vs last week, this source against its own baseline) can be stated directly — they are claims the data supports.
- Cross-context statements are governed by the equivalence registry and the Epistemic Weight principle (§7.8). Where equivalence is established and validation complete, AĒR *does* make the composed observation. Where equivalence is unestablished, AĒR refuses the claim (§7.4), not approximates it.

**The posture:** AĒR does not hide behind "it's complicated." Where the methodology supports a claim, AĒR makes the claim clearly. Where it does not, AĒR refuses clearly. What AĒR never does is produce false confidence or false modesty.

### 7.4 Refusal as a first-class feature

When the BFF returns HTTP 400 (e.g., on `?normalization=zscore` for a metric without a registered equivalence entry), the frontend does not render an error state. It renders a **methodological statement**, built according to Progressive Semantics (§7.7).

The shape of a refusal surface is:

1. A one-sentence refusal in the semantic register — what the user asked, and why it cannot be answered, in plain language.
2. An expand affordance leading to the methodological register — the exact table/gate/rule that triggered the refusal, the relevant Working Paper anchor, the Scientific Operations Guide workflow for resolution.
3. Alternatives — actions the user *can* take that produce valid answers (raw values, different metric, different probe subset).

The refusal is not a dead-end. It is a turn in the road. It preserves the user's agency (alternatives) while protecting the scientific integrity of the system (no silent approximation). This is WP-006 Principle 5 (Interpretive Humility) and Manifesto §II ("Resonance over Truth") made interactive.

### 7.5 Long time, long data, long sessions

The dashboard is designed for sessions that may last hours, not minutes. This shapes defaults:

- **Dark Mode as the primary visual mode.** Light mode is available but is not the default. The default respects the long-session reality of scientific and journalistic work.
- **Typography designed for extended reading.** Inter Variable for UI, IBM Plex Mono for numerical data. Generous line-height on Surface III where prose dominates.
- **No auto-refresh that breaks attention.** Data updates are signaled subtly; the user controls when to incorporate them.
- **Deep-linkable state.** Every filter configuration, every time range, every viewing mode is encoded in the URL. A conversation about AĒR is a conversation about shareable URLs, not screenshots.

### 7.6 Honest Desktop-First with High-Fidelity and Low-Fidelity Modes

The primary consumption environment is a wide screen at reading distance — a researcher's laptop, an analyst's monitor, a journalist's desk. The dashboard is designed for this honestly, without pretending mobile-first.

But a brilliant researcher working on a 2012 laptop must have a complete AĒR experience. The dashboard has two rendering modes, with identical scientific depth but different computational footprints.

**High-Fidelity mode (default on capable hardware).** Full Atmospheric Aesthetics: 3D globe with WebGL, real-time terminator, motion on atmospheric timescales, deep visual layering. All five depth layers on all three surfaces.

**Low-Fidelity mode (fallback or explicit user choice).** The 3D globe is replaced by a static or slowly-updating 2D equirectangular map with identical informational content — probes, terminator, absence regions. Motion is removed. All surfaces, all five depth layers, all scientific features are preserved. **The only things reduced are the computational costs that would make the experience janky on weak hardware.** Progressive Semantics is preserved in text-only form. Refusal surfaces are identical. The methodology tray is identical. Surface III is essentially unchanged — it was already prose-centric.

The rule: **a feature is removable from Low-Fidelity mode only if, without removal, it would make the mode non-functional on the target hardware.** Scientific depth is never the trade-off. Immersion is.

**Mode selection.** Low-Fidelity is entered automatically when WebGL2 is unavailable or reports a known-slow GPU, `prefers-reduced-motion: reduce` is set, effective connection type reports 2G/slow-3G, or the user explicitly chooses it. Every automatic trigger is overridable by the user.

**Small-screen view.** Tablet and phone layouts use Low-Fidelity mode by default, with additional layout adjustments for narrow screens. The left side rail collapses to a bottom tab bar; the methodology tray collapses to a modal sheet. This is not a second implementation — it is Low-Fidelity with adjusted breakpoints.

### 7.7 Dual-Register Communication (Progressive Semantics)

Every number, every refusal, every empty state, every probe glyph, every status badge exists in **two simultaneously valid registers**:

- **Semantic register.** What does this mean? Plain language, connected to everyday experience, not dumbed down. Assumes an intelligent adult without specialist training.
- **Methodological register.** How does this come to be? Algorithm, lexicon, tier, equivalence status, known limitations. Assumes a user who wants to verify or critique.

Both registers exist for both audiences. The researcher benefits from the semantic register (it is often a good sanity check on what she thinks she understands). The informed non-specialist benefits from the methodological register (it teaches, over time, how AĒR thinks). The dashboard must not segregate users into lanes.

**The solution: Progressive Semantics.** Both registers are present in the rendered output, but only one is prominent at a time. The transition between them is a micro-interaction local to the artifact — hover, focus, expand — never a page navigation or modal dialog. The user's spatial focus on the data point never breaks; only the framing of that point shifts.

Concretely:

- The semantic register is **primary at Layers 0–3**. It is what appears when the user first engages with a number, a probe glyph, or a state.
- The methodological register is **primary at Layer 4** — which is the methodology tray (§3.3).
- At Layers 0–3, a compact affordance (typically a small inline badge or a cursor-tracked glyph) signals "there is a methodological register available here." Engaging it expands the methodological content in place, without losing the semantic framing.
- At Layer 4 (tray open), the methodological prose dominates; a sibling affordance provides the semantic summary.

Progressive Semantics also applies to **probe onboarding** (§4.5). A probe glyph's semantic register is the plain-language identity ("German institutional news: Tagesschau and Bundesregierung"); its methodological register is the formal taxonomy classification ("Probe 0 — constellation covering Epistemic Authority and Power Legitimation in WP-001 terms"). A first-time visitor meets probes through the semantic register and can expand to the methodological register when ready.

### 7.8 Epistemic Weight

The visual prominence of a displayed metric scales with its methodological backing. This is an architectural requirement, not a stylistic preference.

**The problem.** AĒR's current pipeline is a proof-of-concept. All five Phase 42 extractors are Tier 1, unvalidated. Any claim built on them is provisional. The Working Papers anticipate this growing substantially: Tier 2 methods with Krippendorff's α ≥ 0.667 (WP-002 §4), cross-context equivalence registries (WP-004 §5.2), longitudinal validation with stability windows. When validations complete, a metric becomes substantively more capable of supporting claims — and the dashboard must reflect that growth, not treat every metric with identical epistemic caution.

**The principle.** The visual weight of a rendered metric is a function of its backing:

| Backing | Visual treatment |
|---|---|
| Tier 1, unvalidated (today's default) | Moderate weight. Default line thickness. Status badge present. Provisional status visible without interaction. |
| Tier 1, validated | Full weight. Clean rendering. Validation badge with expiration date visible on Layer 1 hover. |
| Tier 2 with established validation | Full weight. No caveat marks at Layer 0. Cross-context equivalence, where present, is carried as part of the metric's identity. |
| Tier 3 (LLM-augmented, non-deterministic) | Visible only through Progressive Disclosure from a Tier 1/2 baseline (per ADR-016). Distinct styling (e.g., dashed line weight) signaling its non-deterministic nature. Never a primary display. |
| Expired validation | Reverts to Tier 1 unvalidated treatment. The expiration is shown prominently. |

**Why this is not the opposite of Non-Prescriptive Visualization.** Epistemic Weight and non-prescriptive visualization are complements. Non-prescriptive means: do not smuggle normative judgment into visual encoding (red-green for sentiment). Epistemic Weight means: let the scientific standing of a metric be visible in the metric's rendering. The first is about value judgments on *content*; the second is about confidence judgments on *method*.

### 7.9 Visualization Stack Separation

AĒR's visualization needs span four distinct domains, each with its own performance, interaction, and rendering requirements. The principle: **each domain is served by a framework-agnostic rendering module, and the UI framework is responsible only for chrome.**

| Domain | Purpose | Technology (per ADR-020) | Where it appears |
|---|---|---|---|
| **3D Atmosphere** | The rotating globe, probe points, terminator, absence fields, atmospheric halo, source satellites, reserved Rhizome propagation slot. | **three.js** as vanilla engine module with custom GLSL shaders. No declarative framework wrapper. | Surface I. |
| **2D Geo-Analytics** | Flat maps for regional or sub-regional analysis within a probe context. Choropleths, density maps, proportional symbols, flow-lines. | **MapLibre GL JS** (base layer), **deck.gl** (data overlays). | Surface II, L3, when geo-distribution within a single probe is the subject. |
| **Scientific Charts** | Time series, small multiples, histograms, violin plots, ridgeline plots, heatmaps, streamgraphs, correlation matrices. | **uPlot** (dense time series >1k points). **Observable Plot** (declarative scientific grammar). **D3** (utility for custom visuals). | Surface II and III, L3–L4. |
| **Relational Networks** | Discourse propagation (Rhizome), entity co-occurrence, cross-probe narrative diffusion. 2D force-directed, 3D spatial graph. | **D3-force** (2D). **three.js** (3D, reused from 3D Atmosphere). | Surface II L3 under Network Science view modes; exploratory composition mode (§4.2.5). |

**What Relational Networks actually shows.** This domain was architecturally reserved in earlier iterations with no design articulation. Under the reframing, it concretely serves three purposes:

1. **Entity co-occurrence networks** within a function lane — nodes are entities (persons, organizations, locations), edges are co-occurrence in the same document, node weights reflect frequency, edge weights reflect co-occurrence count. Colored by sentiment, by discourse function, or by temporal cohort.
2. **Cross-source propagation graphs** (Rhizome pillar mode with multi-probe data) — nodes are probes or sources, edges are directed lead-lag relationships, edge weights reflect strength of cross-source topic or entity signal propagation.
3. **Exploratory composition mode** (§4.2.5, reserved for a later iteration) — the Kriesel-style free-form metadata exploration where the user picks dimensions and the instrument renders their relations as a physical-string network with nodes that can be rearranged by direct manipulation.

Each visualization module is a **pure rendering module**: receives data, produces output. Framework-agnostic. The UI framework wraps with a thin adapter but never dictates internals. This has five consequences:

1. **Framework choice is liberated from visualization quality.**
2. **Bundles are per-domain, not per-framework.** three.js only enters when Surface I renders. Observable Plot only when Surface II or III descends to L3.
3. **Performance budgets are per-domain.**
4. **Domains can be tested in isolation.**
5. **Swapping a framework is cheap.**

**What this rules out.** No chart-library tightly coupled to a specific UI framework (Recharts, Chakra-UI charts, Mantine charts). No "visualization platform" that owns both the chart and the surrounding layout (Plotly Dash, Streamlit, Observable Framework). No commercial map tiles that would tie AĒR to a vendor. No framework-specific 3D wrappers (react-three-fiber, Threlte, Solid-Three — even though excellent).

---

## 8. The Extensibility Commitment

This is a core principle of the brief, not an afterthought. AĒR's six Working Papers identify over twenty open research questions. Every one of them may, when answered, require a new surface element, a new lane, a new metric, a new pillar, a new view mode, or a new dimension entirely. The dashboard must absorb this growth without structural redesign.

### 8.1 Structural openness by construction

**Functions are not four.** WP-001 §3 names four discourse functions, but WP-001 §8 Q2 explicitly asks whether this taxonomy is sufficient. A fifth function, or a split of an existing function, must be absorbed by Surface II without redesign. The lane metaphor is chosen precisely because it is a list — adding a lane is a data change, not a layout change.

**Pillars are not three.** Arc42 §1.2 names three pillars. The WP series does not foreclose the possibility of a fourth. The viewing-mode toggle is a set, not a tab strip.

**Metrics are not five.** Every metric view must accept new metrics as a catalog entry — no view is hardcoded to specific metric names.

**Tier treatment is not fixed.** Validation status changes — a Tier 1 metric may complete Workflow 2 and become validated; a Tier 2 metric may expire and revert. The Epistemic Weight treatment (§7.8) must read validation status live from `/api/v1/metrics/available` and `/api/v1/metrics/{metricName}/provenance`, not from a frontend constant.

**Temporal scales are not five.** The resolution switch is a range slider over a semantic scale, not a fixed button group.

**Equivalence levels are not three.** The badge system reads equivalence as a classified string from the API.

**View modes are a catalog, not a menu.** The analytical-disciplines × presentation-forms matrix (§4.2.3) is extensible. New disciplines (e.g. diachronic word-embedding drift) and new presentation forms (e.g. chord diagrams, Sankey flows) register as matrix cells. The frontend reads the catalog from the BFF's content layer; it does not hardcode cells.

**Probes are not one.** Everything assumes Probe 0 is the first of many. The Atmosphere surface is infinitely scalable; the Coverage Map grows as new probes are registered; the multi-probe composition activates *automatically* the first day a second probe's data arrives. A globe with seven luminous points is the same globe; function lanes with three context streams per lane are the same lanes.

**Emic content is not hardcoded.** WP-001 §4.2 requires that emic designations and cultural context travel with each probe as untranslated, source-of-truth documentation. The dashboard renders this content from the database and the Probe Dossier directory — no emic copy is baked into frontend source.

### 8.2 The additive extension test

Any proposed surface, component, or interaction is checked against: *what happens when a new probe is added from a different cultural setting? A new discourse function? A new pillar? A new metric just validated? A previously validated metric that expired? A new view mode (new discipline, new presentation form, or new cell)? A new limitation class? A probe whose cultural granularity has two levels, or seven?* If the answer is "the frontend needs a release" for anything other than displaying new methodological prose, the design is wrong. The dashboard grows with the science, not after it.

### 8.3 What this rules out

This principle explicitly rules out:

- Hardcoded iconography for "the four functions" or "the three pillars."
- Layout logic that assumes a specific number of lanes, metrics, resolutions, or view modes.
- Copy that names specific probes, sources, or metric types in fixed positions.
- Charts that cannot accept new metric types without code changes.
- View-mode registries that enumerate cells in frontend source rather than consuming a backend catalog.
- Region visualizations that hardcode which parts of the map are "covered."
- Dual-Register semantic copy (§7.7) written into the frontend codebase for specific metrics — semantic explanations must come from a content layer (API-backed or versioned content files).
- Emic designations translated into the frontend's UI language — `Tagesschau` is `Tagesschau`, not "German Public TV News."
- A separate "cross-cultural comparison" surface — cultural composition belongs within the existing three surfaces, not in a dedicated arena.
- Epistemic Weight rules hardcoded per metric — the treatment is derived from live validation status.

Everything is driven by the BFF API responses and a content catalog. The frontend is a renderer of what the backend declares, not an author of what the user should see.

### 8.4 The probe is the granularity unit

This deserves its own section because it closes the door on a persistent design temptation: organising the dashboard around a universal cultural hierarchy.

**The temptation.** It would be tempting — and familiar from countless commercial dashboards — to organise cultural content around a schema like *World → Region → Country → Institution → Milieu → Source*. This reads naturally to Western users, maps cleanly to existing geodata, and appears to offer a universal zoom metaphor.

**Why this is forbidden.** WP-001 §2.1 names this pattern: **epistemological colonialism** — the assumption that Western institutional categories are universal. A country-centered hierarchy makes sense for a probe on German institutional media; it makes no sense for a probe on trans-national diaspora networks, or on religious community publications that span borders, or on clan-based oral traditions digitised through WhatsApp.

**The rule.** The probe is the unit of granularity. Each probe declares its own cultural structure through its Probe Dossier (Arc42 §8.15) and its emic categories (WP-001 §4.2). The frontend reads this structure and renders it; it does not impose a schema.

**Interaction-level resolution (from §4.1).** The probe is also the unit of **selection**. Globe emission is probe-first; sources appear as read-only presentation satellites around the probe glyph. Source-scope analysis is fully preserved as a narrowing within the Probe Dossier on Surface II (§4.2.4) — what is removed is source-as-globe-selection, not source-as-analysis-scope.

**Relationship to the extensibility test (§8.2).** A probe with an unusual granularity shape — say, seven nested levels, or no hierarchy at all — is not an edge case. It is the expected case. The frontend that only handles the Germany-shaped probe well is broken.

---

## 9. Silver-Layer Access

Starting in Iteration 5, the dashboard supports **Silver-layer access** on eligible sources as a data-source toggle on Surface II. The same view-mode catalog (§4.2.3) applies equally to Gold metrics and to Silver unified raw data; the difference is the backend data source, not the user-facing machinery. An EDA × heatmap view on Gold `sentiment_score` and an EDA × heatmap view on Silver `cleaned_text` token distributions are the same interaction built over a different query.

### 9.1 UI shape

A compact data-source toggle appears on Surface II (in the top scope bar or within the Probe Dossier, resolved in implementation) offering:

- **Gold** (default). Aggregated metrics from the analysis worker's extractor pipeline.
- **Silver** (available only when the current source is Silver-eligible). Unified raw data — cleaned text, per-document metadata, language classifications — suitable for user-driven analysis.

When the user toggles to Silver, view modes update their queries against the Silver data source. The view-mode matrix stays the same; the data underneath shifts.

### 9.2 Eligibility — the governance gate

Silver exposes raw cleaned text. Unlike Gold, which is already aggregated and k-anonymity-safe by construction, Silver-level access for a source requires an explicit **eligibility flag** on the source record:

- **Probe 0 is auto-flagged.** Its two sources (`tagesschau.de`, `bundesregierung.de`) are institutional public data, government or public-broadcaster RSS, with no re-identification risk per Manifesto §VI and WP-006 §7. The eligibility flag is set in the database migration that adds the column.
- **All other sources require a review process** before the flag is set — applying the WP-006 §5.2 ethical review specifically to the Silver-access question. The review outcome, the reviewer, the date, and the rationale are recorded alongside the flag.
- **Sources without the flag do not appear as Silver options on Surface II**, regardless of their Gold presence. The absence is documented in the UI as an explicit "not Silver-eligible" state, not a silent omission — the user sees *why* a source cannot be viewed at the Silver layer. The Dual-Register methodological view for non-eligible sources explains the review gate and links to WP-006 §5.2.

### 9.3 What Silver access is not

Silver access is not a backdoor around AĒR's architectural commitments:

- **No personal data.** Silver contains the same data that Bronze and Gold do, minus direct identifiers that the Source Adapter strips at the Bronze → Silver boundary (WP-006 §7.2.1). Silver access does not relax these protections.
- **No cross-source joining at the individual-document level** unless the joining itself is subject to review.
- **No unbounded query surface.** The Silver endpoints expose aggregation and sampling; they do not permit arbitrary SQL.
- **No export without acknowledgment.** Downloads (if enabled in later iterations) require the user to acknowledge the eligibility gate.

The ethical commitment (Manifesto §VI) is that AĒR observes discourse patterns, not individuals. Silver access preserves this commitment by confining raw-text exploration to sources where publication is a public act, not a private utterance.

---

## 10. Performance as Design Constraint

Performance is not a nice-to-have. It is a design principle with hard numbers.

**Target hardware (High-Fidelity mode):** A five-year-old mid-range laptop (as of writing: a 2021 M1 MacBook Air or equivalent Intel integrated-graphics ThinkPad), running a mainstream browser, on a 1080p display. If the dashboard does not remain fluid on this hardware in High-Fidelity mode, the design has failed.

**Target hardware (Low-Fidelity mode):** A ten-year-old laptop (as of writing: a 2015 ThinkPad with integrated graphics), on any mainstream browser, on a 5 Mbps connection. If the dashboard is not usable for scientific work on this hardware in Low-Fidelity mode, the design has failed on the equity commitment of §7.6.

**Frame budgets (High-Fidelity):**

| Surface / Layer | Target | Hard ceiling |
|---|---|---|
| Atmosphere (Layer 0–1) | 8 ms/frame (120 fps) | 16 ms/frame (60 fps) |
| Analysis (Layer 3) with ≤10k points | 16 ms/frame | 33 ms/frame (30 fps) |
| Reflection (Layer 4, prose-dominant) | Not applicable (static) | First contentful paint < 1s |

**Frame budgets (Low-Fidelity):**

| Surface / Layer | Hard ceiling |
|---|---|
| Atmosphere (Layer 0–1) | 33 ms/frame (30 fps); acceptable to be static |
| Analysis (Layer 3) | 50 ms/frame; no animation required |
| Reflection (Layer 4) | First contentful paint < 2s |

**Initial load budgets (High-Fidelity):**

- First meaningful paint of the Atmosphere surface: < 1.5 seconds on a 50 Mbps connection.
- Full interactivity at Layer 0: < 3 seconds.
- Descent to any other layer: < 500 ms.

**Initial load budgets (Low-Fidelity):**

- First meaningful paint: < 3 seconds on a 5 Mbps connection.
- Full interactivity at Layer 0: < 6 seconds.
- Descent to any other layer: < 1 second.

**Data budgets:**

- The Atmosphere surface must render a valid state with a single BFF request (≤ 50 kB response).
- The Analysis layer must handle the BFF's hard row ceiling (currently 10,000 points, scaled by resolution multiplier per Arc42 §8.13) without jank.
- The Reflection layer has no data budget — it is served as static content with occasional API enrichments.

### 10.1 Why performance is an ethical requirement

A slow dashboard is not merely annoying; it is epistemically distorting. A user waiting four seconds for a chart unconsciously accepts the first answer they see rather than exploring alternatives. Responsiveness is a precondition for the Question-First Frontend (§7.3). If interactions feel costly, questions go unasked.

The Low-Fidelity mode (§7.6) extends this principle to hardware equity: a researcher on a 2012 laptop must be able to ask AĒR the same questions as a researcher on a 2024 workstation. The answer will look different; the epistemic access must be identical.

---

## 11. The API-key and Authentication Note

The current BFF uses a static API key (ADR-011). This is designed for server-to-server use. A static key in a browser bundle is a security problem — it lands in JS sources, in DevTools, in HAR files.

Per ADR-020 and Phase 98, the production deployment handles this at the ingress layer — the frontend deploys as a static bundle behind Traefik, and Traefik injects the API key server-side. The browser communicates only with its own origin. This is the "short-term pragmatic" path named in earlier iterations of this brief.

A medium-term migration to an OIDC-based authentication flow (a future ADR aligned with ADR-017 Principle 4: Governed Openness) remains the correct direction once user-level differentiation is needed. Until then, the dashboard carries no credentials in-browser and no design principle in this brief assumes in-browser credentials or embeds user identity into URL state.

---

## 12. What is deliberately not decided here

- **Framework, library, rendering strategy.** **Resolved in ADR-020.** SvelteKit (static adapter) with Svelte 5 Runes; vanilla three.js with custom GLSL; Observable Plot + uPlot + D3 for scientific charts; MapLibre + deck.gl for geo-analytics; D3-force for 2D relational networks.
- **Specific color values.** **Resolved in `design_system.md` §3.** Viridis-256 and Cividis-256 interpolators; the theme token families in `design_system.md` §1.
- **Specific component inventory (base primitives).** **Partially resolved in `design_system.md` §5.** Base primitives (Button, Dialog, Tooltip, Badge, SkipLink) landed in Phase 98b. Iteration 5 extends the inventory with navigation-chrome primitives (left rail, top scope bar, methodology tray) and view-mode-switcher components; these land in `design_system.md` as they ship.
- **Exact Low-Fidelity rendering strategy.** The brief commits to the principle (§7.6) and the performance budgets (§10); the specific 2D projection, tile strategy, and asset pipeline are resolved in the implementation phase.
- **Content catalog format.** The brief commits to the principle that Dual-Register semantic content (§7.7) is not hard-coded in frontend source. Per ADR-020, content flows from a BFF endpoint (`GET /api/v1/content/{entityType}/{entityId}?locale=...`); the YAML source structure lives under `services/bff-api/configs/content/`.
- **Specific Epistemic Weight visual mappings.** §7.8 defines the principle (weight scales with backing). `design_system.md` §4 operationalizes the tier-to-class mapping; exact line thicknesses and opacity values stay inside that file.
- **The k-anonymity threshold for L5.** §5.5 commits to the principle (WP-006 §7 applies at Evidence layer) and names *k* = 10 from WP-006 §7.2 as today's default. The exact value per probe type is an operational parameter that may evolve.
- **The methodology tray's push vs. overlay threshold.** §3.3 commits to push-mode as default; the viewport-width threshold at which push-mode falls back to overlay-mode is an implementation detail for `design_system.md`.
- **Accessibility specifics.** Full WCAG 2.2 AA conformance is a baseline (per `design_system.md` §6 and the Phase 98c Playwright + axe gate). Specifics (keyboard-only navigation paths, screen-reader semantics at Layer 0, reduced-motion modes) are resolved in the accessibility spec that accompanies implementation.

---

## 13. Relationship to existing documents

| Document | Relationship |
|---|---|
| `docs/arc42/00_manifesto.md` | Source of the atmospheric metaphor, the observer-position principle, the Resonance-over-Truth commitment, the ethical commitments in §VI, and the "documenting resonance, not judging it" stance that underpins §1.3. |
| `docs/arc42/01_introduction_and_goals.md` | Source of the five quality goals (§2) and the DNA pillars (§2.1). |
| `docs/design/visualization_guidelines.md` | The brief *encompasses* the guidelines. Guidelines constrain rendering; the brief defines structure. |
| `docs/design/design_system.md` | The concrete operationalization of the brief. Tokens, typography, color scales, Epistemic Weight classes, base components. The design system grows alongside the brief; see §12. |
| `docs/design/reframing-note.md` | The working synthesis from the 2026-04-24 review that produced this Iteration 5 rewrite. Expected to be deleted once Iteration 5 is shipping; referenced here for provenance. |
| `docs/arc42/09_architecture_decisions.md` — ADR-003, ADR-016, ADR-017, ADR-020 | ADR-003 (Progressive Disclosure via Metadata Index) is realized as the five-layer descent. ADR-016 (Hybrid Tier Architecture) is realized as §7.8 (Epistemic Weight). ADR-017 (Reflexive Architecture) is realized in Surface III and in refusal-as-feature (§7.4). ADR-020 (Frontend Technology Stack) is written against this brief; its compliance check audits it. |
| WP-001 §2 (epistemological colonialism), §4 (Etic/Emic), §5.3 (Coverage Map), §8 | Source of §8.4 (probe as granularity unit), the emic-visibility requirements, and the extensibility commitments. Also source of the terminology divergence recorded in §6. |
| WP-002 §4 (Validation Protocol), §8 (Tier Architecture) | Source of the validation-gated Epistemic Weight transitions in §7.8. |
| WP-003 §6 (demographic skew, Digital Divide) | Source of the demographic-opacity layer of the Negative Space overlay. |
| WP-004 §4–§5 (Levels of Cross-Cultural Comparison), §8 (Ethical Dimension) | Source of §1.3 (composition rather than comparison) and of the governance of multi-context rendering. |
| WP-005 §6 (Pillar temporal signatures) | Source of the Pillar-to-viewing-mode mapping (§2.1) and of the fractal interaction between Pillars and descent depth (§5.5). |
| WP-006 §3.4, §6.2, §7 (privacy), §8.3 | Source of the Question-First principle, the non-prescriptive visualization stance, the k-anonymity gate at L5 (§5.5), the Silver-eligibility review process (§9.2), and the open interdisciplinary evaluation commitment. |

---

## 14. Open questions for interdisciplinary review

Per WP-006 §8.3 Q5, AĒR's dashboard design should be evaluated with user studies before it hardens. This section surfaces the questions that should structure such a review.

Questions carried forward from prior iterations (still live):

1. **Does the five-layer descent correspond to how researchers and journalists actually navigate AĒR?** Or are some layers conceptually redundant, or one missing?
2. **Does the "empty lane" pattern (§4.2.2, with Dual-Register from §7.7) communicate methodological openness, or does it read as incomplete?**
3. **Does the refusal UI (§7.4 + §7.7) invite inquiry (as intended) or frustrate users into abandoning the question?**
4. **Does the Negative Space overlay (§4.4, §5.4) make absence legible, or is it a clever designer trick that users toggle once and never again?**
5. **Is the Reflection surface (§4.3) used, or is it a methodological archive that looks virtuous but remains unvisited?** Especially critical under the reframing, where Reflection is promoted to a primary surface.
6. **Does Progressive Semantics (§7.7) successfully serve both audiences without patronizing either?**
7. **Does Low-Fidelity mode (§7.6) feel like a first-class experience to users on older hardware?**
8. **Does Epistemic Weight (§7.8) communicate the right thing?** When a metric is rendered at full weight, do users over-trust it? When at moderate weight, do users under-trust legitimate signals?
9. **Does the composition framing (§1.3) hold, or do users read multi-context displays as comparison anyway?**
10. **Does Fractal Cultural Granularity (§5.5) successfully communicate probe-specific structure without appearing inconsistent?**
11. **Does the four-domain visualization separation (§7.9) hold under real scientific review?**

Questions surfaced by the Iteration 5 reframing (new):

12. **Does the user successfully leave the globe, or do they bounce back to it?** The globe's demotion to a landing overview depends on the scientific surfaces being compelling enough that users stay in them. If users keep returning to the globe for re-orientation, Surface II's design has under-delivered.
13. **Is the always-visible methodology tray (§3.3) readable, or does it crowd Surface II?** The push-mode default assumes Surface II tolerates compression. If users close the tray immediately and never reopen it, push-mode is wrong; if they leave it open and lose the chart, the tray's width is wrong.
14. **Does the in-context probe introduction (§4.5) succeed?** Does a first-time visitor form a correct first impression from Progressive Semantics on probe glyphs alone, or do they need the Reflection primer to make sense of the globe?
15. **Does the view-mode matrix (§4.2.3) produce meaningful scientific views at MVP, or does it read as a library of half-baked options?** The MVP commits to three view modes per metric from structurally different cells. If users stay on the default time-series view and never explore, the matrix is either too wide or too shallow.
16. **Is the probe-first emission with source satellites (§4.1) legible?** The reframing resolved the probe/source cognitive slip by making probes canonical and sources presentational. The empirical question: does the visual hierarchy succeed — do users understand that the big glyph is selectable and the small satellites are informational?
17. **Does the right-edge methodology tray (§3.3) succeed as an always-visible affordance, or do users still treat it as a hidden panel?** The closed-state tab is always visible by construction; the question is whether users *perceive* it as central.

These are not rhetorical. They are the evaluation criteria for the first real user study of the AĒR dashboard, to be conducted after Iteration 5 reaches a usable state.

---

## Appendix A: Design principles at a glance

For quick reference during implementation reviews:

1. **Atmospheric Aesthetics** — literal on Surface I, structural across the rest; stratification, currents, stillness-with-motion, observer at the edge, no faces.
2. **Composition, not comparison** — multiple contexts together form a richer Aleph, not a ranked list.
3. **Quality-goal binding** — every visual choice defends a quality goal from Arc42 §1.3.
4. **Navigation is first-class** — persistent, obvious, bidirectional, scope-carrying, return-aware. Three-part chrome: left rail + top scope bar + right methodology tray.
5. **Three surfaces, one overlay, one primary role each** — Surface I as landing overview, Surface II as primary scientific, Surface III as primary methodological; Negative Space everywhere.
6. **Five layers of descent, redistributed across surfaces** — L0–L2 live per-surface; L3 is native on Surfaces II and III; L4 is the methodology tray; L5 is the reader-pane overlay.
7. **Fractal cultural granularity** — the descent narrows the cultural frame fluidly, following each probe's own emic structure rather than a fixed hierarchy.
8. **Probes as selection unit, sources as presentation** — the probe is canonical; sources appear as read-only presentation satellites; source-scope analysis lives inside the Probe Dossier.
9. **The probe concept is introduced in context** — Progressive Semantics on every probe glyph, plus a Reflection primer. Probe fluency is not assumed.
10. **Terminology recorded (Path B)** — current "Probe" = grouping, "Source" = individual, matching the PostgreSQL schema. The WP-001 drift is named in §6; reconciliation deferred to a post-Step-2 ADR.
11. **Ockham's Razor, visible** — no chart junk, no decorative encoding.
12. **No valence, ever** — Viridis, numeric, neutral.
13. **Question-first, with answers where tenable** — invites questions; makes claims the methodology supports; refuses claims it does not.
14. **Refusal is a feature** — HTTP 400 becomes a methodological statement.
15. **Long sessions, dark defaults.**
16. **Honest Desktop-First with High-Fidelity and Low-Fidelity modes.**
17. **Dual-Register / Progressive Semantics** — both semantic and methodological framings co-present; only one prominent at a time.
18. **Epistemic Weight** — visual prominence scales with methodological backing; validated metrics carry their weight, provisional metrics signal their provisionality.
19. **Visualization stack separation** — four rendering domains, framework-agnostic modules.
20. **View-mode matrix** — analytical disciplines × presentation forms; extensible; MVP = three structurally different cells per metric.
21. **Silver-layer access with eligibility flag** — available on flagged sources; the UI is a data-source toggle; governance is an explicit per-source review.
22. **Extensibility by construction** — the probe is the granularity unit; the frontend is a renderer of what the backend declares.
