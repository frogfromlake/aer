<script lang="ts">
  // L4 Provenance fly-out (Design Brief §5.7 — "the methodological
  // register becomes primary at Layer 4"). Opens from the L3 "why this
  // shape?" affordance and renders /metrics/{name}/provenance + the
  // methodological register of the matching Content Catalog metric
  // entry. The fly-out sits on top of L3 without removing it — the
  // chart remains visible behind the overlay, consistent with §4.1
  // rule 2 ("no layer replaces").
  //
  // The fly-out is explicitly *not* URL-addressable (see url-internals.ts
  // `ViewLayer` comment): it is a transient disclosure within L3, not a
  // descent step the user expects Back to resolve.
  import { createQuery } from '@tanstack/svelte-query';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import {
    contentQuery,
    provenanceQuery,
    type ContentResponseDto,
    type FetchContext,
    type MetricProvenanceDto,
    type QueryOutcome
  } from '$lib/api/queries';

  interface Props {
    metricName: string;
    ctx: FetchContext;
    onClose: () => void;
  }

  let { metricName, ctx, onClose }: Props = $props();

  // Snake-case metric ids (e.g. `sentiment_score`) are an internal wire
  // shape, not a display string. Convert to Title Case for the heading
  // while keeping the raw id accessible for debugging/copy-paste.
  let metricTitle = $derived(
    metricName
      .split('_')
      .filter((s) => s.length > 0)
      .map((s) => s[0]!.toUpperCase() + s.slice(1))
      .join(' ')
  );

  const provQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const o = provenanceQuery(ctx, metricName);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', metricName, 'en');
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="l4" role="dialog" aria-modal="false" aria-label="Provenance — {metricName}">
  <header>
    <div>
      <p class="eyebrow">Layer 4 — Provenance</p>
      <h3>{metricTitle}</h3>
    </div>
    <button type="button" class="close" aria-label="Close provenance" onclick={onClose}>×</button>
  </header>

  <div class="body">
    {#if provQ.isPending}
      <p class="muted" aria-busy="true">Loading provenance…</p>
    {:else if provQ.data?.kind === 'refusal'}
      <RefusalSurface refusal={provQ.data} {ctx} />
    {:else if provQ.data?.kind === 'success'}
      {@const p = provQ.data.data}
      <dl class="prov">
        <dt>Tier</dt>
        <dd>{p.tierClassification}</dd>
        <dt>Validation</dt>
        <dd class="status status-{p.validationStatus}">{p.validationStatus}</dd>
        <dt>Extractor hash</dt>
        <dd><code>{p.extractorVersionHash}</code></dd>
      </dl>
      <section class="algo">
        <h4>Algorithm</h4>
        <p>{p.algorithmDescription}</p>
      </section>
      {#if contentQ.data?.kind === 'success'}
        <section class="registers">
          <ProgressiveSemantics registers={contentQ.data.data.registers} emphasis="semantic" />
        </section>
      {/if}
      {#if p.knownLimitations.length > 0}
        <section class="limits">
          <h4>Known limitations</h4>
          <ul>
            {#each p.knownLimitations as lim (lim)}
              <li>{lim}</li>
            {/each}
          </ul>
        </section>
      {/if}
      {#if p.culturalContextNotes}
        <section class="context">
          <h4>Cultural-context notes</h4>
          <p>{p.culturalContextNotes}</p>
        </section>
      {/if}
    {:else}
      <p class="muted">Provenance unavailable.</p>
    {/if}
  </div>
</div>

<style>
  .l4 {
    /* The L3 panel is now dashboard-wide, so anchoring the L4 overlay
       *beside* it would push it into the narrow Atmosphere strip.
       Instead we overlap the L3 panel on its right edge: L4 remains
       visibly on top of L3 (a disclosure, not a replacement — Design
       Brief §4.1 rule 2), the reader can still see the chart behind it. */
    position: fixed;
    top: var(--space-4);
    right: var(--space-4);
    width: min(92vw, 26rem);
    max-height: calc(100dvh - 2 * var(--space-4));
    overflow: auto;
    background: var(--color-surface);
    color: var(--color-fg);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-3);
    z-index: 1100;
  }
  header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }
  .eyebrow {
    margin: 0 0 2px;
    color: var(--color-fg-subtle);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.1em;
  }
  h3 {
    margin: 0;
    font-size: var(--font-size-md);
  }
  .close {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }
  .close:hover,
  .close:focus-visible {
    color: var(--color-fg);
    background: var(--color-surface-hover);
  }
  .body {
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    font-size: var(--font-size-sm);
  }
  dl.prov {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
  }
  dl.prov dt {
    color: var(--color-fg-muted);
  }
  dl.prov dd {
    margin: 0;
  }
  .status {
    display: inline-block;
    padding: 1px 6px;
    border-radius: 10px;
    font-size: 11px;
    text-transform: uppercase;
  }
  .status-unvalidated {
    color: #caa04a;
    background: rgba(202, 160, 74, 0.12);
  }
  .status-validated {
    color: #4ca84c;
    background: rgba(76, 168, 76, 0.12);
  }
  .status-expired {
    color: #c06060;
    background: rgba(192, 96, 96, 0.12);
  }
  h4 {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-sm);
  }
  .limits ul {
    margin: 0;
    padding-left: var(--space-5);
  }
  .muted {
    color: var(--color-fg-muted);
  }

  @media (max-width: 720px) {
    .l4 {
      right: var(--space-3);
      top: auto;
      bottom: var(--space-3);
    }
  }
</style>
