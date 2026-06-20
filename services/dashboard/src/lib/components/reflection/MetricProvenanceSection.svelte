<script lang="ts">
  // MetricProvenanceSection — one collapsible metric entry on the
  // /reflection/metrics aggregate (Phase 127). Self-fetches provenance + dual
  // register content by metric name and renders them through the shared
  // MetricProvenanceView; the <details> anchor id lets the page TOC jump to it.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    provenanceQuery,
    contentQuery,
    type AvailableMetricDto,
    type MetricProvenanceDto,
    type ContentResponseDto,
    type QueryOutcome,
    type FetchContext
  } from '$lib/api/queries';
  import MetricProvenanceView from './MetricProvenanceView.svelte';
  import { locale } from '$lib/state/locale.svelte';
  import { metricLabel } from '$lib/state/labels.svelte';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    metric: AvailableMetricDto;
  }
  let { metric }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const provQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const o = provenanceQuery(ctx, metric.metricName);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', metric.metricName, locale());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const provenance = $derived<MetricProvenanceDto | null>(
    provQ.data?.kind === 'success' ? provQ.data.data : null
  );
  const contentRecord = $derived<ContentResponseDto | null>(
    contentQ.data?.kind === 'success' ? contentQ.data.data : null
  );
  const isPending = $derived(provQ.isPending || contentQ.isPending);
</script>

<details class="agg-section" id="metric-{metric.metricName}" open>
  <summary class="agg-summary">
    <span class="agg-chevron" aria-hidden="true">▸</span>
    <span class="agg-title">{metricLabel(metric.metricName)}</span>
    <code class="agg-machine" title={metric.metricName}>{metric.metricName}</code>
    <span class="agg-status status-{metric.validationStatus}">{metric.validationStatus}</span>
  </summary>
  <div class="agg-body">
    {#if isPending}
      <p class="agg-state" aria-busy="true">{m.reflection_metric_loading()}</p>
    {:else if !provenance && !contentRecord}
      <p class="agg-state">
        {m.reflection_metric_unavailable_pre()}
        <code>{metric.metricName}</code>
        {m.reflection_metric_unavailable_post()}
      </p>
    {:else}
      <MetricProvenanceView {provenance} {contentRecord} />
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
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }

  /* Task B — the machine name kept as a muted technical reference beside the
     friendly title (this is a provenance/transparency surface). */
  .agg-machine {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .agg-status {
    font-size: var(--font-size-xs);
    font-style: italic;
    white-space: nowrap;
    margin-left: auto;
  }

  .status-unvalidated {
    color: var(--color-status-unvalidated);
  }
  .status-validated {
    color: var(--color-status-validated);
  }
  .status-expired {
    color: var(--color-status-expired);
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
