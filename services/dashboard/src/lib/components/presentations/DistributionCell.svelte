<script lang="ts">
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
  import {
    fmtValue,
    markIndexFromEvent,
    HIDDEN_READOUT,
    type ReadoutState
  } from '$lib/presentations/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import HowToRead from './HowToRead.svelte';

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
  const activeBins = $derived(bins ?? 30);

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

  // Phase 133 A — integer-valued metrics (counts, cyclic ordinals) render
  // their histogram bin edges + axis ticks as integers. The equal-width bins
  // otherwise land on fractional boundaries (e.g. `34.133`) that are
  // meaningless for an integer quantity and misread as `34133` by a reader
  // using "." as a thousands separator. The article `count` is unaffected.
  const integerValued = $derived(
    isIntegerMetric(dataLayer === 'silver' ? silverAggType : metricName)
  );

  /** Bin-range readout label. For integer metrics the fractional binning
   *  boundary is rounded to the integer it brackets; a bin that collapses to
   *  a single integer (sub-unit bin width) shows that one value. */
  function fmtBinRange(lower: number, upper: number): string {
    if (integerValued) {
      const lo = Math.round(lower);
      const hi = Math.round(upper);
      return lo === hi ? String(lo) : `${lo} – ${hi}`;
    }
    return `${fmtValue(lower)} – ${fmtValue(upper)}`;
  }

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

  // Histogram rows = in-range bins + one explicit overflow bar (Phase
  // 133 B) when the domain was clamped, values fell beyond it, AND the
  // axis is per-cell (free). The bar sits one bin-width past
  // `clampedUpper`, styled distinctly; its count is never folded silently
  // into the last bin (the caption always discloses it).
  const plotRows = $derived.by(() => {
    const d = activeDist;
    if (!d)
      return [] as Array<{
        center: number;
        width: number;
        rw: number;
        lower: number;
        upper: number;
        count: number;
        overflow: boolean;
      }>;
    // Phase 133 (Issue 1) — `rw` is the half-width used for RENDERING. A
    // degenerate bin (lower == upper, the BFF's single-bin response for a
    // constant metric) has width 0, which draws a zero-width, un-hittable rect
    // and collapses the x-axis. Give it a nominal render width (1 unit for an
    // integer metric, 0.5 otherwise) so the bar is finite and centred on the
    // real value; `lower`/`upper` stay exact for the readout.
    const nominal = integerValued ? 1 : 0.5;
    const rows = d.bins.map((b) => {
      const width = b.upper - b.lower;
      return {
        center: (b.lower + b.upper) / 2,
        width,
        rw: (width || nominal) / 2,
        lower: b.lower,
        upper: b.upper,
        count: b.count,
        overflow: false
      };
    });
    const cu = d.clampedUpper;
    const last = rows[rows.length - 1];
    if (d.overflowCount > 0 && cu != null && last && !sharedXActive) {
      const w = last.width || 1;
      rows.push({
        center: cu + w / 2,
        width: w,
        rw: w / 2,
        lower: cu,
        upper: d.summary.max,
        count: d.overflowCount,
        overflow: true
      });
    }
    return rows;
  });

  // Phase 133 (Issue 1/2) — a degenerate distribution: every bin is zero-width,
  // i.e. every in-scope article shares one value (a constant metric like a
  // paywall flag that is always 0, or image_count that is always 3). Drives an
  // explicit x-domain (so the axis reads the real value, not an auto-domain "0")
  // and an honest caption instead of a misleading single full-width bar.
  const degenerate = $derived(plotRows.length > 0 && plotRows.every((r) => r.width === 0));
  const degenerateValue = $derived(degenerate ? (plotRows[0]?.lower ?? null) : null);
  const plotDomain = $derived.by<[number, number] | null>(() => {
    if (!degenerate) return null;
    const los = plotRows.map((r) => r.lower);
    const his = plotRows.map((r) => r.upper);
    return [Math.min(...los) - 1, Math.max(...his) + 1];
  });

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
        height: 220,
        marginLeft: 56,
        marginBottom: 36,
        x: {
          label: metricName,
          grid: false,
          ...(integerValued ? { tickFormat: 'd' } : {}),
          ...(sharedX ? { domain: [...sharedX] } : plotDomain ? { domain: plotDomain } : {})
        },
        y: { label: 'articles', grid: true, tickFormat: 'd' },
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
      // Observable Plot returns a detached SVG/figure node that we own; the
      // host div is a stable empty container, so a direct mount is safe.
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
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
      title: exportMetricName,
      rows: [
        {
          label: 'range',
          value: b.overflow ? `> ${fmtBinRange(b.lower, b.lower)}` : fmtBinRange(b.lower, b.upper)
        },
        { label: 'articles', value: fmtValue(b.count) }
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
  <header class="cell-header">
    <h3 id="dist-title-{metricName}" class="cell-title">
      <code>{dataLayer === 'silver' ? silverAggType : metricName}</code>
      <span class="muted">— distribution · <strong class="scope-name">{scopeId}</strong></span>
      {#if dataLayer === 'silver'}
        <span class="layer-badge silver" aria-label="Silver layer data">Ag</span>
      {/if}
    </h3>
    {#if activeDist && activeDist.summary.count > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver' && scope !== 'source'}
    <p class="muted">Narrow to a single source to view Silver-layer distribution.</p>
  {:else if isPending}
    <p class="muted" aria-busy="true">Loading distribution…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load distribution.</p>
  {:else if activeDist && activeDist.summary.count === 0}
    <CellEmptyState label={metricName} />
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
      aria-label="Histogram of {metricName}"
      onmousemove={onPlotMove}
      onmouseleave={() => (readout = HIDDEN_READOUT)}
    ></div>
    <CellReadout {readout} />
    {#if degenerate && degenerateValue != null}
      <p class="overflow-note">
        Constant value — all <strong>{fmtValue(s.count)}</strong>
        article{s.count === 1 ? '' : 's'} in scope share
        <strong>{metricName} = {fmt(degenerateValue)}</strong>. A distribution has no shape when
        there is no variation; the single bar marks that value.
      </p>
    {/if}
    {#if activeDist.overflowCount > 0 && activeDist.clampedUpper != null}
      <p class="overflow-note">
        Binned up to {fmtBinRange(activeDist.clampedUpper, activeDist.clampedUpper)} (robust upper bound)
        so outliers don't flatten the shape ·
        <strong>{fmtValue(activeDist.overflowCount)}</strong>
        article{activeDist.overflowCount === 1 ? '' : 's'} above it{sharedXActive
          ? ''
          : ' (amber bar)'} · true max {fmt(activeDist.summary.max)}
      </p>
    {/if}
    <dl class="summary" aria-label="Quantile summary">
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
    <HowToRead
      presentation="distribution"
      facts={{ bins: activeBins, scales: axisScaleState, configOverridden }}
    />
  {/if}
</section>

<style>
  .dist-cell {
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

  .layer-badge {
    font-size: 10px;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
  }

  .layer-badge.silver {
    background: rgba(126, 196, 160, 0.15);
    border: 1px solid #7ec4a0;
    color: #7ec4a0;
  }

  .plot-host {
    width: 100%;
    min-height: 220px;
  }

  .overflow-note {
    margin: var(--space-2) 0 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  .overflow-note strong {
    color: rgba(232, 168, 80, 0.95);
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

  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
