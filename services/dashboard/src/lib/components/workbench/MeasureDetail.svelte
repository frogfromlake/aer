<script lang="ts">
  // MeasureDetail — Phase 148f (the deep methodology, folded into the Reading
  // Guide under the ladder; replaces the standalone CellMethodology block).
  //
  // The WHOLE section is one collapsible (default collapsed) with an explicit
  // "click to expand" hint, so the deep methodology never crowds the ladder.
  // Expanding reveals independently-collapsible blocks (DEFAULT COLLAPSED):
  //   0. How this view works — the reviewed howto_<presentation>.methodological
  //      register, shown for EVERY view so the section is never empty.
  //   1. <view> × <metric>  (per-cell methodology prose, metric subjects)
  //   2. What this metric measures  (dual register, metric subjects)
  //   3. Provenance  (tier / validation / algorithm / extractor hash)
  //   4. Known limitations
  //   • or, for a field subject: the curated field description.
  //   + Working-Paper + (metric-only) full-provenance-page links.
  // `Panel.metric` is overloaded, so the METRIC-keyed fetches are gated by
  // `panelSubjectKind` (metric → provenance/content; field/none → skipped, no
  // 404); the view methodology is fetched for every view.
  import { createQuery } from '@tanstack/svelte-query';
  import { m } from '$lib/paraglide/messages.js';
  import {
    contentQuery,
    provenanceQuery,
    type ContentResponseDto,
    type FetchContext,
    type MetricProvenanceDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import Badge from '$lib/components/base/Badge.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import {
    pickBadgeTier,
    workingPaperHref as parseWorkingPaperHref
  } from '$lib/components/chrome/methodology-tray-internals';
  import { cellContentId, hasCellMethodologyContent, getPresentation } from '$lib/presentations';
  import { panelSubjectKind } from '$lib/presentations/metric-presentation';
  import type { Presentation } from '$lib/state/url-internals';
  import { page } from '$app/state';
  import { urlState } from '$lib/state/url.svelte';
  import {
    metricLabel,
    fieldLabel,
    fieldDescription,
    isRegisteredMetric
  } from '$lib/state/labels.svelte';
  import { locale } from '$lib/state/locale.svelte';

  interface Props {
    metricName: string;
    viewMode: Presentation;
    /** Human label of the active view (for the cell-id chip). */
    viewLabel: string;
  }
  let { metricName, viewMode, viewLabel }: Props = $props();

  // The whole methodology section is collapsible (default collapsed) with an
  // explicit hint, so the deep provenance never crowds the reading ladder.
  let mdExpanded = $state(false);

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  // `metricName` = `Panel.metric` (overloaded). Classify by presentation so the
  // metric-keyed fetches fire ONLY for a real metric subject (otherwise 404).
  const subjectKind = $derived(panelSubjectKind(getPresentation(viewMode)));
  const isMetricSubject = $derived(subjectKind === 'metric');
  const isFieldSubject = $derived(subjectKind === 'field');
  // A metric subject whose `Panel.metric` is a real registered metric. A
  // metric-agnostic view can leave a categorical FIELD in `Panel.metric` (e.g.
  // `author` on a Distribution): that is a metric subject by presentation but has
  // no provenance, so the metric-keyed fetches must NOT fire (else 404 + a broken,
  // empty methodology). Such a view falls back to its view-level methodology only.
  const isRealMetric = $derived(isMetricSubject && isRegisteredMetric(metricName));
  const fieldDesc = $derived(isFieldSubject ? fieldDescription(metricName) : null);

  const provenanceQ = createQuery<
    QueryOutcome<MetricProvenanceDto>,
    Error,
    QueryOutcome<MetricProvenanceDto>
  >(() => {
    const o = provenanceQuery(ctx, metricName);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isRealMetric && metricName.length > 0
    };
  });

  const metricContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', metricName, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: isRealMetric && metricName.length > 0
    };
  });

  const hasViewModeContent = $derived(isRealMetric && hasCellMethodologyContent(viewMode));
  const cellContent_id = $derived(cellContentId(viewMode, metricName));
  const viewModeContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', cellContent_id, locale());
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: hasViewModeContent && metricName.length > 0
    };
  });

  // The view-level methodology — the reviewed `howto_<presentation>.methodological`
  // register, which exists for EVERY presentation. Shown for every view (even the
  // channel-driven / 'none' ones with no single-metric provenance), so the
  // methodology section is never empty. TanStack dedupes this with the
  // ReadingGuide's own howto fetch (same queryKey) — no extra network call.
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

  const provenance = $derived<MetricProvenanceDto | null>(
    provenanceQ.data?.kind === 'success' ? provenanceQ.data.data : null
  );
  const metricContent = $derived<ContentResponseDto | null>(
    metricContentQ.data?.kind === 'success' ? metricContentQ.data.data : null
  );
  const viewModeContent = $derived<ContentResponseDto | null>(
    viewModeContentQ.data?.kind === 'success' ? viewModeContentQ.data.data : null
  );

  const badgeTier = $derived(pickBadgeTier(provenance));
  const hasLimitations = $derived((provenance?.knownLimitations.length ?? 0) > 0);
  // The visible subject chip: the metric or field for those subjects; for a
  // metric-agnostic ('none') view there is no single subject, so the header just
  // names the view.
  const headerSubject = $derived(
    isRealMetric ? metricLabel(metricName) : isFieldSubject ? fieldLabel(metricName) : null
  );

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
    parseWorkingPaperHref(
      metricContent?.workingPaperAnchors?.[0] ?? viewMethodology?.workingPaperAnchors?.[0] ?? null
    )
  );
  const wpHref = $derived(
    rawWpHref ? `${rawWpHref}${rawWpHref.includes('?') ? '&' : '?'}${referrerParams}` : null
  );
