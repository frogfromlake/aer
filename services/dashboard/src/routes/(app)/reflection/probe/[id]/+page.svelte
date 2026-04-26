<script lang="ts">
  // Surface III — Probe Dossier methodology view (Phase 109).
  // The methodological side of what Surface II shows structurally.
  // Fetches the Probe Dossier and renders it as a case study:
  // why these sources, what they represent, what they miss,
  // how they relate to the functional taxonomy (Design Brief §4.3).
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { ScopeBar } from '$lib/components/chrome';
  import { probeDossierQuery, type ProbeDossierDto, type QueryOutcome } from '$lib/api/queries';

  const ctx = { baseUrl: '/api/v1' };
  const probeId = $derived(page.params.id ?? '');

  const FUNCTION_META: Record<string, { label: string; description: string }> = {
    epistemic_authority: {
      label: 'Epistemic Authority',
      description:
        'Sources that define what is considered true, credible, and legitimate knowledge in a given society.'
    },
    power_legitimation: {
      label: 'Power Legitimation',
      description:
        'Sources that articulate and justify the exercise of political, economic, or social power.'
    },
    cohesion_identity: {
      label: 'Cohesion & Identity',
      description:
        'Sources that construct, reinforce, or contest collective identities and social bonds.'
    },
    subversion_friction: {
      label: 'Subversion & Friction',
      description:
        'Sources that challenge dominant narratives, amplify marginalized voices, or contest hegemonic frames.'
    }
  };

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probeId);
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );

  const dossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );

  const coveredFunctions = $derived(
    dossier?.sources.map((s) => s.primaryFunction).filter(Boolean) ?? []
  );
</script>

<svelte:head>
  <title>AĒR — Probe {probeId} · Methodology</title>
</svelte:head>

<ScopeBar label="Reflection — Probe methodology view">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">Reflection</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-label">Probe</span>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-current" aria-current="page">{probeId}</span>
</ScopeBar>

