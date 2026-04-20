# Design Brief — The AĒR Frontend

**Status:** Iteration 4 — open for review.
**Authority:** Derived from Manifesto (§I–§V), Arc42 §1.1–§1.4, ADR-003, ADR-016, ADR-017, WP-001 through WP-006, and `docs/design/visualization_guidelines.md`.
**Audience:** Sole author (for now); future contributors; interdisciplinary reviewers from WP-006 §8.3.
**Relationship to existing documents:** This brief sits *above* `visualization_guidelines.md` — the guidelines constrain individual rendering decisions; this brief defines what the dashboard *is*. It sits *below* a future ADR-018 which will record the technical framework choice. Technology is deliberately absent from this document.

---

## 0. What this document is, and is not

This is a **design brief**, not an architecture decision record. It does not name a framework, a library, or a rendering strategy. It defines the visual identity, interaction architecture, and extensibility commitments of the AĒR dashboard so that any subsequent technical decision can be evaluated against a shared standard.

The brief is a contract with the project's future: any dashboard implementation — in-repo or third-party — that claims to represent AĒR must satisfy the principles recorded here. A dashboard that is fast but prescriptive, or beautiful but closed to extension, fails this contract.

---

## 1. The Design Concept: Atmospheric Aesthetics

The name AĒR comes from ancient Greek ἀήρ — the lower atmosphere, the surrounding climate (Arc42 §1.1). The Manifesto extends this into the governing metaphor: an orbital sensor that captures atmospheric currents rather than individual fates (§I).

The frontend takes this literally. The design concept is **Atmospheric Aesthetics** — not metaphorically, but as a strict governing principle that shapes every visual, interactive, and structural decision.

### 1.1 What "atmospheric" means as a design principle

**Layering.** The atmosphere is not flat. It is stratified — troposphere, stratosphere, mesosphere — each layer with its own physics, yet part of a single continuum. The dashboard mirrors this: content is accessed by altitude, not by navigation. The user does not "switch tabs" — they descend through layers toward evidence, or rise back toward contemplation. This is the structural form of Progressive Disclosure (ADR-003), made visual.

**Currents over points.** Atmospheric phenomena are flows, not dots. Publication volume is not a bar chart — it is an aggregate density. Sentiment is not a single value — it is a pressure gradient with uncertainty at its edges. Entity salience is not a ranking — it is turbulence. This directly operationalizes `visualization_guidelines.md` §1 (Viridis, no discrete valence) and §3 (uncertainty alongside estimates).

**Stillness with motion beneath.** The atmosphere appears macroscopically still and is microscopically chaotic. The dashboard's default state is contemplative; activity reveals itself only to the attentive eye. This is the aesthetic opposite of a status board.

**The observer at the edge.** We live within the atmosphere, but we can only *see* it from outside (orbital sensors) or through its effects (clouds, wind). This is Borges' Aleph paradox — to be simultaneously inside and outside the observed. The dashboard encodes this position spatially: the default viewpoint is external (orbital), and descent into layers brings the observer progressively closer, but never fully in.

**No faces, no names.** The atmosphere shows systems, not individuals. Consistent with Manifesto §I ("atmospheric currents rather than individual fates"), the frontend never displays persons, avatars, or "top influencers" by name as primary content. Individuals appear only at Layer 5 (Evidence), as named entities within a retrieved source document — never as aggregated protagonists.

### 1.2 Why this concept, and not another

Three alternatives were considered and rejected:

- **Instrument aesthetics** (the early working term) correctly captures scientific seriousness but is too technical. It suggests a radiotelescope console — accurate for the backend, wrong for the surface. The frontend must be legible to interdisciplinary collaborators (WP-001 §8 through WP-006 §8), not only engineers.
- **Editorial/journalistic aesthetics** (FT, Reuters, Bloomberg Graphics) is precise and trustworthy but assumes an author-reader relationship. AĒR has no "editorial voice" in the WP-006 §3.4 sense; the dashboard is *the question*, not the answer.
- **Popular-science aesthetics** (Kurzgesagt, infographic style) is accessible but stylized-abstract. AĒR would lose scientific credibility if it looked like a YouTube explainer.

Atmospheric Aesthetics resolves this: it is serious without being operator-like, beautiful without being decorative, approachable without being populist. It is what a scientific instrument looks like when the instrument itself is the atmosphere.

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
| **1. Scientific Integrity & Transparency** | Atmospheric currents *never* smooth over structural limits. Validation status, tier classification, and known limitations are rendered at the layer where they are needed, not hidden in details panels. The dashboard refuses to display what the backend refuses to compute (ADR-017 Principle 5). Ockham's Razor in visual form: no chart is more ornate than the data supports. Epistemic Weight (§5.8) ensures that visual prominence scales with methodological backing, not with aesthetic preference. |
| **2. Extensibility (Modularity)** | The design is **open-ended by construction** — see §6. New discourse functions, new pillars, new probes with their own cultural granularity structures, new metrics, new temporal scales all slot into the existing structure without redesign. A dashboard that requires a refactor when Probe 1 arrives is broken. |
| **3. Scalability & Performance** | The atmospheric metaphor rewards performance design. Stillness is cheap; motion is expensive — so motion is reserved for where it carries information. The default layer is nearly static. Descent into analysis layers progressively loads detail. Hard frame budgets (§7) ensure the dashboard remains fluid on modest hardware, and a Low-Fidelity mode (§5.6) preserves scientific depth on weak hardware. |
| **4. Maintainability** | The design specifies contracts, not implementations. The five layers, the surfaces, and the interaction grammar are stable; the code behind them can evolve. A contributor in 2028 should still recognize the dashboard even if every line of code has been rewritten. |
| **5. Security** | No design principle here weakens the Zero-Trust posture from ADR-013. The dashboard is a client of the BFF; it stores no credentials in the browser (see the API-key note in §8). Refusal-as-feature (§5.4) means the dashboard never reveals what authentication could not authorize. Evidence-layer access (§4.5) respects k-anonymity gates per WP-006 §7. |

### 2.1 Binding to the AĒR DNA (Arc42 §1.2)

The three philosophical pillars — Aleph, Episteme, Rhizome — are not abstract framings. They correspond to three distinct viewing modes, each with its own visual grammar. The mapping is already made explicit in WP-005 §6:

| Pillar | Temporal mode | Atmospheric metaphor | Visual grammar |
|---|---|---|---|
| **A — Aleph** | Synchronic | The weather *now* | Real-time state, spatial distribution, quiet density |
| **E — Episteme** | Diachronic | The climate record | Long trends, slow shifts, multi-year horizons |
| **R — Rhizome** | Relational-temporal | The currents | Propagation between contexts, lag, directionality |

These are not three separate dashboards. They are three **modes of viewing** the same atmosphere, toggled from within any layer. A user looking at Probe 0's sentiment trajectory can switch between "weather now" (the current week), "climate record" (the last year), and "currents" (when multi-probe data enables propagation observation) without leaving their context. Their relationship to cultural granularity is itself fractal — see §4.5.

---

## 3. The Three Surfaces and the Everywhere-Layer

The dashboard has **three surfaces** — not pages, not routes, but three fundamental ways to encounter AĒR — and one persistent overlay available on all of them.

Cross-context composition is not a fourth surface. As more probes are added, each of the three surfaces becomes richer: the globe shows more luminous points, the function lanes carry more contexts simultaneously, and Reflection gains more emic content to articulate. The Aleph principle forbids a dedicated "comparison surface" — multiple contexts belong together within the same atmosphere, not in a separate arena.

### 3.1 Surface I: Atmosphere (the first contact)

The default entry. A three-dimensional Earth, slowly rotating, against deep black. Ocean is a deep, almost-black blue; landmasses are restrained, slightly lighter blue silhouettes — rendered from vector polygons rather than satellite imagery, so the surface stays crisp at any zoom and never competes with the discourse signal it carries. No political borders, no country labels at the highest altitude. An optional borders layer (default off) can be revealed at Layer 2 once the user has descended below the atmospheric register.

The Atmosphere surface answers one question: *what is the state of AĒR's observation right now?* It is the synchronic face (Aleph) — but the other two pillars are latent here too, by the fractal principle of §4.5. Aleph is dominant; Episteme is implicit in the baseline character of each probe; Rhizome is ready but invisible until multi-probe data makes propagation observable. The user is not watching a scoreboard — they are watching an atmosphere.

