<script lang="ts">
  // Atmosphere route (Phases 99b → 100a).
  //
  // 100a adds the L0→L4 descent grammar on top of 99b's live-data
  // Atmosphere. Layer mapping:
  //   L0 Immersion  — 3D globe, always rendered
  //   L1 Orientation — soft top-bar overlay (fade on idle)
  //   L2 Exploration — TimeScrubber + resolution/pillar controls
  //   L3 Analysis    — SidePanel with TimeSeriesChart (uPlot)
  //   L4 Provenance  — fly-out overlay anchored to L3
  //
  // Descent via `document.startViewTransition()` with graceful
  // degradation on browsers without the API. Keyboard descent grammar:
  //   Tab           cycles through probes (sr-only nav)
  //   Enter/Space   descends to L3 on the focused probe
  //   Escape        ascends one layer (L4 → L3 → L0)
  //   Shift+Tab @L0 focuses the L1 overlay
  //   Shift+N       toggles Negative Space overlay (structural only)
  //
  // Shell-chunk rules (enforced by tests/unit/lazy-engine.test.ts):
  //   - MUST NOT statically import three, @aer/engine-3d, or uplot.
  //   - The engine is dynamic-imported inside AtmosphereCanvas.
  //   - uPlot is dynamic-imported inside TimeSeriesChart (L3 chunk).
  import { onMount } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import type { ProbeActivity, ProbeMarker, ProbeSelection } from '@aer/engine-3d';
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import TimeScrubber from '$lib/components/TimeScrubber.svelte';
  import L1Overlay from '$lib/components/L1Overlay.svelte';
  import L2Controls from '$lib/components/L2Controls.svelte';
  import L3AnalysisPanel from '$lib/components/L3AnalysisPanel.svelte';
  import L4ProvenanceFlyout from '$lib/components/L4ProvenanceFlyout.svelte';
  import NegativeSpaceToggle from '$lib/components/NegativeSpaceToggle.svelte';
  import { SidePanel } from '$lib/components/base';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import {
    metricsQuery,
    probesQuery,
    type FetchContext,
    type MetricsResponseDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';

  // The BFF is reachable at `/api/v1` via Traefik in every deployment.
  // Traefik attaches X-API-Key to every /api/* request (see compose.yaml
  // bff-api labels), so the static bundle ships with no secret.
  const ctx: FetchContext = {
    baseUrl: '/api/v1'
  };

  let decision: 'pending' | 'engine' | 'fallback' = $state('pending');

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const forceFallback = params.get('fallback') === '1';
    decision = !forceFallback && hasWebGL2() ? 'engine' : 'fallback';
  });

  // --- Probes ----------------------------------------------------------
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let probeDtos = $derived.by<ProbeDto[]>(() => {
    const d = probesQ.data;
    return d?.kind === 'success' ? d.data : [];
  });

  let probeMarkers = $derived.by<ProbeMarker[]>(() =>
    probeDtos.map((p) => ({
      id: p.probeId,
      language: p.language,
      emissionPoints: p.emissionPoints.map((ep) => ({
        latitude: ep.latitude,
        longitude: ep.longitude,
        label: ep.label
      }))
    }))
  );

  // --- Time window (URL-backed) ---------------------------------------
  const url = $derived(urlState());
  const windowMs = $derived.by<{ start: string; end: string; hours: number }>(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    const safeFrom = Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS;
    const safeTo = Number.isFinite(toMs) ? toMs : now;
    return {
      start: new Date(safeFrom).toISOString(),
      end: new Date(safeTo).toISOString(),
      hours: Math.max(1, (safeTo - safeFrom) / (60 * 60 * 1000))
    };
  });

  // --- Metrics → per-probe activity -----------------------------------
  const metricsQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    const o = metricsQuery(ctx, {
      startDate: windowMs.start,
      endDate: windowMs.end,
      metricName: 'publication_hour',
      resolution: url.resolution ?? 'hourly'
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let activity = $derived.by<ProbeActivity[]>(() => {
    const m = metricsQ.data;
    if (m?.kind !== 'success' || probeDtos.length === 0) return [];
    const perSource: Record<string, number> = {};
    for (const row of m.data.data) {
      perSource[row.source] = (perSource[row.source] ?? 0) + (row.count ?? 0);
    }
    return probeDtos.map((p) => {
      const total = p.sources.reduce((sum, s) => sum + (perSource[s] ?? 0), 0);
      return { probeId: p.probeId, documentsPerHour: total / windowMs.hours };
    });
  });

  // --- Selection + descent layer --------------------------------------
  let selected: ProbeSelection | null = $state(null);
  let panelOpen = $state(false);
  let provenanceOpen = $state(false);

  /**
   * Run a transition wrapped in the View Transitions API when available.
   * The callback mutates reactive state that drives layout; Svelte's
   * next commit provides the "after" frame uPlot/the engine draw into.
   * Browsers without the API fall through to an instant state change —
   * still correct, just less elegant (Roadmap Phase 100a exit criterion).
   */
  function descend(mutator: () => void) {
    const doc = typeof document !== 'undefined' ? document : null;
    const startViewTransition = (
      doc as unknown as { startViewTransition?: (cb: () => void) => unknown } | null
    )?.startViewTransition;
    if (typeof startViewTransition === 'function') {
      startViewTransition.call(doc, mutator);
    } else {
      mutator();
    }
  }

  // --- URL → state hydration (deep links) -----------------------------
  $effect(() => {
    if (!url.probe || selected) return;
    const hit = probeDtos.find((p) => p.probeId === url.probe);
    if (!hit || hit.emissionPoints.length === 0) return;
    const rawIdx = url.emissionPoint ?? 0;
    const idx = rawIdx >= 0 && rawIdx < hit.emissionPoints.length ? rawIdx : 0;
    const ep = hit.emissionPoints[idx]!;
    selected = {
      probeId: hit.probeId,
      emissionPointIndex: idx,
      emissionPointLabel: ep.label
    };
    // Opening the panel on deep-link when a probe is carried by the URL
    // preserves the Phase 99b contract. The `view` parameter is an
    // additive explicit marker: if it's present and not `analysis` the
    // user has opted out of auto-descent, otherwise we default to open.
    panelOpen = url.view !== 'atmosphere';
  });

  function onProbeSelected(sel: ProbeSelection) {
    if (
      selected?.probeId === sel.probeId &&
      selected.emissionPointIndex === sel.emissionPointIndex &&
      panelOpen
    ) {
      onPanelClose();
      return;
    }
    descend(() => {
      selected = sel;
      panelOpen = true;
    });
    setUrl({
      probe: sel.probeId,
      emissionPoint: sel.emissionPointIndex,
      view: 'analysis',
      metric: url.metric ?? 'sentiment_score'
    });
  }

  function onPanelClose() {
    descend(() => {
      provenanceOpen = false;
      panelOpen = false;
      selected = null;
    });
    setUrl({
      probe: null,
      emissionPoint: null,
      view: null,
      metric: null
    });
  }

  // --- Hover tooltip ---------------------------------------------------
  let hovered: ProbeSelection | null = $state(null);
  let pointerX = $state(0);
  let pointerY = $state(0);

  function onProbeHovered(sel: ProbeSelection | null) {
    hovered = sel;
    // Intent-based preload: once the user hovers a probe, warm the
    // uPlot chunk so the descent feels instantaneous.
    if (sel) void import('$lib/components/TimeSeriesChart.svelte').catch(() => void 0);
  }

  function onPointerMove(e: PointerEvent) {
    pointerX = e.clientX;
    pointerY = e.clientY;
  }

  // --- Keyboard descent grammar ---------------------------------------
  let l1El: HTMLElement | undefined = $state();
  interface FlatProbePoint {
    probeId: string;
    emissionPointIndex: number;
    label: string;
    language: string;
  }
  let flatPoints = $derived.by<FlatProbePoint[]>(() => {
    const out: FlatProbePoint[] = [];
    for (const p of probeDtos) {
      p.emissionPoints.forEach((ep, i) => {
        out.push({
          probeId: p.probeId,
          emissionPointIndex: i,
          label: ep.label,
          language: p.language
        });
      });
    }
    return out;
  });

  function onGlobalKeydown(e: KeyboardEvent) {
    // Escape ascends one layer. SidePanel's own Escape handler also
    // closes the panel, but we want L4-first precedence so L4 → L3
    // ascent lands correctly even though the L4 overlay is not inside
    // the SidePanel DOM.
    if (e.key === 'Escape') {
      if (provenanceOpen) {
        e.preventDefault();
        descend(() => {
          provenanceOpen = false;
        });
        return;
      }
      if (panelOpen) {
        e.preventDefault();
        onPanelClose();
        return;
      }
    }
    // Shift+Tab at L0 focuses the L1 overlay. When the panel is open or
    // the user is already inside a form field, yield to native Tab.
    if (e.key === 'Tab' && e.shiftKey && !panelOpen) {
      const t = e.target as HTMLElement | null;
      const inForm = t?.tagName === 'INPUT' || t?.tagName === 'TEXTAREA' || t?.tagName === 'SELECT';
      const inPanel = !!t?.closest?.('[role="dialog"]');
      if (!inForm && !inPanel && l1El) {
        e.preventDefault();
        l1El.focus();
      }
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onGlobalKeydown);
    return () => window.removeEventListener('keydown', onGlobalKeydown);
  });

  let selectedProbeDto = $derived(probeDtos.find((p) => p.probeId === selected?.probeId) ?? null);
  let selectedRate = $derived(
    activity.find((a) => a.probeId === selected?.probeId)?.documentsPerHour ?? null
  );

  // Negative Space overlay — structural only (see component comment).
  let negativeSpaceActive = $state(false);

  let windowLabel = $derived(
    `${new Date(windowMs.start).toISOString().slice(0, 16).replace('T', ' ')}Z → ${new Date(
      windowMs.end
    )
      .toISOString()
      .slice(0, 16)
      .replace('T', ' ')}Z`
  );

  let resolutionForL3 = $derived(url.resolution ?? 'hourly');
  let metricForL4 = $derived(url.metric ?? 'sentiment_score');
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>

{#if decision === 'engine'}
  <div
    class="stage"
    class:dimmed={panelOpen}
    aria-hidden="false"
    onpointermove={onPointerMove}
    onpointerleave={() => onProbeHovered(null)}
    role="presentation"
  >
    <AtmosphereCanvas
      probes={probeMarkers}
      {activity}
      {onProbeSelected}
      {onProbeHovered}
      selection={selected}
      hover={hovered}
    />

    {#if hovered}
      <div
        class="probe-tooltip"
        role="tooltip"
        style:left="{pointerX + 14}px"
        style:top="{pointerY + 14}px"
      >
        {hovered.emissionPointLabel}
      </div>
    {/if}

    <ul class="sr-probe-nav" aria-label="Probes on the globe">
      {#each flatPoints as pt (pt.probeId + '#' + pt.emissionPointIndex)}
        <li>
          <button
            type="button"
            class="sr-only"
            aria-label="Probe {pt.probeId}, {pt.language}, emission point {pt.label}"
            onfocus={() =>
              onProbeHovered({
                probeId: pt.probeId,
                emissionPointIndex: pt.emissionPointIndex,
                emissionPointLabel: pt.label
              })}
            onblur={() => onProbeHovered(null)}
            onclick={() =>
              onProbeSelected({
                probeId: pt.probeId,
                emissionPointIndex: pt.emissionPointIndex,
                emissionPointLabel: pt.label
              })}
          >
            {pt.label}
          </button>
        </li>
      {/each}
    </ul>
  </div>

  <div class="l1-slot" bind:this={l1El}>
    <L1Overlay probes={probeDtos} {windowLabel} normalization="raw" {ctx} />
  </div>

  <div class="l2-slot">
    <L2Controls />
    <TimeScrubber />
  </div>

  <div class="chrome-slot">
    <NegativeSpaceToggle
      active={negativeSpaceActive}
      onToggle={(next) => (negativeSpaceActive = next)}
    />
  </div>

  <SidePanel
    bind:open={panelOpen}
    title={selected?.emissionPointLabel ?? 'Probe'}
    onClose={onPanelClose}
    size="dashboard"
  >
    {#if selected && selectedProbeDto}
      <L3AnalysisPanel
        probe={selectedProbeDto}
        {ctx}
        windowStart={windowMs.start}
        windowEnd={windowMs.end}
        resolution={resolutionForL3}
        publicationRate={selectedRate}
        onOpenProvenance={() => descend(() => (provenanceOpen = true))}
      />
    {/if}
  </SidePanel>

  {#if provenanceOpen && panelOpen}
    <L4ProvenanceFlyout
      metricName={metricForL4}
      {ctx}
      onClose={() => descend(() => (provenanceOpen = false))}
    />
  {/if}

  {#if probesQ.data?.kind === 'refusal'}
    <div class="refusal-slot">
      <RefusalSurface refusal={probesQ.data} {ctx} />
    </div>
  {/if}
{:else if decision === 'fallback'}
  <div class="centered">
    <WebGLFallback probes={probeDtos} {activity} loading={probesQ.isPending} />
  </div>

  {#if probesQ.data?.kind === 'refusal'}
    <div class="refusal-slot">
      <RefusalSurface refusal={probesQ.data} {ctx} />
    </div>
  {/if}
{/if}

<style>
  .stage {
    position: fixed;
    inset: 0;
    background: #000;
    transition: opacity 320ms ease-in-out;
  }
  .stage.dimmed {
    /* §4.1 rule 2: "no layer replaces". Globe stays visible at 30 %
       opacity behind the L3 panel so the reader never loses its place. */
    opacity: 0.3;
  }
  .probe-tooltip {
    position: fixed;
    z-index: 600;
    padding: var(--space-1) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
    pointer-events: none;
    white-space: nowrap;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
  }
  .sr-probe-nav {
    list-style: none;
    padding: 0;
    margin: 0;
  }
  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
  .sr-only:focus,
  .sr-only:focus-visible {
    position: fixed;
    top: var(--space-3);
    left: var(--space-3);
    width: auto;
    height: auto;
    padding: var(--space-1) var(--space-3);
    margin: 0;
    overflow: visible;
    clip: auto;
    z-index: 700;
    background: var(--color-surface);
    color: var(--color-fg);
    border: 2px solid var(--color-accent);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-sm);
  }
  .centered {
    min-height: 100dvh;
    display: grid;
    place-items: center;
  }
  .refusal-slot {
    position: fixed;
    bottom: var(--space-5);
    left: var(--space-5);
    max-width: 28rem;
    z-index: 500;
  }
  .l1-slot {
    /* No positioning of its own — L1Overlay is fixed-positioned. */
    display: contents;
  }
  .l2-slot {
    position: fixed;
    bottom: var(--space-5);
    left: 50%;
    transform: translateX(-50%);
    width: min(90vw, 36rem);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    align-items: stretch;
    z-index: 400;
  }
  .chrome-slot {
    position: fixed;
    top: var(--space-3);
    right: var(--space-3);
    z-index: 460;
  }
</style>
