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
  import {
    DEFAULT_BINS,
    DEFAULT_FORCE_STRENGTH,
    BINS_MIN,
    BINS_MAX,
    BINS_STEP,
    TOPN_MIN,
    TOPN_STEP,
    MAXNODES_MIN,
    MAXNODES_MAX,
    MAXNODES_STEP,
    DEFAULT_MAXNODES,
    FORCE_MIN,
    FORCE_MAX,
    FORCE_STEP
  } from '$lib/workbench/cell-levers';
  import { computeTopNMax } from '$lib/workbench/panel-controls-derive';
  import { effectiveEdgeCap, autoSettleSeconds } from '$lib/presentations/cooccurrence-query';
  import { updatePanel, type PanelPath } from '$lib/workbench/panel-mutators';
  import { m } from '$lib/paraglide/messages.js';
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
  const activeMaxNodes = $derived(boundPanel.maxNodes ?? DEFAULT_MAXNODES);
  // Phase 148g — the edge cap is a coupled DENSITY: unset, it auto-tracks the node
  // count (effectiveEdgeCap), so "more nodes → more edges" holds by default.
  const activeTopN = $derived(boundPanel.topN ?? effectiveEdgeCap(activeMaxNodes));
  const activeShowBand = $derived(boundPanel.showBand ?? true);
  const activeShowEdges = $derived(boundPanel.showEdges ?? false);
  const activeShowLabels = $derived(boundPanel.showLabels ?? true);
  const activeForceStrength = $derived(boundPanel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  // Phase 148g — when the user hasn't set it, the displayed default is the same
  // node-count-scaled value the renderer uses (truthful, not a static 12 s).
  const activeSettle = $derived(boundPanel.settleSeconds ?? autoSettleSeconds(activeMaxNodes));

  // Live slider read-outs — null = not mid-drag, fall back to the committed value.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveMaxNodes = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  let liveSettle = $state<number | null>(null);
  const displayBins = $derived(liveBins ?? activeBins);
  const displayTopN = $derived(liveTopN ?? activeTopN);
  const displayMaxNodes = $derived(liveMaxNodes ?? activeMaxNodes);
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
  function setMaxNodes(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(MAXNODES_MAX, Math.max(MAXNODES_MIN, Math.round(n)));
    if (clamped === activeMaxNodes) return;
    // Phase 148g — the node lever is the SOLE co-occurrence control: the renderer
    // derives the edge budget from the node count (effectiveEdgeCap) and the BFF
    // keeps each node's strongest edges (per-node top-K), so connection density
    // tracks breadth automatically — no separate density slider to balance.
    updatePanel(panelPath, (p) => ({ ...p, maxNodes: clamped }));
  }
  function setForceStrength(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(100, Math.max(0, Math.round(n)));
    if (clamped === activeForceStrength) return;
    updatePanel(panelPath, (p) => ({ ...p, forceStrength: clamped }));
  }
  function setSettle(n: number) {
    if (!Number.isFinite(n)) return;
    const clamped = Math.min(120, Math.max(3, Math.round(n)));
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
  function setShowLabels(next: boolean) {
    if (next === activeShowLabels) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      // Default is ON, so store only the OFF state (URL stays clean by default).
      if (next) delete o.showLabels;
      else o.showLabels = false;
      return o;
    });
  }
</script>

