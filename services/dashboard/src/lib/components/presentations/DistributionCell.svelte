<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
  // EDA × distribution cell (Phase 107).
  // Per-scope histogram + quantile summary backed by
  // `GET /api/v1/metrics/{name}/distribution` (Gold) or
  // `GET /api/v1/silver/aggregations/{type}` (Silver, Phase 111).
  // Renders a histogram with Observable Plot, lazy-imported so Plot lands
  // in its own chunk only when the user picks this view (Brief §7 bundle
  // budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    metricDistributionQuery,
    silverAggregationQuery,
    type DistributionResponseDto,
    type SilverAggregationResponseDto,
    type SilverAggregationType,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportRow, type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import { isIntegerMetric } from '$lib/presentations/metric-presentation';
  import { cyclicMetricAxis } from '$lib/presentations/metric-axis';
  import { locale } from '$lib/state/locale.svelte';
  import {
    fmtValue,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import {
    buildPlotRows,
    isDegenerate,
    computePlotDomain,
    fmtBinRange
  } from '$lib/presentations/distribution-cell-internals';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import CellLoadingState from '$lib/components/base/CellLoadingState.svelte';
  import { metricLabel, metricSubjectAndModel } from '$lib/state/labels.svelte';
  import { useProbeLabels } from '$lib/presentations/use-probe-labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    metricName,
    metadataFilter,
    dataLayer = 'gold',
    sources = [],
    composition,
    bins,
    reportExtent,
    sharedDomains,
    axisScaleState,
    configOverridden
  }: PresentationCellProps = $props();

  // Phase 131 — configurable histogram bin count (default 30, BFF-clamped to
  // [1, 200]). Threaded from PanelControls via the Panel state.
  // Phase 148g — a cyclic metric (publication_hour / publication_weekday) bins
  // one bar per value (24 / 7) and labels its axis with real hours / weekday
  // names; a non-cyclic metric keeps the configurable bin count. These are Gold
  // temporal metrics (no Silver aggregation), so keying on `metricName` is exact.
  const cyclicAxis = $derived(cyclicMetricAxis(metricName));
  const activeBins = $derived(cyclicAxis ? cyclicAxis.bins : (bins ?? 30));
  const metricDisplay = $derived(metricLabel(metricName)); // friendly display label

  // Phase 122i revision (C6). Soft methodology note when the
  // distribution cell aggregates over multiple sources.
  const showMergedNote = $derived(composition === 'merged' && sources.length > 1);

  // Map Gold metric names to the closest Silver aggregation type.
  // Falls back to `word_count` — the universally available Silver field.
  const GOLD_TO_SILVER: Record<string, SilverAggregationType> = {
    word_count: 'word_count',
    entity_count: 'raw_entity_count'
  };
  let silverAggType = $derived<SilverAggregationType>(
    (GOLD_TO_SILVER[metricName] as SilverAggregationType | undefined) ?? 'word_count'
  );

  // Phase 148e — unified cell title. Eyebrow = presentation; subject = metric
  // (model split into its own dimmed slot); scope = resolved probe/source label
  // (no raw probe id leak); silver layer surfaces as a tail badge.
  const probeLabels = useProbeLabels(() => ctx);
  const titleMetricName = $derived(dataLayer === 'silver' ? silverAggType : metricName);
  const titleSubjectModel = $derived(metricSubjectAndModel(titleMetricName));
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_distribution_label(),
    subject: { kind: 'single', label: titleSubjectModel.subject },
    model: titleSubjectModel.model,
    scope: { kind: 'single', label: probeLabels.labelFor(scopeId) },
    qualifiers:
      dataLayer === 'silver'
        ? [
            {
              label: m.cells_dist_silver_badge(),
              tone: 'layer',
              title: m.cells_dist_silver_badge_aria()
            }
          ]
        : [],
    idSeed: `dist-title-${metricName}`
  });

  // Phase 133 A — integer-valued metrics (counts, cyclic ordinals) render
  // their histogram bin edges + axis ticks as integers. The equal-width bins
  // otherwise land on fractional boundaries (e.g. `34.133`) that are
  // meaningless for an integer quantity and misread as `34133` by a reader
  // using "." as a thousands separator. The article `count` is unaffected.
  const integerValued = $derived(
    isIntegerMetric(dataLayer === 'silver' ? silverAggType : metricName)
  );

  const distQ = createQuery<
    QueryOutcome<DistributionResponseDto>,
    Error,
    QueryOutcome<DistributionResponseDto>
  >(() => {
    const o = metricDistributionQuery(ctx, metricName, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      metadataFilter,
      bins: activeBins
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver'
    };
  });

  const silverDistQ = createQuery<
    QueryOutcome<SilverAggregationResponseDto>,
    Error,
    QueryOutcome<SilverAggregationResponseDto>
  >(() => {
    const useSilver = dataLayer === 'silver' && scope === 'source';
    const o = silverAggregationQuery(ctx, silverAggType, {
      sourceId: scopeId,
      start: windowStart,
      end: windowEnd
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: useSilver
    };
  });

  // Normalised distribution data regardless of layer. `clampedUpper` /
  // `overflowCount` are the Phase-133-B outlier-robust binning disclosure
  // (Gold only — the Silver aggregation path carries neither, so its bins
  // are unclamped).
  let activeDist = $derived.by(() => {
    if (dataLayer === 'silver') {
      if (silverDistQ.data?.kind !== 'success') return null;
      const d = silverDistQ.data.data.distribution ?? null;
      if (!d) return null;
      return {
        bins: d.bins,
        summary: d.summary,
        clampedUpper: null as number | null,
        overflowCount: 0
      };
    }
    if (distQ.data?.kind !== 'success') return null;
    const d = distQ.data.data;
    return {
      bins: d.bins,
      summary: d.summary,
      clampedUpper: d.clampedUpper as number | null,
      overflowCount: d.overflowCount
    };
  });

  let isPending = $derived(dataLayer === 'silver' ? silverDistQ.isPending : distQ.isPending);
  let refusalData = $derived(
    dataLayer === 'silver'
      ? silverDistQ.data?.kind === 'refusal'
        ? silverDistQ.data
        : null
      : distQ.data?.kind === 'refusal'
        ? distQ.data
        : null
  );
  let isNetworkError = $derived(
    dataLayer === 'silver'
      ? silverDistQ.isError || silverDistQ.data?.kind === 'network-error'
      : distQ.isError || distQ.data?.kind === 'network-error'
  );

  // Phase 124 — report the value-axis extent so PanelHost can union it into
  // the shared domain. Report the ROBUST upper bound (clampedUpper), not the
  // raw max, so one cell's outlier does not stretch every shared-axis cell
  // (Phase 133 B).
  $effect(() => {
    if (!reportExtent) return;
    const d = activeDist;
    if (d && d.summary.count > 0)
      reportExtent('value', [d.summary.min, d.clampedUpper ?? d.summary.max]);
    else reportExtent('value', null);
  });

  // Whether a shared (union) x-axis is in force (Phase 124). When it is,
  // the visible domain spans every cell's extent, so a per-cell overflow
  // bar drawn just past THIS cell's robust bound would land mid-range and
  // read as a normal in-range bar contradicting its "above the bound"
  // caption (Issue 1). Under a shared axis we therefore disclose the
  // overflow only in the caption, not as a bar.
  const sharedXActive = $derived(!!sharedDomains?.value);

  // Histogram rows (in-range bins + an explicit overflow bar when the domain was
  // clamped on a free axis) + degenerate-distribution detection + x-domain are
  // pure functions in `distribution-cell-internals.ts`.
  const plotRows = $derived.by(() => buildPlotRows(activeDist, integerValued, sharedXActive));
  const degenerate = $derived(isDegenerate(plotRows));
  const degenerateValue = $derived(degenerate ? (plotRows[0]?.lower ?? null) : null);
  const plotDomain = $derived(computePlotDomain(plotRows, degenerate));

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const data = activeDist;
    const rows = plotRows;
    // Phase 124 — when the panel is on a shared axis, apply the union domain
    // to x so identical values plot at identical positions across cells.
    const sharedX = sharedDomains?.value;
    if (!host || !data) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth,
        height: 242,
        marginLeft: 56,
        marginBottom: 36,
        x: {
          label: metricDisplay,
          grid: false,
          // Phase 148g — cyclic metrics get real hour / weekday tick labels at
          // integer positions; otherwise the generic integer/numeric axis.
          ...(cyclicAxis
            ? {
                ticks: cyclicAxis.ticks,
                tickFormat: (v: number) => cyclicAxis.format(v, locale())
              }
            : integerValued
              ? { tickFormat: 'd' }
              : {}),
          ...(sharedX ? { domain: [...sharedX] } : plotDomain ? { domain: plotDomain } : {})
        },
        y: { label: m.cells_dist_axis_articles(), grid: true, tickFormat: 'd' },
        marks: [
          Plot.rectY(rows, {
            x1: (d: { center: number; rw: number }) => d.center - d.rw,
            x2: (d: { center: number; rw: number }) => d.center + d.rw,
            y: 'count',
            // Overflow bar (values above the robust upper bound) gets a
            // distinct amber fill so it never reads as an ordinary bin.
            fill: (d: { overflow: boolean }) =>
              d.overflow ? 'rgba(232, 168, 80, 0.45)' : 'rgba(82, 131, 184, 0.55)',
            stroke: (d: { overflow: boolean }) =>
              d.overflow ? 'rgba(232, 168, 80, 0.95)' : 'rgba(82, 131, 184, 0.95)'
          }),
          Plot.ruleX([data.summary.median], { stroke: '#e0a050', strokeWidth: 1.5 }),
          Plot.ruleX([data.summary.p25, data.summary.p75], {
            stroke: '#e0a050',
            strokeDasharray: '3,2',
            strokeOpacity: 0.7
          }),
          Plot.ruleY([0])
        ]
      });
      // Plot returns a detached node we own; the host div is a stable container.
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(sanitizePlotA11y(next as unknown as HTMLElement));
      plotEl = next as unknown as HTMLElement;
    })();
  });

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  function fmt(n: number): string {
    if (!Number.isFinite(n)) return '—';
    // ADR-038 — an integer value renders without decimals (`image_count = 3`,
    // not `3.000`, which a `.`-thousands reader misreads). Genuinely fractional
    // values (e.g. an integer metric's mean) keep their decimals.
    if (Number.isInteger(n)) return String(n);
    return Math.abs(n) >= 100 ? n.toFixed(0) : n.toFixed(3);
  }

  // Phase 132 — exact-value hover readout. The histogram's only <rect>
  // marks are the bars (the median/quartile rules are <line>s), and
  // `Plot.rectY` renders one rect per bar in input order, so the
  // DOM-order index maps directly onto `plotRows` (bins + overflow).
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function onPlotMove(ev: MouseEvent): void {
    const rows = plotRows;
    const idx = markIndexFromEvent(ev.target, 'rect');
    if (idx === null || !rows[idx]) {
      readout = HIDDEN_READOUT;
      return;
    }
    const b = rows[idx];
    readout = {
      visible: true,
      x: ev.clientX,
      y: ev.clientY,
      title: metricLabel(exportMetricName),
      rows: [
        {
          label: m.cells_dist_readout_range(),
          // Phase 148g — a cyclic bin IS one hour / weekday: show that label, not
          // an integer range, so the readout matches the axis.
          value: cyclicAxis
            ? cyclicAxis.format(Math.round(b.center), locale())
            : b.overflow
              ? `> ${fmtBinRange(b.lower, b.lower, integerValued)}`
              : fmtBinRange(b.lower, b.upper, integerValued)
        },
        { label: m.cells_dist_readout_articles(), value: fmtValue(b.count) }
      ]
    };
  }

  // Phase 131 — export + how-to-read.
  const exportRows = $derived<ExportRow[]>(
    (activeDist?.bins ?? []).map((b) => ({
      lower: b.lower,
      upper: b.upper,
      count: b.count
    }))
  );
  const exportMetricName = $derived(dataLayer === 'silver' ? silverAggType : metricName);
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'distribution',
      metric: exportMetricName,
      scope,
      scopeId,
      layer: dataLayer,
      windowStart,
      windowEnd,
      bins: activeBins
    },
    summary: activeDist
      ? {
          n: activeDist.summary.count,
          min: activeDist.summary.min,
          p05: activeDist.summary.p05,
          p25: activeDist.summary.p25,
          median: activeDist.summary.median,
          mean: activeDist.summary.mean,
          p75: activeDist.summary.p75,
          p95: activeDist.summary.p95,
          max: activeDist.summary.max
        }
      : undefined,
    howToRead: composeHowToRead('distribution', {
      bins: activeBins,
      scales: axisScaleState,
      configOverridden
    }),
    rows: exportRows,
    columns: ['lower', 'upper', 'count']
  });
  const exportFilenameParts = $derived([
    'distribution',
    exportMetricName,
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="dist-cell" aria-labelledby="dist-title-{metricName}" bind:this={cellEl}>
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if activeDist && activeDist.summary.count > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>

  {#if dataLayer === 'silver' && scope !== 'source'}
    <p class="muted">{m.cells_dist_silver_narrow()}</p>
  {:else if isPending}
    <CellLoadingState label={m.cells_dist_loading()} />
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_dist_error()}</p>
  {:else if activeDist && activeDist.summary.count === 0}
    <CellEmptyState label={metricDisplay} />
  {:else if activeDist}
    {@const s = activeDist.summary}
    {#if showMergedNote}
      {@const note = methodologyNotes.alephMergedDistribution(sources.length)}
      <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_dist_plot_aria({ metric: metricDisplay })}
      onmousemove={onPlotMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    {#if degenerate && degenerateValue != null}
      {@const degen = s.count === 1 ? m.cells_dist_degenerate_one : m.cells_dist_degenerate_other}
      <p class="overflow-note">
        {degen({ count: fmtValue(s.count), metric: metricDisplay, value: fmt(degenerateValue) })}
      </p>
    {/if}
    {#if activeDist.overflowCount > 0 && activeDist.clampedUpper != null}
      {@const ovf =
        activeDist.overflowCount === 1 ? m.cells_dist_overflow_one : m.cells_dist_overflow_other}
      <p class="overflow-note">
        {ovf({
          bound: fmtBinRange(activeDist.clampedUpper, activeDist.clampedUpper, integerValued),
          count: fmtValue(activeDist.overflowCount),
          amber: sharedXActive ? '' : m.cells_dist_overflow_amber(),
          max: fmt(activeDist.summary.max)
        })}
      </p>
    {/if}
    <dl class="summary" aria-label={m.cells_dist_summary_aria()}>
      <div>
        <dt>n</dt>
        <dd>{s.count}</dd>
      </div>
      <div>
        <dt>min</dt>
        <dd>{fmt(s.min)}</dd>
      </div>
      <div>
        <dt>p05</dt>
        <dd>{fmt(s.p05)}</dd>
      </div>
      <div>
        <dt>p25</dt>
        <dd>{fmt(s.p25)}</dd>
      </div>
      <div>
        <dt>median</dt>
        <dd>{fmt(s.median)}</dd>
      </div>
      <div>
        <dt>mean</dt>
        <dd>{fmt(s.mean)}</dd>
      </div>
      <div>
        <dt>p75</dt>
        <dd>{fmt(s.p75)}</dd>
      </div>
      <div>
        <dt>p95</dt>
        <dd>{fmt(s.p95)}</dd>
      </div>
      <div>
        <dt>max</dt>
        <dd>{fmt(s.max)}</dd>
      </div>
    </dl>
  {/if}
</section>

<style>
  .dist-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .plot-host {
    width: 100%;
    min-height: 242px;
  }

  .overflow-note {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  /* Plot's default text fills are dark; coerce to muted on the AĒR theme. */
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .plot-host :global(svg) {
    background: transparent;
  }

  .summary {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(72px, 1fr));
    gap: var(--space-2) var(--space-4);
    margin: 0;
    padding: var(--space-3) 0 0;
    border-top: 1px solid var(--color-border);
  }

  .summary div {
    display: flex;
    flex-direction: column;
  }

  .summary dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .summary dd {
    margin: 0;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
