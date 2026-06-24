<script lang="ts">
  // Phase 141 — panel-level soft disclosures, extracted from PanelHost.svelte.
  // Four reflexive notes that sit above the cell grid, all about how the cells
  // may (or may not) be read together:
  //   - shared-axis refusal (Phase 124 / WP-004 §6.3 — cross-context intensive),
  //   - dropped sources lacking the active dimension (ADR-038),
  //   - faceting unavailable on a multi-source / multi-group panel (Phase 125a),
  //   - cross-panel brushing discoverability hint (Phase 125b).
  // Purely presentational — every condition is computed by PanelCellGrid.
  import { m } from '$lib/paraglide/messages.js';
  import { fieldLabel, dimensionLabel, sourceLabel } from '$lib/state/labels.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';

  interface Props {
    /** Cross-context intensive metric → independent axes (Phase 124). */
    shareForbidden: boolean;
    /** The panel's active dimension (named in the share/drop notes). */
    metric: string;
    /** Scoped sources dropped because they lack the active dimension (ADR-038). */
    droppedSources: string[];
    /** No comparable categorical field across the scope — the drop note is
     *  suppressed in favour of the in-grid "no shared dimension" message. */
    noSharedDimension: boolean;
    /** Faceting requested but the panel splits into >1 base unit (Phase 125a). */
    facetUnavailable: boolean;
    facetField: string;
    /** Scatter / parallel-coordinates panel with a live Window selection. */
    showBrushHint: boolean;
  }

  let {
    shareForbidden,
    metric,
    droppedSources,
    noSharedDimension,
    facetUnavailable,
    facetField,
    showBrushHint
  }: Props = $props();
</script>

{#if shareForbidden}
  <MethodologyBanner anchorHref="/reflection/wp/wp-004?section=6.3" anchorLabel="WP-004 §6.3">
    <strong>{m.workbench_disclosure_independent_axes_strong()}</strong>
    {m.workbench_disclosure_independent_axes_body({ metric })}
  </MethodologyBanner>
{/if}

<!-- ADR-038 — disclose sources dropped because they lack the active dimension,
     so absence is named, never silent (Tier 2 "show anyway" payoff). -->
{#if droppedSources.length > 0 && !noSharedDimension}
  <p class="panel-drop-note" role="note">
    {m.workbench_disclosure_dropped_prefix()}
    <strong>{droppedSources.map(sourceLabel).join(', ')}</strong>
    {m.workbench_disclosure_dropped_suffix()}
    <!-- dimensionLabel resolves either a metric OR a categorical field (e.g.
         a cross-tab group-by `author` → "Autor"), so a field-driven view never
         shows the raw/humanised English name here (Phase 148g localize fix). -->
    <code>{dimensionLabel(metric)}</code>
    {m.workbench_disclosure_dropped_not_emitted()}
  </p>
{/if}

<!-- Phase 125a — faceting is set but the panel splits into >1 base scope unit
     (multi-source / multi-group). We refuse to facet rather than silently drop
     sources, and say so via the shared yellow banner so the hint is as visible
     as the other soft refusals. -->
{#if facetUnavailable}
  <MethodologyBanner
    variant="warn"
    anchorHref="/reflection/wp/wp-004?section=6.3"
    anchorLabel="WP-004 §6.3"
  >
    <strong>{m.workbench_disclosure_facet_unavailable_strong()}</strong>
    {m.workbench_disclosure_facet_unavailable_body_pre()}
    <code>{fieldLabel(facetField)}</code>
    {m.workbench_disclosure_facet_unavailable_body_post()}
    <strong>{m.workbench_disclosure_facet_unavailable_merged()}</strong>
    {m.workbench_disclosure_facet_unavailable_body_end()}
  </MethodologyBanner>
{/if}

<!-- Phase 125b — ISSUE 3: make cross-panel linked brushing discoverable. Shown
     on scatter / parallel-coordinates panels; clicking a mark selects that
     article and highlights it in any linked Scatter ↔ Parallel-coordinates
     panel in the same Window. -->
{#if showBrushHint}
  <p class="panel-brush-hint" role="note">
    {m.workbench_disclosure_brush_hint_pre()}
    <strong>{m.workbench_disclosure_brush_hint_scatter()}</strong> ↔
    <strong>{m.workbench_disclosure_brush_hint_parallel()}</strong>
    {m.workbench_disclosure_brush_hint_post()}
  </p>
{/if}

<style>
  /* ADR-038 — dropped-source disclosure note above the cell grid. */
  .panel-drop-note {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .panel-drop-note code {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }
  /* Phase 125b — ISSUE 3: cross-panel brushing discoverability hint. */
  .panel-brush-hint {
    margin: 0 0 var(--space-2);
    padding: var(--space-1) var(--space-2);
    border-left: 2px solid var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 8%, transparent);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }
</style>
