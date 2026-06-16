<script lang="ts">
  // CellConfigChannelLevers — Phase 141 (extracted from CellConfigPopover).
  //
  // The visual-channel / label half of the per-cell override popover, in render
  // order: the co-occurrence node Size/Colour channel selects, the display-
  // language switch, and the scatter X/Y/Size/Colour axis grid. Sibling
  // CellConfigValueLevers owns the scalar sliders + the Band/Scale switches; the
  // split point (after `scales`, before `networkChannels`) preserves the
  // popover's lever ORDER for every presentation. Each lever writes to
  // `Panel.cellOverrides[cellKey]` via `setCellOverride`, clearing the override
  // when the chosen binding equals the panel default.
  import type { PanelPath } from '$lib/workbench/panel-mutators';
  import { setCellOverride } from '$lib/workbench/panel-mutators';
  import { resolveCellConfig } from '$lib/workbench/panel-queries';
  import { NET_COLOR_CHANNELS, NET_SIZE_CHANNELS } from '$lib/workbench/cell-levers';
  import { buildAxisMetricOptions } from '$lib/workbench/cell-config-popover-internals';
  import type { CellChannelBinding, CellChannelPatch, Panel } from '$lib/state/url-internals';

  interface Props {
    panelPath: PanelPath;
    cellKey: string;
    panel: Panel;
    configParams: readonly string[];
    /** Scalar metric names for the scatter axis pickers (scope-available). */
    scalarMetricOptions: readonly string[];
  }
  let { panelPath, cellKey, panel, configParams, scalarMetricOptions }: Props = $props();

  const cfg = $derived(resolveCellConfig(panel, cellKey));
  const override = $derived(panel.cellOverrides?.[cellKey]);

  const ovLabels = $derived(override?.displayLanguage !== undefined);
  function ovChannel(k: keyof CellChannelBinding): boolean {
    return override?.channels?.[k] !== undefined;
  }

  const effLabels = $derived<'source' | 'viewer'>(cfg.displayLanguage ?? 'source');
  const effChannels = $derived<CellChannelBinding>(cfg.channels ?? {});
  const panelLabels = $derived<'source' | 'viewer'>(panel.displayLanguage ?? 'source');

  // Keep any currently-bound metric visible even if it has slipped out of the
  // scope-available set, so the select reflects reality.
  const axisMetricOptions = $derived(buildAxisMetricOptions(scalarMetricOptions, effChannels));

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
</script>

{#if configParams.includes('networkChannels')}
  <div class="ccp-row" role="group" aria-label="Network visual channels">
    <span class="ccp-rk"
      >Size {#if ovChannel('netSize')}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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
        >Size {#if ovChannel('size')}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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
        >Colour {#if ovChannel('color')}<span class="ccp-dot" title="Overridden">●</span>{/if}</span
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

<style>
  /* Per-cell config rows (compact popover variant). Duplicated with
     CellConfigValueLevers — Svelte scopes `<style>` per-component; the small
     duplication is cheaper than a forced shared primitive (PanelHost/L5
     precedent). */
  .ccp-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .ccp-row-grid {
    display: grid;
    /* Phase 126 (fix) — two columns when the popover is wide enough, collapsing
       to one when the cell (and thus the popover) is narrow, so the axis pickers
       stay readable instead of overflowing or cramping. */
    grid-template-columns: repeat(auto-fit, minmax(8.5rem, 1fr));
    gap: var(--space-2);
  }
  .ccp-field {
    display: flex;
    flex-direction: column;
    gap: 2px;
    /* Phase 126 (fix) — allow the grid/flex item to shrink below the select's
       intrinsic (longest-option) width so a long metric name like
       `sentiment_score_bert_multilingual` can't push the dropdown past the
       popover edge. */
    min-width: 0;
  }
  .ccp-field > .ccp-select {
    width: 100%;
  }
  /* Inline (network) selects share the row width and shrink to fit. */
  .ccp-row:not(.ccp-row-grid) > .ccp-select {
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
