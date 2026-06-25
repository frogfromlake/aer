<script lang="ts">
  // ConfigChannelLevers — Phase 141 (extracted from PanelControls' config block).
  //
  // The visual-channel / metric-set per-cell config levers (Phase 125/126/131):
  // network Size·Colour channels (+ their metric pickers), scatter X·Y·Size·
  // Colour, the N-metric set (correlation matrix / parallel coords), cross-tab
  // metric, the Sankey field chain, lead-lag X→Y, faceting, and the panel-level
  // "Reset all cells" affordance. Pools are the scope-available scalar metrics /
  // offerable categorical fields (no-discovery-bias). Sibling ConfigValueLevers
  // owns the slider / toggle levers.
  import type { CellChannelBinding, Panel } from '$lib/state/url-internals';
  import {
    NET_COLOR_CHANNELS,
    NET_SIZE_CHANNELS,
    PROVENANCE_BORDER_MODES
  } from '$lib/workbench/cell-levers';
  import {
    resetAllCellOverrides,
    updatePanel,
    type PanelPath
  } from '$lib/workbench/panel-mutators';
  import { metricLabel, fieldLabel } from '$lib/state/labels.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';
  import MultiSelectControl from './MultiSelectControl.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    configParams: readonly string[];
    scalarMetricOptions: string[];
    /** Offerable categorical fields (shared — computed once in the parent). */
    offerableFields: string[];
    viewUsesMetadataField: boolean;
    viewSupportsFaceting: boolean;
  }

  let {
    panelPath,
    boundPanel,
    configParams,
    scalarMetricOptions,
    offerableFields,
    viewUsesMetadataField,
    viewSupportsFaceting
  }: Props = $props();

  const activeMetric = $derived(boundPanel.metric);
  const activeChannels = $derived<CellChannelBinding>(boundPanel.channels ?? {});
  const activeMetricSet = $derived<readonly string[]>(boundPanel.metricSet ?? []);
  const activeFieldChain = $derived<readonly string[]>(boundPanel.fieldChain ?? []);
  const activeFacetField = $derived<string>(boundPanel.facetField ?? '');
  const activeProvBorder = $derived(boundPanel.provenanceBorder ?? 'none');
  const cellOverrideCount = $derived(Object.keys(boundPanel.cellOverrides ?? {}).length);

  // For a field-driven view the panel's own field rides in `metric`; faceting BY
  // that same field is degenerate, so exclude it from the facet picker.
  const facetFieldOptions = $derived<string[]>(
    viewUsesMetadataField ? offerableFields.filter((f) => f !== activeMetric) : offerableFields
  );

  function toggleMetricSetMember(name: string) {
    updatePanel(panelPath, (p) => {
      const cur = p.metricSet ?? [];
      const next = cur.includes(name) ? cur.filter((mn) => mn !== name) : [...cur, name];
      const out = { ...p };
      if (next.length > 0) out.metricSet = next;
      else delete out.metricSet;
      return out;
    });
  }
  function toggleFieldChainMember(name: string) {
    updatePanel(panelPath, (p) => {
      const cur = p.fieldChain ?? [];
      const next = cur.includes(name) ? cur.filter((mn) => mn !== name) : [...cur, name];
      const out = { ...p };
      if (next.length > 0) out.fieldChain = next;
      else delete out.fieldChain;
      return out;
    });
  }
  // Phase 148g — provenance-border mode (top-level Panel field, like showLabels);
  // 'none' clears it so the default is never persisted.
  function setProvBorder(mode: string) {
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      if (mode === 'source' || mode === 'probe' || mode === 'both') out.provenanceBorder = mode;
      else delete out.provenanceBorder;
      return out;
    });
  }
  function setFacetField(field: string) {
    updatePanel(panelPath, (p) => {
      const out = { ...p };
      if (field) out.facetField = field;
      else delete out.facetField;
      return out;
    });
  }
  // Mutate one visual channel; empty string clears (unbinds) the channel.
  function setChannel(key: keyof CellChannelBinding, value: string) {
    if ((activeChannels[key] ?? '') === value) return;
    updatePanel(panelPath, (p) => {
      const ch: CellChannelBinding = { ...(p.channels ?? {}) };
      if (value === '') delete ch[key];
      else (ch[key] as string) = value;
      // Phase 125 — selecting the network 'metric' channel needs a metric to
      // aggregate; seed one if none is bound yet (size + colour seed
      // independently; colour reuses the size metric when none of its own is set).
      const seedMetric = () =>
        scalarMetricOptions.find((mn) => mn.startsWith('sentiment_score')) ??
        scalarMetricOptions[0];
      if (key === 'netSize' && value === 'metric' && !ch.netMetric) {
        const seed = seedMetric();
        if (seed) ch.netMetric = seed;
      }
      if (key === 'netColor' && value === 'metric' && !ch.netColorMetric && !ch.netMetric) {
        const seed = seedMetric();
        if (seed) ch.netColorMetric = seed;
      }
      const o = { ...p };
      if (Object.keys(ch).length > 0) o.channels = ch;
      else delete o.channels;
      return o;
    });
  }
