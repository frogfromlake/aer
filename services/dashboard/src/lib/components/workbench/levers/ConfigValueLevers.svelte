<script lang="ts">
  // ConfigValueLevers — Phase 141 (extracted from PanelControls' config block).
  //
  // The scalar / toggle per-cell config levers a presentation may declare via
  // `configurableParams`: Bins, Top N, Spread (force), Settle, Connections,
  // Band, Scale (+ custom-axis note), Labels (display language). The slider
  // levers commit to Panel state on pointer release (onchange), updating a live
  // read-out on every oninput tick so a drag doesn't fire ~100 URL writes.
  // Sibling ConfigChannelLevers owns the select / multi-select channel levers.
  import type { Panel } from '$lib/state/url-internals';
  import { DEFAULT_BINS, DEFAULT_FORCE_STRENGTH, DEFAULT_TOPN } from '$lib/workbench/cell-levers';
  import { computeTopNMax } from '$lib/workbench/panel-controls-derive';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import LeverRow from './LeverRow.svelte';
  import LeverButton from './LeverButton.svelte';

  interface Props {
    panelPath: PanelPath;
    boundPanel: Panel;
    configParams: readonly string[];
    viewUsesMetadataField: boolean;
  }

  let { panelPath, boundPanel, configParams, viewUsesMetadataField }: Props = $props();

  const isCooccurrenceView = $derived((boundPanel.view ?? '') === 'cooccurrence_network');
  const topNMax = $derived(computeTopNMax({ isCooccurrenceView, viewUsesMetadataField }));

  const activeBins = $derived(boundPanel.bins ?? DEFAULT_BINS);
  const activeTopN = $derived(boundPanel.topN ?? DEFAULT_TOPN);
  const activeShowBand = $derived(boundPanel.showBand ?? true);
  const activeShowEdges = $derived(boundPanel.showEdges ?? false);
  const activeForceStrength = $derived(boundPanel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const activeSettle = $derived(boundPanel.settleSeconds ?? 12);
  const activeScaleMode = $derived<'shared' | 'free'>(boundPanel.scales ?? 'shared');
  // A cell with its own X/Y axis always reads free regardless of this panel
  // Scale toggle — disclose so the toggle never reads as a promise it can't keep.
  const hasAxisOverride = $derived(
    Object.values(boundPanel.cellOverrides ?? {}).some(
      (ov) => ov.channels?.x !== undefined || ov.channels?.y !== undefined
    )
  );

  // Live slider read-outs — null = not mid-drag, fall back to the committed value.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  let liveSettle = $state<number | null>(null);
  const displayBins = $derived(liveBins ?? activeBins);
  const displayTopN = $derived(liveTopN ?? activeTopN);
  const displayForce = $derived(liveForce ?? activeForceStrength);
  const displaySettle = $derived(liveSettle ?? activeSettle);

  function setBins(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(200, Math.max(1, Math.round(n)));
    if (clamped === activeBins) return;
    updatePanel(panelPath, (p) => ({ ...p, bins: clamped }));
  }
  function setTopN(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(topNMax, Math.max(1, Math.round(n)));
    if (clamped === activeTopN) return;
    updatePanel(panelPath, (p) => ({ ...p, topN: clamped }));
  }
  function setForceStrength(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(100, Math.max(0, Math.round(n)));
    if (clamped === activeForceStrength) return;
    updatePanel(panelPath, (p) => ({ ...p, forceStrength: clamped }));
  }
  function setSettle(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(60, Math.max(3, Math.round(n)));
    if (clamped === activeSettle) return;
    updatePanel(panelPath, (p) => ({ ...p, settleSeconds: clamped }));
  }
  function setShowBand(next: boolean) {
    if (next === activeShowBand) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next) delete o.showBand;
      else o.showBand = false;
      return o;
    });
  }
  function setShowEdges(next: boolean) {
    if (next === activeShowEdges) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next) o.showEdges = true;
      else delete o.showEdges;
      return o;
    });
  }
  function setScaleMode(next: 'shared' | 'free') {
    if (next === activeScaleMode) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next === 'shared') delete o.scales;
      else o.scales = 'free';
      return o;
    });
  }
