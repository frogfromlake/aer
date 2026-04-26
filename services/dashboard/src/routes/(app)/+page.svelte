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
  import { goto } from '$app/navigation';
  import { createQuery } from '@tanstack/svelte-query';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import type {
    ProbeActivity,
    ProbeMarker,
    ProbeSelection,
    SatelliteSelection
  } from '@aer/engine-3d';
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

  // Probe → engine model. Each emission point carries the canonical
  // source name aligned positionally with `probe.sources[i]` (Phase 110:
  // satellite click routes to /lanes/:probeId/dossier?sourceId=…). When
  // sources and emissionPoints have unequal lengths the trailing entries
  // get no sourceName and the engine renders no satellite for them.
  let probeMarkers = $derived.by<ProbeMarker[]>(() =>
    probeDtos.map((p) => ({
      id: p.probeId,
      language: p.language,
      label: p.probeId,
      emissionPoints: p.emissionPoints.map((ep, i) => {
        const source = p.sources[i];
        return source !== undefined
          ? {
              latitude: ep.latitude,
              longitude: ep.longitude,
              label: ep.label,
              sourceName: source
            }
          : { latitude: ep.latitude, longitude: ep.longitude, label: ep.label };
      })
    }))
  );

  function probeCentroid(p: ProbeDto | null): { latitude: number; longitude: number } | null {
    if (!p || p.emissionPoints.length === 0) return null;
    if (p.emissionPoints.length === 1) {
      const e = p.emissionPoints[0]!;
      return { latitude: e.latitude, longitude: e.longitude };
    }
    const DEG = Math.PI / 180;
    let x = 0;
    let y = 0;
    let z = 0;
    for (const ep of p.emissionPoints) {
      const lat = ep.latitude * DEG;
      const lon = ep.longitude * DEG;
      const c = Math.cos(lat);
      x += c * Math.cos(lon);
      y += c * Math.sin(lon);
      z += Math.sin(lat);
    }
    const len = Math.hypot(x, y, z);
    if (len < 1e-9) return { latitude: 0, longitude: 0 };
    return {
      latitude: Math.asin(Math.max(-1, Math.min(1, z / len))) / DEG,
      longitude: Math.atan2(y / len, x / len) / DEG
    };
  }

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
  // Phase 110 makes the probe glyph the only scope-target; the legacy
  // `?ep=…` parameter is ignored on hydration but still parsed for
  // back-compat with bookmarked URLs.
  $effect(() => {
    if (!url.probe || selected) return;
    const hit = probeDtos.find((p) => p.probeId === url.probe);
    if (!hit || hit.emissionPoints.length === 0) return;
    selected = { probeId: hit.probeId };
    panelOpen = url.view !== 'atmosphere';
  });

  function onProbeSelected(sel: ProbeSelection) {
    if (selected?.probeId === sel.probeId && panelOpen) {
      onPanelClose();
      return;
    }
    descend(() => {
      selected = sel;
      panelOpen = true;
    });
    setUrl({
      probe: sel.probeId,
      emissionPoint: null,
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

  // --- Hover tooltips --------------------------------------------------
  // Probe-glyph hover (Progressive Semantics: identity + affordance) and
  // satellite hover (source name + nav affordance) are tracked
  // independently so the engine's "probe wins on overlap" precedence
  // surfaces cleanly in the UI.
  let hoveredProbe: ProbeSelection | null = $state(null);
  let hoveredSatellite: SatelliteSelection | null = $state(null);
  let pointerX = $state(0);
  let pointerY = $state(0);

  function onProbeHovered(sel: ProbeSelection | null) {
    hoveredProbe = sel;
    // Intent-based preload: once the user hovers a probe, warm the
    // dossier route so descent feels instantaneous.
    if (sel) {
      void import('$lib/components/TimeSeriesChart.svelte').catch(() => void 0);
    }
  }

  function onSatelliteHovered(sel: SatelliteSelection | null) {
    hoveredSatellite = sel;
  }

  function onSatelliteSelected(sel: SatelliteSelection) {
    // Phase 110: satellite click is a navigation event, not a scope
    // change on Surface I. Routes to the Probe Dossier with the source
    // pre-filtered.
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface II route
    void goto(`/lanes/${sel.probeId}/dossier?sourceId=${encodeURIComponent(sel.sourceName)}`);
  }

  function onPointerMove(e: PointerEvent) {
    pointerX = e.clientX;
    pointerY = e.clientY;
  }

  // --- Keyboard descent grammar ---------------------------------------
  // One sr-only button per probe (Phase 110: probe is the scope-target).
  interface FlatProbe {
    probeId: string;
    language: string;
  }
  let flatProbes = $derived.by<FlatProbe[]>(() =>
    probeDtos.map((p) => ({ probeId: p.probeId, language: p.language }))
  );

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
  let selectedCentroid = $derived(probeCentroid(selectedProbeDto));
  // Pre-resolve the hovered probe DTO so the tooltip can render the
  // semantic-register identity (probe ID for now; full content-catalog
  // semantic short stays in the L3 panel + methodology tray).
  let hoveredProbeDto = $derived(
    hoveredProbe ? (probeDtos.find((p) => p.probeId === hoveredProbe!.probeId) ?? null) : null
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

<!-- Top scope bar: resolution selector + Negative Space toggle + primer link (Phase 110) -->
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
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface III primer route -->
  <a class="primer-link" href="/reflection/primer/globe">How to read the globe →</a>
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
      {onSatelliteSelected}
      {onSatelliteHovered}
      selection={selected}
      hover={hoveredProbe}
      flyToOnSelection={selectedCentroid}
    />

    {#if hoveredProbe}
      <div
        class="probe-tooltip"
        role="tooltip"
        style:left="{pointerX + 14}px"
        style:top="{pointerY + 14}px"
      >
        <span class="tooltip-headline">{hoveredProbeDto?.probeId ?? hoveredProbe.probeId}</span>
        <span class="tooltip-meta">{(hoveredProbeDto?.language ?? '').toUpperCase()}</span>
        <span class="tooltip-affordance">Click to open dossier · expand methodology in tray</span>
      </div>
    {:else if hoveredSatellite}
      <div
        class="probe-tooltip satellite"
        role="tooltip"
        style:left="{pointerX + 14}px"
        style:top="{pointerY + 14}px"
      >
        <span class="tooltip-headline">{hoveredSatellite.label}</span>
        <span class="tooltip-meta">source · {hoveredSatellite.sourceName}</span>
        <span class="tooltip-affordance">Click to scope the dossier to this source</span>
      </div>
    {/if}

    <ul class="sr-probe-nav" aria-label="Probes on the globe">
      {#each flatProbes as p (p.probeId)}
        <li>
          <button
            type="button"
            class="sr-only"
            aria-label="Probe {p.probeId}, {p.language}"
            onfocus={() => onProbeHovered({ probeId: p.probeId })}
            onblur={() => onProbeHovered(null)}
            onclick={() => onProbeSelected({ probeId: p.probeId })}
          >
            {p.probeId}
          </button>
        </li>
      {/each}
    </ul>
  </div>

  <SidePanel
    bind:open={panelOpen}
    title={selectedProbeDto?.probeId ?? selected?.probeId ?? 'Probe'}
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
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
    pointer-events: none;
    white-space: nowrap;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.4);
    display: flex;
    flex-direction: column;
    gap: 2px;
    max-width: 22rem;
  }
  .probe-tooltip.satellite {
    border-color: var(--color-border-strong, var(--color-border));
    background: color-mix(in srgb, var(--color-surface) 92%, var(--color-accent-muted));
  }
  .tooltip-headline {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .tooltip-meta {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-muted);
  }
  .tooltip-affordance {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    white-space: normal;
  }
  .primer-link {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-border);
    padding-bottom: 1px;
    flex-shrink: 0;
  }
  .primer-link:hover,
  .primer-link:focus-visible {
    color: var(--color-accent);
    border-bottom-color: var(--color-accent);
    outline: none;
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