{#if configParams.includes('bins')}
  <LeverRow
    eyebrow={m.levers_bins_eyebrow()}
    role="group"
    ariaLabel={m.levers_bins_aria()}
    rowClass="config-row"
  >
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min={BINS_MIN}
        max={BINS_MAX}
        step={BINS_STEP}
        value={activeBins}
        oninput={(e) => (liveBins = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setBins(Number((e.currentTarget as HTMLInputElement).value));
          liveBins = null;
        }}
        aria-label={m.levers_bins_slider_aria()}
      />
      <output class="config-value">{displayBins}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('topN')}
  <LeverRow
    eyebrow={m.levers_topn_eyebrow()}
    role="group"
    ariaLabel={m.levers_topn_aria()}
    rowClass="config-row"
  >
    <div
      class="config-inline"
      title={m.levers_topn_title()}
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <input
        type="range"
        min={TOPN_MIN}
        max={topNMax}
        step={TOPN_STEP}
        value={activeTopN}
        oninput={(e) => (liveTopN = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setTopN(Number((e.currentTarget as HTMLInputElement).value));
          liveTopN = null;
        }}
        aria-label={m.levers_topn_slider_aria()}
      />
      <output class="config-value">{displayTopN}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('maxNodes')}
  <LeverRow
    eyebrow={m.levers_maxnodes_eyebrow()}
    role="group"
    ariaLabel={m.levers_maxnodes_aria()}
    rowClass="config-row"
  >
    <div
      class="config-inline"
      title={m.levers_maxnodes_title()}
      onclick={(e) => e.stopPropagation()}
      role="presentation"
    >
      <input
        type="range"
        min={MAXNODES_MIN}
        max={MAXNODES_MAX}
        step={MAXNODES_STEP}
        value={activeMaxNodes}
        oninput={(e) => (liveMaxNodes = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setMaxNodes(Number((e.currentTarget as HTMLInputElement).value));
          liveMaxNodes = null;
        }}
        aria-label={m.levers_maxnodes_slider_aria()}
      />
      <output class="config-value">{displayMaxNodes}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('forceStrength')}
  <LeverRow
    eyebrow={m.levers_spread_eyebrow()}
    role="group"
    ariaLabel={m.levers_spread_aria()}
    rowClass="config-row"
  >
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min={FORCE_MIN}
        max={FORCE_MAX}
        step={FORCE_STEP}
        value={activeForceStrength}
        oninput={(e) => (liveForce = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setForceStrength(Number((e.currentTarget as HTMLInputElement).value));
          liveForce = null;
        }}
        title={m.levers_spread_title()}
        aria-label={m.levers_spread_slider_aria()}
      />
      <output class="config-value">{displayForce}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('settleTime')}
  <LeverRow
    eyebrow={m.levers_settle_eyebrow()}
    role="group"
    ariaLabel={m.levers_settle_aria()}
    rowClass="config-row"
  >
    <div class="config-inline" onclick={(e) => e.stopPropagation()} role="presentation">
      <input
        type="range"
        min="3"
        max="120"
        step="1"
        value={activeSettle}
        oninput={(e) => (liveSettle = Number((e.currentTarget as HTMLInputElement).value))}
        onchange={(e) => {
          setSettle(Number((e.currentTarget as HTMLInputElement).value));
          liveSettle = null;
        }}
        title={m.levers_settle_title()}
        aria-label={m.levers_settle_slider_aria()}
      />
      <output class="config-value">{m.levers_settle_value({ seconds: displaySettle })}</output>
    </div>
  </LeverRow>
{/if}

{#if configParams.includes('showLabels')}
  <LeverRow
    eyebrow={m.levers_labels_toggle_eyebrow()}
    role="group"
    ariaLabel={m.levers_labels_toggle_aria()}
    rowClass="config-row"
  >
    <LeverButton
      role="switch"
      active={activeShowLabels}
      onclick={() => setShowLabels(!activeShowLabels)}
      title={m.levers_labels_toggle_title()}
    >
      {activeShowLabels ? m.levers_labels_toggle_shown() : m.levers_labels_toggle_hidden()}
    </LeverButton>
  </LeverRow>
{/if}

{#if configParams.includes('showEdges')}
  <LeverRow
    eyebrow={m.levers_connections_eyebrow()}
    role="group"
    ariaLabel={m.levers_connections_aria()}
    rowClass="config-row"
  >
    <LeverButton
      role="switch"
      active={activeShowEdges}
      onclick={() => setShowEdges(!activeShowEdges)}
      title={m.levers_connections_title()}
    >
      {activeShowEdges ? m.levers_connections_shown() : m.levers_connections_hidden()}
    </LeverButton>
  </LeverRow>
{/if}

{#if configParams.includes('band')}
  <LeverRow
    eyebrow={m.levers_band_eyebrow()}
    role="group"
    ariaLabel={m.levers_band_aria()}
    rowClass="config-row"
  >
    <LeverButton
      role="switch"
      active={activeShowBand}
      onclick={() => setShowBand(!activeShowBand)}
      title={m.levers_band_title()}
    >
      {activeShowBand ? m.levers_band_shown() : m.levers_band_hidden()}
    </LeverButton>
  </LeverRow>
{/if}

<!-- Phase 151 — the Scale (shared/free axis) toggle moved onto the Composition
     row (CompositionControls) to save vertical space. -->
