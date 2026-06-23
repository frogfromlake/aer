<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
  // Aleph × Silent-Edit Observability — Phase 122d.0 (ADR-032).
  //
  // Synchronic per-source bar chart over `aer_gold.article_revisions`.
  // The Aleph cell collapses the active window to one bucket and asks
  // "which source edits most right now"; the diachronic counterpart is
  // `RevisionTimelineCell` (Episteme). One BFF endpoint feeds both.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    revisionActivityQuery,
    type RevisionActivityResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/article/ArticleListModal.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportPayload, type ExportRow } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import {
    fmtValue,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import { useProbeLabels } from '$lib/presentations/use-probe-labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import CellReadout from './CellReadout.svelte';
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';

  let { ctx, scope, scopeId, windowStart, windowEnd }: PresentationCellProps = $props();

  // Phase 148e — unified cell title. Eyebrow = presentation; no metric
  // subject (silent-edit observability has none); scope = resolved probe/source
  // label (no raw probe id leak). This presentation has 0 config levers, so
  // there is no resolution qualifier.
  const probeLabels = useProbeLabels(() => ctx);
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_revision_activity_label(),
    subject: { kind: 'none' },
    scope: { kind: 'single', label: probeLabels.labelFor(scopeId) },
    idSeed: `rev-title-${scopeId}`
  });

  // Phase 122d.1 — drill-down: click a source bar to open the article
  // list filtered to that source's revisions in the active window.
  let drilldownSource = $state<string | null>(null);

  const revisionQ = createQuery<
    QueryOutcome<RevisionActivityResponseDto>,
    Error,
    QueryOutcome<RevisionActivityResponseDto>
  >(() => {
    const o = revisionActivityQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      resolution: 'snapshot'
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  type Entry = { source: string; revisions: number; articlesAffected: number };
  const entries = $derived<Entry[]>(
    revisionQ.data?.kind === 'success'
      ? (revisionQ.data.data.entries ?? []).map((e) => ({
          source: e.source,
          revisions: e.revisions,
          articlesAffected: e.articlesAffected
        }))
      : []
  );

  // Phase 122d.1 (Run-3) — Observable Plot restored for the chart LOOK
  // (axes, grid, polished bars). The earlier click failures were NOT
  // the rendering — they were `tip: true`, whose interaction overlay
  // swallowed pointer events (sticky tooltips + clicks never reached
  // the bars). Fix: render WITHOUT tip, and bind clicks via a single
  // delegated SVG listener that maps the clicked <rect> to its data
  // row by DOM index. `barX` + `ruleX` produce exactly one <rect> per
  // bar and zero other rects (the rule is a <line>), so the index ↔
  // entries mapping is exact. Hover affordance is pure CSS (no JS, so
  // nothing can stick).
  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = entries;
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth,
        height: Math.max(160, rows.length * 38 + 50),
        marginLeft: 150,
        marginBottom: 36,
        x: { label: m.cells_revact_axis_revisions(), grid: true, nice: true },
        y: { label: null, domain: rows.map((r) => r.source) },
        marks: [
          Plot.barX(rows, {
            x: 'revisions',
            y: 'source',
            fill: 'rgba(154, 143, 184, 0.55)',
            stroke: 'rgba(154, 143, 184, 0.95)',
            rx: 2
          }),
          Plot.ruleX([0])
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(sanitizePlotA11y(next as unknown as HTMLElement));
      plotEl = next as unknown as HTMLElement;

      // ROOT-CAUSE FIX (Run-4): the listener is attached to the stable
      // `host` div, NOT to `plotEl.querySelector('svg')`. Plot.plot()
      // returns the <svg> DIRECTLY when there's no legend, so
      // `plotEl.querySelector('svg')` returned null (an svg has no
      // descendant svg) and the listener was never bound — every prior
      // Plot iteration silently failed here. `host` always exists and
      // bar clicks bubble up to it. The clicked rect's own
      // `ownerSVGElement` is the authoritative chart svg, so the
      // index lookup is scoped correctly even if a legend svg exists.
    })();
  });

  function onHostClick(ev: MouseEvent): void {
    const target = ev.target as Element | null;
    const rect = target?.closest('rect');
    if (!rect) return;
    const svg = (rect as SVGRectElement).ownerSVGElement;
    if (!svg) return;
    const barRects = Array.from(svg.querySelectorAll('rect'));
    const idx = barRects.indexOf(rect as SVGRectElement);
    if (idx >= 0 && entries[idx]) {
      drilldownSource = entries[idx].source;
    }
  }

  // Phase 132 — exact-value hover readout, sharing the host div with the
  // delegated click above. `barX` + `ruleX` produce exactly one <rect>
  // per bar in input order, so the DOM index maps onto `entries`. No
  // Observable Plot `tip` is enabled here — that is what swallowed the
  // click in Phase 122d.1; this hover path is independent of it.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onHostMove(ev: MouseEvent): void {
    const idx = markIndexFromEvent(ev.target, 'rect');
    if (idx === null || !entries[idx]) {
      readout = HIDDEN_READOUT;
      return;
    }
    const e = entries[idx];
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: e.source,
      rows: [
        { label: m.cells_revact_readout_revisions(), value: fmtValue(e.revisions) },
        { label: m.cells_revact_readout_articles(), value: fmtValue(e.articlesAffected) }
      ],
      hint: m.cells_revact_readout_hint()
    };
  }

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  const exportRows = $derived<ExportRow[]>(
    entries.map((e) => ({
      source: e.source,
      revisions: e.revisions,
      articles_affected: e.articlesAffected
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'revision_activity',
      scope,
      scopeId,
      windowStart,
      windowEnd
    },
    howToRead: composeHowToRead('revision_activity', {}),
    rows: exportRows,
    columns: ['source', 'revisions', 'articles_affected']
  });
  const exportFilenameParts = $derived([
    'revision-activity',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="rev-cell" aria-labelledby="rev-title-{scopeId}" bind:this={cellEl}>
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if entries.length > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>

  {#if revisionQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_revact_loading()}</p>
  {:else if revisionQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={revisionQ.data} {ctx} />
  {:else if revisionQ.isError || revisionQ.data?.kind === 'network-error'}
    <p class="muted">{m.cells_revact_error()}</p>
  {:else if entries.length === 0}
    <CellEmptyState />
  {:else}
    <p class="click-hint" aria-hidden="true">
      <span class="click-hint-icon">↻</span>
      {m.cells_revact_click_hint()}
    </p>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_revact_plot_aria()}
      onclick={onHostClick}
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
  {/if}
</section>

<!-- Phase 122d.1 drill-down — opens articles for the clicked source's
     revisions in the active window. -->
{#if drilldownSource}
  <ArticleListModal
    open={drilldownSource !== null}
    title={m.cells_revact_drilldown_title({ source: drilldownSource })}
    {ctx}
    {windowStart}
    {windowEnd}
    onClose={() => (drilldownSource = null)}
    config={{
      mode: 'revisions-articles',
      scope: 'source',
      scopeId: drilldownSource
    }}
  />
{/if}

<style>
  .rev-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .plot-host {
    width: 100%;
    min-height: 160px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  /* Pure-CSS click affordance + hover — no JS mouse listeners, so no
     sticky-tooltip class of bug. The delegated click handler is the
     only JS interaction. */
  .plot-host :global(svg [aria-label='bar'] rect),
  .plot-host :global(svg rect) {
    cursor: pointer;
    transition: opacity var(--motion-duration-instant) var(--motion-ease-standard);
  }
  .plot-host :global(svg rect:hover) {
    opacity: 0.8;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
  .click-hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    margin: 0;
    font-style: italic;
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .click-hint-icon {
    font-style: normal;
    color: rgba(154, 143, 184, 0.95);
  }
</style>
