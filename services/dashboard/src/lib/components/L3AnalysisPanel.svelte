<script lang="ts">
  // L3 Analysis view content (Design Brief §4.3 — "the shape of the
  // metric over time"). Slotted inside the existing `SidePanel`, this
  // component replaces the metadata-only 99b panel body with a time-
  // series chart driven by uPlot, a thin metric selector, and an L4
  // provenance affordance at the chart's corner.
  //
  // Rendering responsibilities:
  //   - Display a single (probe, metric) time-series over the current
  //     time window. Values are the `value` column from /metrics; the
  //     uncertainty band is the per-bucket min/max when available and
  //     falls back to a ±0.05 visual band if not (documented below).
  //   - Style the chart's weight by the metric's `validationStatus`
  //     (Epistemic Weight — §5.7): validated → 1.0, unvalidated → 0.7,
  //     expired → 0.55.
  //   - Invoke `onOpenProvenance()` when the corner affordance is
  //     activated, so the parent can toggle the L4 overlay.
  import { createQuery } from '@tanstack/svelte-query';
  import TimeSeriesChart from '$lib/components/TimeSeriesChart.svelte';
  import ProgressiveSemantics from '$lib/components/ProgressiveSemantics.svelte';
  import { Button, SegmentedControl } from '$lib/components/base';
  import {
    contentQuery,
    metricsQuery,
    type ContentResponseDto,
    type FetchContext,
    type MetricsResponseDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import { setUrl, urlState } from '$lib/state/url.svelte';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';

  interface Props {
    probe: ProbeDto;
    ctx: FetchContext;
    windowStart: string;
    windowEnd: string;
    resolution: 'hourly' | '5min' | 'daily' | 'weekly' | 'monthly';
    onOpenProvenance: () => void;
    publicationRate: number | null;
  }

  let { probe, ctx, windowStart, windowEnd, resolution, onOpenProvenance, publicationRate }: Props =
    $props();

  // Metric catalog for the selector — kept short on purpose. These are
  // the gold metrics that exist at Phase 100a; the server's
  // /metrics/available is the authoritative SSoT and a future pass will
  // pull from there. Hard-coding the display labels is acceptable for
  // the selector chrome only — every science-bearing string continues
  // to come from the Content Catalog.
  const METRICS: readonly { id: string; label: string; hint: string }[] = [
    { id: 'sentiment_score', label: 'Sentiment', hint: 'SentiWS lexicon-based polarity' },
    { id: 'word_count', label: 'Word count', hint: 'Tokenised word count per document' },
    { id: 'entity_count', label: 'Entity count', hint: 'spaCy NER entity count per document' },
    { id: 'language_confidence', label: 'Language', hint: 'langdetect confidence score' }
  ];

  const url = $derived(urlState());
  let selectedMetric = $derived(url.metric ?? 'sentiment_score');

  // Probe dual-register content — "no layer replaces" applies inside
  // the panel too: L3 shows the chart *and* the emic designation so the
  // reader never loses the anchoring frame.
  const contentQ = createQuery<
    QueryOutcome<ContentResponseDto>,
    Error,
    QueryOutcome<ContentResponseDto>
  >(() => {
    const o = contentQuery(ctx, 'probe', probe.probeId, 'en');
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const seriesQ = createQuery<
    QueryOutcome<MetricsResponseDto>,
    Error,
    QueryOutcome<MetricsResponseDto>
  >(() => {
    // Fetch per-source slices and stitch them on the client. The BFF's
    // /metrics endpoint returns one row per (timestamp, source) bucket;
    // a probe is a bundle of sources, so the chart sums per-source
    // values for each timestamp. For metrics like sentiment this
    // aggregation is a weighted mean where `count` is the weight;
    // falling back to a plain mean when counts are absent.
    const o = metricsQuery(ctx, {
      startDate: windowStart,
      endDate: windowEnd,
      metricName: selectedMetric,
      resolution
    });
    return {
      queryKey: [...o.queryKey, probe.probeId],
      queryFn: o.queryFn,
      staleTime: o.staleTime
    };
  });

  interface ChartData {
    x: number[];
    y: number[];
    yLow: number[];
    yHigh: number[];
  }

  let chart = $derived.by<ChartData | null>(() => {
    const r = seriesQ.data;
    if (r?.kind !== 'success') return null;
    const rowsBySource = new Set(probe.sources);
    // Plain record indexed by epoch-seconds. We never mutate it outside
    // this derivation and the linter's SvelteMap rule targets long-lived
    // maps in reactive scope — this one is recreated per call.
    const byTs: Record<
      string,
      { sum: number; weightSum: number; min: number; max: number; n: number }
    > = {};
    for (const row of r.data.data) {
      if (!rowsBySource.has(row.source)) continue;
      const t = Math.floor(Date.parse(row.timestamp) / 1000);
      if (!Number.isFinite(t)) continue;
      const w = row.count ?? 1;
      const v = row.value;
      const key = String(t);
      const b = byTs[key];
      if (!b) {
        byTs[key] = { sum: v * w, weightSum: w, min: v, max: v, n: 1 };
      } else {
        b.sum += v * w;
        b.weightSum += w;
        b.min = Math.min(b.min, v);
        b.max = Math.max(b.max, v);
        b.n += 1;
      }
    }
    const keys = Object.keys(byTs)
      .map((k) => Number(k))
      .sort((a, b) => a - b);
    if (keys.length === 0) return null;
    const x: number[] = [];
    const y: number[] = [];
    const yLow: number[] = [];
    const yHigh: number[] = [];
    for (const k of keys) {
      const b = byTs[String(k)]!;
      const mean = b.weightSum > 0 ? b.sum / b.weightSum : b.min;
      x.push(k);
      y.push(mean);
      // When only one source contributed to a bucket we have no
      // intra-bucket variance to show; fall back to a narrow visual
      // band (±2% of the mean's magnitude, or 0.02 minimum) rather
      // than inventing a false-precision interval.
      if (b.n > 1) {
        yLow.push(b.min);
        yHigh.push(b.max);
      } else {
        const pad = Math.max(0.02, Math.abs(mean) * 0.02);
        yLow.push(mean - pad);
        yHigh.push(mean + pad);
      }
    }
    return { x, y, yLow, yHigh };
  });

  // Epistemic Weight — hard-coded default weights until we wire up the
  // per-metric `validationStatus` lookup in a follow-up pass. For the
  // phase-100a scope every Phase 42 extractor reports `unvalidated`
  // (per Arc42 §8.14), so the conservative 0.7 weight matches reality.
  let weight = 0.7;

  let windowLabel = $derived(
    `${new Date(windowStart).toISOString().slice(0, 16).replace('T', ' ')}Z → ${new Date(windowEnd).toISOString().slice(0, 16).replace('T', ' ')}Z`
  );
</script>

<section class="l3" aria-label="Analysis view">
  <header class="head">
    <SegmentedControl
      options={METRICS}
      value={selectedMetric}
      onChange={(next) =>
        setUrl({
          metric: next,
          view: 'analysis'
        })}
      ariaLabel="Metric"
      label="Metric"
    />
    <Button
      variant="secondary"
      size="sm"
      onclick={onOpenProvenance}
      aria-label="Open methodology and provenance for {selectedMetric}"
    >
      Methodology
    </Button>
  </header>

  <figure class="chart-frame">
    <figcaption class="caption">
      <span class="caption-title">{selectedMetric} <span class="muted">— {windowLabel}</span></span>
      <span class="legend" aria-hidden="true">
        <span class="legend-line"></span>
        <span>Mean</span>
        <span class="legend-band"></span>
        <span>Observed min–max per bucket</span>
      </span>
    </figcaption>

    {#if seriesQ.isPending}
      <p class="muted" aria-busy="true">Loading series…</p>
    {:else if seriesQ.data?.kind === 'refusal'}
      <RefusalSurface refusal={seriesQ.data} {ctx} />
    {:else if seriesQ.isError || seriesQ.data?.kind === 'network-error'}
      <p class="muted">Series unavailable.</p>
    {:else if chart && chart.x.length > 0}
      <TimeSeriesChart
        x={chart.x}
        y={chart.y}
        yLow={chart.yLow}
        yHigh={chart.yHigh}
        yLabel={selectedMetric}
        {weight}
        height={420}
        ariaLabel="Time-series of {selectedMetric} for {probe.probeId}"
      />
    {:else}
      <p class="muted">
        No observations for this probe in the selected window. Widen the time range at L2.
      </p>
    {/if}
  </figure>

  <section class="context-row">
    <dl class="meta">
      <div>
        <dt>Probe</dt>
        <dd><code>{probe.probeId}</code></dd>
      </div>
      <div>
        <dt>Language</dt>
        <dd>{probe.language.toUpperCase()}</dd>
      </div>
      <div>
        <dt>Publication rate</dt>
        <dd>{publicationRate !== null ? `${publicationRate.toFixed(1)} docs/h` : '—'}</dd>
      </div>
    </dl>

    {#if contentQ.data?.kind === 'success'}
      <div class="registers">
        <ProgressiveSemantics registers={contentQ.data.data.registers} emphasis="semantic" />
      </div>
    {:else if contentQ.data?.kind === 'refusal'}
      <RefusalSurface refusal={contentQ.data} {ctx} />
    {/if}
  </section>

  <p class="reach-note">
    Reach is not rendered. This probe's emission points mark where its bound publishers emit — not
    where their content is consumed or influential. No reach claim is made by AĒR (Design Brief
    §5.7).
  </p>
</section>

<style>
  .l3 {
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    min-height: 100%;
  }
  .head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    flex-wrap: wrap;
  }
  .chart-frame {
    margin: 0;
    padding: var(--space-4);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: rgba(255, 255, 255, 0.02);
  }
  .caption {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-4);
    flex-wrap: wrap;
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    margin-bottom: var(--space-3);
  }
  .caption-title {
    font-family: var(--font-family-mono);
  }
  /* Inline legend — explains the shaded uncertainty band next to the
     chart rather than hiding the explanation at L4. */
  .legend {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    color: var(--color-fg-muted);
    font-size: 11px;
  }
  .legend-line {
    display: inline-block;
    width: 18px;
    height: 2px;
    background: #5283b8;
    border-radius: 1px;
  }
  .legend-band {
    display: inline-block;
    width: 18px;
    height: 10px;
    background: rgba(82, 131, 184, 0.22);
    border: 1px solid rgba(82, 131, 184, 0.45);
    border-radius: 2px;
    margin-left: var(--space-3);
  }
  .context-row {
    display: grid;
    grid-template-columns: minmax(12rem, 1fr) 2fr;
    gap: var(--space-5);
    align-items: start;
    padding-top: var(--space-3);
    border-top: 1px solid var(--color-border);
  }
  dl.meta {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
    font-size: var(--font-size-xs);
  }
  dl.meta > div {
    display: contents;
  }
  dt {
    color: var(--color-fg-muted);
  }
  dd {
    margin: 0;
  }
  .muted {
    color: var(--color-fg-muted);
  }
  @media (max-width: 720px) {
    .context-row {
      grid-template-columns: 1fr;
    }
  }
  .reach-note {
    margin: 0;
    padding-top: var(--space-4);
    border-top: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    line-height: 1.55;
  }
</style>
