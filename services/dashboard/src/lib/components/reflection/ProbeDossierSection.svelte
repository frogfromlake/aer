<script lang="ts">
  // ProbeDossierSection — one collapsible probe entry on the /reflection/probes
  // aggregate (Phase 127). Self-fetches the full dossier by probeId and renders
  // it through the shared ProbeDossierView; the <details> anchor id lets the
  // page TOC jump straight to it. Open by default so the reader sees every
  // probe's content at once and collapses what they don't need.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probeDossierQuery,
    type ProbeDto,
    type ProbeDossierDto,
    type QueryOutcome,
    type FetchContext
  } from '$lib/api/queries';
  import ProbeDossierView from './ProbeDossierView.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    probe: ProbeDto;
  }
  let { probe }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probe.probeId);
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );
  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );
  const failed = $derived(!dossierQ.isPending && dossierQ.data?.kind !== 'success');
</script>

<details class="agg-section" id="probe-{probe.probeId}" open>
  <summary class="agg-summary">
    <span class="agg-chevron" aria-hidden="true">▸</span>
    <span class="agg-title">{probe.displayName}</span>
    <span class="agg-tag">{probe.shortName}</span>
    <span class="agg-lang">{probe.language.toUpperCase()}</span>
  </summary>
  <div class="agg-body">
    {#if dossierQ.isPending}
      <p class="agg-state" aria-busy="true">{m.reflection_probe_loading()}</p>
    {:else if failed || !dossier}
      <p class="agg-state">{m.reflection_probe_error_body()}</p>
    {:else}
      <ProbeDossierView {dossier} />
    {/if}
  </div>
</details>

<style>
  .agg-section {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    background: color-mix(in srgb, var(--color-surface) 60%, transparent);
    overflow: hidden;
  }

  .agg-summary {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-4) var(--space-5);
    cursor: pointer;
    list-style: none;
    user-select: none;
  }

  .agg-summary::-webkit-details-marker {
    display: none;
  }

  .agg-summary:hover,
  .agg-summary:focus-visible {
    background: var(--color-surface-hover);
    outline: none;
  }

  .agg-chevron {
    color: var(--color-fg-subtle);
    font-size: var(--font-size-sm);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .agg-section[open] .agg-chevron {
    transform: rotate(90deg);
  }

  .agg-title {
    font-size: var(--font-size-base);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    flex: 1;
  }

  .agg-tag {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
  }

  .agg-lang {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
    letter-spacing: 0.05em;
  }

  .agg-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
    padding: var(--space-3) var(--space-5) var(--space-6);
    border-top: 1px solid var(--color-border);
  }

  .agg-state {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .agg-chevron {
      transition: none;
    }
  }
</style>
