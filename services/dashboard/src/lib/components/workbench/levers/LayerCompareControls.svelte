<script lang="ts">
  // LayerCompareControls — Phase 141 (extracted from PanelControls).
  //
  // Two low-frequency levers grouped on one split row: Layer (Au Gold /
  // Ag Silver) and Compare (raw / deviation / percentile). Compare is shown only
  // where the active view consumes normalization; deviation/percentile are
  // disabled unless the metric carries a deviation/absolute equivalence grant
  // (ADR-016 / Phase 115), with a "?" explainer instead of a silent refusal.
  import type { Normalization, Panel } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    viewUsesNormalization: boolean;
    /** Metric carries a deviation/absolute equivalence grant. */
    canNormalize: boolean;
  }

  let { panelPath, boundPanel, viewUsesNormalization, canNormalize }: Props = $props();

  const activeLayer = $derived<'gold' | 'silver'>(boundPanel.layer);
  const activeNormalization = $derived<Normalization>(boundPanel.normalization ?? 'raw');
  const activeMetric = $derived(boundPanel.metric);
  let showCompareHelp = $state(false);

  function pickLayer(next: 'gold' | 'silver') {
    if (next === activeLayer) return;
    updatePanel(panelPath, (p) => ({ ...p, layer: next }));
  }
  function pickNorm(next: Normalization) {
    if (next === activeNormalization) return;
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      if (next === 'raw') delete out.normalization;
      else out.normalization = next;
      return out;
    });
  }
</script>

<!-- Layer + Compare on one row — both low-frequency; grouped to save space. -->
<div class="ctrl-row ctrl-row-split">
  <div class="ctrl-group" role="radiogroup" aria-label="Data layer">
    <span class="ctrl-eyebrow">Layer</span>
    <div class="ctrl-options">
      <LeverButton
        role="radio"
        active={activeLayer === 'gold'}
        variant="layer-btn"
        title="Au Gold — aggregated metrics"
        onclick={() => pickLayer('gold')}
      >
        Au Gold
      </LeverButton>
      <LeverButton
        role="radio"
        active={activeLayer === 'silver'}
        variant="layer-btn silver"
        title="Ag Silver — document-level data (WP-006 §5.2)"
        onclick={() => pickLayer('silver')}
      >
        Ag Silver
      </LeverButton>
    </div>
  </div>

  {#if viewUsesNormalization}
    <div class="ctrl-group" role="radiogroup" aria-label="Normalization">
      <span class="ctrl-eyebrow">Compare</span>
      <div class="ctrl-options">
        <LeverButton
          role="radio"
          active={activeNormalization === 'raw'}
          title="Raw values"
          onclick={() => pickNorm('raw')}
        >
          raw
        </LeverButton>
        <LeverButton
          role="radio"
          active={activeNormalization === 'zscore'}
          disabled={!canNormalize}
          title={canNormalize
            ? 'Z-score deviation from the baseline'
            : 'Needs a validated baseline + cross-context equivalence (Phase 115) — not available for this metric yet. Click ? to learn why.'}
          onclick={() => pickNorm('zscore')}
        >
          deviation
        </LeverButton>
        <LeverButton
          role="radio"
          active={activeNormalization === 'percentile'}
          disabled={!canNormalize}
          title={canNormalize
            ? 'Percentile rank within scope'
            : 'Needs a validated baseline + cross-context equivalence (Phase 115) — not available for this metric yet. Click ? to learn why.'}
          onclick={() => pickNorm('percentile')}
        >
          percentile
        </LeverButton>
        {#if !canNormalize}
          <button
            type="button"
            class="ctrl-help"
            aria-expanded={showCompareHelp}
            title="Why are deviation / percentile disabled?"
            onclick={(e) => {
              e.stopPropagation();
              showCompareHelp = !showCompareHelp;
            }}
          >
            ?
          </button>
        {/if}
      </div>
    </div>
  {/if}
</div>
{#if viewUsesNormalization && !canNormalize && showCompareHelp}
  <p class="compare-help" role="note">
    <strong>deviation</strong> (z-score) and <strong>percentile</strong> compare values
    <em>across contexts</em> — which asserts the metric measures the same thing in each. AĒR only
    allows that once a baseline + a cross-context equivalence study exist for the metric (ADR-016 /
    WP-004); <code>{activeMetric}</code> has none yet, so these stay disabled rather than show an
    unproven comparison. <strong>raw</strong> always works.
  </p>
{/if}

<style>
  .ctrl-row-split {
    gap: var(--space-4);
  }

  .ctrl-group {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }

  /* Phase 131 (BUG1) — Compare "?" explainer. */
  .ctrl-help {
    appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 1;
    cursor: pointer;
    padding: 0;
  }
  .ctrl-help:hover,
  .ctrl-help:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
  .compare-help {
    margin: var(--space-2) 0 0;
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-xs);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    background: color-mix(in srgb, var(--color-accent) 6%, transparent);
    border-left: 2px solid var(--color-accent-muted);
    border-radius: var(--radius-sm);
  }
  .compare-help code {
    font-family: var(--font-mono);
  }
</style>
