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
  import CellExport from './CellExport.svelte';
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
        height: 240,
        marginLeft: 56,
        marginBottom: 36,
        x: { label: 'bucket', type: 'time' },
        y: { label: 'revisions', grid: true },
        color: { legend: true },
        marks: [
          Plot.line(rows, {
            x: 'bucket',
            y: 'revisions',
            stroke: 'source',
            strokeWidth: 1.4
          }),
          Plot.dot(rows, {
            x: 'bucket',
            y: 'revisions',
            stroke: 'source',
            r: 2.2
          }),
          Plot.ruleY([0])
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;

      // Phase 122d.1 — bucket-click drill-down. The dot marks are
      // small targets; we delegate click on the SVG and resolve the
      // nearest point by index.
      const svg = plotEl?.querySelector('svg');
      if (svg) {
        for (const dot of svg.querySelectorAll('circle')) {
          (dot as SVGCircleElement).style.cursor = 'pointer';
        }
        svg.addEventListener('click', (ev) => {
          const target = ev.target as Element | null;
          const circle = target?.closest('circle');
          if (!circle) return;
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const idx = (circle as any).__data__ as number | undefined;
          if (typeof idx !== 'number' || !rows[idx]) return;
          const pt = rows[idx];
          // Pick the bucket's matching window. For daily / weekly / monthly
          // we expand bucket_start → bucket_start + duration. Approximate
          // via the next bucket's start when available; otherwise add the
          // resolution's nominal length.
          const bucketStartMs = pt.bucket.getTime();
          const nextRow = rows[idx + 1];
          const bucketEndMs = nextRow
            ? nextRow.bucket.getTime()
            : bucketStartMs + bucketDurationMs(activeResolution);
          drilldown = {
            source: pt.source,
            bucketStart: new Date(bucketStartMs).toISOString(),
            bucketEnd: new Date(bucketEndMs).toISOString()
          };
        });
      }
    })();
  });

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
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Revisions over time. Click a point to view articles in that bucket."
    ></div>
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
    min-height: 240px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .plot-host :global(svg) {
    background: transparent;
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
</style>
