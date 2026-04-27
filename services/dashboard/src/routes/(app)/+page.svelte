<script lang="ts">
  // Atmosphere route (Surface I — landing overview).
  //
  // Phase 113c reframing: clicking a probe descends directly to Surface II
  // L1 Probe Dossier. The legacy in-page L3 Analysis SidePanel ("flyout")
  // is retired — its information has been folded into the Dossier where it
  // belongs. Surface I now does exactly what Brief §4.1 asks of it:
  // welcome, orient, invite descent.
  //
  // Layer mapping (per Brief §5.2 — canonical):
  //   L0 Immersion   — 3D globe, always rendered
  //   L1 Orientation — hover tooltips (Progressive Semantics)
  //   L2 Exploration — TimeScrubber + ScopeBar resolution selector
  //   L3 Analysis    — NOT hosted on Surface I. Lives natively on Surface II.
  //   L4 Provenance  — methodology tray (right-edge, all surfaces)
  //   L5 Evidence    — reader-pane overlay (Surfaces II/III)
  //
  // Keyboard descent grammar:
  //   Tab           cycles through probes (sr-only nav)
  //   Enter/Space   navigates to /lanes/{probeId}/dossier
  //   Shift+N       toggles Negative Space overlay (structural only)
  //
  // Shell-chunk rules (enforced by tests/unit/lazy-engine.test.ts):
  //   - MUST NOT statically import three, @aer/engine-3d, or uplot.
  //   - The engine is dynamic-imported inside AtmosphereCanvas.
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
  import { ScopeBar } from '$lib/components/chrome';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { negativeSpaceActive } from '$lib/state/tray.svelte';
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

  // --- Selection (highlight only) -------------------------------------
  // Surface I no longer hosts L3 — selection is a navigation event.
  // Hover state is local; a click descends to Surface II.
  let hoveredProbe: ProbeSelection | null = $state(null);
  let hoveredSatellite: SatelliteSelection | null = $state(null);
  let pointerX = $state(0);
  let pointerY = $state(0);

  /**
   * Run a transition wrapped in the View Transitions API when available.
   * Used for descent into Surface II so the visual continuity from globe
   * to Dossier is preserved on browsers that support it.
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

  // --- URL deeplink -> redirect to Dossier (Phase 113c) ---------------
  // Old bookmarks (`/?probe=X[&view=analysis…]`) descended into the in-page
  // L3 panel. The panel is gone; redirect to the canonical descent target.
  // Strip the descent-related params on the way out so the Dossier URL
  // stays canonical.
  $effect(() => {
    if (typeof window === 'undefined') return;
    if (!url.probe) return;
    const probeId = url.probe;
    setUrl({ probe: null, emissionPoint: null, view: null, metric: null });
    descend(() => {
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface II route
      void goto(`/lanes/${encodeURIComponent(probeId)}/dossier`);
    });
  });

  function onProbeSelected(sel: ProbeSelection) {
    descend(() => {
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface II route
      void goto(`/lanes/${encodeURIComponent(sel.probeId)}/dossier`);
    });
  }

  function onProbeHovered(sel: ProbeSelection | null) {
    hoveredProbe = sel;
    // Intent-based preload: once the user hovers a probe, warm the
    // dossier route so descent feels instantaneous.
    if (sel) {
      void import('$lib/components/lanes/ProbeDossier.svelte').catch(() => void 0);
    }
  }

  function onSatelliteHovered(sel: SatelliteSelection | null) {
    hoveredSatellite = sel;
  }

  function onSatelliteSelected(sel: SatelliteSelection) {
    // Satellite click: descend to the Dossier with source scope pre-set.
    // We use setUrl to update the in-memory state so the dossier receives
    // the narrowed source immediately on render (without a query param).
    setUrl({ sourceIds: [sel.sourceName] });
    descend(() => {
      // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface II route
      void goto(`/lanes/${encodeURIComponent(sel.probeId)}/dossier`);
    });
  }

  function onPointerMove(e: PointerEvent) {
    pointerX = e.clientX;
    pointerY = e.clientY;
  }

  // --- Keyboard descent grammar ---------------------------------------
  interface FlatProbe {
    probeId: string;
    language: string;
  }
  let flatProbes = $derived.by<FlatProbe[]>(() =>
    probeDtos.map((p) => ({ probeId: p.probeId, language: p.language }))
  );

  // Pre-resolve the hovered probe DTO so the tooltip can render the
  // semantic-register identity.
  let hoveredProbeDto = $derived(
    hoveredProbe ? (probeDtos.find((p) => p.probeId === hoveredProbe!.probeId) ?? null) : null
  );

  // Negative Space overlay state lives in $lib/state/tray (Phase 108)
  // so the methodology tray can switch into limitations-first mode
  // without prop-drilling through the (app) layout.
  const negSpace = $derived(negativeSpaceActive());

  let resolution = $derived(url.resolution ?? 'hourly');
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>

<!-- Top scope bar: primer link only. Resolution + time window moved into TimeScrubber (Phase 113d). -->
<ScopeBar label="Atmosphere surface controls">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface III primer route -->
  <a class="primer-link" href="/reflection/primer/globe">How to read the globe →</a>
</ScopeBar>

{#if decision === 'engine'}
  <div
    class="stage"
    class:neg-space={negSpace}
    aria-hidden="false"
    onpointermove={onPointerMove}
    onpointerleave={() => onProbeHovered(null)}
    role="presentation"
  >
    {#if negSpace}
      <!-- Surface I — Negative Space mode: coverage boundary banner per WP-001 §5.3 -->
      <aside class="absence-banner" aria-label="Negative Space mode active">
        <span class="absence-glyph" aria-hidden="true">∅</span>
        <span class="absence-text">Coverage boundary mode — unmonitored regions foregrounded</span>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal WP link -->
        <a class="absence-link" href="/reflection/wp/wp-001?section=5.3">WP-001 §5.3</a>
      </aside>
    {/if}
    <AtmosphereCanvas
      probes={probeMarkers}
      {activity}
      {onProbeSelected}
      {onProbeHovered}
      {onSatelliteSelected}
      {onSatelliteHovered}
      selection={null}
      hover={hoveredProbe}
      flyToOnSelection={null}
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
        <span class="tooltip-affordance">Click to open the Probe Dossier (Surface II)</span>
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
        <span class="tooltip-affordance"
          >Click to open the Dossier — narrowed to source {hoveredSatellite.sourceName}</span
        >
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

<!-- Time scrubber: bottom center (Design Brief §4.2, L2 Exploration). Hosts resolution selector. -->
<div class="l2-slot">
  <TimeScrubber {resolution} onResolutionChange={(r) => setUrl({ resolution: r })} />
</div>

<style>
  .stage {
    position: fixed;
    inset: 0;
    background: #000;
    transition: opacity 320ms ease-in-out;
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
    width: min(90vw, 52rem);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    align-items: stretch;
    z-index: 400;
  }

  /* Negative Space mode — Surface I visual treatment (Phase 112). */
  .stage.neg-space {
    filter: saturate(0.6) hue-rotate(20deg) brightness(0.85);
  }

  /* Coverage boundary banner — floats above the globe in negSpace mode. */
  .absence-banner {
    position: absolute;
    top: calc(var(--scope-bar-height) + var(--space-3));
    left: calc(var(--rail-width) + var(--space-4));
    z-index: 350;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: rgba(0, 0, 0, 0.72);
    border: 1px solid rgba(82, 131, 184, 0.5);
    border-radius: var(--radius-sm);
    color: #a0b8d8;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    pointer-events: auto;
    backdrop-filter: blur(4px);
  }
  .absence-glyph {
    font-size: 1rem;
    line-height: 1;
  }
  .absence-text {
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }
  .absence-link {
    color: #5283b8;
    text-decoration: none;
    border-bottom: 1px dotted #5283b8;
    padding-bottom: 1px;
  }
  .absence-link:hover,
  .absence-link:focus-visible {
    color: #a0b8d8;
    border-bottom-color: #a0b8d8;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
</style>
