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
  //   L2 Exploration — Surface I no longer hosts time controls; the
  //                    TimeScrubber + resolution selector were dropped
  //                    because they had no visible effect on the globe.
  //                    Time-range / resolution will return as view-mode-
  //                    specific controls inside Surface II L3 cells.
  //   L3 Analysis    — NOT hosted on Surface I. Lives natively on Surface II.
  //   L4 Provenance  — inline methodology accordion on Surface II L3.
  //   L5 Evidence    — reader-pane overlay (Surfaces II/III)
  //
  // Keyboard descent grammar:
  //   Tab           cycles through probes (sr-only nav)
  //   Enter/Space   navigates to /dossier/{probeId} (Phase 122h)
  //   Shift+N       toggles Negative Space overlay (structural only)
  //
  // Shell-chunk rules (enforced by tests/unit/lazy-engine.test.ts):
  //   - MUST NOT statically import three, @aer/engine-3d, or uplot.
  //   - The engine is dynamic-imported inside AtmosphereCanvas.
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import { m } from '$lib/paraglide/messages.js';
  import { formatNumber } from '$lib/localization/format';
  import { createQuery } from '@tanstack/svelte-query';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import type {
    AtmosphereEngine,
    ProbeActivity,
    ProbeMarker,
    ProbeSelection,
    SatelliteSelection
  } from '@aer/engine-3d';
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import AtmosphereChrome from '$lib/components/atmosphere/AtmosphereChrome.svelte';
  import WelcomeAmbient from '$lib/components/atmosphere/WelcomeAmbient.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import { ScopeBar } from '$lib/components/chrome';
  import { urlState, setUrl, toggleOverlay } from '$lib/state/url.svelte';
  import { markGlobeReady } from '$lib/state/boot.svelte';
  import { buildSelectionWorkbenchUrl } from '$lib/workbench/panel-queries';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import {
    metricsQuery,
    probesQuery,
    type FetchContext,
    type MetricsResponseDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    buildProbeMarkers,
    computeWindow,
    computeActivity,
    resolveFlyTo,
    buildFlatProbes,
    type FlatProbe
  } from './atmosphere-surface-internals';

  // The BFF is reachable at `/api/v1` via Traefik in every deployment.
  // Browsers authenticate via the `__Host-` session cookie (Phase 134 /
  // ADR-040); X-API-Key is machine-only and is NEVER injected by Traefik,
  // so the static bundle ships with no secret.
  const ctx: FetchContext = {
    baseUrl: '/api/v1'
  };

  let decision: 'pending' | 'engine' | 'fallback' = $state('pending');
  let shiftHeld = $state(false);
  // Phase 123a — the last-clicked probe drives the engine's shader
  // highlight + camera flyTo (tier-2 in-place selection). Distinct from
  // `selectedProbes` (the banner set) and from `?probe=`/`?dossier=` (the
  // overlay). A plain click sets the selection to just this probe; a
  // SHIFT-click grows the set. Neither opens the overlay (tier 3 is explicit).
  let activeProbeId = $state<string | null>(null);
  // Bumped on every select-click so the camera re-centers (flyTo) even when
  // the same probe is re-clicked from a rotated view (Phase 123a).
  let flyToNonce = $state(0);
  // Engine handle (captured via AtmosphereCanvas `onready`) — lets the click
  // handler ask whether the camera is already on a probe, to choose
  // re-center vs. deselect.
  let engineHandle: AtmosphereEngine | null = $state(null);

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const forceFallback = params.get('fallback') === '1';
    decision = !forceFallback && hasWebGL2() ? 'engine' : 'fallback';

    // No-WebGL2 path: there is no engine chunk to load, so the globe is "ready"
    // the moment the fallback is chosen — release the boot splash's globe gate.
    if (decision === 'fallback') markGlobeReady();

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Shift') shiftHeld = true;
    };
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.key === 'Shift') shiftHeld = false;
    };
    document.addEventListener('keydown', onKeyDown);
    document.addEventListener('keyup', onKeyUp);
    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.removeEventListener('keyup', onKeyUp);
    };
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

  // Probe → engine model (buildProbeMarkers): source names aligned positionally
  // with emission points; the marker label is the human-friendly short name.
  let probeMarkers = $derived.by<ProbeMarker[]>(() => buildProbeMarkers(probeDtos));

  // Phase 123a — stable selection object for the engine (memoised: a new
  // reference only when activeProbeId changes, so the canvas effect does not
  // re-fire flyTo on unrelated re-renders like pointer-move).
  const engineSelection = $derived<ProbeSelection | null>(
    activeProbeId ? { probeId: activeProbeId } : null
  );

  // Phase 123a — flyTo target for the active probe (its first emission point).
  // Reading `flyToNonce` makes this recompute (fresh object) on every
  // select-click, so the canvas effect re-fires flyTo to re-center even when
  // the active probe is unchanged.
  const activeFlyTo = $derived.by<{ latitude: number; longitude: number } | null>(() => {
    void flyToNonce;
    return resolveFlyTo(probeDtos, activeProbeId);
  });

  // --- Time window (URL-backed) ---------------------------------------
  const url = $derived(urlState());
  const windowMs = $derived.by(() =>
    computeWindow(url.from, url.to, Date.now(), DEFAULT_LOOKBACK_MS)
  );

  // Phase 128 — pause the GPU render loop while a full-screen overlay covers the
  // globe (Dossier / account / admin / analyses). The globe is mounted
  // persistently (Phase 135), so without this it would keep rendering hidden
  // behind the overlay, burning frame budget for nothing (Brief §10). The
  // engine also self-pauses on tab-blur via the Page Visibility API.
  const overlayOpen = $derived(
    url.dossier === 'open' ||
      url.account === 'open' ||
      url.admin === 'open' ||
      url.analyses === 'open' ||
      url.about === 'open'
  );
  $effect(() => {
    engineHandle?.setActive(!overlayOpen);
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
    return computeActivity(m.data.data, probeDtos, windowMs.hours);
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

  function onProbeSelected(sel: ProbeSelection) {
    // Phase 123a — pointer-click is in-place selection (tier 2): highlight +
    // camera flyTo + banner. It NEVER opens the overlay (tier 3 is explicit).
    const ep = probeDtos.find((d) => d.probeId === sel.probeId)?.emissionPoints[0];
    if (shiftHeld) {
      // SHIFT-click grows/toggles the selection set (the banner shows 1…N).
      const current = url.selectedProbes;
      const has = current.includes(sel.probeId);
      setUrl({
        selectedProbes: has ? current.filter((id) => id !== sel.probeId) : [...current, sel.probeId]
      });
      if (has) {
        // Removed → drop the highlight if this was the focused probe.
        if (activeProbeId === sel.probeId) activeProbeId = null;
      } else {
        activeProbeId = sel.probeId;
        flyToNonce++;
      }
      return;
    }
    // Plain click — camera-aware toggle: re-center if the camera is pointed
    // elsewhere; deselect only when it is already on this probe.
    const cameraOnIt = ep
      ? (engineHandle?.isCameraNear(ep.latitude, ep.longitude) ?? false)
      : false;
    if (activeProbeId === sel.probeId && cameraOnIt) {
      activeProbeId = null;
      setUrl({ selectedProbes: [] });
    } else {
      activeProbeId = sel.probeId;
      setUrl({ selectedProbes: [sel.probeId] });
      flyToNonce++;
    }
  }

  function onProbeHovered(sel: ProbeSelection | null) {
    hoveredProbe = sel;
    // Intent-based preload: once the user hovers a probe, warm the
    // dossier route so descent feels instantaneous. (Component path
    // unchanged — only the route URL moved in Phase 122h.)
    if (sel) {
      void import('$lib/components/dossier/ProbeCard.svelte').catch(() => void 0);
    }
  }

  function onSatelliteHovered(sel: SatelliteSelection | null) {
    hoveredSatellite = sel;
  }

  function onSatelliteSelected(sel: SatelliteSelection) {
    // Phase 123a — a source satellite belongs to its probe; clicking it
    // performs the same in-place selection as clicking the probe glyph
    // (select + flyTo + banner). Source-level scope lives in the ScopeEditor.
    onProbeSelected({ probeId: sel.probeId });
  }

  function onPointerMove(e: PointerEvent) {
    pointerX = e.clientX;
    pointerY = e.clientY;
  }

  // --- Keyboard descent grammar ---------------------------------------
  let flatProbes = $derived.by<FlatProbe[]>(() => buildFlatProbes(probeDtos));

  // Pre-resolve the hovered probe DTO so the tooltip can render the
  // semantic-register identity.
  let hoveredProbeDto = $derived(
    hoveredProbe ? (probeDtos.find((p) => p.probeId === hoveredProbe!.probeId) ?? null) : null
  );

  // Phase 135 — this surface is now rendered persistently by the (app) layout so
  // the globe never remounts. Its interactive chrome (selection banner, hover
  // tooltips, probe nav, refusal, primer link) shows ONLY on the Atmosphere
  // route; on other surfaces the canvas stays as a glassy backdrop behind the
  // page content (which overlays it, so the globe is non-interactive there).
  const onAtmosphere = $derived(page.url.pathname === '/');

  // Phase 151 — dataset quick-stats woven into the chrome (Design Brief §4.1).
  // This window reflects the DATASET, never the analysis window: counts come
  // from the probe registry + the all-time `documentCount` field (BFF), NOT the
  // windowed metrics query. It is scoped ONLY by the globe selection cart
  // (`url.selectedProbes`) — a selection shows those probes' totals, an empty
  // selection shows ALL probes. The Workbench scope and the time scrubber never
  // touch it. No fabrication: documents shows "—" when a count is unavailable,
  // and dataset age has no honest source yet, so it stays a provisional em-dash.
  const scopedProbes = $derived.by<ProbeDto[]>(() => {
    const sel = url.selectedProbes;
    if (sel.length === 0) return probeDtos;
    return probeDtos.filter((p) => sel.includes(p.probeId));
  });
  const statActiveProbes = $derived(scopedProbes.length);
  const statSources = $derived(scopedProbes.reduce((n, p) => n + (p.sources?.length ?? 0), 0));
  const statDocuments = $derived.by<number | null>(() => {
    if (scopedProbes.length === 0) return null;
    let sum = 0;
    for (const p of scopedProbes) {
      if (p.documentCount == null) return null; // unavailable → honest "—", never a fake zero
      sum += p.documentCount;
    }
    return sum;
  });
  const docsDisplay = $derived(statDocuments == null ? '—' : formatNumber(statDocuments));
</script>

<svelte:head>
  <title>{m.atmosphere_doc_title()}</title>
</svelte:head>

{#if onAtmosphere}
  <!-- Top scope bar: just the Surface · Layer chip. The dataset summary moved
       to the bottom-left quick-info window (Phase 151), and the "how to read
       the globe" primer to a bottom-right link on the globe itself. -->
  <ScopeBar label={m.atmosphere_scopebar_label()} />
{/if}

{#if decision === 'engine'}
  <div
    class="stage"
    aria-hidden="false"
    onpointermove={onPointerMove}
    onpointerleave={() => onProbeHovered(null)}
    role="presentation"
  >
    {#if onAtmosphere}
      <!-- Phase 122d.2 — honest globe-level Negative-Space disclosure, shown by
           default now (the toggle is retired) as an unobtrusive bottom-right
           note. AĒR cannot know which regions a source's discourse actually
           reaches (reach is unmeasurable), so it deliberately makes NO
           geographic blind-spot claim here. Per-source Negative Space (date
           provenance, silent edits) lives in the Dossier where it is measurable. -->
      <AtmosphereChrome
        activeProbes={formatNumber(statActiveProbes)}
        sources={formatNumber(statSources)}
        documents={docsDisplay}
      />
      <!-- First-visit ambient greeting (once per device, self-dismissing). -->
      <WelcomeAmbient />
    {/if}
    <AtmosphereCanvas
      probes={probeMarkers}
      {activity}
      {onProbeSelected}
      {onProbeHovered}
      {onSatelliteSelected}
      {onSatelliteHovered}
      onready={(e) => {
        engineHandle = e;
        // Engine mounted its first frame → the globe is interactive; clear the
        // boot splash's globe gate.
        markGlobeReady();
      }}
      selection={engineSelection}
      selectedProbeIds={url.selectedProbes}
      hover={hoveredProbe}
      flyToOnSelection={activeFlyTo}
    />

    {#if onAtmosphere && hoveredProbe}
      {@const inCompose = url.selectedProbes.includes(hoveredProbe.probeId)}
      <div
        class="probe-tooltip"
        class:in-compose={inCompose}
        role="tooltip"
        style:left="{pointerX + 14}px"
        style:top="{pointerY + 14}px"
      >
        <span class="tooltip-headline">{hoveredProbeDto?.displayName ?? hoveredProbe.probeId}</span>
        <span class="tooltip-meta">{(hoveredProbeDto?.language ?? '').toUpperCase()}</span>
        {#if inCompose}
          <span class="tooltip-compose-badge">{m.atmosphere_tooltip_in_selection()}</span>
          <span class="tooltip-affordance">{m.atmosphere_tooltip_affordance_in_selection()}</span>
        {:else if url.selectedProbes.length > 0}
          <span class="tooltip-affordance">{m.atmosphere_tooltip_affordance_add()}</span>
        {:else}
          <span class="tooltip-affordance">{m.atmosphere_tooltip_affordance_multi()}</span>
        {/if}
      </div>
    {:else if hoveredSatellite}
      <div
        class="probe-tooltip satellite"
        role="tooltip"
        style:left="{pointerX + 14}px"
        style:top="{pointerY + 14}px"
      >
        <span class="tooltip-headline">{hoveredSatellite.label}</span>
        <span class="tooltip-meta"
          >{m.atmosphere_tooltip_satellite_meta({ sourceName: hoveredSatellite.sourceName })}</span
        >
        <span class="tooltip-affordance">{m.atmosphere_tooltip_satellite_affordance()}</span>
      </div>
    {/if}

    {#if onAtmosphere}
      <ul class="sr-probe-nav" aria-label={m.atmosphere_probe_nav_label()}>
        {#each flatProbes as p (p.probeId)}
          <li>
            <button
              type="button"
              class="sr-only"
              aria-label={m.atmosphere_probe_nav_item_label({
                displayName: p.displayName,
                language: p.language
              })}
              onfocus={() => onProbeHovered({ probeId: p.probeId })}
              onblur={() => onProbeHovered(null)}
              onclick={() => onProbeSelected({ probeId: p.probeId })}
            >
              {p.displayName}
            </button>
          </li>
        {/each}
      </ul>
    {/if}

    {#if onAtmosphere && url.selectedProbes.length > 0}
      <!-- Phase 123a / 151 — bottom-center Selection Banner (tier 2). The zone
           is click-through (pointer-events:none) so the globe stays clickable;
           only the strip itself is interactive. It NEVER auto-opens the large
           overlay — "Open Dossier" is an explicit CTA (tier 3). -->
      <div class="banner-zone">
        <div
          class="atm-banner"
          role="status"
          aria-live="polite"
          aria-label={m.atmosphere_banner_label()}
        >
          <span class="atm-banner-count">
            {#if url.selectedProbes.length === 1}
              {m.atmosphere_banner_count_one({ count: formatNumber(url.selectedProbes.length) })}
            {:else}
              {m.atmosphere_banner_count_other({ count: formatNumber(url.selectedProbes.length) })}
            {/if}
          </span>
          <div class="atm-banner-actions">
            <Button
              variant="ghost"
              size="sm"
              onclick={() => {
                activeProbeId = null;
                setUrl({ selectedProbes: [] });
              }}>{m.atmosphere_banner_clear()}</Button
            >
            <Button variant="secondary" size="sm" onclick={() => toggleOverlay('dossier')}
              >{m.atmosphere_banner_open_dossier()}</Button
            >
            <Button
              variant="primary"
              size="sm"
              onclick={() => {
                // Issue 3 — carry ONLY the selection to the Workbench (no
                // pre-built pillar state). The Workbench then auto-opens the
                // ScopeEditor seeded from `?selectedProbes=`, so the user picks
                // sources rather than landing on a whole-probe panel over all
                // sources with the editor skipped.
                const qs = buildSelectionWorkbenchUrl(url.selectedProbes);
                descend(() => {
                  // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Workbench route
                  void goto(`/workbench${qs}`);
                });
              }}>{m.atmosphere_banner_open_workbench()}</Button
            >
          </div>
        </div>
      </div>
    {/if}
  </div>

  {#if onAtmosphere && probesQ.data?.kind === 'refusal'}
    <div class="refusal-slot">
      <RefusalSurface refusal={probesQ.data} {ctx} />
    </div>
  {/if}
{:else if decision === 'fallback' && onAtmosphere}
  <div class="centered">
    <WebGLFallback probes={probeDtos} {activity} loading={probesQ.isPending} />
    <!-- The globe primer stays reachable without WebGL2 (it explains the
         probe concept, not just the 3D view). Phase 151 moved the primer to
         the globe's bottom-right corner via AtmosphereChrome, which renders
         only on the engine path — so the fallback keeps its own copy here. -->
    <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Surface III primer route -->
    <a class="fallback-primer" href="/reflection/primer/globe">{m.atmosphere_primer_link()}</a>
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
    background: var(--globe-backdrop, #000);
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
    /* Issue 1 — long satellite labels (e.g. elysee) must wrap inside the
       tooltip, not overflow it. `nowrap` defeated the max-width; allow
       normal wrapping + break over-long unbroken tokens. */
    white-space: normal;
    overflow-wrap: anywhere;
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
  .probe-tooltip.in-compose {
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-surface) 88%, var(--color-accent));
  }
  .tooltip-compose-badge {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-transform: uppercase;
    letter-spacing: 0.06em;
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
  .fallback-primer {
    margin-top: var(--space-5);
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }
  .fallback-primer:hover,
  .fallback-primer:focus-visible {
    text-decoration: underline;
  }

  /* Multi-probe Selection Banner (Phase 151 design) — a solid, elevated strip
     centered at the bottom of the globe stage, bottom-aligned with the
     quick-stats window. Slighter rounding than the design's pill (radius-lg,
     matching the stats window) per operator direction. */
  .banner-zone {
    position: absolute;
    left: 0;
    right: 0;
    bottom: var(--space-6);
    z-index: 300;
    display: flex;
    justify-content: center;
    /* Click-through zone: the globe stays clickable everywhere except the
       strip itself (.atm-banner), which re-enables pointer events. */
    pointer-events: none;
  }
  .atm-banner {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: var(--space-4);
    /* Vertical padding matches the quick-stats window so the pair reads as the
       same height. */
    padding: var(--space-4) var(--space-4) var(--space-4) var(--space-5);
    background: var(--color-surface);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-lg);
    box-shadow: var(--elevation-3);
    color: var(--color-fg);
  }
  .atm-banner-count {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    white-space: nowrap;
  }
  .atm-banner-actions {
    display: flex;
    gap: var(--space-2);
  }
</style>