<main class="probe-main" id="main-probe-methodology">
  <div class="probe-inner">
    {#if dossierQ.isPending}
      <p class="state-msg" aria-busy="true">Loading probe dossier…</p>
    {:else if dossierQ.isError || dossierQ.data?.kind === 'network-error'}
      <div class="error-state">
        <h1>Could not load probe dossier</h1>
        <p>Connect to the AĒR backend and try again.</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection" class="back-link">← Back to Reflection</a>
      </div>
    {:else if !dossier}
      <div class="error-state">
        <h1>Probe not found</h1>
        <p>No dossier available for probe <code>{probeId}</code>.</p>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection" class="back-link">← Back to Reflection</a>
      </div>
    {:else}
      <header class="probe-header">
        <p class="probe-eyebrow">Probe Dossier — Methodology View</p>
        <h1 class="probe-title">{dossier.probeId}</h1>
        <p class="probe-sub">
          The methodological case study for this probe: why these sources, what discursive functions
          they serve, what they represent, and what they do not cover.
        </p>
      </header>

      <!-- Function coverage -->
      <section class="coverage-section" aria-labelledby="coverage-heading">
        <h2 id="coverage-heading" class="section-title">Discourse Function Coverage</h2>
        <p class="section-body">
          This probe covers <strong>{dossier.functionCoverage.covered}</strong> of
          <strong>{dossier.functionCoverage.total}</strong> discourse functions defined in
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection/wp/wp-001?section=3" class="wp-link">WP-001 §3</a>.
        </p>
        <ul class="function-list" role="list">
          {#each Object.entries(FUNCTION_META) as [key, fn] (key)}
            {@const isCovered = coveredFunctions.includes(key as (typeof coveredFunctions)[number])}
            <li class="function-item" class:covered={isCovered} class:uncovered={!isCovered}>
              <span class="function-indicator" aria-hidden="true">{isCovered ? '●' : '○'}</span>
              <div>
                <p class="function-label">{fn.label}</p>
                <p class="function-desc">{fn.description}</p>
              </div>
            </li>
          {/each}
        </ul>
      </section>

      <!-- Sources -->
      <section class="sources-section" aria-labelledby="sources-heading">
        <h2 id="sources-heading" class="section-title">Sources in this Probe</h2>
        <ul class="source-list" role="list">
          {#each dossier.sources as source (source.name)}
            <li class="source-item">
              <div class="source-head">
                <code class="source-id">{source.name}</code>
                {#if source.primaryFunction}
                  <span class="source-fn">
                    {FUNCTION_META[source.primaryFunction]?.label ?? source.primaryFunction}
                  </span>
                {/if}
              </div>
              {#if source.emicDesignation}
                <p class="source-meta">
                  <span class="meta-label">Emic designation:</span>
                  {source.emicDesignation}
                </p>
              {/if}
              {#if source.emicContext}
                <p class="source-meta">
                  <span class="meta-label">Emic context:</span>
                  {source.emicContext}
                </p>
              {/if}
              <p class="source-meta">
                <span class="meta-label">Articles in window:</span>
                {source.articlesInWindow.toLocaleString()}
              </p>
            </li>
          {/each}
        </ul>
      </section>

      <!-- Methodological context cross-links -->
      <nav class="methodology-links" aria-label="Methodological context">
        <h2 class="section-title">Methodological context</h2>
        <ul class="link-list" role="list">
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/wp-001?section=5" class="ctx-link">
              WP-001 §5 — The Probe Constellation: From Individual Sources to Systemic Observation
            </a>
          </li>
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/wp-001?section=6" class="ctx-link">
              WP-001 §6 — Probe 0 Revisited: Classification Under the Taxonomy
            </a>
          </li>
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/wp-003?section=6" class="ctx-link">
              WP-003 §6 — Demographic Skew and the Digital Divide
            </a>
          </li>
          <li>
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
            <a href="/reflection/wp/wp-006?section=5" class="ctx-link">
              WP-006 §5 — The Ethics of Making Discourse Visible
            </a>
          </li>
        </ul>
      </nav>

      <footer class="probe-footer">
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/lanes/{probeId}/dossier" class="footer-link">
          View on Surface II (Function Lanes) →
        </a>
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection" class="footer-link">← Back to Reflection</a>
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
    background: var(--color-bg);
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
    margin: 0 0 var(--space-3);
    line-height: var(--line-height-tight);
    font-family: var(--font-mono);
  }

  .probe-sub {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
  }

  .section-body {
    font-size: var(--font-size-base);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
    margin: 0 0 var(--space-4);
  }

  .wp-link {
    color: var(--color-accent);
    font-family: var(--font-mono);
    font-size: 0.875em;
    text-decoration: none;
  }

  .wp-link:hover {
    text-decoration: underline;
  }

  /* Coverage */
  .function-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .function-item {
    display: flex;
    gap: var(--space-3);
    padding: var(--space-3);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    align-items: flex-start;
  }

  .function-item.covered {
    background: rgba(126, 196, 160, 0.08);
    border-color: rgba(126, 196, 160, 0.3);
  }

  .function-item.uncovered {
    opacity: 0.55;
  }

  .function-indicator {
    font-size: var(--font-size-xs);
    color: var(--color-status-validated);
    flex-shrink: 0;
    padding-top: 2px;
  }

  .function-item.uncovered .function-indicator {
    color: var(--color-border-strong);
  }

  .function-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0 0 var(--space-1);
  }

  .function-desc {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0;
    line-height: var(--line-height-loose);
  }

  /* Sources */
  .source-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .source-item {
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .source-head {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-fn {
    font-size: var(--font-size-xs);
    color: var(--color-accent-muted);
    padding: 1px var(--space-2);
    border: 1px solid var(--color-accent-muted);
    border-radius: var(--radius-pill);
  }

  .source-meta {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .meta-label {
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
    margin-right: var(--space-1);
  }

  /* Context links */
  .link-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .ctx-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .ctx-link:hover {
    text-decoration: underline;
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

  .back-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
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
