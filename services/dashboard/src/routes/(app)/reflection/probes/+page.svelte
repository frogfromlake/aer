<script lang="ts">
  // Surface III — Probe dossiers aggregate (Phase 127). Every probe's
  // methodological dossier on one page: a TOC sidebar of probes (same pattern as
  // the Working Paper reader), a sticky back control, and one expand/collapse
  // section per probe (open by default). Replaces the per-probe tiles on the
  // Reflection landing; the singular /reflection/probe/[id] page still backs
  // inline links elsewhere in the app.
  import { createQuery } from '@tanstack/svelte-query';
  import { ScopeBar } from '$lib/components/chrome';
  import ReflectionToc from '$lib/components/reflection/ReflectionToc.svelte';
  import ReflectionBackLink from '$lib/components/reflection/ReflectionBackLink.svelte';
  import ProbeDossierSection from '$lib/components/reflection/ProbeDossierSection.svelte';
  import {
    probesQuery,
    type ProbeDto,
    type QueryOutcome,
    type FetchContext
  } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probes = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);
  const failed = $derived(!probesQ.isPending && probesQ.data?.kind !== 'success');

  const tocItems = $derived(
    probes.map((p) => ({ id: `probe-${p.probeId}`, label: p.displayName }))
  );
</script>

<svelte:head>
  <title>{m.reflection_probes_head_title()}</title>
</svelte:head>

<ScopeBar label={m.reflection_probes_scopebar_label()}>
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">{m.reflection_probe_breadcrumb_root()}</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page"
    >{m.reflection_probes_breadcrumb_current()}</span
  >
</ScopeBar>

<div class="agg-layout" class:no-toc={tocItems.length === 0} id="main-reflection-probes">
  {#if tocItems.length > 0}
    <ReflectionToc items={tocItems} />
  {/if}
  <div class="agg-scroll">
    <ReflectionBackLink />
    <div class="agg-inner">
      <header class="agg-header">
        <p class="agg-eyebrow">{m.reflection_probes_eyebrow()}</p>
        <h1 class="agg-h1">{m.reflection_probes_heading()}</h1>
        <p class="agg-lede">{m.reflection_probes_sub()}</p>
      </header>

      {#if probesQ.isPending}
        <p class="agg-state" aria-busy="true">{m.reflection_landing_probes_loading()}</p>
      {:else if failed}
        <p class="agg-state">{m.reflection_landing_probes_error()}</p>
      {:else if probes.length === 0}
        <p class="agg-state">{m.reflection_landing_probes_empty()}</p>
      {:else}
        <div class="agg-list">
          {#each probes as p (p.probeId)}
            <ProbeDossierSection probe={p} />
          {/each}
        </div>
      {/if}
    </div>
  </div>
</div>

<style>
  .agg-layout {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    display: grid;
    grid-template-columns: 220px 1fr;
    overflow: hidden;
    background: color-mix(in srgb, var(--color-bg) 72%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
  }

  .agg-layout.no-toc {
    grid-template-columns: 1fr;
  }

  @media (max-width: 900px) {
    .agg-layout {
      grid-template-columns: 1fr;
    }
  }

  .agg-scroll {
    overflow-y: auto;
  }

  .agg-inner {
    max-width: 74ch;
    margin: 0 auto;
    padding: var(--space-6) var(--space-6) var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .agg-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
  }

  .agg-h1 {
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0 0 var(--space-3);
    line-height: var(--line-height-tight);
  }

  .agg-lede {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .agg-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .agg-state {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .breadcrumb-root {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    text-decoration: none;
    transition: color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .breadcrumb-root:hover,
  .breadcrumb-root:focus-visible {
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .breadcrumb-sep {
    font-size: var(--font-size-xs);
    color: var(--color-border-strong);
  }

  .breadcrumb-current {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }

  @media (prefers-reduced-motion: reduce) {
    .breadcrumb-root {
      transition: none;
    }
  }
</style>
