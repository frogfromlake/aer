<script lang="ts">
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
  import ArticleListModal from '$lib/components/lanes/ArticleListModal.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload, type ExportRow } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import {
    fmtValue,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/viewmodes/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import HowToRead from './HowToRead.svelte';

  let { ctx, scope, scopeId, windowStart, windowEnd }: ViewModeCellProps = $props();

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
        x: { label: 'revisions →', grid: true, nice: true },
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
      host.appendChild(next as unknown as HTMLElement);
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
        { label: 'revisions', value: fmtValue(e.revisions) },
        { label: 'articles', value: fmtValue(e.articlesAffected) }
      ],
      hint: 'Click to see articles'
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
  <header class="cell-header">
    <h3 id="rev-title-{scopeId}" class="cell-title">
      <span>Revision activity</span>
      <span class="muted">— <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if entries.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if revisionQ.isPending}
    <p class="muted" aria-busy="true">Loading revision activity…</p>
  {:else if revisionQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={revisionQ.data} {ctx} />
  {:else if revisionQ.isError || revisionQ.data?.kind === 'network-error'}
    <p class="muted">Could not load revision activity.</p>
  {:else if entries.length === 0}
    <p class="muted">
      No silent-edit activity observed in this window. Either Wayback CDX has no snapshots for these
      sources yet, or the publishers have not bumped their sitemap-lastmod inside the window.
    </p>
  {:else}
    <p class="click-hint" aria-hidden="true">
      <span class="click-hint-icon">↻</span> Click any bar to see the articles edited under that source.
    </p>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Revision counts per source. Click a bar to view its articles."
      onclick={onHostClick}
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <HowToRead presentation="revision_activity" facts={{}} />
  {/if}
</section>

<!-- Phase 122d.1 drill-down — opens articles for the clicked source's
     revisions in the active window. -->
{#if drilldownSource}
  <ArticleListModal
    open={drilldownSource !== null}
    title={`Articles edited — ${drilldownSource}`}
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
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }
  .cell-title {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    margin: 0;
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
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
  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
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