</script>

{#if configParams.includes('bins')}
  <LeverRow eyebrow="Bins" role="group" ariaLabel="Histogram bins" rowClass="config-row">
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min="5"
        max="120"
        step="1"
        value={activeBins}
        oninput={(e) => (liveBins = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setBins(Number((e.currentTarget as HTMLInputElement).value));
          liveBins = null;
        }}
        aria-label="Histogram bin count slider"
      />
      <output class="config-value">{displayBins}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('topN')}
  <LeverRow eyebrow="Top N" role="group" ariaLabel="Top edges" rowClass="config-row">
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min="5"
        max={topNMax}
        step="5"
        value={activeTopN}
        oninput={(e) => (liveTopN = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setTopN(Number((e.currentTarget as HTMLInputElement).value));
          liveTopN = null;
        }}
        aria-label="Top N slider"
      />
      <output class="config-value">{displayTopN}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('forceStrength')}
  <LeverRow eyebrow="Spread" role="group" ariaLabel="Graph spread" rowClass="config-row">
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min="0"
        max="100"
        step="1"
        value={activeForceStrength}
        oninput={(e) => (liveForce = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setForceStrength(Number((e.currentTarget as HTMLInputElement).value));
          liveForce = null;
        }}
        title="How strongly nodes repel each other — higher spreads a crowded graph apart"
        aria-label="Graph spread (node repulsion) slider"
      />
      <output class="config-value">{displayForce}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('settleTime')}
  <LeverRow eyebrow="Settle" role="group" ariaLabel="Layout settle time" rowClass="config-row">
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min="3"
        max="60"
        step="1"
        value={activeSettle}
        oninput={(e) => (liveSettle = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setSettle(Number((e.currentTarget as HTMLInputElement).value));
          liveSettle = null;
        }}
        title="Seconds the large-scale layout runs before it freezes. Raise it to give a big map more time to relax into clusters."
        aria-label="Layout settle time in seconds"
      />
      <output class="config-value">{displaySettle}s</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('showEdges')}
  <LeverRow eyebrow="Connections" role="group" ariaLabel="Connection lines" rowClass="config-row">
    <LeverButton
      role="switch"
      active={activeShowEdges}
      onclick={() => setShowEdges(!activeShowEdges)}
      title="Show or hide the edge lines between nodes. A nodes-only view is clearer for a dense map; clustering still shows the relationships."
    >
      {activeShowEdges ? 'lines shown' : 'lines hidden'}
    </LeverButton>
  </LeverRow>
{/if}

{#if configParams.includes('band')}
  <LeverRow eyebrow="Band" role="group" ariaLabel="Uncertainty band" rowClass="config-row">
    <LeverButton
      role="switch"
      active={activeShowBand}
      onclick={() => setShowBand(!activeShowBand)}
      title="Toggle the ±1σ uncertainty band around each series"
    >
      {activeShowBand ? '±1σ shown' : '±1σ hidden'}
    </LeverButton>
  </LeverRow>
{/if}

{#if configParams.includes('scales')}
  <LeverRow eyebrow="Scale" role="group" ariaLabel="Axis scale" rowClass="config-row">
    <LeverButton
      role="switch"
      active={activeScaleMode === 'shared'}
      onclick={() => setScaleMode(activeScaleMode === 'shared' ? 'free' : 'shared')}
      title="Shared: every cell in this panel uses one axis domain, so identical values plot at identical positions (directly comparable). Free: each cell scales to its own data."
    >
      {activeScaleMode === 'shared' ? 'Shared axis' : 'Free axis'}
    </LeverButton>
  </LeverRow>
  {#if hasAxisOverride}
    <p class="scale-note">ⓘ Cells with a custom X/Y axis always read free — independent of this.</p>
  {/if}
{/if}

<style>
  /* Phase 126 — clarifying note under the panel Scale toggle when a cell has a
     custom X/Y axis (then the panel default doesn't apply to that cell). */
  .scale-note {
    margin: 0 0 0 calc(3.5rem + var(--space-2));
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }
</style>