On this globe:

- **Luminous probe points** mark active probes. Core brightness reflects publication volume in the selected time window. Pulse rate carries current activity density — a probe publishing 40 documents per hour pulses perceptibly faster than one publishing 5 per hour. The fastest pulse remains slow by human standards: this is still "stillness with motion beneath" (§1.1). Pulse rate is bounded; it does not race.

- **Reach aura** surrounds each probe point — a soft, translucent field that indicates *where the probe observes*, not its impact or influence. The aura's shape is read from the Probe Dossier (§6.4) and may be a simple territorial region (a national probe) or a non-contiguous pattern of cultural affinity (a diasporic or subcultural probe). The aura is static; it does not animate. Multiple overlapping auras compose additively, revealing regions of methodological attention that no single probe dominates.

- **The terminator** (day/night boundary) is real and current. Publication rhythms follow it. The terminator moves at Earth rotation speed — visually almost static, cosmologically alive.

- **Absence regions** — every area where AĒR has no active probe — are marked as *explicitly unmonitored*. Not dim, not greyed, but positively tagged. This is the Coverage Map from WP-001 §5.3 integrated into the first impression.

- **Propagation arcs** (latent). The engine reserves a rendering slot for Rhizome propagation — arcs tracing how discourse events move between probes. These arcs remain invisible until multi-probe data produces measurable cross-context propagation (WP-005 §6). When they appear, their motion is on the scale of minutes, not seconds. Until then, the slot is inert and carries no visual cost.

