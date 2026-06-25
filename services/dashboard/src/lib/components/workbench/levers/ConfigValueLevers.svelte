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
  import { pickViewerLabelLanguage } from '$lib/presentations/viewer-language';
  import { locale } from '$lib/state/locale.svelte';
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
  const activeLabelPct = $derived(boundPanel.labelTopPercent ?? 100);
  const activeLabelRank = $derived(boundPanel.labelRankBy ?? 'size');
  const activeForceStrength = $derived(boundPanel.forceStrength ?? DEFAULT_FORCE_STRENGTH);
  // Phase 148g — when the user hasn't set it, the displayed default is the same
  // node-count-scaled value the renderer uses (truthful, not a static 12 s).
  const activeSettle = $derived(boundPanel.settleSeconds ?? autoSettleSeconds(activeMaxNodes));
  // Phase 148e — display-language (cross-lingual relabel) toggle. Moved here from
  // ConfigChannelLevers so it sits in the same compact row as the label toggle +
  // density slider (the header already claims "Labels (display language)" as this
  // component's remit). 'viewer' relabels QID-linked nodes to the app-language
  // Wikidata label; the toggle text names the resolved language.
  const activeDisplayLanguage = $derived<'source' | 'viewer'>(
    boundPanel.displayLanguage ?? 'source'
  );
  const viewerLanguage = $derived(pickViewerLabelLanguage(locale()));

  // Live slider read-outs — null = not mid-drag, fall back to the committed value.
  let liveBins = $state<number | null>(null);
  let liveTopN = $state<number | null>(null);
  let liveMaxNodes = $state<number | null>(null);
  let liveForce = $state<number | null>(null);
  let liveSettle = $state<number | null>(null);
  let liveLabelPct = $state<number | null>(null);
  const displayLabelPct = $derived(liveLabelPct ?? activeLabelPct);
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
  function setLabelPct(n: number) {
    if (!Number.isFinite(n)) return;
    // Phase 148e — 1% granularity (was 5%): round to the nearest whole percent.
    const clamped = Math.min(100, Math.max(5, Math.round(n)));
    if (clamped === activeLabelPct) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      // 100% = all (the default) → drop the key so the URL stays clean.
      if (clamped >= 100) delete o.labelTopPercent;
      else o.labelTopPercent = clamped;
      return o;
    });
  }
  function setLabelRank(next: 'size' | 'colour') {
    if (next === activeLabelRank) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      if (next === 'size') delete o.labelRankBy;
      else o.labelRankBy = next;
      return o;
    });
  }
  function setDisplayLanguage(next: 'source' | 'viewer') {
    if (next === activeDisplayLanguage) return;
    updatePanel(panelPath, (p) => {
      const o = { ...p };
      // Default is 'source', so store only the 'viewer' opt-in (URL stays clean).
      if (next === 'viewer') o.displayLanguage = 'viewer';
      else delete o.displayLanguage;
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

<!-- Phase 148e — co-occurrence compact layout. The three layout sliders
     (Entities · Spread · Settle) share one 3-column row, and the label controls
     (labels toggle · density · display-language) share one row, so the strip
     stays short instead of stacking six full-width rows. All of these params are
     co-occurrence-exclusive, so this layout never reshapes another presentation;
     each control keeps its prior aria-label / title verbatim. -->
{#if configParams.includes('maxNodes') || configParams.includes('forceStrength') || configParams.includes('settleTime')}
  <div class="ctrl-row config-row cooc-sliders">
    {#if configParams.includes('maxNodes')}
      <div class="cooc-mini" role="group" aria-label={m.levers_maxnodes_aria()}>
        <span class="ctrl-eyebrow">{m.levers_maxnodes_eyebrow()}</span>
        <span
          class="cooc-mini-control"
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
          <!-- Phase 148e — editable read-out. The 50..10000 range makes the slider
               coarse to drag (≈15 entities per pixel), so a number field gives the
               exact 1-entity control the slider physically cannot. Both commit via
               setMaxNodes; the slider tracks the typed value live. -->
          <input
            type="number"
            class="cooc-num"
            min={MAXNODES_MIN}
            max={MAXNODES_MAX}
            step="1"
            value={displayMaxNodes}
            oninput={(e) => (liveMaxNodes = Number((e.currentTarget as HTMLInputElement).value))}
            onchange={(e) => {
              setMaxNodes(Number((e.currentTarget as HTMLInputElement).value));
              liveMaxNodes = null;
            }}
            aria-label={m.levers_maxnodes_slider_aria()}
          />
        </span>
      </div>
    {/if}
    {#if configParams.includes('forceStrength')}
      <div class="cooc-mini" role="group" aria-label={m.levers_spread_aria()}>
        <span class="ctrl-eyebrow">{m.levers_spread_eyebrow()}</span>
        <span
          class="cooc-mini-control"
          title={m.levers_spread_title()}
          onclick={(e) => e.stopPropagation()}
          role="presentation"
        >
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
            aria-label={m.levers_spread_slider_aria()}
          />
          <output class="config-value">{displayForce}</output>
        </span>
      </div>
    {/if}
    {#if configParams.includes('settleTime')}
      <div class="cooc-mini" role="group" aria-label={m.levers_settle_aria()}>
        <span class="ctrl-eyebrow">{m.levers_settle_eyebrow()}</span>
        <span
          class="cooc-mini-control"
          title={m.levers_settle_title()}
          onclick={(e) => e.stopPropagation()}
          role="presentation"
        >
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
            aria-label={m.levers_settle_slider_aria()}
          />
          <output class="config-value">{m.levers_settle_value({ seconds: displaySettle })}</output>
        </span>
      </div>
    {/if}
  </div>
{/if}

{#if configParams.includes('showLabels') || configParams.includes('labelFilter') || configParams.includes('displayLanguage')}
  <div class="ctrl-row config-row cooc-labels">
    {#if configParams.includes('showLabels')}
      <span class="cooc-lg">
        <span class="ctrl-eyebrow">{m.levers_labels_toggle_eyebrow()}</span>
        <LeverButton
          role="switch"
          active={activeShowLabels}
          onclick={() => setShowLabels(!activeShowLabels)}
          title={m.levers_labels_toggle_title()}
        >
          {activeShowLabels ? m.levers_labels_toggle_shown() : m.levers_labels_toggle_hidden()}
        </LeverButton>
      </span>
    {/if}
    <!-- Ranking sits BETWEEN the labels toggle and the density slider: it governs
         HOW the top-% nodes are chosen, so it reads "label by size/colour, top N%".
         Only meaningful while a density filter is active (< 100%). -->
    {#if configParams.includes('labelFilter') && activeLabelPct < 100}
      <span class="cooc-lg">
        <span class="ctrl-eyebrow">{m.levers_labelrank_eyebrow()}</span>
        <LeverButton
          role="switch"
          active={activeLabelRank === 'colour'}
          onclick={() => setLabelRank(activeLabelRank === 'size' ? 'colour' : 'size')}
          title={m.levers_labelrank_title()}
        >
          {activeLabelRank === 'colour' ? m.levers_labelrank_colour() : m.levers_labelrank_size()}
        </LeverButton>
      </span>
    {/if}
    {#if configParams.includes('labelFilter')}
      <span class="cooc-lg">
        <span class="ctrl-eyebrow">{m.levers_labelpct_eyebrow()}</span>
        <span
          class="cooc-pct"
          title={m.levers_labelpct_title()}
          onclick={(e) => e.stopPropagation()}
          role="presentation"
        >
          <input
            type="range"
            min="5"
            max="100"
            step="1"
            value={activeLabelPct}
            oninput={(e) => (liveLabelPct = Number((e.currentTarget as HTMLInputElement).value))}
            onchange={(e) => {
              setLabelPct(Number((e.currentTarget as HTMLInputElement).value));
              liveLabelPct = null;
            }}
            aria-label={m.levers_labelpct_slider_aria()}
          />
          <output class="config-value">{m.levers_labelpct_value({ pct: displayLabelPct })}</output>
        </span>
      </span>
    {/if}
    {#if configParams.includes('displayLanguage')}
      <span class="cooc-lg">
        <span class="ctrl-eyebrow">{m.levers_labels_eyebrow()}</span>
        <LeverButton
          role="switch"
          active={activeDisplayLanguage === 'viewer'}
          onclick={() =>
            setDisplayLanguage(activeDisplayLanguage === 'viewer' ? 'source' : 'viewer')}
          title={m.levers_labels_title({ language: viewerLanguage })}
        >
          {activeDisplayLanguage === 'viewer'
            ? m.levers_labels_app({ language: viewerLanguage })
            : m.levers_labels_source()}
        </LeverButton>
      </span>
    {/if}
  </div>
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

<style>
  /* Phase 148e — co-occurrence compact clusters. The shared eyebrow / config-value
     styles come from LeverRow's :global rules (these rows carry `.ctrl-row`); only
     the cluster geometry + the range inputs (which are NOT in `.config-inline`, so
     they miss the global slider rule) are declared here. */
  .cooc-sliders {
    /* Three layout sliders packed at the LEFT, each capped at a sane width and
       shrinking on a narrow panel. Packing left (not stretching 1fr columns
       across the whole strip) keeps the sliders adjacent — a wide panel leaves
       the slack at the right edge instead of as ugly gaps between them. */
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 12rem));
    justify-content: start;
    gap: var(--space-3);
    align-items: end;
  }
  .cooc-mini {
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
  }
  .cooc-mini-control,
  .cooc-pct {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
  .cooc-mini-control input[type='range'],
  .cooc-pct input[type='range'] {
    flex: 1 1 auto;
    min-width: 0;
    /* WCAG 2.2 (2.5.8) — 24px minimum target height, mirroring the global rule. */
    min-height: 24px;
    accent-color: var(--color-accent);
    cursor: pointer;
  }
  /* The layout sliders fill their (≤12rem) grid column — the column already caps
     the width, so no separate max-width is needed (an 18rem cap left a gap when
     a wide panel made the column wider than the slider). */
  .cooc-mini-control input[type='range'] {
    width: 100%;
  }
  /* Density slider: fixed at ~half a layout slider and non-shrinking, so the row
     after it (the language button) never shifts. */
  .cooc-pct input[type='range'] {
    flex: 0 0 5.5rem;
    width: 5.5rem;
  }
  /* Keep the numeric read-outs (e.g. "top 10%") on one line so a tight column
     never stacks the value vertically. */
  .cooc-mini-control output,
  .cooc-pct output {
    white-space: nowrap;
  }
  /* Fixed-width % read-out (fits "Top 100%") so a 1→2 digit change never nudges
     the language button rightward. */
  .cooc-pct output {
    width: 3.5rem;
    text-align: left;
  }
  /* Phase 148e — editable Entities count: the slider is coarse over 50..10000, so
     a compact spinner sits beside it for exact 1-entity entry. */
  .cooc-num {
    width: 4.25rem;
    flex-shrink: 0;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 2px var(--space-1);
    min-height: 24px;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }
  .cooc-num:hover,
  .cooc-num:focus-visible {
    border-color: var(--color-accent);
  }
  /* Labels row: labels toggle · ranking · density · display-language, each kept as
     a tight (eyebrow + control) group; groups are spaced apart and wrap gracefully
     on a narrow panel so the language button never crowds the density slider. */
  .cooc-labels {
    gap: var(--space-2) var(--space-4);
  }
  .cooc-lg {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
  /* In the labels row the eyebrow hugs its control (no 3.5rem reservation) so a
     group reads as one unit. */
  .cooc-lg .ctrl-eyebrow {
    min-width: auto;
  }
  .cooc-pct {
    flex: 0 0 auto;
  }
</style>
