<script lang="ts">
  // Phase 122i revision (R5) — per-probe source picker, scoped to the
  // GeneralFreeComposeSection. Loads the probe's dossier (for the
  // source list) and renders one checkbox per source, calling the
  // parent's `onToggle` on change.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    probeDossierQuery,
    type FetchContext,
    type ProbeDossierDto,
    type QueryOutcome
  } from '$lib/api/queries';

  interface Props {
    ctx: FetchContext;
    probeId: string;
    windowStart: string;
    windowEnd: string;
    selected: readonly string[];
    onToggle: (sourceName: string) => void;
  }

  let { ctx, probeId, windowStart, windowEnd, selected, onToggle }: Props = $props();

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, probeId, { windowStart, windowEnd });
      return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
    }
  );

  const sources = $derived(dossierQ.data?.kind === 'success' ? dossierQ.data.data.sources : []);
</script>

<div class="source-picker">
  {#if dossierQ.isPending}
    <p class="muted" aria-busy="true">Loading sources…</p>
  {:else if sources.length === 0}
    <p class="muted">No sources in this probe.</p>
  {:else}
    <ul class="source-list" role="list">
      {#each sources as source (source.name)}
        {@const checked = selected.includes(source.name)}
        <li>
          <label class="source-row" class:checked>
            <input
              type="checkbox"
              {checked}
              onchange={() => onToggle(source.name)}
              aria-label="Include {source.emicDesignation ?? source.name}"
            />
            <span class="source-label">
              <span class="source-name">{source.emicDesignation ?? source.name}</span>
              {#if source.emicDesignation && source.emicDesignation !== source.name}
                <code class="source-id">{source.name}</code>
              {/if}
            </span>
          </label>
        </li>
      {/each}
    </ul>
  {/if}
</div>

<style>
  .source-picker {
    margin-top: var(--space-2);
  }

  .source-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
    gap: var(--space-1);
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    background: var(--color-bg-elevated);
    font-size: var(--font-size-xs);
  }

  .source-row.checked {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .source-label {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .source-name {
    color: var(--color-fg);
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .muted {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
