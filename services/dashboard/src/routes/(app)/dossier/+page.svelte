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
  import { goto } from '$app/navigation';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { urlState } from '$lib/state/url.svelte';
  import ProbeCard from '$lib/components/dossier/ProbeCard.svelte';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  const expandedProbeId = $derived(page.url.searchParams.get('expand') ?? '');

  // Phase 131a — Dossier default window is **the whole dataset**, not a
  // rolling 7-day lookback. Rationale: the rolling-7d default produced a
  // confusing "245 in window / 288 total" split that an unprompted user
  // (or the operator returning after a week) could not interpret. Now:
  // without explicit `?from=` / `?to=` URL params the cell passes
  // `undefined` to ProbeCard → the BFF treats absent bounds as
  // "no filter" → `in_window == total` and the per-day rate falls back
  // to the long-run average over the published-date span (already
  // implemented in `dossier_store.go::fetchSourceCounts`). A
  // user-controllable date-range picker lands with Phase 123a.
  const windowMs = $derived.by<{ start: string | undefined; end: string | undefined }>(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : NaN;
    const toMs = url.to ? Date.parse(url.to) : NaN;
    return {
      start: Number.isFinite(fromMs) ? new Date(fromMs).toISOString() : undefined,
      end: Number.isFinite(toMs)
        ? new Date(toMs).toISOString()
        : url.from
          ? new Date(now).toISOString()
          : undefined
    };
  });

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  // Phase 122k K2 — the dossier is now a pure catalogue. When a probe
  // selection exists (set via Atmos SHIFT-click or the Probe-Filter
  // Modal), the catalogue auto-filters and auto-expands the selected
  // probes. Otherwise: a single probe starts expanded (the common
  // single-probe production case), and a deep-link `?expand=<id>` takes
  // precedence over both behaviours.
  function visibleProbeIds(): string[] {
    if (url.selectedProbes.length > 0) return [...url.selectedProbes];
    return probeList.map((p) => p.probeId);
  }
  function startCollapsedFor(probeId: string): boolean {
    // Phase 122k — direct Dossier navigation lands with ALL probes
    // collapsed regardless of catalogue size. Deep-link via `?expand=<id>`
    // takes precedence; `?selectedProbes=` auto-expands the selected
    // probes. Plain `/dossier` is read-only catalogue browsing.
    if (expandedProbeId === probeId) return false;
    if (expandedProbeId) return true;
    if (url.selectedProbes.length > 0) return !url.selectedProbes.includes(probeId);
    return true;
  }
  const visibleProbes = $derived<ProbeDto[]>(
    probeList.filter((p) => visibleProbeIds().includes(p.probeId))
  );

  function openWorkbench() {
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Workbench route
    void goto('/workbench');
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
      metadata-coverage matrix. To analyse, open the Workbench's ScopeEditor — pre-seeded from your
      probe selection when present.
    </p>
  </header>

  <!-- Phase 122k F1 — Top-banner CTA. Single Workbench entry; selection
       configuration happens on the Atmosphere (SHIFT-click) or via the
       Probe-Filter Modal in the SideRail (K3). -->
  <section class="banner" aria-labelledby="banner-heading">
    <div class="banner-content">
      <h2 id="banner-heading">Browse the catalogue · analyse in the Workbench</h2>
      <p class="lede">
        {#if url.selectedProbes.length > 0}
          {url.selectedProbes.length} probe{url.selectedProbes.length === 1 ? '' : 's'} selected. The
          Workbench will open with the ScopeEditor pre-seeded.
        {:else}
          Configure a scope in the Workbench's ScopeEditor. Sources, probes, and discourse-function
          restrictions are all editable there.
        {/if}
      </p>
    </div>
    <button type="button" class="cta" onclick={openWorkbench}>Open Workbench →</button>
  </section>

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
    {:else if visibleProbes.length === 0}
      <p class="muted">
        No probes match the current selection. Adjust your selection on the Atmosphere or in the
        Probe-Filter modal.
      </p>
    {:else}
      <div class="probe-cards">
        {#each visibleProbes as probe (probe.probeId)}
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

  .banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-accent);
    border-radius: var(--radius-md);
  }
  .banner-content {
    flex: 1 1 auto;
  }
  .banner-content h2 {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-lg);
    color: var(--color-fg);
  }
  .lede {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    max-width: 48rem;
  }
  .cta {
    appearance: none;
    background: var(--color-accent);
    color: var(--color-bg);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-4);
    font-size: var(--font-size-sm);
    font-weight: 600;
    cursor: pointer;
    flex-shrink: 0;
  }
  .cta:hover,
  .cta:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, var(--color-fg));
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
