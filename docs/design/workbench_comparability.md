# Workbench Metadata Comparability

> Living specification for how the Workbench offers, compares, and renders metadata
> dimensions across sources and probes. Companion to **ADR-038**. When behaviour
> and this document disagree, the document is the intent — fix the code or update
> the document deliberately.

## Why this exists

The Workbench compares discourse across sources and probes. Metadata coverage is
**genuinely asymmetric** — an institutional press office emits no `section`/`author`
per article; a large newsroom emits many fields; a small one emits few. Comparison
is only meaningful between dimensions the compared sources actually share. Without a
single, uniform model this produces chaos: empty cells beside full ones, pickers
offering dimensions a source lacks, and inconsistent behaviour between metrics and
categorical fields. This document fixes the model once, for all dimensions, at any
scale of probes/sources.

**Non-negotiable principles**

- **Disclose-never-coerce** — an absent field is never `0`/`""`; absence is shown as
  absence (WP-003 §3.2).
- **No discovery bias** — offering/search uses universal availability, never a
  source's capability richness as a ranking signal.
- **Modular / data-driven** — *no probe-specific or source-specific code anywhere*.
  Everything derives from the availability endpoints + the presentation registry, so
  adding the Nth probe/source needs only config + a migration.
- **Uniform** — metrics and categorical fields behave identically, and within-frame
  (one probe, many sources) behaves identically to cross-probe.
- **Coherent by default** — the default panel configuration is always fully populated;
  every visible cell carries data. Deviation from apples-to-apples comparison is
  always an explicit, signposted user choice.

## The metadata model (where dimensions live)

| Kind | Storage | Surfaced as | Picker |
|---|---|---|---|
| **Scalar** (image_count, paywall_status, external_citation_count, comment_count, reading_time_minutes) | `aer_gold.metrics` as ordinary `metric_name`s | distribution / time_series / scatter | **Metric** picker (a "Metadata" group) |
| **Categorical** (section, author, tags, categories, article_type, editorial_labels, dateline_location, editor) | `aer_gold.article_metadata` (one row per (article, field), Array value) | `categorical_distribution` | **Group by** picker |

Non-dimensional metadata is deliberately **not** offered: free text (`description`),
URLs (`image_url`, `comment_url`), dates (`published_date`, `modified_date`,
`revision_date` — these are the time axis), and `word_count` (already a metric). The
per-source **Metadata coverage** matrix (Dossier) shows the full field vocabulary;
the Workbench offers exactly the subset that is a *comparable dimension*.

Availability is resolved per scope by `GET /scope/available-metrics` and
`GET /scope/available-metadata`, which bucket every dimension into:

- **`available`** — present on **every** scoped source (the intersection).
- **`partial`** — present on **some** scoped sources (with the source list).

## The three-tier comparability model

| Tier | Offers | Comparability | Mechanism |
|---|---|---|---|
| **1 — Panel default** | only the **intersection** (`available`) | ✓ apples-to-apples; every cell filled | `available[]` |
| **2 — Panel "show anyway"** | a **partial** dimension across the sources that have it; lacking sources are **dropped from the panel + disclosed in a note** | ✓ same dimension, fewer cells | `showWithheld` + `partial[]` |
| **3 — Per-cell peek** | *one* cell overrides to a **different dimension valid for its own source** (same kind as the view) | ✗ deliberately off-comparison | `cellOverrides[key].metric` + a **loud banner** |

Read it as: **the panel compares; the cell peeks.** Tier 1 is the safe default. Tier
2 lets you compare a dimension that not all sources share, across the subset that
does — still one dimension, still a valid comparison, with the missing sources named
rather than shown empty. Tier 3 is the explicit escape hatch to glance at a
cell-specific dimension; it breaks comparability by design, so it carries a prominent
"not comparable to the sibling cells" banner (reusing the Phase-126 override-with-
signposting pattern, escalated for a dimension change).

### What this means concretely

- A source that lacks the chosen dimension is **dropped from the fan-out and named in
  a panel note** — for both metrics and categorical fields, in split and (by pooling)
  merged. Never an empty cell with a wall of explanatory text next to a full one.
- The "N withheld · show anyway" disclosure + toggle appears **whenever partials
  exist** — within a single multi-source probe just as much as cross-probe. Partials
  are never folded in silently.
- The default seed for a field/metric is the first **intersection** dimension
  (deterministic), never a hard-coded field name, never a partial.

## Feature-interaction matrix (standing test checklist)

Behaviour of a **source that lacks the chosen dimension** — every row must be
identical (that uniformity *is* the contract):

| view | single source | multi-source, one probe | cross-probe |
|---|---|---|---|
| **distribution** (scalar) | offers only its dims | default = intersection; partial → show-anyway → lacking source **dropped + noted** | same |
| **categorical_distribution** (field) | offers only its dims | default = intersection; partial → show-anyway → lacking source **dropped + noted** | same |
| **time_series** (scalar) | per-source lanes | same intersection / drop rule | merged intensive → refusal (unchanged) |
| **metric_scatter / cooccurrence** | channel-driven | channel-driven (per-cell channel overrides) | — |

Composition interaction:

- **split** — one cell per (kept) source; dropped sources are named in the panel note.
- **merged** — sources pool server-side; a lacking source contributes nothing
  (invisible); the cell is empty only if the whole union is empty.
- **overlay** — per-source lines on one canvas; lacking source contributes no line.

Per-cell peek (Tier 3) verification: on a rendered cell, the config popover offers
dimensions valid for *that cell's source*; choosing one re-renders the cell with a
loud "different dimension" banner, leaves siblings untouched, and survives a URL
round-trip.

Empty-state cases that legitimately remain (rare, after drop+note):

| case | render |
|---|---|
| merged union genuinely empty | one compact `CellEmptyState` note |
| single source, no rows in window | one compact `CellEmptyState` note |
| constant metric (zero variance) | finite centred bar + "constant value N" caption |

## Intentions / open questions

- Tier 2 ("show anyway") and Tier 3 (per-cell peek) are complementary: Tier 2 keeps
  one dimension across a subset; Tier 3 puts a different dimension on one cell. Both
  signposted; neither silent.
- The intersection-only default applies to **all** dimensions, including sentiment
  tiers — a within-frame partial sentiment model sits behind "show anyway" for
  consistency (ADR-038). Reversible to metadata-only if it proves too strict.
- Cross-cultural *equivalence* gating (WP-004) is orthogonal and unchanged.

## Implementation anchors

- Availability + withheld + seed: `PanelControls.svelte` (`isScopeAvailable`,
  `offerableMetadataFields`, `firstMetadataField`, the withheld blocks).
- Drop + note + per-cell render: `PanelHost.svelte` (`metricSourceFilter` /
  `sourceNamesForProbe`).
- Per-cell override: `panel-queries.ts` (`resolveCellConfig`), `url-internals.ts`
  (`CellOverride` + codec), `CellConfigPopover.svelte`.
- Dimension classes: `metric-presentation.ts` (`isMetadataMetric`, `isIntegerMetric`).
- Shared empty-state: `CellEmptyState.svelte`.
