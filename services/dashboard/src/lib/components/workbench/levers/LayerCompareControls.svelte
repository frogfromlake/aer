<script lang="ts">
  // LayerCompareControls — Phase 141 (extracted from PanelControls); Phase 151
  // narrowed to Compare only (the Layer toggle moved onto the Window row).
  //
  // Compare (raw / deviation / percentile) is shown only where the active view
  // consumes normalization; deviation/percentile are disabled unless the metric
  // carries a deviation/absolute equivalence grant (ADR-016 / Phase 115), with a
  // "?" explainer instead of a silent refusal.
  import type { Normalization, Panel } from '$lib/state/url-internals';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    viewUsesNormalization: boolean;
    /** Metric carries a deviation/absolute equivalence grant. */
    canNormalize: boolean;
  }

  let { panelPath, boundPanel, viewUsesNormalization, canNormalize }: Props = $props();

  const activeNormalization = $derived<Normalization>(boundPanel.normalization ?? 'raw');
  const activeMetric = $derived(boundPanel.metric);
  let showCompareHelp = $state(false);

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

<!-- Compare lever (Phase 151 — Layer moved to the Window row). Shown only where
     the active view consumes normalization. -->
{#if viewUsesNormalization}
  <div class="ctrl-row ctrl-row-split">
    <div class="ctrl-group" role="radiogroup" aria-label={m.levers_compare_aria()}>
      <span class="ctrl-eyebrow">{m.levers_compare_eyebrow()}</span>
      <div class="ctrl-options">
        <LeverButton
          role="radio"
          active={activeNormalization === 'raw'}
          title={m.levers_compare_raw_title()}
          onclick={() => pickNorm('raw')}
        >
          {m.levers_compare_raw()}
        </LeverButton>
        <LeverButton
          role="radio"
          active={activeNormalization === 'zscore'}
          disabled={!canNormalize}
          title={canNormalize
            ? m.levers_compare_deviation_title()
            : m.levers_compare_disabled_title()}
          onclick={() => pickNorm('zscore')}
        >
          {m.levers_compare_deviation()}
        </LeverButton>
        <LeverButton
          role="radio"
          active={activeNormalization === 'percentile'}
          disabled={!canNormalize}
          title={canNormalize
            ? m.levers_compare_percentile_title()
            : m.levers_compare_disabled_title()}
          onclick={() => pickNorm('percentile')}
        >
          {m.levers_compare_percentile()}
        </LeverButton>
        {#if !canNormalize}
          <button
            type="button"
            class="ctrl-help"
            aria-expanded={showCompareHelp}
            title={m.levers_compare_help_aria()}
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
  </div>
{/if}
{#if viewUsesNormalization && !canNormalize && showCompareHelp}
  <p class="compare-help" role="note">
    <strong>{m.levers_compare_help_deviation()}</strong>
    {m.levers_compare_help_pre()}
    <strong>{m.levers_compare_help_percentile()}</strong>
    {m.levers_compare_help_mid()}
    <em>{m.levers_compare_help_across_contexts()}</em>
    {m.levers_compare_help_body()}
    <code>{activeMetric}</code>
    {m.levers_compare_help_post()}
    <strong>{m.levers_compare_help_raw()}</strong>
    {m.levers_compare_help_raw_always()}
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
