<script lang="ts">
  // Surface III — Metric provenance page (Phase 109).
  // Long-form rendering of a metric's algorithm, lexicon, training data,
  // validation status, known limitations, and WP cross-references — via the
  // shared MetricProvenanceView (Phase 127 — same body as /reflection/metrics).
  import { createQuery } from '@tanstack/svelte-query';
  import { page } from '$app/state';
  import { ScopeBar } from '$lib/components/chrome';
  import Badge from '$lib/components/base/Badge.svelte';
  import {
    provenanceQuery,
    contentQuery,
    type MetricProvenanceDto,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { pickBadgeTier } from '$lib/components/chrome/methodology-tray-internals';
  import MetricProvenanceView from '$lib/components/reflection/MetricProvenanceView.svelte';
  import ReflectionBackLink from '$lib/components/reflection/ReflectionBackLink.svelte';
  import { locale } from '$lib/state/locale.svelte';
  import { m } from '$lib/paraglide/messages.js';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const metricName = $derived(page.params.name ?? '');

  const provQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const o = provenanceQuery(ctx, metricName);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: metricName.length > 0
    };
  });

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', metricName, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: metricName.length > 0
    };
  });

  const provenance = $derived<MetricProvenanceDto | null>(
    provQ.data?.kind === 'success' ? provQ.data.data : null
  );
  const contentRecord = $derived<ContentResponseDto | null>(
    contentQ.data?.kind === 'success' ? contentQ.data.data : null
  );

  const badgeTier = $derived(pickBadgeTier(provenance));
  const isPending = $derived(provQ.isPending || contentQ.isPending);
</script>

<svelte:head>
  <title>{m.reflection_metric_head_title({ metricName })}</title>
</svelte:head>

<ScopeBar label={m.reflection_metric_scopebar_label()}>
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">{m.reflection_metric_breadcrumb_root()}</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-label">{m.reflection_metric_breadcrumb_label()}</span>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <code class="breadcrumb-current" aria-current="page">{metricName}</code>
</ScopeBar>

<main class="metric-main" id="main-metric-provenance">
  <ReflectionBackLink />
  <div class="metric-inner">
    {#if isPending}
      <p class="state-msg" aria-busy="true">{m.reflection_metric_loading()}</p>
    {:else}
      <header class="metric-header">
        <p class="metric-eyebrow">{m.reflection_metric_eyebrow()}</p>
        <div class="metric-title-row">
          <h1 class="metric-title"><code>{metricName}</code></h1>
          <Badge tier={badgeTier} />
        </div>
        <p class="metric-sub">
          {m.reflection_metric_sub_pre()}
          <span class="tray-note">{m.reflection_metric_sub_tray()}</span>
          {m.reflection_metric_sub_post()}
        </p>
      </header>

      {#if !provenance && !contentRecord}
        <div class="unavailable">
          <p>
            {m.reflection_metric_unavailable_pre()} <code>{metricName}</code>
            {m.reflection_metric_unavailable_post()}
          </p>
        </div>
      {:else}
        <MetricProvenanceView {provenance} {contentRecord} />
      {/if}

      <footer class="metric-footer">
        <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
        <a href="/reflection" class="footer-link">{m.reflection_metric_back()}</a>
      </footer>
    {/if}
  </div>
</main>

<style>
  .metric-main {
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

  .metric-inner {
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

  .metric-eyebrow {
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    margin: 0 0 var(--space-2);
    font-family: var(--font-mono);
  }

  .metric-title-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-bottom: var(--space-3);
  }

  .metric-title {
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
    line-height: var(--line-height-tight);
  }

  .metric-title code {
    font-family: var(--font-mono);
  }

  .metric-sub {
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .tray-note {
    color: var(--color-accent);
    text-decoration: none;
    cursor: default;
    border-bottom: 1px dashed var(--color-accent-muted);
  }

  .unavailable {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .metric-footer {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
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

  .breadcrumb-sep,
  .breadcrumb-label {
    font-size: var(--font-size-xs);
    color: var(--color-border-strong);
  }

  .breadcrumb-current {
    font-family: var(--font-mono);
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
