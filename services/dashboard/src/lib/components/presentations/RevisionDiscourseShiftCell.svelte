<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
  // Episteme × Silent-Edit Discourse Shift — Phase 122d.3.
  //
  // Goes one level deeper than `RevisionTimelineCell` (which charts HOW
  // OFTEN a source edits): this cell charts what the edits DO to the
  // discourse — the mean sentiment delta per source over the window, with
  // the semantic (topic) shift + entity churn surfaced on hover. Deltas
  // are re-extracted from each archived snapshot version (provisional
  // backbones; disclosed in the how-to-read note). One line per source;
  // the resolution control buckets the trajectory (calendar grain only —
  // sub-daily would expose CDX jitter, not signal).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    revisionDiscourseShiftQuery,
    type RevisionDiscourseShiftResponseDto,
    type RevisionActivityResolution,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/article/ArticleListModal.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportPayload, type ExportRow } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import {
    fmtValue,
    fmtTimestamp,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import HowToRead from './HowToRead.svelte';
  import { m } from '$lib/paraglide/messages.js';

  let { ctx, scope, scopeId, windowStart, windowEnd, resolution }: PresentationCellProps = $props();

  let drilldown = $state<{ source: string; bucketStart: string; bucketEnd: string } | null>(null);

  const activeResolution = $derived<RevisionActivityResolution>(
    resolution === 'weekly' || resolution === 'monthly' ? resolution : 'daily'
  );

  const shiftQ = createQuery<
    QueryOutcome<RevisionDiscourseShiftResponseDto>,
    Error,
    QueryOutcome<RevisionDiscourseShiftResponseDto>
  >(() => {
    const o = revisionDiscourseShiftQuery(ctx, {
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

  type Point = {
    bucket: Date;
    source: string;
    avgSentimentDelta: number;
    netSentimentDrift: number;
    avgTopicShift: number;
    entitiesAddedTotal: number;
    entitiesRemovedTotal: number;
    editsWithDeltas: number;
  };
  const points = $derived<Point[]>(
    shiftQ.data?.kind === 'success'
      ? (shiftQ.data.data.entries ?? []).map((e) => ({
          bucket: new Date(e.bucket),
          source: e.source,
          avgSentimentDelta: e.avgSentimentDelta,
          netSentimentDrift: e.netSentimentDrift,
          avgTopicShift: e.avgTopicShift,
          entitiesAddedTotal: e.entitiesAddedTotal,
          entitiesRemovedTotal: e.entitiesRemovedTotal,
          editsWithDeltas: e.editsWithDeltas
        }))
      : []
  );

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
        marginLeft: 52,
        marginBottom: 40,
        x: { label: null, type: 'time', grid: false },
        // Sentiment delta is signed and centred on zero — a flat line at 0
        // means edits did not move sentiment. The zero rule is the baseline.
        y: { label: m.cells_revds_axis_sentiment(), grid: true, nice: true },
        color: { legend: true },
        marks: [
          Plot.line(rows, {
            x: 'bucket',
            y: 'avgSentimentDelta',
            stroke: 'source',
            strokeWidth: 1.6,
            curve: 'monotone-x'
          }),
          Plot.dot(rows, {
            x: 'bucket',
            y: 'avgSentimentDelta',
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
      host.appendChild(sanitizePlotA11y(next as unknown as HTMLElement));
      plotEl = next as unknown as HTMLElement;
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

  // Phase 132 hover readout — shares the host div with the delegated click.
  // No Plot `tip` is enabled (it would swallow the click). The hovered
  // circle's `ownerSVGElement` scopes the index lookup to the data dots.
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
        { label: m.cells_revds_readout_bucket(), value: fmtTimestamp(pt.bucket.getTime() / 1000) },
        { label: m.cells_revds_readout_sentiment_mean(), value: fmtValue(pt.avgSentimentDelta) },
        { label: m.cells_revds_readout_net_drift(), value: fmtValue(pt.netSentimentDrift) },
        { label: m.cells_revds_readout_topic_shift(), value: fmtValue(pt.avgTopicShift) },
        {
          label: m.cells_revds_readout_entities(),
          value: `${pt.entitiesAddedTotal} / ${pt.entitiesRemovedTotal}`
        },
        { label: m.cells_revds_readout_edits(), value: fmtValue(pt.editsWithDeltas) }
      ],
      hint: m.cells_revds_readout_hint()
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
      avgSentimentDelta: p.avgSentimentDelta,
      netSentimentDrift: p.netSentimentDrift,
      avgTopicShift: p.avgTopicShift,
      entitiesAddedTotal: p.entitiesAddedTotal,
      entitiesRemovedTotal: p.entitiesRemovedTotal,
      editsWithDeltas: p.editsWithDeltas
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'revision_discourse_shift',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      resolution: activeResolution
    },
    howToRead: composeHowToRead('revision_discourse_shift', {}),
    rows: exportRows,
    columns: [
      'bucket',
      'source',
      'avgSentimentDelta',
      'netSentimentDrift',
      'avgTopicShift',
      'entitiesAddedTotal',
      'entitiesRemovedTotal',
      'editsWithDeltas'
    ]
  });
  const exportFilenameParts = $derived([
    'discourse-shift',
    scope === 'source' ? scopeId : 'probe',
    activeResolution
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="rev-cell" aria-labelledby="rev-ds-title-{scopeId}" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="rev-ds-title-{scopeId}" class="cell-title">
      <span>{m.cells_revds_title()}</span>
      <span class="muted">
        — <strong class="scope-name">{scopeId}</strong> · <code>{activeResolution}</code>
      </span>
    </h3>
    {#if points.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if shiftQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_revds_loading()}</p>
  {:else if shiftQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={shiftQ.data} {ctx} />
  {:else if shiftQ.isError || shiftQ.data?.kind === 'network-error'}
    <p class="muted">{m.cells_revds_error()}</p>
  {:else if points.length === 0}
    <p class="muted">{m.cells_revds_empty()}</p>
  {:else}
    <p class="click-hint" aria-hidden="true">
      <span class="click-hint-icon">↻</span>
      {m.cells_revds_click_hint()}
    </p>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_revds_plot_aria()}
      onclick={onHostClick}
      onmousemove={onHostMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    <HowToRead presentation="revision_discourse_shift" facts={{}} />
  {/if}
</section>

{#if drilldown}
  <ArticleListModal
    open={drilldown !== null}
    title={m.cells_revds_drilldown_title({ source: drilldown.source })}
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