</script>

{#if viewMethodology || isMetricSubject || isFieldSubject}
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
      aria-controls="md-body-{metricName}-{viewMode}"
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
      {#if isRealMetric}
        <Badge tier={badgeTier} />
      {/if}
      {#if hasLimitations}
        <span class="limitations-pill" title={m.workbench_meth_known_limitations_pill_title()}>
          {m.workbench_meth_known_limitations_pill()}
        </span>
      {/if}
      <span class="md-hint">{m.rg_methodology_hint()}</span>
    </button>

    {#if mdExpanded}
      <div class="md-body" id="md-body-{metricName}-{viewMode}">
        <!-- The view-level methodology (reviewed howto.<presentation>.methodological)
             — present for EVERY view, so the section is never empty. -->
        {#if viewMethodology}
          <details class="meth-block" data-section="view-method">
            <summary class="meth-block-summary">{m.workbench_meth_view_method()}</summary>
            <p class="cell-method-text">{viewMethodology.registers.methodological.long}</p>
          </details>
        {/if}

        {#if isFieldSubject}
          <details class="meth-block" data-section="field">
            <summary class="meth-block-summary">{m.workbench_meth_what_field_means()}</summary>
            <p class="cell-method-text">{fieldDesc ?? m.workbench_meth_field_no_desc()}</p>
          </details>
        {:else if isRealMetric && (provenanceQ.isPending || metricContentQ.isPending)}
          <p class="muted" aria-busy="true">{m.workbench_meth_loading()}</p>
        {:else if isRealMetric}
          {#if viewModeContent}
            <details class="meth-block" data-section="cell-method">
              <summary class="meth-block-summary"
                >{m.workbench_meth_cell_method_heading({
                  view: viewLabel,
                  metric: headerSubject ?? viewLabel
                })}</summary
              >
              <p class="cell-method-text">{viewModeContent.registers.methodological.long}</p>
            </details>
          {/if}

          {#if metricContent}
            <details class="meth-block" data-section="dual-register">
              <summary class="meth-block-summary">{m.workbench_meth_what_metric_measures()}</summary
              >
              <ProgressiveSemantics registers={metricContent.registers} emphasis="methodological" />
            </details>
          {/if}

          {#if provenance}
            <details class="meth-block" data-section="provenance">
              <summary class="meth-block-summary">{m.workbench_meth_provenance()}</summary>
              <dl class="provenance-dl">
                <dt>{m.workbench_meth_provenance_tier()}</dt>
                <dd><Badge tier={badgeTier} /></dd>
                <dt>{m.workbench_meth_provenance_validation()}</dt>
                <dd class="status status-{provenance.validationStatus}">
                  {provenance.validationStatus}
                </dd>
                <dt>{m.workbench_meth_provenance_algorithm()}</dt>
                <dd>{provenance.algorithmDescription}</dd>
                <dt>{m.workbench_meth_provenance_extractor()}</dt>
                <dd><code>{provenance.extractorVersionHash}</code></dd>
              </dl>
              {#if provenance.culturalContextNotes}
                <p class="cultural-notes">{provenance.culturalContextNotes}</p>
              {/if}
            </details>
          {/if}

          {#if hasLimitations && provenance}
            <details class="meth-block" data-section="limitations">
              <summary class="meth-block-summary">{m.workbench_meth_known_limitations()}</summary>
              <ul class="limitations-list">
                {#each provenance.knownLimitations as lim (lim)}
                  <li>{lim}</li>
                {/each}
              </ul>
            </details>
          {/if}
        {/if}

        {#if wpHref || isRealMetric}
          <div class="meth-links">
            {#if wpHref}
              <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
              <a class="meth-link" href={wpHref}>{m.workbench_meth_link_working_paper()}</a>
            {/if}
            {#if isRealMetric}
              <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
              <a class="meth-link" href="/reflection/metric/{metricName}">
                {m.workbench_meth_link_provenance_page()}
              </a>
            {/if}
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

  /* The collapsible header — a real button with a hover surface + a hint so it is
     unmistakably expandable. */
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
  /* The hint pushes to the right so the row clearly reads "expandable section". */
  .md-hint {
    margin-left: auto;
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-subtle);
  }
  /* Calm, serious header: the label + view are quiet mono tones; the only colour
     is the semantic Tier badge + limitations pill (the meaningful differences). */
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

  .limitations-pill {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    padding: 1px var(--space-2);
    border: 1px solid var(--color-status-expired);
    color: var(--color-status-expired);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
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
    color: var(--color-fg-subtle);
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

  .limitations-list {
    margin: 0;
    padding-left: var(--space-5);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .provenance-dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-2) var(--space-4);
    margin: 0;
    font-size: var(--font-size-sm);
  }
  .provenance-dl dt {
    color: var(--color-fg-muted);
    font-weight: var(--font-weight-medium);
  }
  .provenance-dl dd {
    margin: 0;
    color: var(--color-fg);
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .provenance-dl dd code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
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

  .cultural-notes {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }
  .cell-method-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
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

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
