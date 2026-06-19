<script lang="ts">
  // Surface III — Probe Dossier methodology view (Phase 109).
  // The methodological side of what Surface II shows structurally.
  // Fetches the Probe Dossier and renders it as a case study via the shared
  // ProbeDossierView (Phase 127 — same body as the /reflection/probes aggregate).
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { ScopeBar } from '$lib/components/chrome';
  import { probeDossierQuery, type ProbeDossierDto, type QueryOutcome } from '$lib/api/queries';
  import ProbeDossierView from '$lib/components/reflection/ProbeDossierView.svelte';
  import ReflectionBackLink from '$lib/components/reflection/ReflectionBackLink.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ctx = { baseUrl: '/api/v1' };
  const probeId = $derived(page.params.id ?? '');

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probeId);
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );

  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );
</script>

<svelte:head>
  <title>{m.reflection_probe_head_title({ probeId })}</title>
</svelte:head>

<ScopeBar label={m.reflection_probe_scopebar_label()}>
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">{m.reflection_probe_breadcrumb_root()}</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-label">{m.reflection_probe_breadcrumb_label()}</span>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page">{probeId}</span>
</ScopeBar>

<main class="probe-main" id="main-probe-methodology">
  <ReflectionBackLink />
  <div class="probe-inner">
    {#if dossierQ.isPending}
      <p class="state-msg" aria-busy="true">{m.reflection_probe_loading()}</p>
    {:else if dossierQ.isError || dossierQ.data?.kind === 'network-error'}
      <div class="error-state">
        <h1>{m.reflection_probe_error_title()}</h1>
        <p>{m.reflection_probe_error_body()}</p>
      </div>
    {:else if !dossier}
      <div class="error-state">
        <h1>{m.reflection_probe_notfound_title()}</h1>
        <p>{m.reflection_probe_notfound_body_pre()} <code>{probeId}</code>.</p>
      </div>
    {:else}
      <header class="probe-header">
        <p class="probe-eyebrow">{m.reflection_probe_eyebrow()}</p>
        <h1 class="probe-title">{dossier.displayName}</h1>
        <code class="probe-id-sub">{dossier.probeId}</code>
      </header>

      <ProbeDossierView {dossier} />

      <footer class="probe-footer">
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/?dossier=open&selectedProbes={probeId}" class="footer-link">
          {m.reflection_probe_open_dossier()}
        </a>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection" class="footer-link">{m.reflection_probe_back()}</a>
      </footer>
    {/if}
  </div>
</main>

<style>
  .probe-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    overflow-y: auto;
    background: color-mix(in srgb, var(--color-bg) 72%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
  }

  .probe-inner {
    max-width: 66ch;
    margin: 0 auto;
    padding: var(--space-7) var(--space-6) var(--space-9);
    display: flex;
    flex-direction: column;
    gap: var(--space-7);
  }

  .state-msg {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .error-state {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .probe-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
  }

  .probe-title {
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    letter-spacing: var(--letter-spacing-tight);
    color: var(--color-fg);
    margin: 0 0 var(--space-1);
    line-height: var(--line-height-tight);
  }

  /* Phase 123 — machine probeId as a muted subtitle under the display name. */
  .probe-id-sub {
    display: block;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0;
  }

  /* Footer */
  .probe-footer {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: var(--space-3);
    padding-top: var(--space-5);
    border-top: 1px solid var(--color-border);
  }

  .footer-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .footer-link:hover {
    text-decoration: underline;
  }

  /* ScopeBar */
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

  .breadcrumb-label {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .breadcrumb-current {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }

  @media (prefers-reduced-motion: reduce) {
    .breadcrumb-root {
      transition: none;
    }
  }
</style>
