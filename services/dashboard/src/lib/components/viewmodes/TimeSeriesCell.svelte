<script lang="ts">
  // NLP × time-series cell (Phase 107).
  // Per-source small-multiples baseline view. Reuses the existing
  // SourceLaneChart so the Phase 106 rendering survives unchanged when
  // this cell is selected.
  // Phase 111: not applicable in Silver layer (Silver documents have no
  // Gold-equivalent time-series; the distribution cell covers Silver).
  import SourceLaneChart from '$lib/components/lanes/SourceLaneChart.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let {
    ctx,
    sources,
    windowStart,
    windowEnd,
    metricName,
    dataLayer = 'gold',
    composition
  }: ViewModeCellProps = $props();

  // Phase 122i revision (D1). Composition gates the fan-out semantic:
  //   merged → one SourceLaneChart over the unioned source list, BFF
  //            unions server-side via `?sourceIds=a,b,…`.
  //   split  → one SourceLaneChart per source (small-multiples) — the
  //            legacy Phase-106 behaviour.
  // Composition absent → fan-out per source (legacy callers, e.g. the
  // Phase-122h shells when the user hits a legacy flat URL).
  const sourceNames = $derived(sources.map((s) => s.name));
</script>

<div class="cell-body">
  {#if dataLayer === 'silver'}
    <p class="notice">
      Time-series view is not available for Silver-layer data. Switch to Distribution to explore
      Silver-layer document characteristics.
    </p>
  {:else if sources.length === 0}
    <p class="empty">No sources in the active scope.</p>
  {:else if composition === 'merged'}
    <SourceLaneChart
      sourceNames={[...sourceNames]}
      emicDesignation={null}
      {ctx}
      {windowStart}
      {windowEnd}
      {metricName}
    />
  {:else}
    {#each sources as source (source.name)}
      <SourceLaneChart
        sourceName={source.name}
        emicDesignation={source.emicDesignation}
        {ctx}
        {windowStart}
        {windowEnd}
        {metricName}
      />
    {/each}
  {/if}
</div>

<style>
  .cell-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-6);
  }

  .empty,
  .notice {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .notice {
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px dashed var(--color-border-strong);
    border-radius: var(--radius-md);
    max-width: 36rem;
  }
</style>
