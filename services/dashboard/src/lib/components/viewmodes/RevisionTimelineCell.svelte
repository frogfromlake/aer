<script lang="ts">
  // Episteme × Silent-Edit Observability — Phase 122d.0 (ADR-032).
  //
  // Diachronic per-source line chart over `aer_gold.article_revisions`.
  // The Episteme cell buckets the window on a calendar grain (daily /
  // weekly / monthly per the shared Resolution control); one line per
  // source. The synchronic counterpart is `RevisionActivityCell` (Aleph)
  // which collapses the same signal to one bucket per source.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    revisionActivityQuery,
    type RevisionActivityResponseDto,
    type RevisionActivityResolution,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/lanes/ArticleListModal.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload, type ExportRow } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import {
    fmtValue,
    fmtTimestamp,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/viewmodes/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import HowToRead from './HowToRead.svelte';

  let { ctx, scope, scopeId, windowStart, windowEnd, resolution }: ViewModeCellProps = $props();

  // Phase 122d.1 — drill-down: click a bucket point to open the article
  // list filtered to that source's revisions in the bucket's window.
  let drilldown = $state<{ source: string; bucketStart: string; bucketEnd: string } | null>(null);

  // The shared Resolution control surfaces `'5min' | 'hourly' | 'daily' |
  // 'weekly' | 'monthly'`. The revision endpoint only supports
  // calendar-grain bucketing for the timeline (sub-day buckets would
  // expose CDX-API jitter rather than analytical signal), so any
  // sub-daily request collapses to daily.
  const activeResolution = $derived<RevisionActivityResolution>(
    resolution === 'weekly' || resolution === 'monthly' ? resolution : 'daily'
  );

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
      resolution: activeResolution
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  type Point = { bucket: Date; source: string; revisions: number };
  const points = $derived<Point[]>(
    revisionQ.data?.kind === 'success'
      ? (revisionQ.data.data.entries ?? []).map((e) => ({
          bucket: new Date(e.bucket),
          source: e.source,
          revisions: e.revisions
        }))
      : []
  );

  // Phase 122d.1 (Run-3) — Observable Plot restored for the chart LOOK
  // (time axis, grid, multi-source colour legend). Same fix as the
  // activity cell: the earlier click failures were `tip: true`'s
  // interaction overlay, not the rendering. Render WITHOUT tip and
  // bind clicks via a delegated SVG listener that maps the clicked
  // <circle> to its data point by DOM index. Plot's `dot` mark
  // renders one <circle> per `points` row in input order, so the
  // index ↔ points mapping is exact. Hover affordance is pure CSS.
  function bucketDurationMs(res: RevisionActivityResolution): number {
    switch (res) {
      case 'monthly':
        return 30 * 24 * 3_600_000;
      case 'weekly':
        return 7 * 24 * 3_600_000;
      default:
        return 24 * 3_600_000;
    }
  }

  function openPointDrilldown(idx: number): void {
    const pt = points[idx];
    if (!pt) return;
    const bucketStartMs = pt.bucket.getTime();
    drilldown = {
      source: pt.source,
      bucketStart: new Date(bucketStartMs).toISOString(),
      bucketEnd: new Date(bucketStartMs + bucketDurationMs(activeResolution)).toISOString()
    };
  }

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = points;
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth,
        height: 260,
        marginLeft: 48,
        marginBottom: 40,
        x: { label: null, type: 'time', grid: false },
        y: { label: 'revisions ↑', grid: true, nice: true },
        color: { legend: true },
        marks: [
          Plot.line(rows, {
            x: 'bucket',
            y: 'revisions',
            stroke: 'source',
            strokeWidth: 1.6,
            curve: 'monotone-x'
          }),
          Plot.dot(rows, {
            x: 'bucket',
            y: 'revisions',
            stroke: 'source',
            fill: 'var(--color-surface)',
            r: 4,
            strokeWidth: 2
          }),
          Plot.ruleY([0])
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;

      // ROOT-CAUSE FIX (Run-4): see RevisionActivityCell. Listener on
      // the stable `host` div; the clicked circle's `ownerSVGElement`
      // is the chart svg (NOT the colour-legend swatch svg that
      // Plot prepends inside the returned <figure>), so the index
      // lookup is scoped to the real data circles.
    })();
  });

  function onHostClick(ev: MouseEvent): void {
    const target = ev.target as Element | null;
    const circle = target?.closest('circle');
    if (!circle) return;
    const svg = (circle as SVGCircleElement).ownerSVGElement;
    if (!svg) return;
    const dots = Array.from(svg.querySelectorAll('circle'));
    const idx = dots.indexOf(circle as SVGCircleElement);
    if (idx >= 0 && points[idx]) openPointDrilldown(idx);
  }

  // Phase 132 — exact-value hover readout, sharing the host div with the
  // delegated click above. `Plot.dot` renders one <circle> per `points`
  // row in input order (the colour legend swatch lives in a separate
  // <svg> inside the returned <figure>, so the hovered circle's
  // `ownerSVGElement` scopes the lookup to the real data dots). No Plot
  // `tip` is enabled here — the click path stays intact.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onHostMove(ev: MouseEvent): void {
    const idx = markIndexFromEvent(ev.target, 'circle');
    if (idx === null || !points[idx]) {
      readout = HIDDEN_READOUT;
      return;
    }
    const pt = points[idx];
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: pt.source,
      rows: [
        { label: 'bucket', value: fmtTimestamp(pt.bucket.getTime() / 1000) },
        { label: 'revisions', value: fmtValue(pt.revisions) }
      ],
      hint: 'Click to see articles in this bucket'
    };
  }

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  const exportRows = $derived<ExportRow[]>(
    points.map((p) => ({
      bucket: p.bucket.toISOString(),
      source: p.source,
      revisions: p.revisions
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'revision_timeline',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      resolution: activeResolution
    },
    howToRead: composeHowToRead('revision_timeline', {}),
    rows: exportRows,
    columns: ['bucket', 'source', 'revisions']
  });
  const exportFilenameParts = $derived([
    'revision-timeline',
    scope === 'source' ? scopeId : 'probe',
    activeResolution
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="rev-cell" aria-labelledby="rev-tl-title-{scopeId}" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="rev-tl-title-{scopeId}" class="cell-title">
      <span>Revision timeline</span>
      <span class="muted">
        — <strong class="scope-name">{scopeId}</strong> · <code>{activeResolution}</code>
      </span>
    </h3>
    {#if points.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if revisionQ.isPending}
    <p class="muted" aria-busy="true">Loading revision timeline…</p>
  {:else if revisionQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={revisionQ.data} {ctx} />
  {:else if revisionQ.isError || revisionQ.data?.kind === 'network-error'}
    <p class="muted">Could not load revision timeline.</p>
  {:else if points.length === 0}
    <p class="muted">No silent-edit activity observed in this window.</p>
  {:else}
    <p class="click-hint" aria-hidden="true">
      <span class="click-hint-icon">↻</span> Click any point to see the articles edited in that bucket.
    </p>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Revisions over time per source. Click a point to view articles in that bucket."
      onclick={onHostClick}
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <HowToRead presentation="revision_timeline" facts={{}} />
  {/if}
</section>

<!-- Phase 122d.1 drill-down — opens articles for the clicked bucket. -->
{#if drilldown}
  <ArticleListModal
    open={drilldown !== null}
    title={`Articles edited — ${drilldown.source}`}
    {ctx}
    windowStart={drilldown.bucketStart}
    windowEnd={drilldown.bucketEnd}
    onClose={() => (drilldown = null)}
    config={{
      mode: 'revisions-articles',
      scope: 'source',
      scopeId: drilldown.source
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
  .cell-title code {
    font-family: var(--font-mono);
  }
  .plot-host {
    width: 100%;
    min-height: 260px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
  }
  /* Pure-CSS click affordance + hover on the dot marks. No JS mouse
     listeners → nothing can stick. The delegated click handler is the
     only JS interaction. */
  .plot-host :global(svg circle) {
    cursor: pointer;
    transition: r var(--motion-duration-instant) var(--motion-ease-standard);
  }
  .plot-host :global(svg circle:hover) {
    r: 7;
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
