<script lang="ts">
  // Atmosphere route (Phase 99b): full-bleed 3D globe with live probe data.
  //
  // Shell-chunk rules (enforced by tests/unit/lazy-engine.test.ts):
  //   - MUST NOT statically import three or @aer/engine-3d (except
  //     /capability, which is side-effect free).
  //   - The engine is dynamic-imported inside AtmosphereCanvas.
  //
  // Data flow:
  //   /api/v1/probes             → engine.setProbes(...)
  //   /api/v1/metrics            → per-probe docs/hour → engine.setActivity(...)
  //                                (display-logic mapping; no data transform
  //                                 beyond summation and division)
  //   click → /api/v1/content/probe/{id} → SidePanel + ProgressiveSemantics
  //   400 on any query → RefusalSurface via the QueryOutcome union
  import { onMount } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import type { ProbeActivity, ProbeMarker, ProbeSelection } from '@aer/engine-3d';
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import TimeScrubber from '$lib/components/TimeScrubber.svelte';
  import { SidePanel } from '$lib/components/base';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import {
    contentQuery,
    metricsQuery,
    probesQuery,
    type ContentResponseDto,
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
  // The /metrics query is parameterised on the URL-persisted time range.
  // When no URL params are present we fall back to a 24 h window ending
  // now so a cold page load still renders activity.
  const url = $derived(urlState());
  const DEFAULT_WINDOW_MS = 24 * 60 * 60 * 1000;
  const windowMs = $derived.by<{ start: string; end: string; hours: number }>(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_WINDOW_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    const safeFrom = Number.isFinite(fromMs) ? fromMs : now - DEFAULT_WINDOW_MS;
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
      perSource[row.source] = (perSource[row.source] ?? 0) + row.value;
    }
    return probeDtos.map((p) => {
      const total = p.sources.reduce((sum, s) => sum + (perSource[s] ?? 0), 0);
      return { probeId: p.probeId, documentsPerHour: total / windowMs.hours };
    });
  });

  // --- Selection -------------------------------------------------------
  let selected: ProbeSelection | null = $state(null);
  let panelOpen = $state(false);

  // If the URL carries `?probe=<id>` on load, surface the panel for that
  // probe once probes land. Emission-point index is not URL-encoded in
  // 99b (only the probe is deep-linkable), so we open on point index 0.
  $effect(() => {
    if (!url.probe || selected) return;
    const hit = probeDtos.find((p) => p.probeId === url.probe);
    const firstEp = hit?.emissionPoints[0];
    if (!hit || !firstEp) return;
    selected = {
      probeId: hit.probeId,
      emissionPointIndex: 0,
      emissionPointLabel: firstEp.label
    };
    panelOpen = true;
  });

  function onProbeSelected(sel: ProbeSelection) {
    selected = sel;
    panelOpen = true;
    setUrl({ probe: sel.probeId });
  }

  function onPanelClose() {
    panelOpen = false;
    setUrl({ probe: null });
  }

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const probeId = selected?.probeId ?? '';
    const o = contentQuery(ctx, 'probe', probeId, 'en');
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: selected !== null
    };
  });
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>

{#if decision === 'engine'}
  <div class="stage" aria-hidden="false">
    <AtmosphereCanvas probes={probeMarkers} {activity} {onProbeSelected} />
  </div>

  <div class="scrubber-slot">
    <TimeScrubber />
  </div>

  <SidePanel
    bind:open={panelOpen}
    title={selected?.emissionPointLabel ?? 'Probe'}
    onClose={onPanelClose}
  >
    {#if selected}
      <section class="probe-meta" aria-label="Probe metadata">
        <dl>
          <dt>Probe</dt>
          <dd><code>{selected.probeId}</code></dd>
          <dt>Emission point</dt>
          <dd>{selected.emissionPointLabel}</dd>
        </dl>
      </section>

      {#if contentQ.isPending}
        <p class="muted" aria-busy="true">Loading probe content…</p>
      {:else if contentQ.data?.kind === 'success'}
        <ProgressiveSemantics registers={contentQ.data.data.registers} emphasis="semantic" />
      {:else if contentQ.data?.kind === 'refusal'}
        <RefusalSurface refusal={contentQ.data} {ctx} />
      {:else if contentQ.isError}
        <p class="muted">Content Catalog unavailable.</p>
      {/if}

      <p class="reach-note">
        Reach is not rendered. This probe's emission points mark where its bound publishers emit —
        not where their content is consumed or influential. No reach claim is made by AĒR (Design
        Brief §5.7).
      </p>
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

<style>
  .stage {
    position: fixed;
    inset: 0;
    background: #000;
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
  .scrubber-slot {
    position: fixed;
    bottom: var(--space-5);
    left: 50%;
    transform: translateX(-50%);
    width: min(90vw, 36rem);
    z-index: 400;
  }
  .probe-meta {
    margin-bottom: var(--space-5);
  }
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--font-size-sm);
  }
  dt {
    color: var(--color-fg-muted);
  }
  dd {
    margin: 0;
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
  }
  .reach-note {
    margin-top: var(--space-5);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    line-height: 1.55;
  }
</style>
