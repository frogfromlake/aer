<script lang="ts">
  // Phase 122d.2 — per-article Negative-Space section for the L5 Evidence
  // Reader (Phase 141 extraction). Lists every NS-marker that applies to this
  // article (methodological register, never a defect), open-by-default when ≥1
  // fires. The detailed headline before/after diff lives in the Diff tab.
  import type { ArticleRevisionEntryDto } from '$lib/api/queries';
  import { m } from '$lib/paraglide/messages.js';
  import NegativeSpaceBadge from '$lib/components/base/NegativeSpaceBadge.svelte';
  import { getNSClassDef } from '$lib/negative-space';
  import { silentEditSignals, nsMarkersFor } from './l5-evidence-internals';

  let {
    revisionList,
    revisionStatus,
    articleId
  }: {
    revisionList: ArticleRevisionEntryDto[];
    revisionStatus: string;
    articleId: string | null;
  } = $props();

  const signals = $derived(silentEditSignals(revisionList, revisionStatus));
  const nsMarkers = $derived(nsMarkersFor(signals));

  let nsSectionOpen = $state(true);
  // Re-open the section by default whenever a new article with markers loads.
  $effect(() => {
    void articleId;
    nsSectionOpen = true;
  });
</script>

{#if nsMarkers.length > 0}
  <details class="ns-section" bind:open={nsSectionOpen}>
    <summary class="ns-summary">
      {#each nsMarkers as nsClass (nsClass)}
        <NegativeSpaceBadge {nsClass} size="md" showLabel showInfo />
      {/each}
      <span class="ns-summary-text">{m.evidence_ns_cannot_see()}</span>
    </summary>
    <div class="ns-body">
      {#each nsMarkers as nsClass (nsClass)}
        {@const def = getNSClassDef(nsClass)}
        {#if def}
          <p class="ns-class-prose">{def.description}</p>
        {/if}
      {/each}
      <ul class="ns-signals">
        {#each signals as sig (sig)}
          <li>{sig}</li>
        {/each}
      </ul>
    </div>
  </details>
{/if}

<style>
  /* Phase 122d.2 — Negative-Space section (methodological dim, never warning). */
  .ns-section {
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-3);
    background: var(--color-surface-2, var(--color-surface));
  }
  .ns-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    cursor: pointer;
    flex-wrap: wrap;
  }
  .ns-summary-text {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .ns-body {
    padding: 0 var(--space-3) var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .ns-class-prose {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }
  .ns-signals {
    margin: 0;
    padding-left: var(--space-4);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
</style>
