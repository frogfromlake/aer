<script lang="ts">
  // Phase 126 — per-cell configuration popover.
  //
  // A split / small-multiple panel shares one set of cell-shape levers (the
  // Panel default rendered by PanelControls). This popover lets ONE cell
  // override a subset of those levers when the shared value harms its
  // readability — the signposted exception to comparison-as-default (Brief
  // §1.3). It renders exactly the active presentation's `configurableParams`
  // (the same lever set PanelControls offers), bound to the cell instead of the
  // panel: writes go to `Panel.cellOverrides[cellKey]` via `setCellOverride`.
  // "Reset to panel default" clears the whole cell override.
  //
  // It deliberately does NOT touch panel-identity levers (view / metric /
  // composition / scope / window / layer / resolution / normalization) —
  // changing one of those is a new panel, not a cell tweak.
  //
  // Phase 141 — the lever rows themselves live in two per-type children
  // (CellConfigValueLevers = dimension peek + sliders + Band/Scale switches;
  // CellConfigChannelLevers = network/scatter channel selects + label switch);
  // this file is now the popover shell + the inherit/custom note + Reset.
  import { onMount, tick } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import { resetCellOverride } from '$lib/workbench/panel-mutators';
  import { resolveCellConfig } from '$lib/workbench/panel-queries';
  import type { Panel } from '$lib/state/url-internals';
  import type { PresentationDefinition } from '$lib/presentations';
  import CellConfigValueLevers from './CellConfigValueLevers.svelte';
  import CellConfigChannelLevers from './CellConfigChannelLevers.svelte';

  interface Props {
    panelPath: PanelPath;
    cellKey: string;
    /** Human label for the cell (its source / scope id) — shown in the header. */
    cellLabel: string;
    /** The live panel (for the effective values + which levers are overridden). */
    panel: Panel;
    presentation: PresentationDefinition;
    /** Scalar metric names for the scatter axis pickers (scope-available). */
    scalarMetricOptions: readonly string[];
    /** ADR-038 — dimensions valid for THIS cell's own source (same kind as the
     *  view), for the per-cell dimension-peek picker. */
    cellDimensionOptions: readonly string[];
    onClose: () => void;
  }
  let {
    panelPath,
    cellKey,
    cellLabel,
    panel,
    presentation,
    scalarMetricOptions,
    cellDimensionOptions,
    onClose
  }: Props = $props();

  const configParams = $derived(presentation.configurableParams ?? []);
  const cfg = $derived(resolveCellConfig(panel, cellKey));

  // ADR-038 — the per-cell dimension peek is offered for the single-dimension
  // views (distribution / time_series → a metric; categorical_distribution → a
  // field). Scatter / co-occurrence are channel-driven and use the channel
  // pickers instead, so the peek row is hidden there.
  const dimensionPeekable = $derived(
    (presentation.usesMetric ?? false) || (presentation.usesMetadataField ?? false)
  );
  const dimensionNoun = $derived(
    presentation.usesMetadataField
      ? m.workbench_ccp_dimension_noun_group_by()
      : m.workbench_ccp_dimension_noun_metric()
  );
  // Drives the view-dependent Top N ceiling in the value levers (mirrors the
  // panel-level lever's `viewUsesMetadataField`).
  const viewUsesMetadataField = $derived(presentation.usesMetadataField ?? false);

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      onClose();
    }
  }

  // Phase 128 a11y — keyboard operability. The Escape handler lives on the
  // popover, so focus must move INTO it on open (otherwise the key never
  // reaches the handler and Esc can't close it); on close, restore focus to
  // the trigger the user came from. Mirrors the DossierOverlay focus contract.
  let popoverEl: HTMLDivElement | undefined = $state();
  onMount(() => {
    const lastFocused = document.activeElement as HTMLElement | null;
    void tick().then(() => popoverEl?.focus());
    return () => lastFocused?.focus?.();
  });
</script>

<div
  bind:this={popoverEl}
  class="cell-config-popover"
  role="dialog"
  aria-label={m.workbench_ccp_aria_label({ label: cellLabel })}
  onclick={(e) => e.stopPropagation()}
  onkeydown={onKeydown}
  tabindex="-1"
  data-export-exclude="provenance"
>
  <header class="ccp-header">
    <span class="ccp-eyebrow">{m.workbench_ccp_eyebrow()}</span>
    <code class="ccp-label" title={cellLabel}>{cellLabel}</code>
    <button type="button" class="ccp-close" onclick={onClose} aria-label={m.workbench_ccp_close()}>
      ×
    </button>
  </header>

  {#if cfg.isOverridden}
    <p class="ccp-note">
      {m.workbench_ccp_note_custom_pre()} <strong>{m.workbench_ccp_note_custom_strong()}</strong>
      {m.workbench_ccp_note_custom_post()}
    </p>
  {:else}
    <p class="ccp-note ccp-note-muted">{m.workbench_ccp_note_inheriting()}</p>
  {/if}

  <div class="ccp-body">
    <CellConfigValueLevers
      {panelPath}
      {cellKey}
      {panel}
      {configParams}
      {dimensionPeekable}
      {dimensionNoun}
      {cellDimensionOptions}
      {viewUsesMetadataField}
    />
    <CellConfigChannelLevers {panelPath} {cellKey} {panel} {configParams} {scalarMetricOptions} />
  </div>

  <footer class="ccp-footer">
    <button
      type="button"
      class="ccp-reset"
      disabled={!cfg.isOverridden}
      onclick={() => {
        resetCellOverride(panelPath, cellKey);
        onClose();
      }}
    >
      {m.workbench_ccp_reset()}
    </button>
  </footer>
</div>

<style>
  .cell-config-popover {
    position: absolute;
    top: calc(var(--space-5) + var(--space-1));
    right: var(--space-2);
    z-index: 30;
    width: min(22rem, calc(100% - var(--space-4)));
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-md);
    box-shadow: 0 8px 24px rgb(0 0 0 / 45%);
  }

  .ccp-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
  }
  .ccp-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
  }
  .ccp-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    flex: 1;
  }
  .ccp-close {
    appearance: none;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24×24px minimum target size. */
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 24px;
    min-height: 24px;
    background: transparent;
    border: none;
    color: var(--color-fg-subtle);
    font-size: var(--font-size-md);
    line-height: 1;
    cursor: pointer;
    padding: 0 var(--space-1);
  }
  .ccp-close:hover,
  .ccp-close:focus-visible {
    color: var(--color-fg);
  }

  .ccp-note {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-accent);
  }
  .ccp-note-muted {
    color: var(--color-fg-muted);
  }

  .ccp-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .ccp-footer {
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-2);
  }
  .ccp-reset {
    appearance: none;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24px minimum target height. */
    display: inline-flex;
    align-items: center;
    min-height: 24px;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .ccp-reset:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .ccp-reset:not(:disabled):hover {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }
</style>
