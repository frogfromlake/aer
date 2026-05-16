<script lang="ts">
  // Phase 122i revision (R5) — ProbeCard.
  //
  // Container component for a single probe on the top-level Dossier
  // page. Loads the probe's Dossier DTO via TanStack Query and renders
  // the existing `ProbeDossier` component as its body. Multiple
  // ProbeCards live side-by-side on the new `/dossier` route, each
  // with its own expand/collapse state managed by `ProbeDossier`
  // internally.
  //
  // Deep-link via `?expand=<probeId>`: the page-level reader passes
  // `startCollapsed` based on whether the requested probe is this one.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import ProbeDossier from '$lib/components/lanes/ProbeDossier.svelte';

  interface Props {
    probe: ProbeDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    /** When true, the inner ProbeDossier section starts collapsed.
     *  Driven by the `/dossier?expand=<probeId>` deep-link reader. */
    startCollapsed?: boolean;
  }

  let { probe, ctx, windowStart, windowEnd, startCollapsed = false }: Props = $props();

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probe.probeId, { windowStart, windowEnd });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: probe.probeId !== ''
      };
    }
  );
</script>

<article class="probe-card" id="probe-{probe.probeId}" aria-label="Probe card {probe.probeId}">
  {#if dossierQ.isPending}
    <div class="state-slot">
      <p class="muted" aria-busy="true">Loading {probe.probeId}…</p>
    </div>
  {:else if dossierQ.isError}
    <div class="state-slot">
      <p class="error">Failed to load {probe.probeId}. Check network connectivity.</p>
    </div>
  {:else if dossierQ.data?.kind === 'success'}
    <ProbeDossier
      dossier={dossierQ.data.data}
      {ctx}
      {windowStart}
      {windowEnd}
      showIntro={false}
      {startCollapsed}
    />
  {:else if dossierQ.data?.kind === 'refusal'}
    <div class="state-slot">
      <p class="muted">Refused: {dossierQ.data.message}</p>
    </div>
  {:else if dossierQ.data?.kind === 'network-error'}
    <div class="state-slot">
      <p class="error">Network error: {dossierQ.data.message}</p>
    </div>
  {/if}
</article>

<style>
  .probe-card {
    display: contents;
  }

  .state-slot {
    display: grid;
    place-items: center;
    min-height: 6rem;
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
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
