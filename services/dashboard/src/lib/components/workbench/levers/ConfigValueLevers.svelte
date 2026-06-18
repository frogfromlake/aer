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
  const activeTopN = $derived(boundPanel.topN ?? DEFAULT_TOPN);
  const activeShowBand = $derived(boundPanel.showBand ?? true);
  const activeShowEdges = $derived(boundPanel.showEdges ?? false);
  const activeForceStrength = $derived(boundPanel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  const activeSettle = $derived(boundPanel.settleSeconds ?? 12);

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
        min="5"
        max="120"
        step="1"
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
        aria-label={m.levers_topn_slider_aria()}
      />
      <output class="config-value">{displayTopN}</output>
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
        min="0"
        max="100"
        step="1"
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
        max="60"
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
