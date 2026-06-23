<script lang="ts">
  // Phase 148e — the single cell-title bar, used by all 17 presentation cells.
  //
  // Two-tier grammar (Design Brief / Phase 148e sign-off):
  //   • eyebrow  — presentation label, mono small-caps, viridis-50 teal (anchor 1)
  //   • strong line — subject (fg) · model (dimmed) · scope pill (viridis-25, anchor 2)
  //                   · muted qualifier tail (resolution, r=, ≥2 sources, Top N…)
  // Exactly two viridis anchors; everything else neutral. The cell hands a
  // declarative `CellTitleSpec`; this bar composes + renders it and hosts the
  // cell's export controls via the optional `actions` snippet, so the whole
  // header row (title + export) is unified in one place.
  import type { Snippet } from 'svelte';
  import { composeCellTitle, type CellTitleSpec } from '$lib/presentations/cell-title';

  let { spec, actions }: { spec: CellTitleSpec; actions?: Snippet } = $props();

  const title = $derived(composeCellTitle(spec));
</script>

<header class="cell-header">
  <h3 id={title.idSeed} class="cell-title">
    <span class="ct-eyebrow">{title.presentation}</span>
    <span class="ct-line">
      {#if title.subject.kind !== 'none'}
        <span class="ct-subject">
          {#if title.subject.kind === 'pair'}
            {title.subject.left}<span class="ct-op">{title.subject.op}</span>{title.subject.right}
          {:else}
            {title.subject.label}
          {/if}
        </span>
      {/if}
      {#if title.model}
        <span class="ct-model">{title.model}</span>
      {/if}
      {#if title.scope.kind === 'single'}
        <span class="ct-scope">{title.scope.label}</span>
      {:else if title.scope.kind === 'pair'}
        <span class="ct-scope">{title.scope.left}</span>
        <span class="ct-rel">{title.scope.relation}</span>
        <span class="ct-scope">{title.scope.right}</span>
      {/if}
      {#each title.qualifiers as q (q.label)}
        <span
          class="ct-qual"
          class:tier={q.tone === 'tier'}
          class:layer={q.tone === 'layer'}
          title={q.title}>{q.label}</span
        >
      {/each}
    </span>
  </h3>
  {@render actions?.()}
</header>

<style>
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-2);
  }

  .cell-title {
    margin: 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  /* Tier 1 — presentation eyebrow (viridis anchor 1). */
  .ct-eyebrow {
    font-family: var(--font-mono);
    font-size: 10.5px;
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: var(--letter-spacing-wide);
    color: var(--color-viridis-50);
  }

  /* Tier 2 — the strong line: subject · model · scope · qualifiers. */
  .ct-line {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-base);
  }

  .ct-subject {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }

  .ct-op {
    color: var(--color-fg-muted);
    margin: 0 0.35em;
  }

  /* Model sub-slot — dimmed, with its own leading separator. */
  .ct-model {
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-regular);
  }
  .ct-model::before {
    content: '·';
    color: var(--color-fg-subtle);
    margin-right: var(--space-2);
  }

  /* Scope pill (viridis anchor 2). */
  .ct-scope {
    padding: 0 var(--space-2);
    border-radius: var(--radius-pill);
    background: color-mix(in srgb, var(--color-viridis-25) 22%, transparent);
    border: 1px solid color-mix(in srgb, var(--color-viridis-25) 55%, transparent);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }
  .ct-rel {
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  /* Muted qualifier tail. */
  .ct-qual {
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }
  /* Tier / layer badges keep a faint chip outline so they read as a status. */
  .ct-qual.tier,
  .ct-qual.layer {
    padding: 0 var(--space-2);
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border);
  }
</style>
