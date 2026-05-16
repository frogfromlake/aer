<script lang="ts">
  // Phase 122i revision (R5) — Dossier Home (top-level surface).
  //
  // The Dossier is now a first-class top-level surface (ADR-033
  // amendment): one page that hosts the entire probe catalogue plus
  // AĒR's most powerful entry into the Workbench — General Free-
  // Compose, composing across probes.
  //
  // Page structure (top to bottom):
  //   1. Header strip — identity + orientation copy.
  //   2. GeneralFreeComposeSection — cross-probe composer.
  //   3. ProbeCard list — one card per probe in the catalogue.
  //
  // Deep-link via `?expand=<probeId>`: the named probe's card starts
  // expanded; others start collapsed. Reaching this route via the
  // Atmosphere globe-click (which writes `?expand=<probeId>`) puts
  // the user's probe of interest in focus while keeping the rest of
  // the catalogue accessible.
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import { DEFAULT_LOOKBACK_MS } from '$lib/state/url-internals';
  import ProbeCard from '$lib/components/dossier/ProbeCard.svelte';
  import GeneralFreeComposeSection from '$lib/components/dossier/GeneralFreeComposeSection.svelte';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  const expandedProbeId = $derived(page.url.searchParams.get('expand') ?? '');

  const windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
    };
  });

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  // Phase 122i revision (R5). If the user landed via `?expand=<id>` the
  // named probe's card starts expanded; others collapse. If no expand
  // hint is given AND the catalogue has only one probe, that probe
  // starts expanded by default (the common single-probe production
  // case).
  function startCollapsedFor(probeId: string): boolean {
    if (expandedProbeId && expandedProbeId !== probeId) return true;
    if (!expandedProbeId && probeList.length > 1) return true;
    return false;
  }
</script>

<svelte:head>
  <title>AĒR — Dossier</title>
</svelte:head>

<main class="dossier-main" id="main-dossier">
  <header class="dossier-header">
    <p class="dossier-eyebrow">Dossier</p>
    <h1 class="dossier-title">Atmospheric record of AĒR's probes</h1>
    <p class="dossier-lede">
      The complete catalogue of probes AĒR observes. Inside each card: sources, function coverage,
      and probe-internal composition. Above the catalogue: General Free-Compose, the only way to
      compose across probes.
    </p>
  </header>

  <GeneralFreeComposeSection {ctx} windowStart={windowMs.start} windowEnd={windowMs.end} />

  <section class="probes-section" aria-label="Probe catalogue">
    <header class="probes-section-header">
      <h2>Probes</h2>
    </header>

    {#if probesQ.isPending}
      <p class="muted" aria-busy="true">Loading probe catalogue…</p>
    {:else if probesQ.isError || probesQ.data?.kind === 'network-error'}
      <p class="error">Could not load the probe catalogue. Check network connectivity.</p>
    {:else if probeList.length === 0}
      <p class="muted">No probes in the catalogue yet.</p>
    {:else}
      <div class="probe-cards">
        {#each probeList as probe (probe.probeId)}
          <ProbeCard
            {probe}
            {ctx}
            windowStart={windowMs.start}
            windowEnd={windowMs.end}
            startCollapsed={startCollapsedFor(probe.probeId)}
          />
        {/each}
      </div>
    {/if}
  </section>
</main>

<style>
  .dossier-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: 0;
    right: 0;
    overflow-y: auto;
    background: var(--color-bg);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  .dossier-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-width: 60rem;
  }

  .dossier-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0;
  }

  .dossier-title {
    margin: 0;
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    line-height: 1.2;
  }

  .dossier-lede {
    margin: 0;
    font-size: var(--font-size-md);
    color: var(--color-fg-muted);
    line-height: 1.45;
  }

  .probes-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .probes-section-header h2 {
    margin: 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }

  .probe-cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .error {
    font-size: var(--font-size-sm);
    color: var(--color-status-expired);
    margin: 0;
  }
</style>