The Atmosphere surface is kept minimal in cognitive weight, not in informational depth. A single glance shows: which cultural contexts are observed, how active each is now, and — negatively — which contexts are absent. The user sees a *constellation*, not a scoreboard. It is the layer where the user breathes.
```

**Begründung:** Der ursprüngliche §3.1 behandelte L0 als einfache Punkt-Visualisierung und widersprach damit implizit §4.5 (Fractal Cultural Granularity), das fordert, alle drei Pillars auf jeder Tiefe mitwirken zu lassen. Der neue Text verankert Aleph-dominant, Episteme-latent (Pulse-Rate als Aktivitätsdichte), Rhizome-bereit (Propagations-Slot). Das historische Weight-Ring-Konzept ist entfernt — dem Einwand entsprechend, dass es bei vielen Probes überlädt. Stattdessen trägt der Puls selbst die Aleph-Information.


### 3.2 Surface II: Function Lanes (the scientific work)

The methodological backbone. The four discourse functions from WP-001 (Epistemic Authority, Power Legitimation, Cohesion & Identity, Subversion & Friction) appear as horizontal lanes, each with its own time series, entity cloud, and uncertainty band.

Today only two of the four lanes contain data, from a single probe (Germany, institutional). The other two lanes are **not hidden and not greyed out**. They are present as empty lanes, accompanied by the Dual-Register invitation described in §5.7. The empty lane *is* the question — operationalized from WP-006 §3.4.

When multiple probes are active, each lane carries **parallel context streams** — the institutional German probe beside, say, a diasporic Telegram probe, each with its own baseline, its own within-context deviation signal, its own emic content. They appear together (composition) without being placed on a single common scale (comparison). The user can hover on any stream to surface the emic layer (WP-001 §4) that explains what that function *means* in its own context.

Critically, this surface is built so that a **fifth function**, or a split of an existing function, requires no layout redesign. See §6.

### 3.3 Surface III: Reflection (the instrument about itself)

The documentary surface. Here prose is dominant. Every metric's provenance, every probe's dossier, every known limitation, every observer-effect assessment is rendered as long-form, typographically disciplined content — not as tooltip content, not as modal fragments. Interactive visualizations appear inline within the prose, Distill.pub style, illustrating the claims of the text.

This is where:

- `GET /api/v1/metrics/{metricName}/provenance` is rendered in human form
- The Probe Dossier structure (Arc42 §8.15) is made legible to outside readers
- WP-001 through WP-006 can be linked and referenced at the exact point where their methodological claim becomes user-visible
- Version history of interpretations (WP-006 §4.3) is surfaced — showing how the meaning of a metric has changed over time
- **Emic documentation** (WP-001 §4.2) lives natively — the untranslated local designation, the cultural context, the societal role description for each probe

Surface III is what distinguishes AĒR from every commercial dashboard. Commercial dashboards hide methodology. AĒR foregrounds it. This is the architectural answer to WP-006 §8.3 Q5 ("How should AĒR's dashboard be designed to minimize reification and maximize critical engagement?").

### 3.4 The Everywhere-Layer: Negative Space

Not a surface. A persistent, user-toggled overlay available on every view. Named — for now — **"What AĒR doesn't see"**.

When activated on the Atmosphere surface, unmonitored regions become more prominent, not less. On the Function Lanes, empty lanes gain explicit demographic-skew annotations (WP-003 §6). On Reflection, known limitations scroll into the margin.

**Demographic opacity.** The overlay is also the visual home of demographic absence. WP-003 §6.1 documents what is systematically missing from digital discourse — older cohorts, non-dominant languages, authoritarian-context voices, populations behind access barriers. These absences intensify as the descent goes deeper: at L0 (Immersion) the overlay shows regional coverage gaps; at L3 (Analysis) it adds demographic-skew annotations on the active context; at L4 (Provenance) it foregrounds the full WP-003 bias profile. The absence is visible at the level of detail where it matters for that view.

This is WP-006 §8.3 Q6 operationalized. Absence is not a failure state to be hidden; it is a first-class feature of the instrument. The toggle's existence answers the question WP-006 asks of every dashboard: *can the user see what the instrument is blind to?*

---

## 4. The Five Layers of Progressive Descent

Each surface is navigable by depth. The five layers are the governing interaction architecture of the dashboard and are uniform across all three surfaces.

| Layer | Name | User stance | What is present | Cultural reach | What is absent |
|---|---|---|---|---|---|
| **0** | **Immersion** | Watching | The current surface in its quietest form. Motion, if any, is long-period. | All active probes co-present. Widest cultural breadth. | All UI chrome. No panels, no toolbars, no labels beyond the essential. |
| **1** | **Orientation** | First engagement | Soft context overlay: where am I, which probes, which time range, which normalization mode. | Multi-probe context still visible; active selection emerging. | Controls beyond what is needed to understand *where* one is. |
| **2** | **Exploration** | Steering | Filters, toggles, zoom. Resolution switch. Raw vs z-score. Probe selection. | User action narrows to one or a few probes. Cultural frame tightens. | Any synthesis across filters that the backend would refuse. |
| **3** | **Analysis** | Measuring | Charts, small multiples, uncertainty bands, within-context measurement. | Single probe context prominent. Emic layer (WP-001 §4.2) accessible on hover. | Composite "health scores" or ranking axes. Cross-context placement on a single scale without equivalence. |
| **4** | **Provenance** | Auditing | Tier classification, algorithm description, known limitations, validation status, equivalence level, observer-effect notes, extractor version hash, probe dossier. | One source or one probe. Emic context at full fidelity. | Reassurance language. The goal is accurate accounting, not trust-building. |
| **5** | **Evidence** | Verifying | The individual Bronze document that underlies a metric point. The full raw text, the source metadata, the ingestion timestamp, the trace ID. | A single voice, within a single source, within a single context — subject to k-anonymity gates (§4.5). | Interpretation. The raw document stands alone. |

The **Cultural reach** column makes explicit what is otherwise implicit: the descent through layers is not only hermeneutic (from contemplation to evidence) but also cultural (from wide co-presence to a single voice). This is not a separate axis the user controls — it is an observable property of the descent itself. See §4.5.

### 4.1 Governing rules of the descent

1. **Each layer must be reachable from the one above it in one interaction.** No hidden navigation, no deep menus. The user is always one click, tap, or keypress from going deeper or returning.
2. **Each layer adds; no layer replaces.** A user who reaches Layer 3 does not lose the context of Layer 0. The Atmosphere remains visible, perhaps dimmed, in the periphery. This is the architectural form of the ADR-003 principle: Progressive Disclosure never hides.
3. **A user can stop at any layer.** Layer 0 is a complete, valid experience. A contemplative viewer need never leave it. A forensic analyst who descends to Layer 5 does not feel the upper layers were wasted.
4. **Refusal is visible at the appropriate layer.** If the BFF returns HTTP 400 on `?normalization=zscore` (ADR-016), the refusal manifests at Layer 2 (where the user requested it), with the explanation surfaced in Layer 4 (provenance). See §5.4.
5. **The descent is reversible in both directions.** Deep-linking into Layer 4 is a legitimate entry point (e.g., someone sharing a methodology reference). The ascent from Layer 4 to Layer 0 must remain intuitive.

### 4.2 The Surface × Layer matrix

Surfaces and Layers are orthogonal. Every surface has all five layers, but what the layer *shows* differs by surface. The matrix below makes this concrete.

| | **Surface I: Atmosphere** | **Surface II: Function Lanes** | **Surface III: Reflection** |
|---|---|---|---|
| **L0 — Immersion** | Rotating globe. Probe points as luminous dots. Terminator live. No UI. | Four horizontal lanes, each showing a long, slow time signature. No labels beyond function names. Empty lanes are empty space — no error feeling. | A single long-form essay at reading width. No sidebar. No table of contents unless scrolled to. |
| **L1 — Orientation** | Soft overlay: current time range, active probe count, normalization mode, active contexts. Region label under cursor. | Overlay: active probes per lane, current time window, equivalence status per lane. Empty-lane captions appear. | Breadcrumb: which metric, which probe, which WP reference. Reading progress indicator. |
| **L2 — Exploration** | Time-range scrubber. Resolution switch. Region zoom. Pillar mode toggle (Aleph/Episteme/Rhizome). Probe selection (single/constellation). | Lane filters. Cross-lane overlay toggle. Raw/z-score switch (refused where appropriate — §5.4). Source subset selector. Context toggle when multi-probe. | Linked interactive elements within the prose become manipulable. Chart parameters are adjustable in-line. |
| **L3 — Analysis** | When a probe point is focused, a companion panel opens with that probe's time series, uncertainty bands, and within-context deviation — without leaving the globe. Multiple probes show side-by-side panels in composition, not on shared axes. | Per-lane charts deepen: small multiples by source, entity clouds, detail histograms, within-function trajectories. When multi-probe, parallel context streams per lane, each with its own baseline. | The embedded interactive visualizations gain full chart controls. Reading remains the primary activity; analysis supports the text. |
| **L4 — Provenance** | Per-probe dossier fly-out: classification, emic designation, emic context, bias context, observer-effect status, documentation URL. | Per-metric provenance per lane: tier, algorithm, known limitations, equivalence level, cultural context notes. | The prose *is* provenance. L4 on Reflection is not a fly-out — it is the text itself, at its native depth. |
| **L5 — Evidence** | Click a data point → the single Bronze document appears in a reader pane, with full trace metadata. Subject to §4.5 gates. | Click any chart point → the same reader pane, scoped to that lane's source. Subject to §4.5 gates. | Inline "example source" affordances in the prose open Bronze documents cited by the methodological text. |

**How to read the matrix.** Each cell describes *what the user sees at this depth on this surface*. The matrix is the architecture — not the implementation. It answers the question "is there a defined state here?" not "how is it rendered?" Implementation details (panels, fly-outs, overlays) are suggestions; the contract is *what information is present at what depth*.

### 4.3 A narrative walkthrough: from breath to evidence

To make the matrix tangible, one complete descent as a single user journey. The user is a researcher encountering AĒR for the first time.

**Arrival.** She opens the dashboard. The globe appears — Europe turning into view, dawn terminator crossing the Atlantic. Two luminous points over Germany. Black everywhere else on the continent. She watches for a few seconds. [Surface I, Layer 0]

**Orientation.** She moves the mouse. A thin bar at the top softens into view: *"Probe 0 · institutional DE · last 24 hours · 47 documents · 2 active sources."* Her cursor crosses the Atlantic — no label. The absence is honest. [Surface I, Layer 1]

**First exploration.** She notices the time range and drags the scrubber back three months. The two dots pulse faintly as their historical activity replays. She sees a density gradient: busy weekdays, quieter weekends — atmospheric rhythm without explanation. Curious, she clicks one of the probes. [Surface I, Layer 2]

**Analysis opens.** A companion panel slides in from the right — the globe stays present, dimmed slightly. The panel shows `tagesschau.de`: a sentiment time series with a gentle uncertainty band, a publication-volume line, and an entity cloud. A small badge next to the sentiment curve reads: *"Tier 1 · unvalidated · raw values."* The curve itself is rendered at moderate visual weight — neither muted nor prominent — because its epistemic backing is preliminary (§5.8). She hovers the badge. Progressive Semantics unfolds — the same space now shows a one-sentence semantic explanation ("Values without independent human calibration — use with care"), with a small affordance to dig deeper. [Surface I, Layer 3 + §5.7 + §5.8]

**Provenance.** She clicks the affordance. The panel reorganizes: classification (Epistemic Authority, secondary Power Legitimation), emic designation (*Tagesschau* — public broadcaster under inter-party proportional governance), bias context (chronological visibility mechanism, no algorithmic amplification), known limitations from WP-002 §3 (negation blindness, compound words). Each limitation is a short paragraph in plain language, with a "Read the full methodology" link to WP-002. She reads one. [Surface I, Layer 4]

**Evidence.** A sentiment change catches her eye on 2026-03-14. She clicks the point. A reader pane opens with the single RSS item — the raw `cleaned_text`, the source, the timestamp, the trace ID. No interpretation, no labels. She reads it herself. The pattern was a budget announcement; the lexicon picked up on several negation-sensitive phrases. [Surface I, Layer 5]

**Lateral shift, not descent.** She wants to see this function-stratified. She uses the keyboard shortcut to jump to Surface II. The globe gives way to four lanes. The two lanes with data show the same sentiment curve in their respective function contexts. The two empty lanes are present, with their Dual-Register invitation visible. [Surface II, Layer 1]

**The refusal encounter.** She tries to toggle raw/z-score, hoping to see the data on a different scale. The toggle dims. A soft inline notice appears — the Dual-Register refusal from §5.4. She reads the semantic explanation ("This transformation requires established equivalence across contexts — not yet available for sentiment on a single probe"), then clicks to see the methodological layer ("No `metric_equivalence` row exists for `sentiment_score`..."). She realizes: the refusal is not a bug — it is the system's honesty about what it does not yet know. [Surface II, Layer 2 + §5.4]

**The reading turn.** Intrigued, she wants to understand *why* cross-context equivalence is hard. She clicks through to Surface III, to WP-004 rendered in-situ. She reads for ten minutes. Small interactive charts embedded in the prose let her manipulate the example. She closes her laptop with something she did not have before: not an answer, but a better-shaped question. [Surface III, Layers 0–3]

**A note on what she has not yet seen.** On this day, Probe 0 is all AĒR observes. But the interface is built so that when a second probe arrives from a different context — say, a diasporic identity-formation probe running in Arabic over Telegram — she will see another luminous point on the globe, a parallel stream in the Cohesion & Identity lane, and Reflection will gain a new probe dossier with untranslated emic designations. No part of what she experienced today will have changed; it will have grown. She will also notice that the new probe's cultural granularity does not line up neatly with Probe 0's — that is the point (§4.5).

This is the intended arc: from contemplation to inquiry, with the instrument honest about its current depth and honest about its capacity to grow.

### 4.4 The Negative Space overlay across the matrix

The Negative Space overlay is a perceptual modifier — it does not add content, it re-weights it. Activating the overlay (one keyboard shortcut, one toggle) changes what the user sees at every layer on every surface:

| | **With overlay off** | **With overlay on** |
|---|---|---|
| **Atmosphere L0** | Probe points luminous; absence is dark. | Absence regions become positively marked — a subtle atmospheric field indicates "no observation here." |
| **Atmosphere L1** | Active probe count. | Active + *missing* probe-function count per region. Coverage Map (WP-001 §5.3) becomes legible. |
| **Function Lanes L0–L2** | Empty lanes present but quiet. | Empty lanes gain prominence: what a probe *would* need to contribute here, which contexts are unaddressed. |
| **Function Lanes L3** | Charts of available data. | Charts include explicit demographic-skew annotations (WP-003 §6.1) in the margin. The specific populations that are underrepresented in *this* probe appear as marginalia. |
| **Reflection L0–L3** | Prose about what AĒR observes. | Prose about what AĒR cannot observe scrolls into the margin alongside the primary text. |
| **All surfaces L4** | Known limitations listed as methodological context. | Known limitations *lead* the provenance display — the first thing shown, not the last. |

The overlay is not a "dark mode." It is a **stance mode** — the user declares "I want to see what is missing," and the interface re-organizes its emphasis without changing its structure. This is WP-006's negative-space requirement made operable.

### 4.5 Fractal Cultural Granularity

The descent through layers is simultaneously a **hermeneutic deepening** (from contemplation toward evidence) and a **cultural narrowing** (from broad co-presence toward a single voice). These two motions are not separate axes the user controls — they are **one continuous movement** with both properties.

**The motion is fluid, not stepped.** There are no discrete cultural zoom levels — no "regional → national → institutional → milieu" ladder. Such a ladder would reproduce precisely the Western administrative taxonomy that WP-001 §2 warns against. Cultural granularity is instead **probe-defined**: each probe carries its own emic structure (WP-001 §4.2), and what counts as "the cultural context" at any descent depth is whatever the active probe declares it to be.

**Different probes have different granularity shapes.** A probe on "institutional epistemic authority in Germany" has a relatively flat cultural structure — the relevant emic categories are institutional (Tagesschau, Bundesregierung). A probe on "diasporic identity-formation across Arabic-speaking Telegram channels in Europe" has a layered structure — national diasporas, generational cohorts, linguistic registers (MSA vs. regional dialects), religious affiliations. A probe on "subversion and friction in Russian post-2022 exile media" has yet another shape — platforms, exile geographies, editorial genealogies. There is no single hierarchy that contains all three. This is Foucault's episteme taken seriously: how a society organises its own discourse is itself a cultural fact, not a universal schema.

**The Pillars are fractal.** Aleph, Episteme, and Rhizome are not tied to specific descent depths. They act at every depth — but with the cultural frame appropriate to that depth:

- **Aleph** is strongest at L0–L1: the *co-presence of multiple contexts* as the first impression. The user sees many probes at once. Aleph's signature is breadth.
- **Episteme** is strongest at L2–L3: as the user narrows to a probe and a time range, the *boundaries of the expressible in that specific discursive formation* become the subject. Aleph's many contexts give way to Episteme's deep single context. Episteme's signature is depth.
- **Rhizome** is strongest at the *transitions* between depths, and between probes once a constellation is active: how a topic moves from one context to another, how a frame propagates from a broad discourse to a niche milieu, or back. Rhizome is not a depth — it is the vector between depths. Its signature is movement.

A user on Surface I at L0 (Atmosphere, Immersion) is in an Aleph-dominant mode. A user on Surface II at L3 (Function Lanes, Analysis), focused on one probe in one lane, is in an Episteme-dominant mode. A user who moves between probes, or follows a topic across them, is activating Rhizome. The Pillar toggle (§4.2, L2) allows the user to re-emphasize one Pillar at any depth — but the default emphasis follows the layer.

**Practical consequence.** The dashboard must never render a cultural-granularity control as a fixed selector ("Region → Country → Institution"). Where granularity appears as a control, it reads from the active probe's emic structure, which may have two levels, or seven, or none beyond the probe itself. The probe is the unit of granularity; the probe defines its own depths.

**What this rules out.** No hard-coded hierarchy "Planet → Region → Country → City". No assumption that every probe has a "nation-state" level. No breadcrumb that forces a non-existent parent. No rendering of cultural granularity as a continent/country dropdown. The granularity structure is read from the BFF, from the Probe Dossier, and from the emic layer — never authored in the frontend.

**Relationship to k-anonymity.** At L5 (Evidence), where the cultural reach narrows to a single voice, WP-006 §7 gates apply: a retrieved document must be part of an aggregation of at least *k* = 10 source documents for the metric being viewed. Probes that cannot meet this threshold (very small, very specific) cannot descend to L5 through the dashboard — the descent is refused at L4 with an explanation aligned with §5.4. This is not a limitation of the design; it is the design respecting the ethical commitment.

---

## 5. Design Principles

These are the rules any screen, component, or interaction is checked against.

### 5.1 Ockham's Razor is visible

Arc42 §1.1 commits AĒR to Ockham's Razor. The dashboard extends this commitment to its own visual grammar. Every chart junk element is a small violation. Every decorative gradient without informational content is a small violation. Ornamentation that does not encode data is forbidden at Layers 2–5; at Layer 0 (Immersion), ornamentation is permitted only if it carries atmospheric meaning (e.g., the terminator on the globe is ornamental *and* informational).

### 5.2 No valence, ever

No red-green axis. No "warm" palettes on one extreme. No normative labels ("high", "concerning", "healthy"). The `visualization_guidelines.md` §1–§2 constraints are not negotiable and are not to be softened for "legibility" or "intuitiveness."

A sentiment score of −0.4 appears as its numeric value with its uncertainty band, colored on a perceptually uniform Viridis scale. It does not appear as an orange bar trending downward with a concerned-face icon. The dashboard's job is to make the user *ask* whether −0.4 is a meaningful shift, not to tell them.

### 5.3 The Question-First Frontend, with Answers Where Tenable

WP-006 §3.4 states the principle: *the dashboard invites questions, not conclusions*. This translates into concrete interaction design:

- Every primary number is paired with its uncertainty (shading, bands, status badges).
- Every chart has a visible "why this shape?" affordance at Layer 3 that surfaces the provenance panel at Layer 4.
- Empty states are never apologetic ("no data available"); they are explanatory.
- Within-context trends (this week vs last week, this source against its own baseline) can be stated directly — they are claims the data supports.
- Cross-context statements are governed by the equivalence registry and the Epistemic Weight principle (§5.8). Where equivalence is established and validation complete, AĒR *does* make the composed observation. Where equivalence is unestablished, AĒR refuses the claim (§5.4), not approximates it.

**The posture:** AĒR does not hide behind "it's complicated." Where the methodology supports a claim, AĒR makes the claim clearly. Where it does not, AĒR refuses clearly. What AĒR never does is produce false confidence or false modesty. Claims scale with their epistemic weight (§5.8), and the weight is always visible.

### 5.4 Refusal as a first-class feature

When the BFF returns HTTP 400 (e.g., on `?normalization=zscore` for a metric without a registered equivalence entry), the frontend does not render an error state. It renders a **methodological statement**, built according to Progressive Semantics (§5.7).

The shape of a refusal surface is:

1. A one-sentence refusal in the semantic register — what the user asked, and why it cannot be answered, in plain language.
2. An expand affordance leading to the methodological register — the exact table/gate/rule that triggered the refusal, the relevant Working Paper anchor, the Scientific Operations Guide workflow for resolution.
3. Alternatives — actions the user *can* take that produce valid answers (raw values, different metric, different probe subset).

The refusal is not a dead-end. It is a turn in the road. It preserves the user's agency (alternatives) while protecting the scientific integrity of the system (no silent approximation). This is WP-006 Principle 5 (Interpretive Humility) and Manifesto §II ("Resonance over Truth") made interactive.

### 5.5 Long time, long data, long sessions

The dashboard is designed for sessions that may last hours, not minutes. This shapes defaults:

- **Dark Mode as the primary visual mode.** Light mode is available but is not the default. The default respects the long-session reality of scientific and journalistic work.
- **Typography designed for extended reading.** Inter Variable for UI, IBM Plex Mono (or a stricter tabular-numeral face) for numerical data. Generous line-height at Layer 4 where prose dominates.
- **No auto-refresh that breaks attention.** Data updates are signaled subtly; the user controls when to incorporate them.
- **Deep-linkable state.** Every filter configuration, every time range, every viewing mode is encoded in the URL. A conversation about AĒR is a conversation about shareable URLs, not screenshots.

### 5.6 Honest Desktop-First with High-Fidelity and Low-Fidelity Modes

The primary consumption environment is a wide screen at reading distance — a researcher's laptop, an analyst's monitor, a journalist's desk. The dashboard is designed for this honestly, without pretending mobile-first.

But a brilliant researcher working on a 2012 laptop must have a complete AĒR experience. The dashboard therefore has two rendering modes, with identical scientific depth but different computational footprints.

**High-Fidelity mode (default on capable hardware).** Full Atmospheric Aesthetics: 3D globe with WebGL, real-time terminator, motion on atmospheric timescales, deep visual layering. All five depth layers on all three surfaces.

**Low-Fidelity mode (fallback or explicit user choice).** The 3D globe is replaced by a static or slowly-updating 2D equirectangular map with identical informational content — probes, terminator, absence regions. Motion is removed. All surfaces, all five depth layers, all scientific features are preserved. **The only things reduced are the computational costs that would make the experience janky on weak hardware.** Progressive Semantics (§5.7) is preserved in its text-only form. Refusal surfaces (§5.4) are identical. The Reflection surface is essentially unchanged — it was already prose-centric.

The rule: **a feature is removable from Low-Fidelity mode only if, without removal, it would make the mode non-functional on the target hardware.** Scientific depth is never the trade-off. Immersion is.

**Mode selection.** Low-Fidelity is entered automatically when: (a) WebGL2 is unavailable or reports a known-slow GPU, (b) `prefers-reduced-motion: reduce` is set, (c) effective connection type reports 2G/slow-3G, or (d) the user explicitly chooses it. Every automatic trigger is overridable by the user in both directions.

**Small-screen view.** Tablet and phone layouts use Low-Fidelity mode by default, with additional layout adjustments for narrow screens. This is not a second implementation — it is Low-Fidelity with adjusted breakpoints. A scientific instrument that tries to fit on a phone full-fidelity becomes a toy; Low-Fidelity on a phone remains a real instrument.

### 5.7 Dual-Register Communication (Progressive Semantics)

Every number, every refusal, every empty state, every status badge exists in **two simultaneously valid registers**:

- **Semantic register.** What does this mean? Plain language, connected to everyday experience, not dumbed down. Assumes an intelligent adult without specialist training.
- **Methodological register.** How does this come to be? Algorithm, lexicon, tier, equivalence status, known limitations. Assumes a user who wants to verify or critique.

Both registers exist for both audiences. The researcher benefits from the semantic register (it is often a good sanity check on what she thinks she understands). The informed non-specialist benefits from the methodological register (it teaches, over time, how AĒR thinks). The dashboard must not segregate users into lanes.

**The design problem.** Showing both registers simultaneously on every surface would drown the user in text. The atmosphere would be buried under explanations. The contemplative Layer 0 would collapse.

**The solution: Progressive Semantics.** Both registers are present in the rendered output, but only one is prominent at a time. The transition between them is a micro-interaction local to the artifact — hover, focus, expand — never a page navigation or modal dialog. The user's spatial focus on the data point never breaks; only the framing of that point shifts.

Concretely:

- The semantic register is **primary at Layers 0–3**. It is what appears when the user first engages with a number or state.
- The methodological register is **primary at Layer 4** (Provenance). It is what Layer 4 *is for*.
- At Layers 0–3, a compact affordance (typically a small inline badge or a cursor-tracked glyph) signals "there is a methodological register available here." Engaging it expands the methodological content in-place, without losing the semantic framing.
- At Layer 4, the reverse: the methodological prose dominates, but a sibling affordance provides the semantic summary. The researcher reading deep provenance can still see the plain-language statement of what the limitation means for the data she is looking at.

**Example: the −0.4 sentiment value on Surface I, Layer 3 (current Tier 1 state).**

Primary rendering (semantic register):

> −0.4 · This week's sentiment on institutional German news.
> The value is lower than last week's −0.2. Over the last six months, weekly values range from −0.6 to +0.1. Values near zero are most common.

On expand (methodological register):

> Tier 1 lexicon-based score (SentiWS v2.0). Unvalidated. Score is the arithmetic mean of matched token polarities, clamped to [−1, +1]. Known limitations: negation blindness, compound-word failure, register-neutral institutional language tends to produce scores close to zero. No cross-context equivalence established — this value is comparable across time within `tagesschau`, not across probes.

**Example: the same metric after validation completes (future Tier 2 state).**

Primary rendering (semantic register, with elevated epistemic weight per §5.8):

> −0.4 · This week's sentiment on institutional German news.
> The value is lower than last week's −0.2. This shift is statistically meaningful relative to the established six-month baseline. Similar shifts have preceded budget announcements in 27 of 34 prior cases.

On expand (methodological register):

> Validated Tier 2 score (fine-tuned transformer `sentiment_de_v2.1`, Krippendorff's α = 0.78 across 5 annotators, correlation with reference labels 0.81, validated 2027-01 through 2029-01). Deviation significance derived from `metric_baselines` row (n = 12,847 documents). Known limitations: domain-specific calibration to German institutional news; does not transfer to social-media German without re-validation.

The visual rendering of the validated metric is more prominent than the provisional one — thicker line weight, higher opacity, no "provisional" hatching — because its methodological backing supports that prominence.

**Example: the refusal surface from §5.4, with Progressive Semantics applied.**

Primary rendering (semantic register):

> This transformation cannot currently be supported.
> You asked for a z-score — a value that would say "compared to a reference, how unusual is this?" Sentiment scores, however, are tied to language and cultural register: a score of −0.4 in German government-language does not mean the same thing as −0.4 in American news English. This comparability must be established by researchers — it cannot be computed.

On expand (methodological register):

> The BFF's validation gate requires both: (1) a `metric_baselines` row for `sentiment_score` on `tagesschau` (exists), and (2) a `metric_equivalence` row confirming cross-context comparability for this metric (does not exist). Probe 0 is monocultural and monolingual; cross-context equivalence is defined over at least two probes from distinct cultural contexts. See WP-004 §5.2 for the theory and Scientific Operations Guide Workflow 3 for the operational process.

Alternatives (always visible in both registers):

> → View raw values (default)
> → Choose a different metric with established equivalence
> → Wait for a second probe from a different context

**Example: the empty-lane caption on Surface II.**

Primary rendering (semantic register):

> This lane is not yet traversed.
> The function *Cohesion & Identity* describes sources where communities tell each other who they are — a fan forum, a diaspora news outlet, a religious community's publications. AĒR currently has no active probe serving this function for any context.

On expand (methodological register):

> No row in `source_classifications` with `primary_function = 'cohesion_identity'` is currently registered. A probe serving this function requires (a) nomination via WP-001 §4.4 Step 1 (area expert), (b) peer review, (c) technical feasibility assessment, (d) ethical review, (e) registration. See WP-001 §3.5 for the functional definition and Scientific Operations Guide Workflow 1 for registration.

**The principle in a sentence.** Both registers are always present in the DOM; only one is prominent at a time. No user is locked out of either register. No user is forced to read both.

This principle concretizes §5.3 and §5.4 — those sections describe *what* the dashboard communicates; §5.7 describes *how* it communicates.

### 5.8 Epistemic Weight

The visual prominence of a displayed metric scales with its methodological backing. This is an architectural requirement, not a stylistic preference.

**The problem.** AĒR's current pipeline is a proof-of-concept. All five Phase 42 extractors are Tier 1, unvalidated (Arc42 §13.3). Any claim built on them is provisional. The Working Papers anticipate this growing substantially: Tier 2 methods with Krippendorff's α ≥ 0.667 (WP-002 §4), cross-context equivalence registries (WP-004 §5.2), longitudinal validation with at least six-month stability windows (validation study template). When these validations complete, a metric becomes substantively more capable of supporting claims — and the dashboard must be able to reflect that growth, not treat every metric with identical epistemic caution.

**The principle.** The visual weight of a rendered metric is a function of its backing:

| Backing | Visual treatment |
|---|---|
| Tier 1, unvalidated (today's default) | Moderate weight. Default line thickness. Status badge present. Provisional status visible without interaction. |
| Tier 1, validated (post-Workflow-2) | Full weight. Clean rendering. Validation badge with expiration date visible on Layer 1 hover. |
| Tier 2 with established validation | Full weight. No caveat marks at Layer 0. Cross-context equivalence, where present, is carried as part of the metric's identity. |
| Tier 3 (LLM-augmented, non-deterministic) | Visible only through Progressive Disclosure from a Tier 1/2 baseline (per ADR-016). Rendered with distinct styling (e.g., dashed line weight) that signals its non-deterministic nature. Never a primary display. |
| Expired validation (`validation_status = 'expired'`) | Reverts to Tier 1 unvalidated treatment. The expiration is shown prominently — this is a revocation, not a demotion. |

**Why this is not the opposite of Non-Prescriptive Visualization (WP-006 §6.2).** Epistemic Weight and non-prescriptive visualization are complements. Non-prescriptive means: do not smuggle normative judgment into visual encoding (red-green for sentiment). Epistemic Weight means: let the scientific standing of a metric be visible in the metric's rendering, so the user knows what it can and cannot tell them. The first is about value judgments on *content*; the second is about confidence judgments on *method*. Both are truthful; only the first would distort the data.

**Why this is not the opposite of Tier 1 immutability (ADR-016).** ADR-016 guarantees that the Tier 1 baseline is never hidden by a Tier 2/3 enrichment. Epistemic Weight does not override this: when both a Tier 1 and a Tier 2 reading of the same phenomenon exist, both are visible. But the Tier 2 reading, being validated, may carry the line that appears by default, while the Tier 1 reading — the immutable baseline — remains available as an overlay or sibling trace. The user can always see both; the default selection follows methodological merit.

**The growth path.** As more metrics complete the validation protocol, AĒR's dashboard will render them with increasing confidence. This is not a cosmetic change — it is AĒR's core promise fulfilled. The instrument that today renders cautiously becomes, through accumulated interdisciplinary validation (WP-002 §4, WP-004 §5.2), an instrument capable of supporting substantive claims. The dashboard must be architecturally ready to carry those claims when they arrive.

### 5.9 Visualization Stack Separation

AĒR's visualization needs span four distinct domains, each with its own performance, interaction, and rendering requirements. The principle: **each domain is served by a framework-agnostic rendering module, and the UI framework is responsible only for chrome — panels, controls, layouts, routing, forms.** This separation is architectural, not stylistic.

| Domain | Purpose | Technology | Where it appears |
|---|---|---|---|
| **3D Atmosphere** | The rotating globe, probe points, terminator, absence fields, atmospheric halo. Interactive camera control: orbit, zoom, fly-to, tilt. Raycast interaction: hover, click, lasso-select. | **three.js** as vanilla engine module with custom GLSL shaders (terminator, probe glow, absence fields, Rayleigh/Mie atmospheric scattering). No declarative framework wrapper. | Surface I, L0–L2. The foundational visual identity of AĒR. |
| **2D Geo-Analytics** | Flat maps for regional or sub-regional analysis within a probe context. Choropleths, density maps, proportional symbols, flow-lines, data-layer overlays. | **MapLibre GL JS** (vector tiles, GPU-rendered) as base layer, **deck.gl** for high-performance data overlays. Standards-based, open-source, non-commercial. | Surface I/II, L3. When geo-distribution within a single probe is the subject. |
| **Scientific Charts** | Time series, small multiples, histograms, violin plots, ridgeline plots, box plots, heatmaps, streamgraphs, and the deep vocabulary of scientific visualization. | **uPlot** for dense time series (>1k points). **Observable Plot** as the primary declarative grammar for scientific diagrams (2026 state-of-the-art). **D3** as a utility library for custom visualizations that Plot cannot express (Sankey, chord, force-directed, Rhizome propagation graphs). | Surface II and III, L3–L4. The quantitative face of the instrument. |
| **Relational Networks** | Discourse propagation (Rhizome), entity co-occurrence, cross-probe narrative diffusion, multi-source agenda alignment. 2D force-directed, 3D spatial graph rendering. | **D3-force** for 2D relational graphs with deterministic layouts. **three.js** (reused from 3D Atmosphere) for 3D network visualization where spatial embedding carries meaning — e.g., Rhizome propagation vectors across cultural contexts. | Surface II, L3. Activated when Rhizome pillar mode is selected with multi-probe data. |

**The architectural rule.** Every visualization module in AĒR is a **pure rendering module**: it receives data and produces DOM, SVG, or Canvas/WebGL output. It is framework-agnostic. The UI framework wraps it with a thin adapter but never dictates its internals.

This rule has five concrete consequences:

1. **Framework choice is liberated from visualization quality.** The dashboard's scientific rigor does not depend on whether the UI framework has good charting libraries. It depends on whether the *rendering modules* — which live alongside the framework, not inside it — are chosen well.

2. **Bundles are per-domain, not per-framework.** three.js (~250 kB gzipped, lazy-loaded) only enters the bundle when Surface I renders. Observable Plot only when Surface II or III descends to L3. MapLibre+deck.gl only when a geo-analytic view is activated. The user who contemplates Layer 0 all day never downloads the scientific-chart stack.

3. **Performance budgets are per-domain.** The 3D Atmosphere holds the 60fps hard ceiling on capable hardware (§7). Scientific charts hold the 16ms/frame analysis target with ≤10k points. Each module is benchmarked against its own bar, not a global one.

4. **Domains can be tested in isolation.** A Storybook-style development environment for each rendering module, without the UI framework running. This is how AĒR's visualization infrastructure is kept scientifically auditable — a researcher reviewing the visualization code should not have to understand the UI framework to critique how a ridgeline-plot is built.

5. **Swapping a framework is cheap.** If a future maintainer decides to migrate from React to Svelte (or back, or to something newer), the visualization modules — where the hard engineering lives — are untouched. Only the chrome layer is rewritten.

**What this rules out.** No chart-library that is tightly coupled to a specific UI framework (Recharts, Chakra-UI charts, Mantine charts, Ant Design charts). No "visualization platform" that owns both the chart and the surrounding layout (Plotly Dash, Streamlit, Observable Framework). No commercial map tiles that would tie AĒR to a vendor contract (Mapbox's post-2020 licensing). No framework-specific 3D wrappers that would couple the engine to the chrome (react-three-fiber, Threlte, Solid-Three — even though they are excellent tools, they would violate the separation).

**What this is not.** This is not a prescription to reinvent charting primitives. Observable Plot, uPlot, MapLibre, deck.gl, D3, and three.js are mature, community-maintained, scientifically-respected libraries. The separation principle applies to *how they integrate into AĒR*, not to whether they exist.

**Relationship to §5.8 Epistemic Weight.** Every rendering module, regardless of domain, honors the Epistemic Weight rules from §5.8. A ridgeline-plot of a Tier 1 unvalidated metric renders with the same provisional treatment as a time series does. Weight is a property of the metric, not of the chart type.

---

## 6. The Extensibility Commitment

This is a core principle of the brief, not an afterthought. AĒR's six Working Papers identify over twenty open research questions. Every one of them may, when answered, require a new surface element, a new lane, a new metric, a new pillar, or a new dimension entirely. The dashboard must absorb this growth without structural redesign.

### 6.1 Structural openness by construction

**Functions are not four.** WP-001 §3 names four discourse functions, but WP-001 §8 Q2 explicitly asks whether this taxonomy is sufficient. A fifth function (e.g., "mediation" — actors bridging between functions), or a split of an existing function (religious epistemic authority vs. scientific epistemic authority), must be absorbed by Surface II without redesign. The lane metaphor is chosen precisely because it is a list — adding a lane is a data change, not a layout change.

**Pillars are not three.** Arc42 §1.2 names three pillars (Aleph, Episteme, Rhizome). The WP series does not foreclose the possibility of a fourth. If a future working paper introduces a new analytical stance, it must be integrable as a viewing mode (see §2.1) without the existing three being disrupted. The viewing mode toggle is therefore a set, not a tab strip.

**Metrics are not five.** The current extractors (`word_count`, `sentiment_score`, `language`, `temporal_distribution`, `entity_count`) are Tier 1. WP-002 and ADR-020 (planned) anticipate Tier 2 and Tier 3 methods. Every metric view must accept new metrics as a catalog entry — no view is hardcoded to specific metric names.

**Tier treatment is not fixed.** Validation status changes — a Tier 1 metric may complete Workflow 2 and become validated; a Tier 2 metric may expire and revert. The Epistemic Weight treatment (§5.8) must read validation status live from `/api/v1/metrics/available` and `/api/v1/metrics/{metricName}/provenance`, not from a frontend constant.

**Temporal scales are not five.** `5min / hourly / daily / weekly / monthly` is today's resolution set. WP-005 §6 anticipates scales beyond these (multi-year cultural drift, sub-minute event response). The resolution switch is therefore a range slider over a semantic scale, not a fixed five-button group.

**Equivalence levels are not three.** `temporal / deviation / absolute` (WP-004 §5.2) may grow. The badge system on composition views reads equivalence as a classified string from the API, not as a three-case enum in the frontend.

**Probes are not one — and their composition is not "comparison."** Everything about the design assumes Probe 0 is the first of many. The spatial distribution on the Atmosphere surface is infinitely scalable; the Coverage Map grows as new probes are registered; the multi-probe composition activates *automatically* the first day a second probe's data arrives, without a frontend release. Critically: the arrival of additional probes does not introduce a new surface. It makes the existing surfaces richer. A globe with seven luminous points is the same globe; function lanes with three context streams per lane are the same lanes. This is the Aleph principle enforced architecturally.

**Emic content is not hardcoded.** WP-001 §4.2 requires that emic designations and cultural context travel with each probe as untranslated, source-of-truth documentation. The dashboard renders this content from the database and the Probe Dossier directory (Arc42 §8.15) — no emic copy is baked into frontend source.

### 6.2 The additive extension test

Any proposed surface, component, or interaction is checked against: *what happens when a new probe is added from a different cultural setting? A new discourse function? A new pillar? A new metric that has just been validated? A previously validated metric that expired? A new limitation class? A probe whose cultural granularity has two levels, or seven?* If the answer is "the frontend needs a release" for anything other than displaying new methodological prose, the design is wrong. The dashboard grows with the science, not after it.

### 6.3 What this rules out

This principle explicitly rules out:

- Hardcoded iconography for "the four functions" or "the three pillars"
- Layout logic that assumes a specific number of lanes, metrics, or resolutions
- Copy that names specific probes, sources, or metric types in fixed positions
- Charts that cannot accept new metric types without code changes
- Region visualizations that hardcode which parts of the map are "covered"
- Dual-Register semantic copy (§5.7) that is written into the frontend codebase for specific metrics — semantic explanations must be sourced from a content layer (API-backed or versioned content files) so that new metrics arrive with their own explanations
- Emic designations that are translated into the frontend's UI language — `Tagesschau` is `Tagesschau`, not "German Public TV News"; `官方媒体` stays in its own script, with descriptive context adjacent, not transliterated into Latin
- A separate "cross-cultural comparison" surface or page — cultural composition belongs within the existing three surfaces, not in a dedicated arena (§3, §1.3)
- Epistemic Weight rules (§5.8) hardcoded per metric — the treatment is derived from live validation status, not from a frontend lookup table

Everything is driven by the BFF API responses and a content catalog. The frontend is a renderer of what the backend declares, not an author of what the user should see.

### 6.4 The probe is the granularity unit

This deserves its own section because it closes the door on a persistent design temptation: organising the dashboard around a universal cultural hierarchy.

**The temptation.** It would be tempting — and familiar from countless commercial dashboards — to organise cultural content around a schema like *World → Region → Country → Institution → Milieu → Source*. This reads naturally to Western users, maps cleanly to existing geodata, and appears to offer a universal zoom metaphor.

**Why this is forbidden.** WP-001 §2.1 names this pattern: **epistemological colonialism** — the assumption that Western institutional categories are universal. A country-centered hierarchy makes sense for a probe on German institutional media; it makes no sense for a probe on trans-national diaspora networks, or on religious community publications that span borders, or on clan-based oral traditions digitised through WhatsApp. Forcing such probes into a "country" slot erases what makes them distinct. WP-001 §3 is explicit: discourse functions vary in their institutional form across societies, and the AĒR system is built to observe that variation, not to flatten it.

**The rule.** The probe is the unit of granularity. Each probe declares its own cultural structure through its Probe Dossier (Arc42 §8.15) and its emic categories (WP-001 §4.2). The frontend reads this structure and renders it; it does not impose a schema. A probe may describe its context in one line ("institutional public-broadcasting German-language discourse"), or in a nested structure of its own design, or in a non-hierarchical set of overlapping affinity groups. The dashboard accommodates all of these.

**Practical consequence for the descent (§4, §4.5).** The cultural narrowing that accompanies the descent does not follow a fixed taxonomy. It follows the probe's own structure. When the user is at L3 on a probe about "the Russian exile-media ecosystem," the cultural reach is *that ecosystem's structure* — editorial genealogies, platform affiliations, geographic displacement — not "Russia" as a country. When the user is at L3 on a probe about German institutional epistemic authority, the cultural reach is institutional. Different probes, different shapes.

**Relationship to the extensibility test (§6.2).** A probe with an unusual granularity shape — say, seven nested levels, or no hierarchy at all — is not an edge case. It is the expected case. The frontend that only handles the Germany-shaped probe well is broken.

---

## 7. Performance as Design Constraint

Performance is not a nice-to-have. It is a design principle with hard numbers.

**Target hardware (High-Fidelity mode):** A five-year-old mid-range laptop (as of writing: a 2021 M1 MacBook Air or equivalent Intel integrated-graphics ThinkPad), running a mainstream browser, on a 1080p display. If the dashboard does not remain fluid on this hardware in High-Fidelity mode, the design has failed — no matter how beautiful the high-end experience.

**Target hardware (Low-Fidelity mode):** A ten-year-old laptop (as of writing: a 2015 ThinkPad with integrated graphics), on any mainstream browser with JavaScript enabled, on a 5 Mbps connection. If the dashboard is not usable for scientific work on this hardware in Low-Fidelity mode, the design has failed on the equity commitment of §5.6.

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

- First meaningful paint of the Atmosphere surface: < 1.5 seconds on a 50 Mbps connection
- Full interactivity at Layer 0: < 3 seconds
- Descent to any other layer: < 500 ms

**Initial load budgets (Low-Fidelity):**

- First meaningful paint: < 3 seconds on a 5 Mbps connection
- Full interactivity at Layer 0: < 6 seconds
- Descent to any other layer: < 1 second

**Data budgets:**

- The Atmosphere surface must render a valid state with a single BFF request (≤ 50 kB response)
- The Analysis layer must handle the BFF's hard row ceiling (currently 10,000 points, scaled by resolution multiplier per Arc42 §8.13) without jank
- The Reflection layer has no data budget — it is served as static content with occasional API enrichments

### 7.1 Why performance is an ethical requirement

A slow dashboard is not merely annoying; it is epistemically distorting. A user waiting four seconds for a chart unconsciously accepts the first answer they see rather than exploring alternatives. Responsiveness is a precondition for the Question-First Frontend (§5.3). If interactions feel costly, questions go unasked.

The Low-Fidelity mode (§5.6) extends this principle to hardware equity: a researcher on a 2012 laptop must be able to ask AĒR the same questions as a researcher on a 2024 workstation. The answer will look different; the epistemic access must be identical.

---

## 8. The API-key and Authentication Note

The current BFF uses a static API key (ADR-011). This is designed for server-to-server use. A static key in a browser bundle is a security problem — it lands in JS sources, in DevTools, in HAR files.

This brief does not resolve the issue; it names it as an architectural dependency. The brief assumes that before production deployment, one of two paths is taken:

- **Short-term pragmatic:** The frontend container injects the API key server-side through Traefik, and the browser communicates only with its own origin. This keeps the current ADR-011 posture intact while hiding the key from the client.
- **Medium-term correct:** An OIDC-based authentication flow (ADR-018, to be written), aligned with the deferred Principle 4 (Governed Openness) from ADR-017.

The design of the frontend must not foreclose either path. No design principle in this brief assumes in-browser credentials or embeds user identity into URL state.

---

## 9. What is deliberately not decided here

- **Framework, library, rendering strategy.** Deferred to a technical ADR after this brief is ratified.
- **Specific color values.** The brief commits to Viridis-family palettes on dark backgrounds; the exact stops, UI grays, and typographic scale are resolved in `design_system.md` (Phase 98).
- **Specific component inventory.** The brief names surfaces, layers, and overlays; the component tree will be derived from them.
- **Exact Low-Fidelity rendering strategy.** The brief commits to the principle (§5.6) and the performance budgets (§7); the specific 2D projection, tile strategy, and asset pipeline are resolved in the implementation phase.
- **Content catalog format.** The brief commits to the principle that Dual-Register semantic content (§5.7) is not hard-coded in frontend source; the specific storage format (API-served, versioned JSON, CMS, or other) is an implementation decision.
- **Specific Epistemic Weight visual mappings.** §5.8 defines the principle (weight scales with backing) and the tier-to-treatment table at a conceptual level; the exact line thicknesses, opacity values, and badge designs are resolved in `design_system.md` §4 (Phase 98).
- **The k-anonymity threshold for L5.** §4.5 commits to the principle (WP-006 §7 applies at Evidence layer) and names *k* = 10 from WP-006 §7.2 as today's default. The exact value per probe type is an operational parameter that may evolve as privacy research develops.
- **Accessibility specifics.** Full WCAG 2.2 AA conformance is a baseline assumption, not a design decision. Specifics (keyboard-only navigation paths, screen-reader semantics at Layer 0, reduced-motion modes) are resolved in the accessibility spec that accompanies implementation.

---

## 10. Relationship to existing documents

| Document | Relationship |
|---|---|
| `docs/arc42/00_manifesto.md` | Source of the atmospheric metaphor, the observer-position principle, the Resonance-over-Truth commitment, and the "documenting resonance, not judging it" stance that underpins §1.3 (Composition, not comparison). |
| `docs/arc42/01_introduction_and_goals.md` | Source of the five quality goals (§1.3) and the DNA pillars (§1.2). §2 of this brief binds every design principle to those goals; §4.5 renders the pillars fractally across descent depths. |
| `docs/design/visualization_guidelines.md` | The brief *encompasses* the guidelines. Guidelines constrain rendering; the brief defines structure. A rendering that follows the guidelines but violates the brief is still wrong. |
| `docs/arc42/09_architecture_decisions.md` (ADR-003, ADR-016, ADR-017) | ADR-003 (Progressive Disclosure via Metadata Index) is realized as the five-layer descent. ADR-016 (Hybrid Tier Architecture) is realized as §5.8 (Epistemic Weight). ADR-017 Principles 2 and 5 are realized as §5.2 (no valence) and §5.4 (refusal as feature). |
| WP-001 §2 (epistemological colonialism), §4 (Etic/Emic), §5.3 (Coverage Map), §8 | Source of §6.4 (probe as granularity unit), §3.4 (emic visibility), and the extensibility requirements. |
| WP-002 §4 (Validation Protocol), §8 (Tier Architecture) | Source of the validation-gated Epistemic Weight transitions in §5.8. |
| WP-003 §6 (demographic skew, Digital Divide) | Source of the demographic-opacity layer of the Negative Space overlay (§3.4, §4.4). |
| WP-004 §4–§5 (Levels of Cross-Cultural Comparison), §8 (Ethical Dimension) | Source of §1.3 (composition rather than comparison) and of the governance of multi-context rendering in §3.1, §3.2, §4.2. |
| WP-005 §6 (Pillar temporal signatures) | Source of the Pillar-to-viewing-mode mapping (§2.1) and of the fractal interaction between Pillars and descent depth (§4.5). |
| WP-006 §3.4, §6.2, §7 (privacy), §8.3 | Source of the Question-First principle, the non-prescriptive visualization stance, the k-anonymity gate at L5 (§4.5), and the open interdisciplinary evaluation commitment. |
| Future ADR-018 (Frontend Technology) | Is written *against* this brief. Any framework proposal must demonstrate compliance with §4 (layers), §4.5 (fractal granularity), §5.6 (fidelity modes), §5.7 (Progressive Semantics), §5.8 (Epistemic Weight), §5.9 (Visualization Stack Separation), §6 (extensibility), §7 (performance), and `visualization_guidelines.md`. |

---

## 11. Open questions for interdisciplinary review

Per WP-006 §8.3 Q5, AĒR's dashboard design should be evaluated with user studies before it hardens. This brief surfaces the questions that should structure such a review:

1. **Does the five-layer descent correspond to how researchers and journalists actually navigate AĒR?** Or are some layers conceptually redundant, or one missing?
2. **Does the "empty lane" pattern (§3.2, with Dual-Register from §5.7) communicate methodological openness, or does it read as incomplete?**
3. **Does the refusal UI (§5.4 + §5.7) invite inquiry (as intended) or frustrate users into abandoning the question?**
4. **Does the Negative Space overlay (§3.4, §4.4) make absence legible, or is it a clever designer trick that users toggle once and never again?** Specifically: does the demographic-opacity aspect of the overlay (WP-003 §6 absences) successfully communicate *whose voices are missing* from any given probe?
5. **Is the Reflection surface (§3.3) used, or is it a methodological archive that looks virtuous but remains unvisited?**
6. **Does Progressive Semantics (§5.7) successfully serve both audiences without patronizing either, or does one register systematically dominate?**
7. **Does Low-Fidelity mode (§5.6) feel like a first-class experience to users on older hardware, or does it read as a downgrade?**
8. **Does Epistemic Weight (§5.8) communicate the right thing?** When a metric is rendered at full weight (Tier 2 validated), do users over-trust it? When rendered at moderate weight (Tier 1 unvalidated), do users under-trust the data to the point of ignoring legitimate signals?
9. **Does the composition framing (§1.3) hold, or do users read multi-context displays as comparison anyway?** This is the critical WP-004 §8 question: can the interface resist the cognitive pull toward ranking?
10. **Does Fractal Cultural Granularity (§4.5) successfully communicate probe-specific structure without appearing inconsistent?** The risk is that users, accustomed to Western administrative hierarchies, read varying granularity shapes across probes as *confusion* rather than as a reflection of genuine cultural difference. What interface affordances help the user understand that the structure is the probe's, not the interface's?
11. **Does the four-domain visualization separation (§5.9) hold under real scientific review?** When a researcher wants a chart type that bridges domains — say, a geo-embedded network graph showing propagation flows over a map — does the separation force awkward integration, or does it clarify which module owns the rendering?

These are not rhetorical. They are the evaluation criteria for the first real user study of the AĒR dashboard, to be conducted after Iteration 1 reaches a usable state.

---

## Appendix A: Design principles at a glance

For quick reference during implementation reviews:

1. **Atmospheric Aesthetics** — stratification, currents, stillness-with-motion, observer at the edge, no faces.
2. **Composition, not comparison** — multiple contexts together form a richer Aleph, not a ranked list.
3. **Quality-goal binding** — every visual choice defends a quality goal from Arc42 §1.3.
4. **Three surfaces, one overlay** — Atmosphere, Function Lanes, Reflection; Negative Space everywhere.
5. **Five layers of descent, uniform across surfaces** — Immersion, Orientation, Exploration, Analysis, Provenance, Evidence. See the Surface × Layer matrix in §4.2.
6. **Fractal cultural granularity** — the descent narrows the cultural frame fluidly, following each probe's own emic structure rather than a fixed hierarchy. Pillars act at every depth with the cultural frame appropriate to that depth.
7. **Ockham's Razor, visible** — no chart junk, no decorative encoding.
8. **No valence, ever** — Viridis, numeric, neutral.
9. **Question-first, with answers where tenable** — invites questions; makes claims that the methodology supports; refuses claims that it does not.
10. **Refusal is a feature** — HTTP 400 becomes a methodological statement.
11. **Long sessions, dark defaults** — designed for hours of research work.
12. **Honest Desktop-First with High-Fidelity and Low-Fidelity modes** — the same science on a 2012 laptop and a 2024 workstation; only the atmosphere differs.
13. **Dual-Register / Progressive Semantics** — both semantic and methodological framings co-present; only one prominent at a time; never patronize, never exclude.
14. **Epistemic Weight** — visual prominence scales with methodological backing; validated metrics carry their weight, provisional metrics signal their provisionality.
15. **Visualization stack separation** — four rendering domains (3D Atmosphere, 2D Geo-Analytics, Scientific Charts, Relational Networks) served by framework-agnostic modules; the UI framework handles only chrome.
16. **The probe is the granularity unit** — no hardcoded cultural hierarchy; each probe declares its own structure; the frontend is a renderer, not an author.
17. **Extensibility by construction** — functions, pillars, metrics, resolutions, probes, tier treatments, emic content all additive.
18. **Performance as ethics** — responsiveness is a precondition for honest inquiry, including for users on weak hardware.