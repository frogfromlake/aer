<script lang="ts">
  // NLP × time-series cell (Phase 107).
  // Per-source small-multiples baseline view. Reuses the existing
  // SourceLaneChart so the Phase 106 rendering survives unchanged when
  // this cell is selected.
  import SourceLaneChart from '$lib/components/lanes/SourceLaneChart.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let { ctx, sources, windowStart, windowEnd, metricName }: ViewModeCellProps = $props();
</script>

<div class="cell-body">
  {#if sources.length === 0}
    <p class="empty">No sources in the active scope.</p>
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

  .empty {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