</script>

{#if configParams.includes('networkChannels')}
  <div class="ctrl-row config-row" role="group" aria-label={m.levers_network_channels_aria()}>
    <span class="ctrl-eyebrow">{m.levers_network_size_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.netSize ?? 'total_count'}
      onchange={(e) => setChannel('netSize', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_network_size_select_aria()}
    >
      {#each NET_SIZE_CHANNELS as c (c.id)}
        <option value={c.id}>{c.label()}</option>
      {/each}
    </select>
    <span class="ctrl-eyebrow">{m.levers_network_colour_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.netColor ?? 'community'}
      onchange={(e) => setChannel('netColor', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_network_colour_select_aria()}
    >
      {#each NET_COLOR_CHANNELS as c (c.id)}
        <option value={c.id}>{c.label()}</option>
      {/each}
    </select>
  </div>
  {#if activeChannels.netSize === 'metric'}
    <div class="ctrl-row config-row" role="group" aria-label={m.levers_network_size_metric_aria()}>
      <span class="ctrl-eyebrow">{m.levers_network_size_metric_eyebrow()}</span>
      <select
        class="config-select"
        value={activeChannels.netMetric ?? ''}
        onchange={(e) => setChannel('netMetric', (e.currentTarget as HTMLSelectElement).value)}
        onclick={(e) => e.stopPropagation()}
        aria-label={m.levers_network_size_metric_select_aria()}
      >
        <option value="" disabled>{m.levers_pick_metric_placeholder()}</option>
        {#each scalarMetricOptions as mn (mn)}
          <option value={mn}>{metricLabel(mn)}</option>
        {/each}
      </select>
    </div>
  {/if}
  {#if activeChannels.netColor === 'metric'}
    <div
      class="ctrl-row config-row"
      role="group"
      aria-label={m.levers_network_colour_metric_aria()}
    >
      <span class="ctrl-eyebrow">{m.levers_network_colour_metric_eyebrow()}</span>
      <select
        class="config-select"
        value={activeChannels.netColorMetric ?? activeChannels.netMetric ?? ''}
        onchange={(e) => setChannel('netColorMetric', (e.currentTarget as HTMLSelectElement).value)}
        onclick={(e) => e.stopPropagation()}
        aria-label={m.levers_network_colour_metric_select_aria()}
      >
        <option value="" disabled>{m.levers_pick_metric_placeholder()}</option>
        {#each scalarMetricOptions as mn (mn)}
          <option value={mn}>{metricLabel(mn)}</option>
        {/each}
      </select>
    </div>
  {/if}
{/if}

<!-- Phase 148g — provenance border: per-node ring(s) for source / probe, drawn on
     TOP of the metric/community fill so the reader sees who published a node and
     its size + colour at once. Vivid border palette stays clear of the metric ramp. -->
{#if configParams.includes('provenanceBorder')}
  <div class="ctrl-row config-row" role="group" aria-label={m.levers_provborder_aria()}>
    <span class="ctrl-eyebrow">{m.levers_provborder_eyebrow()}</span>
    <select
      class="config-select"
      value={activeProvBorder}
      onchange={(e) => setProvBorder((e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_provborder_select_aria()}
    >
      {#each PROVENANCE_BORDER_MODES as mode (mode.id)}
        <option value={mode.id}>{mode.label()}</option>
      {/each}
    </select>
  </div>
{/if}

{#if configParams.includes('scatterAxes')}
  <div class="ctrl-row config-row" role="group" aria-label={m.levers_scatter_position_aria()}>
    <span class="ctrl-eyebrow">{m.levers_scatter_xy_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.x ?? ''}
      onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_scatter_x_select_aria()}
    >
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
    <select
      class="config-select"
      value={activeChannels.y ?? ''}
      onchange={(e) => setChannel('y', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_scatter_y_select_aria()}
    >
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
  </div>
  <div class="ctrl-row config-row" role="group" aria-label={m.levers_scatter_sizecolour_aria()}>
    <span class="ctrl-eyebrow">{m.levers_scatter_size_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.size ?? ''}
      onchange={(e) => setChannel('size', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_scatter_size_select_aria()}
    >
      <option value="">{m.levers_none_placeholder()}</option>
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
    <span class="ctrl-eyebrow">{m.levers_scatter_colour_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.color ?? ''}
      onchange={(e) => setChannel('color', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_scatter_colour_select_aria()}
    >
      <option value="">{m.levers_none_placeholder()}</option>
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
  </div>
{/if}

<!-- Phase 125 — N-metric set picker (correlation matrix, parallel coords).
     Phase 148g — collapsed multi-select dropdown (was an always-expanded chip
     row) so it reads like the sibling single-selects and saves vertical space. -->
{#if configParams.includes('metricSet')}
  <LeverRow
    eyebrow={m.levers_metric_set_eyebrow()}
    role="group"
    ariaLabel={m.levers_metric_set_aria()}
    rowClass="config-row"
  >
    <MultiSelectControl
      options={scalarMetricOptions}
      selected={activeMetricSet}
      label={metricLabel}
      onToggle={toggleMetricSetMember}
      ariaLabel={m.levers_metric_set_aria()}
    />
  </LeverRow>
{/if}

<!-- Phase 125 — cross-tab numeric metric (bound to channels.x). -->
{#if configParams.includes('crossMetric')}
  <LeverRow
    eyebrow={m.levers_crosstab_eyebrow()}
    role="group"
    ariaLabel={m.levers_crosstab_aria()}
    rowClass="config-row"
  >
    <select
      class="config-select"
      value={activeChannels.x ?? ''}
      onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_crosstab_select_aria()}
    >
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
  </LeverRow>
{/if}

<!-- Phase 125a — Sankey field chain (ordered multi-select of categorical fields). -->
{#if configParams.includes('sankeyFields')}
  <LeverRow
    eyebrow={m.levers_sankey_eyebrow()}
    role="group"
    ariaLabel={m.levers_sankey_aria()}
    rowClass="config-row"
  >
    <div class="metric-set-options" onclick={(e) => e.stopPropagation()} role="presentation">
      {#each offerableFields as f (f)}
        <label class="metric-set-chip" class:active={activeFieldChain.includes(f)}>
          <input
            type="checkbox"
            checked={activeFieldChain.includes(f)}
            onchange={() => toggleFieldChainMember(f)}
          />
          <code>{fieldLabel(f)}</code>
        </label>
      {/each}
      {#if offerableFields.length === 0}
        <span class="field-empty">{m.levers_sankey_empty()}</span>
      {/if}
    </div>
  </LeverRow>
{/if}

<!-- Phase 125 — lead-lag x/y metric pickers (x leads y). -->
{#if configParams.includes('leadLagAxes')}
  <div class="ctrl-row config-row" role="group" aria-label={m.levers_leadlag_aria()}>
    <span class="ctrl-eyebrow">{m.levers_leadlag_xy_eyebrow()}</span>
    <select
      class="config-select"
      value={activeChannels.x ?? ''}
      onchange={(e) => setChannel('x', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_leadlag_x_select_aria()}
    >
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
    <select
      class="config-select"
      value={activeChannels.y ?? ''}
      onchange={(e) => setChannel('y', (e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_leadlag_y_select_aria()}
    >
      {#each scalarMetricOptions as mn (mn)}
        <option value={mn}>{metricLabel(mn)}</option>
      {/each}
    </select>
  </div>
{/if}

<!-- Phase 125a — faceting / small-multiples (per-article presentations only). -->
{#if viewSupportsFaceting}
  <LeverRow
    eyebrow={m.levers_facet_eyebrow()}
    role="group"
    ariaLabel={m.levers_facet_aria()}
    rowClass="config-row"
  >
    <select
      class="config-select"
      value={activeFacetField}
      onchange={(e) => setFacetField((e.currentTarget as HTMLSelectElement).value)}
      onclick={(e) => e.stopPropagation()}
      aria-label={m.levers_facet_select_aria()}
    >
      <option value="">{m.levers_facet_none()}</option>
      {#each facetFieldOptions as f (f)}
        <option value={f}>{fieldLabel(f)}</option>
      {/each}
    </select>
    {#if facetFieldOptions.length === 0}
      <span class="field-empty">{m.levers_facet_empty()}</span>
    {/if}
  </LeverRow>
{/if}

<!-- Phase 126 — panel-level "Reset all cells" (clears every per-cell override). -->
{#if cellOverrideCount > 0}
  <LeverRow
    eyebrow={m.levers_cells_eyebrow()}
    role="group"
    ariaLabel={m.levers_cells_aria()}
    rowClass="config-row"
  >
    <LeverButton
      onclick={() => resetAllCellOverrides(panelPath)}
      title={m.levers_cells_reset_title({ count: cellOverrideCount })}
    >
      {cellOverrideCount === 1
        ? m.levers_cells_reset_one({ count: cellOverrideCount })
        : m.levers_cells_reset_other({ count: cellOverrideCount })}
    </LeverButton>
  </LeverRow>
{/if}

<style>
  /* Phase 125 — metric-set multi-select chips. */
  .metric-set-options {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    min-width: 0;
  }
  .metric-set-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .metric-set-chip.active {
    border-color: var(--color-accent);
    color: var(--color-accent);
  }
  .metric-set-chip code {
    font-family: var(--font-mono);
  }
</style>
