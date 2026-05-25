<script lang="ts">
  // "How to read this" note — Phase 131.
  //
  // Every configurable cell renders one of these. The note is COMPOSED: a
  // per-presentation template line (pulled from the content-catalog
  // Dual-Register `view_mode/howto_<presentation>` entry when it exists, else
  // a built-in fallback) followed by config-derived building blocks that
  // reflect the cell's live configuration. Composition logic lives in the
  // pure `how-to-read.ts` helper; this component only fetches the template and
  // renders the result.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { composeHowToRead, type HowToReadFacts } from '$lib/viewmodes/how-to-read';
  import type { ViewMode } from '$lib/state/url-internals';

  interface Props {
    presentation: ViewMode;
    facts: HowToReadFacts;
  }

  let { presentation, facts }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  let expanded = $state(true);

  const templateQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', `howto_${presentation}`);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const templateBase = $derived(
    templateQ.data?.kind === 'success' ? templateQ.data.data.registers.semantic.short : null
  );
  const lines = $derived(composeHowToRead(presentation, facts, templateBase));
</script>

<aside class="how-to-read" aria-label="How to read this cell">
  <button
    type="button"
    class="htr-toggle"
    aria-expanded={expanded}
    onclick={(e) => {
      e.stopPropagation();
      expanded = !expanded;
    }}
  >
    <span class="htr-chevron" class:expanded aria-hidden="true">›</span>
    <span class="htr-title">How to read this</span>
  </button>
  {#if expanded}
    <ul class="htr-body">
      {#each lines as line (line)}
        <li>{line}</li>
      {/each}
    </ul>
  {/if}
</aside>

<style>
  .how-to-read {
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--color-accent) 5%, transparent);
  }

  .htr-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    width: 100%;
    padding: var(--space-2) var(--space-3);
    background: none;
    border: none;
    cursor: pointer;
    color: var(--color-fg-muted);
    text-align: left;
  }
  .htr-toggle:hover,
  .htr-toggle:focus-visible {
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .htr-chevron {
    display: inline-flex;
    width: 0.9rem;
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    color: var(--color-accent);
  }
  .htr-chevron.expanded {
    transform: rotate(90deg);
  }

  .htr-title {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
  }

  .htr-body {
    margin: 0;
    padding: 0 var(--space-4) var(--space-3) var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  @media (prefers-reduced-motion: reduce) {
    .htr-chevron {
      transition: none;
    }
  }
</style>
