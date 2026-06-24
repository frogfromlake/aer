<script lang="ts">
  // MeasureDetail — Phase 148f (deep methodology folded into the Reading Guide),
  // generalised in Phase 148g to EVERY bound subject of a cell.
  //
  // The whole section is one collapsible (default collapsed) with a "click to
  // expand" hint, so the deep methodology never crowds the ladder. Expanding
  // reveals:
  //   0. How this view works — the reviewed howto_<presentation>.methodological
  //      register, shown for EVERY view so the section is never empty.
  //   • one SubjectMethodology block PER bound subject (metric or field), each
  //     with its role(s) in the view. The single-metric views (distribution,
  //     time_series) bind one metric; scatter binds x/y/size/colour, correlation
  //     /parallel-coordinates a metric SET, cross-tab a group-by field + metric,
  //     lead-lag two metrics, co-occurrence node-size/colour metrics, sankey a
  //     field chain — plus any view's facet field. cellSubjects() enumerates them
  //     so "every presentation × every metric × every field" is literally true.
  //   + a Working-Paper link (from the view methodology).
  import { createQuery } from '@tanstack/svelte-query';
  import { m } from '$lib/paraglide/messages.js';
  import {
    contentQuery,
    type ContentResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { workingPaperHref as parseWorkingPaperHref } from '$lib/components/chrome/methodology-tray-internals';
  import SubjectMethodology from './SubjectMethodology.svelte';
  import type { CellSubject } from '$lib/presentations';
  import type { Presentation } from '$lib/state/url-internals';
  import { page } from '$app/state';
  import { urlState } from '$lib/state/url.svelte';
  import { metricLabel, fieldLabel, isRegisteredMetric } from '$lib/state/labels.svelte';
  import { locale } from '$lib/state/locale.svelte';
  import { splitParagraphs } from '$lib/prose';

  interface Props {
    /** The cell's bound subjects (metrics + fields), from cellSubjects(). */
    subjects: CellSubject[];
    viewMode: Presentation;
    /** Human label of the active view. */
    viewLabel: string;
  }
  let { subjects, viewMode, viewLabel }: Props = $props();

  // The whole methodology section is collapsible (default collapsed).
  let mdExpanded = $state(false);

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  // The view-level methodology — the reviewed `howto_<presentation>.methodological`
  // register, which exists for EVERY presentation. Shown for every view (even the
  // channel-driven / 'none' ones with no per-subject methodology), so the section
  // is never empty. TanStack dedupes this with the ReadingGuide's own howto fetch
  // (same queryKey) — no extra network call.
  const viewMethodologyQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', `howto_${viewMode}`, locale());
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const viewMethodology = $derived<ContentResponseDto | null>(
    viewMethodologyQ.data?.kind === 'success' ? viewMethodologyQ.data.data : null
  );

  // Header subject summary: the lone subject's label when there is exactly one,
  // else the view alone (the per-subject labels live in their own blocks).
  const headerSubject = $derived.by(() => {
    if (subjects.length !== 1) return null;
    const only = subjects[0]!;
    return isRegisteredMetric(only.name) ? metricLabel(only.name) : fieldLabel(only.name);
  });

  // Working-paper anchor enriched with referrer params so the WP page deep-links
  // back to the cell the reader came from.
  const url = $derived(urlState());
  const referrerParams = $derived.by(() => {
    const probe = url.selectedProbes[0] ?? null;
    const fn = page.url.searchParams.get('functionKey');
    const pillar = url.activePillar ?? null;
    // eslint-disable-next-line svelte/prefer-svelte-reactivity -- ephemeral URL string builder, not shared state
    const p = new URLSearchParams();
    p.set('from', 'workbench');
    if (probe) p.set('probe', probe);
    if (fn) p.set('fn', fn);
    if (pillar) p.set('pillar', pillar);
    return p.toString();
  });
  const rawWpHref = $derived(
    parseWorkingPaperHref(viewMethodology?.workingPaperAnchors?.[0] ?? null)
  );
  const wpHref = $derived(
    rawWpHref ? `${rawWpHref}${rawWpHref.includes('?') ? '&' : '?'}${referrerParams}` : null
  );
</script>

{#if viewMethodology || subjects.length > 0}
  <section
    class="measure-detail epistemic-weight"
    aria-label={m.workbench_meth_aria_label({
      metric: headerSubject ?? viewLabel,
      view: viewLabel
    })}
  >
    <button
      type="button"
      class="md-toggle"
      aria-expanded={mdExpanded}
      aria-controls="md-body-{viewMode}"
      onclick={() => (mdExpanded = !mdExpanded)}
    >
      <span class="md-chevron" class:expanded={mdExpanded} aria-hidden="true">›</span>
      <span class="md-title">{m.workbench_meth_title()}</span>
      <span class="md-cell-id">
        {#if headerSubject}
          <code class="md-metric">{headerSubject}</code>
          <span class="md-sep" aria-hidden="true">·</span>
        {/if}
        <span class="md-view">{viewLabel}</span>
      </span>
      <span class="md-hint">{m.rg_methodology_hint()}</span>
    </button>

    {#if mdExpanded}
      <div class="md-body" id="md-body-{viewMode}">
        <!-- The view-level methodology — present for EVERY view, never empty. -->
        {#if viewMethodology}
          <details class="meth-block" data-section="view-method">
            <summary class="meth-block-summary">{m.workbench_meth_view_method()}</summary>
            <div class="cell-method-text">
              {#each splitParagraphs(viewMethodology.registers.methodological.long) as para (para)}
                <p>{para}</p>
              {/each}
            </div>
          </details>
        {/if}

        <!-- One methodology block per bound subject (metric or field). -->
        {#each subjects as subject (subject.name)}
          <SubjectMethodology name={subject.name} roles={subject.roles} {viewMode} {viewLabel} />
        {/each}

        {#if wpHref}
          <div class="meth-links">
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
            <a class="meth-link" href={wpHref}>{m.workbench_meth_link_working_paper()}</a>
          </div>
        {/if}
      </div>
    {/if}
  </section>
{/if}

<style>
  /* Clear, airy separation from the reading ladder: a generous top margin + a
     full divider, then the whole section is one collapsible affordance. */
  .measure-detail {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    margin-top: var(--space-3);
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
  }

  .md-toggle {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-3);
    width: 100%;
    padding: var(--space-2);
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    cursor: pointer;
    text-align: left;
  }
  .md-toggle:hover,
  .md-toggle:focus-visible {
    background: var(--color-surface);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: calc(-1 * var(--focus-ring-width));
  }
  .md-chevron {
    display: inline-flex;
    width: 0.9rem;
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .md-chevron.expanded {
    transform: rotate(90deg);
  }
  .md-hint {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }
  .md-title {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-subtle);
  }
  .md-cell-id {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 1px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
  }
  .md-metric {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }
  .md-sep {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .md-view {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .md-body {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .meth-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .meth-block-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    margin: 0;
    list-style: none;
    cursor: pointer;
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    /* Phase 148g — full-strength section title (was dimmed) for overview. */
    color: var(--color-fg);
  }
  .meth-block-summary::-webkit-details-marker {
    display: none;
  }
  .meth-block-summary::before {
    content: '›';
    display: inline-block;
    color: var(--color-fg-subtle);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .meth-block[open] > .meth-block-summary::before {
    transform: rotate(90deg);
  }
  .meth-block-summary:hover {
    color: var(--color-fg);
  }

  /* Phase 148g — stacked paragraphs + comfortable measure for readable prose. */
  .cell-method-text {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: 1.65;
    margin: 0;
    max-inline-size: 70ch;
  }
  .cell-method-text p {
    margin: 0;
  }

  .meth-links {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding-top: var(--space-2);
    border-top: 1px dashed var(--color-border);
  }
  .meth-link {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-accent);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-accent-muted);
    align-self: flex-start;
  }
  .meth-link:hover,
  .meth-link:focus-visible {
    color: var(--color-fg);
    border-bottom-style: solid;
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  @media (prefers-reduced-motion: reduce) {
    .md-chevron,
    .meth-block-summary::before {
      transition: none;
    }
  }
</style>
