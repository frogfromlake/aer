<script lang="ts">
  // CellConfigValueLevers — Phase 141 (extracted from CellConfigPopover).
  //
  // The scalar / toggle half of the per-cell override popover, in render order:
  // the ADR-038 dimension peek (a select) + Bins / Top N / Spread sliders + the
  // Band / Scale switches. Sibling CellConfigChannelLevers owns the channel
  // selects (network + scatter) and the display-language switch; the split point
  // (after `scales`, before `networkChannels`) preserves the popover's lever
  // ORDER for every presentation. Each lever commits to
  // `Panel.cellOverrides[cellKey]` via `setCellOverride`, clearing the override
  // when the chosen value equals the panel default (so a cell that matches its
  // siblings is never falsely flagged "custom").
  import { m } from '$lib/paraglide/messages.js';
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import { setCellOverride } from '$lib/workbench/panel-mutators';
  import { resolveCellConfig } from '$lib/workbench/panel-queries';
  import { DEFAULT_BINS, DEFAULT_FORCE_STRENGTH, DEFAULT_TOPN } from '$lib/workbench/cell-levers';
  import {
    clampBins,
    clampForce,
    clampTopN,
    axisOverrideForcesFree,
    buildDimensionOptions
  } from '$lib/workbench/cell-config-popover-internals';
  import type { Panel } from '$lib/state/url-internals';

  interface Props {
    panelPath: PanelPath;
    cellKey: string;
    panel: Panel;
    configParams: readonly string[];
    /** ADR-038 — whether this presentation offers a per-cell dimension peek. */
    dimensionPeekable: boolean;
    /** "Metric" (metric views) or "Group by" (metadata-field views). */
    dimensionNoun: string;
    /** Dimensions valid for THIS cell's own source. */
    cellDimensionOptions: readonly string[];
  }
  let {
    panelPath,
    cellKey,
    panel,
    configParams,
    dimensionPeekable,
    dimensionNoun,
    cellDimensionOptions
  }: Props = $props();

  const cfg = $derived(resolveCellConfig(panel, cellKey));
  const override = $derived(panel.cellOverrides?.[cellKey]);

  // Per-lever "is this overridden" — drives the per-row override dot.
  const ovMetric = $derived(override?.metric !== undefined);
  const ovBins = $derived(override?.bins !== undefined);
  const ovTopN = $derived(override?.topN !== undefined);
  const ovForce = $derived(override?.forceStrength !== undefined);
  const ovBand = $derived(override?.showBand !== undefined);
  const ovScales = $derived(override?.scales !== undefined);

  // Effective values (override ?? panel default ?? cell default).
  const effBins = $derived(cfg.bins ?? DEFAULT_BINS);
  const effTopN = $derived(cfg.topN ?? DEFAULT_TOPN);
  const effForce = $derived(cfg.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const effShowBand = $derived(cfg.showBand ?? true);
  const effScale = $derived<'shared' | 'free'>(cfg.scales ?? 'shared');
  // A per-cell X/Y axis override changes WHAT the axis measures, so the cell can
  // no longer share the panel axis — PanelHost forces it to 'free'. Reflect that
  // here so the Scale lever never claims 'shared' for a cell that renders free.
  const forcesFree = $derived(axisOverrideForcesFree(override));
  const effScaleDisplay = $derived<'shared' | 'free'>(forcesFree ? 'free' : effScale);

  // The select shows the OVERRIDE when set, else '' (= inherit the panel default).
  const dimensionSelectValue = $derived(override?.metric ?? '');
  const dimensionOptions = $derived(buildDimensionOptions(cellDimensionOptions, override?.metric));

  // Live slider read-outs — commit to the override only on `onchange`.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  const displayBins = $derived(liveBins ?? effBins);
  const displayTopN = $derived(liveTopN ?? effTopN);
  const displayForce = $derived(liveForce ?? effForce);

  // The panel-level value each lever inherits when this cell has no override —
  // setting a cell back to this clears the override.
  const panelBins = $derived(panel.bins ?? DEFAULT_BINS);
  const panelTopN = $derived(panel.topN ?? DEFAULT_TOPN);
  const panelForce = $derived(panel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const panelBand = $derived(panel.showBand ?? true);
  const panelScale = $derived<'shared' | 'free'>(panel.scales ?? 'shared');

  // Inherit reverts the cell to the panel dimension; choosing the panel's own
  // dimension also clears the override (no no-op "custom").
  function setDimension(value: string) {
    const chosen = value === '' ? undefined : value;
    setCellOverride(panelPath, cellKey, { metric: chosen === panel.metric ? undefined : chosen });
  }
  function setBins(n: number) {
    const v = clampBins(n);
    if (v === null) return;
    setCellOverride(panelPath, cellKey, { bins: v === panelBins ? undefined : v });
  }
  function setTopN(n: number) {
    const v = clampTopN(n);
    if (v === null) return;
    setCellOverride(panelPath, cellKey, { topN: v === panelTopN ? undefined : v });
  }
  function setForce(n: number) {
    const v = clampForce(n);
    if (v === null) return;
    setCellOverride(panelPath, cellKey, { forceStrength: v === panelForce ? undefined : v });
  }
  function setBand(next: boolean) {
    setCellOverride(panelPath, cellKey, { showBand: next === panelBand ? undefined : next });
  }
  function setScale(next: 'shared' | 'free') {
    setCellOverride(panelPath, cellKey, { scales: next === panelScale ? undefined : next });
  }
</script>

{#if dimensionPeekable}
  <!-- ADR-038 — per-cell dimension peek. Choosing a dimension other than the
       panel's puts THIS cell off-comparison (a loud banner shows on the cell).
       Options are limited to dimensions this cell's own source emits. -->
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_cell_dimension_group()}>
    <span class="ccp-rk"
      >{dimensionNoun}
      {#if ovMetric}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <select
      class="ccp-select"
      value={dimensionSelectValue}
      onchange={(e) => setDimension((e.currentTarget as HTMLSelectElement).value)}
      aria-label={m.workbench_ccp_cell_dimension_select_label()}
      title={m.workbench_ccp_cell_dimension_select_title()}
    >
      <option value="">{m.workbench_ccp_dimension_inherit({ metric: panel.metric })}</option>
      {#each dimensionOptions as d (d)}
        <option value={d}>{d}</option>
      {/each}
    </select>
  </div>
{/if}

{#if configParams.includes('bins')}
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_bins_group()}>
    <span class="ccp-rk"
      >{m.workbench_ccp_bins_label()}
      {#if ovBins}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <div class="ccp-inline">
      <input
        type="range"
        min="5"
        max="120"
        step="1"
        value={effBins}
        oninput={(e) => (liveBins = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setBins(Number((e.currentTarget as HTMLInputElement).value));
          liveBins = null;
        }}
        aria-label={m.workbench_ccp_bins_input_label()}
      />
      <output class="ccp-val">{displayBins}</output>
    </div>
  </div>
{/if}

{#if configParams.includes('topN')}
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_topn_group()}>
    <span class="ccp-rk"
      >{m.workbench_ccp_topn_label()}
      {#if ovTopN}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <div class="ccp-inline">
      <input
        type="range"
        min="5"
        max="500"
        step="5"
        value={effTopN}
        oninput={(e) => (liveTopN = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setTopN(Number((e.currentTarget as HTMLInputElement).value));
          liveTopN = null;
        }}
        aria-label={m.workbench_ccp_topn_input_label()}
      />
      <output class="ccp-val">{displayTopN}</output>
    </div>
  </div>
{/if}

{#if configParams.includes('forceStrength')}
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_spread_group()}>
    <span class="ccp-rk"
      >{m.workbench_ccp_spread_label()}
      {#if ovForce}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <div class="ccp-inline">
      <input
        type="range"
        min="0"
        max="100"
        step="1"
        value={effForce}
        oninput={(e) => (liveForce = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setForce(Number((e.currentTarget as HTMLInputElement).value));
          liveForce = null;
        }}
        aria-label={m.workbench_ccp_spread_input_label()}
      />
      <output class="ccp-val">{displayForce}</output>
    </div>
  </div>
{/if}

{#if configParams.includes('band')}
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_band_group()}>
    <span class="ccp-rk"
      >{m.workbench_ccp_band_label()}
      {#if ovBand}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <button
      type="button"
      role="switch"
      aria-checked={effShowBand}
      class="ccp-btn"
      class:active={effShowBand}
      onclick={() => setBand(!effShowBand)}
    >
      {effShowBand ? m.workbench_ccp_band_shown() : m.workbench_ccp_band_hidden()}
    </button>
  </div>
{/if}

{#if configParams.includes('scales')}
  <div class="ccp-row" role="group" aria-label={m.workbench_ccp_scale_group()}>
    <span class="ccp-rk"
      >{m.workbench_ccp_scale_label()}
      {#if ovScales}<span class="ccp-dot" title={m.workbench_ccp_overridden()}>●</span>{/if}</span
    >
    <button
      type="button"
      role="switch"
      aria-checked={effScaleDisplay === 'shared'}
      class="ccp-btn"
      class:active={effScaleDisplay === 'shared'}
      disabled={forcesFree}
      onclick={() => setScale(effScaleDisplay === 'shared' ? 'free' : 'shared')}
      title={forcesFree ? m.workbench_ccp_scale_title_forced_free() : m.workbench_ccp_scale_title()}
    >
      {effScaleDisplay === 'shared' ? m.workbench_ccp_scale_shared() : m.workbench_ccp_scale_free()}
    </button>
  </div>
{/if}

<style>
  /* Per-cell config rows (compact popover variant — distinct from the roomier
     control-strip `.ctrl-row`). Duplicated with CellConfigChannelLevers: Svelte
     scopes `<style>` per-component and the markup differs enough that a shared
     primitive would not pay for itself (cf. PanelHost/L5 precedent). */
  .ccp-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  /* The dimension select fills the row (the network/scatter selects do too, in
     the sibling child); the scatter grid opts out via `.ccp-row-grid` there. */
  .ccp-row > .ccp-select {
    flex: 1 1 0;
  }

  .ccp-rk {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    min-width: 3rem;
  }
  .ccp-dot {
    color: var(--color-accent);
    font-size: 9px;
    vertical-align: middle;
  }
  .ccp-inline {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1;
  }
  .ccp-inline input[type='range'] {
    flex: 1;
    accent-color: var(--color-accent);
  }
  .ccp-val {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    min-width: 2.5rem;
    text-align: right;
  }

  .ccp-btn {
    appearance: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .ccp-btn.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }
  .ccp-btn:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .ccp-select {
    appearance: none;
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
    /* Phase 126 (fix) — never exceed the popover; truncate the visible label
       instead of overflowing the container. */
    min-width: 0;
    max-width: 100%;
    text-overflow: ellipsis;
  }
  .ccp-select:hover,
  .ccp-select:focus-visible {
    border-color: var(--color-accent);
  }
</style>
