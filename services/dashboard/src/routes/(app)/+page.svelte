<script lang="ts">
  // Atmosphere route (Phases 99b → 105).
  //
  // Phase 105 (Navigation Chrome): integrates ScopeBar into Surface I,
  // relocates the resolution selector and NegativeSpaceToggle from the
  // bottom L2 strip into the top scope bar, and retires the standalone
  // L2Controls component (pillar toggle now lives in SideRail; resolution
  // now lives in ScopeBar).
  //
  // Layer mapping:
  //   L0 Immersion  — 3D globe, always rendered
  //   L1 Orientation — (removed Phase 105; content moves to SideRail/Dossier per reframing §5)
  //   L2 Exploration — TimeScrubber (bottom) + controls in ScopeBar (top)
  //   L3 Analysis    — SidePanel with TimeSeriesChart (uPlot)
  //   L4 Provenance  — fly-out overlay anchored to L3 (Phase 108: moves to MethodologyTray)
  //
  // Keyboard descent grammar:
  //   Tab           cycles through probes (sr-only nav)
  //   Enter/Space   descends to L3 on the focused probe
  //   Escape        ascends one layer (L4 → L3 → L0)
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
  import L3AnalysisPanel from '$lib/components/L3AnalysisPanel.svelte';
  import NegativeSpaceToggle from '$lib/components/NegativeSpaceToggle.svelte';
  import { ScopeBar } from '$lib/components/chrome';
  import { SidePanel } from '$lib/components/base';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import { setFocusedMetric } from '$lib/state/metric.svelte';
  import {
    negativeSpaceActive,
    setNegativeSpaceActive,
    setTrayOpen,
    trayOpen
  } from '$lib/state/tray.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import type { Resolution } from '$lib/state/url-internals';
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

  const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];

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
      panelOpen = false;
      selected = null;
    });
    setTrayOpen(false);
    setFocusedMetric(null);
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
      if (panelOpen) {
        e.preventDefault();
        onPanelClose();
        return;
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

  // Negative Space overlay state lives in $lib/state/tray (Phase 108)
  // so the methodology tray can switch into limitations-first mode
  // without prop-drilling through the (app) layout.
  const negSpace = $derived(negativeSpaceActive());

  let windowLabel = $derived(
    `${new Date(windowMs.start).toISOString().slice(0, 16).replace('T', ' ')}Z → ${new Date(
      windowMs.end
    )
      .toISOString()
      .slice(0, 16)
      .replace('T', ' ')}Z`
  );

  let resolutionForL3 = $derived(url.resolution ?? 'hourly');
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>

<!-- Top scope bar: resolution selector + Negative Space toggle for Surface I -->
<ScopeBar label="Atmosphere surface controls">
  <span class="window-label" aria-label="Time window: {windowLabel}">{windowLabel}</span>
  <label class="resolution-label">
    <span class="label-text">Resolution</span>
    <select
      value={resolutionForL3}
      onchange={(e) =>
        setUrl({ resolution: (e.currentTarget as HTMLSelectElement).value as Resolution })}
      aria-label="Temporal resolution"
    >
      {#each RESOLUTIONS as r (r)}
        <option value={r}>{r}</option>
      {/each}
    </select>
  </label>
  <NegativeSpaceToggle active={negSpace} onToggle={setNegativeSpaceActive} />
</ScopeBar>

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
        publicationRate={selectedRate}
        onOpenProvenance={() => {
          // Toggle, not just open: a second click on the panel's
          // affordance closes the tray, matching the right-edge tab
          // behaviour. Focus is set unconditionally so re-opening
          // lands on the current metric.
          setFocusedMetric({ metricName: url.metric ?? 'sentiment_score' });
          setTrayOpen(!trayOpen());
        }}
      />
    {/if}
  </SidePanel>

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

<!-- Time scrubber: remains at bottom center (Design Brief §4.2, L2 Exploration) -->
<div class="l2-slot">
  <TimeScrubber />
</div>

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
    top: calc(var(--scope-bar-height) + var(--space-3));
    left: calc(var(--rail-width) + var(--space-3));
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
    left: calc(var(--rail-width) + var(--space-5));
    max-width: 28rem;
    z-index: 500;
  }
  .l2-slot {
    position: fixed;
    bottom: var(--space-5);
    left: calc(var(--rail-width) + 50%);
    transform: translateX(-50%);
    width: min(90vw, 36rem);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    align-items: stretch;
    z-index: 400;
  }

  /* Scope bar content styles (specific to Surface I) */
  .window-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .resolution-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-shrink: 0;
  }
  .label-text {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-fg-subtle);
  }
  select {
    appearance: none;
    -webkit-appearance: none;
    background-color: rgba(0, 0, 0, 0.45);
    background-image:
      linear-gradient(45deg, transparent 50%, var(--color-fg-muted) 50%),
      linear-gradient(135deg, var(--color-fg-muted) 50%, transparent 50%);
    background-position:
      calc(100% - 14px) 50%,
      calc(100% - 9px) 50%;
    background-size:
      5px 5px,
      5px 5px;
    background-repeat: no-repeat;
    color: var(--color-fg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px 22px 2px var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
  }
  select:hover,
  select:focus-visible {
    border-color: var(--color-accent);
    outline: none;
  }
  select option {
    background: var(--color-surface);
    color: var(--color-fg);
  }
</style>
