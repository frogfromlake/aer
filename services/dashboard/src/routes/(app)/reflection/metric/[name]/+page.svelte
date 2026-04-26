<script lang="ts">
  // Surface III — Metric provenance page (Phase 109).
  // Long-form rendering of a metric's algorithm, lexicon, training data,
  // validation status, known limitations, and WP cross-references.
  // This is what the methodology tray summarises in its open state.
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
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';

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
    const o = contentQuery(ctx, 'metric', metricName, 'en');
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
  <title>AĒR — {metricName} · Provenance</title>
</svelte:head>

<ScopeBar label="Reflection — Metric provenance navigation">
  <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
  <a href="/reflection" class="breadcrumb-root">Reflection</a>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <span class="breadcrumb-label">Metric</span>
  <span class="breadcrumb-sep" aria-hidden="true">›</span>
  <code class="breadcrumb-current" aria-current="page">{metricName}</code>
</ScopeBar>

<main class="metric-main" id="main-metric-provenance">
  <div class="metric-inner">
    {#if isPending}
      <p class="state-msg" aria-busy="true">Loading metric provenance…</p>
    {:else}
      <header class="metric-header">
        <p class="metric-eyebrow">Metric Provenance</p>
        <div class="metric-title-row">
          <h1 class="metric-title"><code>{metricName}</code></h1>
          <Badge tier={badgeTier} />
        </div>
        <p class="metric-sub">
          The full provenance record for this metric: algorithm, validation status, known
          limitations, and Working Paper cross-references. The
          <span class="tray-note">Methodology Tray</span> shows a summary of this content when the metric
          is focused in any chart.
        </p>
      </header>

      {#if !provenance && !contentRecord}
        <div class="unavailable">
          <p>
            Provenance data for <code>{metricName}</code> is not available. Connect to the AĒR backend
            or check that this metric name is correct.
          </p>
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection" class="back-link">← Back to Reflection</a>
        </div>
      {:else}
        <!-- Dual-register content -->
        {#if contentRecord}
          <section class="prov-section" aria-labelledby="registers-heading">
            <h2 id="registers-heading" class="section-title">What this metric measures</h2>
            <ProgressiveSemantics registers={contentRecord.registers} emphasis="methodological" />
          </section>
        {/if}

        <!-- Provenance details -->
        {#if provenance}
          <section class="prov-section" aria-labelledby="prov-heading">
            <h2 id="prov-heading" class="section-title">Provenance details</h2>
            <dl class="prov-dl">
              <dt>Tier classification</dt>
              <dd><Badge tier={badgeTier} /> Tier {provenance.tierClassification}</dd>

              <dt>Validation status</dt>
              <dd class="status-{provenance.validationStatus}">{provenance.validationStatus}</dd>

              <dt>Algorithm</dt>
              <dd>{provenance.algorithmDescription}</dd>

              <dt>Extractor version</dt>
              <dd><code class="mono">{provenance.extractorVersionHash}</code></dd>

              {#if provenance.culturalContextNotes}
                <dt>Cultural context</dt>
                <dd>{provenance.culturalContextNotes}</dd>
              {/if}
            </dl>
          </section>

          {#if provenance.knownLimitations.length > 0}
            <section class="prov-section limitations-section" aria-labelledby="limits-heading">
              <h2 id="limits-heading" class="section-title">Known limitations</h2>
              <ul class="limits-list">
                {#each provenance.knownLimitations as lim (lim)}
                  <li>{lim}</li>
                {/each}
              </ul>
            </section>
          {/if}
        {/if}

        <!-- WP cross-references from content catalog -->
        {#if contentRecord?.workingPaperAnchors && contentRecord.workingPaperAnchors.length > 0}
          <section class="prov-section" aria-labelledby="wp-refs-heading">
            <h2 id="wp-refs-heading" class="section-title">Working Paper references</h2>
            <ul class="wp-ref-list" role="list">
              {#each contentRecord.workingPaperAnchors as anchor (anchor)}
                {@const parts = anchor.match(/^(WP-\d+)\s*§?\s*(.*)$/i)}
                {#if parts}
                  {@const wpId = (parts[1] ?? '').toLowerCase()}
                  {@const section = (parts[2] ?? '').trim()}
                  <li>
                    <!-- eslint-disable svelte/no-navigation-without-resolve -->
                    <a
                      href="/reflection/wp/{wpId}{section
                        ? `?section=${encodeURIComponent(section)}`
                        : ''}"
                      class="wp-ref-link">{anchor}</a
                    >
                    <!-- eslint-enable svelte/no-navigation-without-resolve -->
                  </li>
                {:else}
                  <li class="wp-ref-raw">{anchor}</li>
                {/if}
              {/each}
            </ul>
          </section>
        {/if}

        <footer class="metric-footer">
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
          <a href="/reflection" class="footer-link">← Back to Reflection</a>
        </footer>
      {/if}
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
    background: var(--color-bg);
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

  .prov-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .section-title {
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    margin: 0;
  }

  /* Provenance DL */
  .prov-dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-5);
    font-size: var(--font-size-sm);
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .prov-dl dt {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }

  .prov-dl dd {
    margin: 0;
    color: var(--color-fg);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }

  .mono {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
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

  /* Known limitations */
  .limitations-section {
    padding: var(--space-4);
    background: rgba(192, 96, 96, 0.06);
    border: 1px solid rgba(192, 96, 96, 0.2);
    border-radius: var(--radius-md);
  }

  .limits-list {
    margin: 0;
    padding-left: var(--space-5);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .limits-list li {
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    color: var(--color-fg);
  }

  /* WP refs */
  .wp-ref-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .wp-ref-link {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .wp-ref-link:hover {
    text-decoration: underline;
  }

  .wp-ref-raw {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .metric-footer {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-5);
  }

  .footer-link,
  .back-link {
    font-size: var(--font-size-sm);
    color: var(--color-accent);
    text-decoration: none;
  }

  .footer-link:hover,
  .back-link:hover {
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
