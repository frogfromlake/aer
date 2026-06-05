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
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import { resetCellOverride, setCellOverride } from '$lib/workbench/panel-mutators';
  import { resolveCellConfig } from '$lib/workbench/panel-queries';
  import {
    DEFAULT_BINS,
    DEFAULT_FORCE_STRENGTH,
    DEFAULT_TOPN,
    NET_COLOR_CHANNELS,
    NET_SIZE_CHANNELS
  } from '$lib/workbench/cell-levers';
  import type { CellChannelBinding, CellChannelPatch, Panel } from '$lib/state/url-internals';
  import type { PresentationDefinition } from '$lib/viewmodes';

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
    onClose: () => void;
  }
  let { panelPath, cellKey, cellLabel, panel, presentation, scalarMetricOptions, onClose }: Props =
    $props();

  const configParams = $derived(presentation.configurableParams ?? []);
  const cfg = $derived(resolveCellConfig(panel, cellKey));
  const override = $derived(panel.cellOverrides?.[cellKey]);

  // Per-lever "is this overridden" — drives the per-row override dot.
  const ovBins = $derived(override?.bins !== undefined);
  const ovTopN = $derived(override?.topN !== undefined);
  const ovForce = $derived(override?.forceStrength !== undefined);
  const ovBand = $derived(override?.showBand !== undefined);
  const ovScales = $derived(override?.scales !== undefined);
  const ovLabels = $derived(override?.displayLanguage !== undefined);
  function ovChannel(k: keyof CellChannelBinding): boolean {
    return override?.channels?.[k] !== undefined;
  }

  // Effective values (override ?? panel default ?? cell default).
  const effBins = $derived(cfg.bins ?? DEFAULT_BINS);
  const effTopN = $derived(cfg.topN ?? DEFAULT_TOPN);
  const effForce = $derived(cfg.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const effShowBand = $derived(cfg.showBand ?? true);
  const effScale = $derived<'shared' | 'free'>(cfg.scales ?? 'shared');
  const effLabels = $derived<'source' | 'viewer'>(cfg.displayLanguage ?? 'source');
  const effChannels = $derived<CellChannelBinding>(cfg.channels ?? {});

  // Live slider read-outs — commit to the override only on `onchange`.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  const displayBins = $derived(liveBins ?? effBins);
  const displayTopN = $derived(liveTopN ?? effTopN);
  const displayForce = $derived(liveForce ?? effForce);

  // The scatter pickers must keep any currently-bound metric visible even if it
  // has slipped out of the scope-available set, so the select reflects reality.
  const axisMetricOptions = $derived.by<string[]>(() => {
    const seen: Record<string, true> = {};
    const out: string[] = [];
    for (const m of scalarMetricOptions) {
      if (!seen[m]) {
        seen[m] = true;
        out.push(m);
      }
    }
    for (const bound of [effChannels.x, effChannels.y, effChannels.size, effChannels.color]) {
      if (bound && !seen[bound]) {
        seen[bound] = true;
        out.push(bound);
      }
    }
    return out;
  });

  // The panel-level value each lever inherits when this cell has no override —
  // setting a cell back to this clears the override (so a cell that equals its
  // siblings is never falsely flagged "custom").
  const panelBins = $derived(panel.bins ?? DEFAULT_BINS);
  const panelTopN = $derived(panel.topN ?? DEFAULT_TOPN);
  const panelForce = $derived(panel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const panelBand = $derived(panel.showBand ?? true);
  const panelScale = $derived<'shared' | 'free'>(panel.scales ?? 'shared');
  const panelLabels = $derived<'source' | 'viewer'>(panel.displayLanguage ?? 'source');

  function setBins(n: number) {
    if (!Number.isFinite(n)) return;
    const v = Math.min(200, Math.max(1, Math.round(n)));
    setCellOverride(panelPath, cellKey, { bins: v === panelBins ? undefined : v });
  }
  function setTopN(n: number) {
    if (!Number.isFinite(n)) return;
    const v = Math.min(500, Math.max(1, Math.round(n)));
    setCellOverride(panelPath, cellKey, { topN: v === panelTopN ? undefined : v });
  }
  function setForce(n: number) {
    if (!Number.isFinite(n)) return;
    const v = Math.min(100, Math.max(0, Math.round(n)));
    setCellOverride(panelPath, cellKey, { forceStrength: v === panelForce ? undefined : v });
  }
  function setBand(next: boolean) {
    setCellOverride(panelPath, cellKey, { showBand: next === panelBand ? undefined : next });
  }
  function setScale(next: 'shared' | 'free') {
    setCellOverride(panelPath, cellKey, { scales: next === panelScale ? undefined : next });
  }
  function setLabels(next: 'source' | 'viewer') {
    setCellOverride(panelPath, cellKey, {
      displayLanguage: next === panelLabels ? undefined : next
    });
  }
  // Empty string = revert THIS channel to the panel default (inherit). Choosing
  // the panel's own binding also clears the override so the cell is not flagged
  // custom for a no-op.
  function setChannel(key: keyof CellChannelBinding, value: string) {
    const chosen = value === '' ? undefined : value;
    const panelVal = panel.channels?.[key];
    const patch: CellChannelPatch = { [key]: chosen === panelVal ? undefined : chosen };
    setCellOverride(panelPath, cellKey, { channels: patch });
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.stopPropagation();
      onClose();
    }
  }
</script>

<div
  class="cell-config-popover"
  role="dialog"
  aria-label="Cell configuration for {cellLabel}"
  onclick={(e) => e.stopPropagation()}
  onkeydown={onKeydown}
  tabindex="-1"
  data-export-exclude="provenance"
>
  <header class="ccp-header">
    <span class="ccp-eyebrow">Cell config</span>
    <code class="ccp-label" title={cellLabel}>{cellLabel}</code>
    <button type="button" class="ccp-close" onclick={onClose} aria-label="Close cell configuration">
      ×
    </button>
  </header>

  {#if cfg.isOverridden}
    <p class="ccp-note">
      This cell is on a <strong>custom</strong> configuration — not directly comparable to its sibling
      cells.
    </p>
  {:else}
    <p class="ccp-note ccp-note-muted">Inheriting the panel default for every lever.</p>
  {/if}

  <div class="ccp-body">
    {#if configParams.includes('bins')}
      <div class="ccp-row" role="group" aria-label="Histogram bins">
        <span class="ccp-rk"
          >Bins {#if ovBins}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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
            aria-label="Cell histogram bin count"
          />
          <output class="ccp-val">{displayBins}</output>
        </div>
      </div>
    {/if}

    {#if configParams.includes('topN')}
      <div class="ccp-row" role="group" aria-label="Top edges">
        <span class="ccp-rk"
          >Top N {#if ovTopN}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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
            aria-label="Cell top co-occurrence edge count"
          />
          <output class="ccp-val">{displayTopN}</output>
        </div>
      </div>
    {/if}

    {#if configParams.includes('forceStrength')}
      <div class="ccp-row" role="group" aria-label="Graph spread">
        <span class="ccp-rk"
          >Spread {#if ovForce}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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
            aria-label="Cell graph spread"
          />
          <output class="ccp-val">{displayForce}</output>
        </div>
      </div>
    {/if}

    {#if configParams.includes('band')}
      <div class="ccp-row" role="group" aria-label="Uncertainty band">
        <span class="ccp-rk"
          >Band {#if ovBand}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
        >
        <button
          type="button"
          role="switch"
          aria-checked={effShowBand}
          class="ccp-btn"
          class:active={effShowBand}
          onclick={() => setBand(!effShowBand)}
        >
          {effShowBand ? '±1σ shown' : '±1σ hidden'}
        </button>
      </div>
    {/if}

    {#if configParams.includes('scales')}
      <div class="ccp-row" role="group" aria-label="Axis scale">
        <span class="ccp-rk"
          >Scale {#if ovScales}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
        >
        <button
          type="button"
          role="switch"
          aria-checked={effScale === 'shared'}
          class="ccp-btn"
          class:active={effScale === 'shared'}
          onclick={() => setScale(effScale === 'shared' ? 'free' : 'shared')}
          title="Shared: this cell sits on the panel's union axis (comparable). Free: this cell scales to its own data."
        >
          {effScale === 'shared' ? 'Shared axis' : 'Free axis'}
        </button>
      </div>
    {/if}

    {#if configParams.includes('networkChannels')}
      <div class="ccp-row" role="group" aria-label="Network visual channels">
        <span class="ccp-rk"
          >Size {#if ovChannel('netSize')}<span class="ccp-dot" title="Overridden">●</span
            >{/if}</span
        >
        <select
          class="ccp-select"
          value={effChannels.netSize ?? 'total_count'}
          onchange={(e) => setChannel('netSize', (e.currentTarget as HTMLSelectElement).value)}
          aria-label="Cell node size channel"
        >
          {#each NET_SIZE_CHANNELS as c (c.id)}
            <option value={c.id}>{c.label}</option>
          {/each}
        </select>
        <span class="ccp-rk"
          >Colour {#if ovChannel('netColor')}<span class="ccp-dot" title="Overridden">●</span
            >{/if}</span
        >
        <select
          class="ccp-select"
          value={effChannels.netColor ?? 'label'}
          onchange={(e) => setChannel('netColor', (e.currentTarget as HTMLSelectElement).value)}
          aria-label="Cell node colour channel"
        >
          {#each NET_COLOR_CHANNELS as c (c.id)}
            <option value={c.id}>{c.label}</option>
          {/each}
        </select>
      </div>
    {/if}

    {#if configParams.includes('displayLanguage')}
      <div class="ccp-row" role="group" aria-label="Display language">
        <span class="ccp-rk"
          >Labels {#if ovLabels}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
        >
        <button
          type="button"
          role="switch"
          aria-checked={effLabels === 'viewer'}
          class="ccp-btn"
          class:active={effLabels === 'viewer'}
          onclick={() => setLabels(effLabels === 'viewer' ? 'source' : 'viewer')}
        >
          {effLabels === 'viewer' ? 'App language' : 'Source form'}
        </button>
      </div>
    {/if}

    {#if configParams.includes('scatterAxes')}
      <div class="ccp-row ccp-row-grid" role="group" aria-label="Scatter axes">
        <label class="ccp-field">
          <span class="ccp-rk"
            >X {#if ovChannel('x')}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
          >
          <select
            class="ccp-select"
            value={effChannels.x ?? ''}
            onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
          >
            <option value="">— inherit —</option>
            {#each axisMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </label>
        <label class="ccp-field">
          <span class="ccp-rk"
            >Y {#if ovChannel('y')}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
          >
          <select
            class="ccp-select"
            value={effChannels.y ?? ''}
            onchange={(e) => setChannel('y', (e.currentTarget as HTMLSelectElement).value)}
          >
            <option value="">— inherit —</option>
            {#each axisMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </label>
        <label class="ccp-field">
          <span class="ccp-rk"
            >Size {#if ovChannel('size')}<span class="ccp-dot" title="Overridden">●</span
              >{/if}</span
          >
          <select
            class="ccp-select"
            value={effChannels.size ?? ''}
            onchange={(e) => setChannel('size', (e.currentTarget as HTMLSelectElement).value)}
          >
            <option value="">— inherit —</option>
            {#each axisMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </label>
        <label class="ccp-field">
          <span class="ccp-rk"
            >Colour {#if ovChannel('color')}<span class="ccp-dot" title="Overridden">●</span
              >{/if}</span
          >
          <select
            class="ccp-select"
            value={effChannels.color ?? ''}
            onchange={(e) => setChannel('color', (e.currentTarget as HTMLSelectElement).value)}
          >
            <option value="">— inherit —</option>
            {#each axisMetricOptions as m (m)}
              <option value={m}>{m}</option>
            {/each}
          </select>
        </label>
      </div>
    {/if}
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
      ↺ Reset to panel default
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

  .ccp-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .ccp-row-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--space-2);
  }
  .ccp-field {
    display: flex;
    flex-direction: column;
    gap: 2px;
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
  }
  .ccp-select:hover,
  .ccp-select:focus-visible {
    border-color: var(--color-accent);
  }

  .ccp-footer {
    display: flex;
    justify-content: flex-end;
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-2);
  }
  .ccp-reset {
    appearance: none;
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
