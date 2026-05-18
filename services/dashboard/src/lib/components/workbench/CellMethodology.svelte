<script lang="ts">
  // CellMethodology — Phase 122h Findings round 2 §F1.
  //
  // Inline methodology block rendered UNDER each Pillar Shell's cell.
  // Carries every piece of provenance the user needs to interpret the
  // active (metric × view) cell:
  //
  //   - Tier badge + validation status
  //   - Known limitations (lifted under Negative Space)
  //   - Dual-Register prose for the metric (semantic ↔ methodological)
  //   - Provenance details (algorithm, extractor hash, cultural notes)
  //   - Per-cell view-mode methodology text
  //   - Working-Paper deep link(s)
  //
  // Reuses the data shape the retired right-edge MethodologyTray used.
  // Phase 124b's inline accordion pattern continues here — one block,
  // collapsible, default-open.
  import { createQuery } from '@tanstack/svelte-query';
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
  import { cellContentId } from '$lib/viewmodes';
  import type { ViewMode } from '$lib/state/url-internals';
  import { page } from '$app/state';
  import { urlState } from '$lib/state/url.svelte';
  import { negativeSpaceActive } from '$lib/state/tray.svelte';

  interface Props {
    metricName: string;
    viewMode: ViewMode;
    /** Human label of the active view (for the cell-id chip). */
    viewLabel: string;
  }

  let { metricName, viewMode, viewLabel }: Props = $props();

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  // Default open — methodology IS the dashboard's distinguishing feature
  // (Brief §5.8 Epistemic Weight + WP-006 §6 Reflexive Architecture).
  let expanded = $state(true);

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
      enabled: metricName.length > 0
    };
  });

  const metricContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'metric', metricName, 'en');
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: metricName.length > 0
    };
  });

  const cellContent_id = $derived(cellContentId(viewMode, metricName));
  const viewModeContentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'view_mode', cellContent_id);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

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

  // Working-paper anchor enriched with referrer params so the WP page can
  // render a "Back to Workbench" link. The probe + function + pillar are
  // pulled from URL state so the WP page deep-links back to the cell the
  // user was reading.
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
    parseWorkingPaperHref(metricContent?.workingPaperAnchors?.[0] ?? null)
  );
  const wpHref = $derived(
    rawWpHref ? `${rawWpHref}${rawWpHref.includes('?') ? '&' : '?'}${referrerParams}` : null
  );

  const negSpace = $derived(negativeSpaceActive());
</script>

<section
  class="cell-methodology epistemic-weight"
  aria-label="Methodology — {metricName} · {viewLabel}"
>
  <button
    type="button"
    class="meth-toggle"
    aria-expanded={expanded}
    aria-controls="meth-body-{metricName}-{viewMode}"
    onclick={() => (expanded = !expanded)}
  >
    <span class="meth-chevron" aria-hidden="true" class:expanded>›</span>
    <span class="meth-title">Methodology</span>
    <span class="meth-cell-id">
      <code class="meth-metric">{metricName}</code>
      <span class="meth-sep" aria-hidden="true">·</span>
      <span class="meth-view">{viewLabel}</span>
    </span>
    <span class="meth-badges">
      <Badge tier={badgeTier} />
      {#if hasLimitations}
        <span class="limitations-pill" title="Known limitations apply to this metric">
          Known limitations
        </span>
      {/if}
    </span>
  </button>

  {#if expanded}
    <div
      class="meth-body"
      class:limitations-first={negSpace}
      id="meth-body-{metricName}-{viewMode}"
    >
      {#if provenanceQ.isPending || metricContentQ.isPending}
        <p class="muted" aria-busy="true">Loading methodology…</p>
      {:else}
        {#if hasLimitations && provenance}
          <section class="meth-block" data-section="limitations">
            <h4>Known limitations</h4>
            <ul class="limitations-list">
              {#each provenance.knownLimitations as lim (lim)}
                <li>{lim}</li>
              {/each}
            </ul>
          </section>
        {/if}

        {#if metricContent}
          <section class="meth-block" data-section="dual-register">
            <h4>What this metric measures</h4>
            <ProgressiveSemantics registers={metricContent.registers} emphasis="methodological" />
          </section>
        {/if}

        {#if provenance}
          <section class="meth-block" data-section="provenance">
            <h4>Provenance</h4>
            <dl class="provenance-dl">
              <dt>Tier</dt>
              <dd><Badge tier={badgeTier} /></dd>
              <dt>Validation</dt>
              <dd class="status status-{provenance.validationStatus}">
                {provenance.validationStatus}
              </dd>
              <dt>Algorithm</dt>
              <dd>{provenance.algorithmDescription}</dd>
              <dt>Extractor</dt>
              <dd><code>{provenance.extractorVersionHash}</code></dd>
            </dl>
            {#if provenance.culturalContextNotes}
              <p class="cultural-notes">{provenance.culturalContextNotes}</p>
            {/if}
          </section>
        {/if}

        {#if viewModeContent}
          <section class="meth-block" data-section="cell-method">
            <h4>{viewLabel} × {metricName}</h4>
            <p class="cell-method-text">{viewModeContent.registers.methodological.long}</p>
          </section>
        {/if}

        <div class="meth-links">
          {#if wpHref}
            <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
            <a class="meth-link" href={wpHref}>Read the metric's Working Paper →</a>
          {/if}
          <!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- internal Reflection route -->
          <a class="meth-link" href="/reflection/metric/{metricName}">
            Full metric provenance page →
          </a>
        </div>
      {/if}
    </div>
  {/if}
</section>

<style>
  .cell-methodology {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
  }

  .meth-toggle {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg-elevated);
    border: none;
    cursor: pointer;
    color: var(--color-fg-muted);
    text-align: left;
    width: 100%;
    flex-wrap: wrap;
    transition: background var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .meth-toggle:hover,
  .meth-toggle:focus-visible {
    background: var(--color-surface);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .meth-chevron {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1rem;
    height: 1rem;
    color: var(--color-fg-subtle);
    transform: rotate(0deg);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
    flex-shrink: 0;
  }

  .meth-chevron.expanded {
    transform: rotate(90deg);
  }

  .meth-title {
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
  }

  .meth-cell-id {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    padding: 1px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-family: var(--font-mono);
  }

  .meth-metric {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .meth-sep {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .meth-view {
    font-size: var(--font-size-xs);
    color: var(--color-accent);
  }

  .meth-badges {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin-left: auto;
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

  .meth-body {
    padding: var(--space-5);
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    border-top: 1px solid var(--color-border);
  }

  .meth-body.limitations-first :global([data-section='limitations']) {
    order: -1;
  }

  .meth-block {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .meth-block h4 {
    margin: 0;
    font-size: var(--font-size-xs);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg-subtle);
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

  @media (prefers-reduced-motion: reduce) {
    .meth-toggle,
    .meth-chevron {
      transition: none;
    }
  }
</style>
