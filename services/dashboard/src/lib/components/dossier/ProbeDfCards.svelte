<script lang="ts">
  // ProbeDfCards — Phase 141 (extracted from ProbeCard).
  //
  // The "Discourse Functions" region: each of the four canonical functions is a
  // collapsable container whose body lists the Source-Cards of sources classified
  // under it; uncovered functions render with an explicit "uncovered" hint
  // (Negative Space). A trailing container holds any unclassified sources. Owns
  // the per-DF expand state; the grouping/coverage logic is pure
  // (`probe-card-internals.ts`).
  import {
    DISCOURSE_FUNCTIONS,
    FUNCTION_DEFINITIONS,
    type DiscourseFunction
  } from '$lib/discourse-function';
  import type { FetchContext, ProbeDossierSourceDto } from '$lib/api/queries';
  import SourceCard from '$lib/components/source/SourceCard.svelte';
  import {
    groupSourcesByFunction,
    unclassifiedSources,
    coveredFunctions
  } from './probe-card-internals';
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    /** The probe's machine id — namespaces the aria-controls ids. */
    probeId: string;
    sources: ProbeDossierSourceDto[];
    ctx: FetchContext;
    windowStart: string | undefined;
    windowEnd: string | undefined;
  }
  let { probeId, sources, ctx, windowStart, windowEnd }: Props = $props();

  // Per-DF expand state — default closed; user toggles per container.
  let dfExpanded = $state<Record<DiscourseFunction, boolean>>({
    epistemic_authority: false,
    power_legitimation: false,
    cohesion_identity: false,
    subversion_friction: false
  });

  const sourcesByFunction = $derived(groupSourcesByFunction(sources));
  const unclassified = $derived(unclassifiedSources(sources));
  const coveredSet = $derived(coveredFunctions(sourcesByFunction));

  function toggleDf(fn: DiscourseFunction) {
    dfExpanded = { ...dfExpanded, [fn]: !dfExpanded[fn] };
  }
</script>

<!-- Discourse-Function Cards as containers. Each is collapsable; the body lists
     Source-Cards for sources whose primary function matches. Uncovered DFs
     render with an explicit "uncovered" hint to surface the Negative Space. -->
<section class="df-cards" aria-labelledby="df-heading">
  <h3 id="df-heading" class="section-title">{m.dossier_df_title()}</h3>
  <ul class="df-list" role="list">
    {#each DISCOURSE_FUNCTIONS as fn (fn)}
      {@const meta = FUNCTION_DEFINITIONS[fn]}
      {@const covered = coveredSet.has(fn)}
      {@const fnSources = sourcesByFunction[fn] ?? []}
      {@const isOpen = dfExpanded[fn]}
      <li>
        <article
          class="df-card"
          class:covered
          class:expanded={isOpen}
          style:--fn-color={meta.color}
        >
          <button
            type="button"
            class="df-head"
            aria-expanded={isOpen}
            aria-controls="df-body-{probeId}-{fn}"
            onclick={() => toggleDf(fn)}
          >
            <span class="df-chev" class:expanded={isOpen} aria-hidden="true">›</span>
            <span class="df-abbr">{meta.abbr}</span>
            <span class="df-label">{meta.label}</span>
            <span class="df-state" aria-hidden="true">
              {covered
                ? (fnSources.length === 1
                    ? m.dossier_df_state_sources_one
                    : m.dossier_df_state_sources_other)({ count: fnSources.length })
                : m.dossier_df_state_uncovered()}
            </span>
          </button>
          {#if isOpen}
            <div class="df-body" id="df-body-{probeId}-{fn}">
              <p class="df-desc">{meta.description}</p>
              {#if fnSources.length > 0}
                <ul class="source-grid" role="list">
                  {#each fnSources as source (source.name)}
                    <li><SourceCard {source} {ctx} {windowStart} {windowEnd} /></li>
                  {/each}
                </ul>
              {:else}
                <p class="muted">{m.dossier_df_empty()}</p>
              {/if}
            </div>
          {/if}
        </article>
      </li>
    {/each}
  </ul>

  {#if unclassified.length > 0}
    <div class="df-card df-card-unclassified">
      <header class="df-head df-head-static">
        <span class="df-label">{m.dossier_df_unclassified_label()}</span>
        <span class="df-state">
          {(unclassified.length === 1
            ? m.dossier_df_state_sources_one
            : m.dossier_df_state_sources_other)({ count: unclassified.length })}
        </span>
      </header>
      <div class="df-body">
        <ul class="source-grid" role="list">
          {#each unclassified as source (source.name)}
            <li><SourceCard {source} {ctx} {windowStart} {windowEnd} /></li>
          {/each}
        </ul>
      </div>
    </div>
  {/if}
</section>

<style>
  .df-cards {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .section-title {
    margin: 0 0 var(--space-1) 0;
    font-size: var(--font-size-md);
    color: var(--color-fg);
  }

  .df-list {
    list-style: none;
    padding: 0;
    margin: 0;
    /* Phase 122k §6 finding — fixed 2x2 raster. The four DFs always occupy two
       columns; expanding a card lets it grow downward inside its column without
       breaking the layout. Each DF gets ~50% of the probe-card width. */
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    grid-auto-rows: min-content;
    align-items: start;
    gap: var(--space-2);
  }
  .df-list > li {
    min-width: 0;
  }

  .df-card {
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--fn-color, var(--color-border-strong));
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    overflow: hidden;
  }
  .df-card.covered .df-label {
    color: var(--color-fg);
  }
  .df-card:not(.covered) .df-label {
    color: var(--color-fg-subtle);
  }

  .df-card-unclassified {
    border-left-color: var(--color-border);
  }

  .df-head {
    appearance: none;
    background: transparent;
    border: none;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: var(--space-3);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    text-align: left;
  }
  .df-head-static {
    cursor: default;
  }
  .df-head:hover,
  .df-head:focus-visible {
    background: var(--color-bg-elevated);
  }

  .df-chev {
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    display: inline-block;
  }
  .df-chev.expanded {
    transform: rotate(90deg);
  }

  .df-abbr {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    font-weight: 700;
    color: var(--fn-color, var(--color-fg));
    background: color-mix(in srgb, var(--fn-color, var(--color-surface)) 12%, var(--color-surface));
    padding: 1px 6px;
    border-radius: var(--radius-sm);
  }

  .df-label {
    font-size: var(--font-size-sm);
  }

  .df-state {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    margin-left: auto;
  }

  .df-body {
    padding: var(--space-2) var(--space-3) var(--space-3) var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    border-top: 1px dashed var(--color-border);
  }

  .df-desc {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: 1.55;
    max-width: 60rem;
  }

  .source-grid {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
