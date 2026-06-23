<script lang="ts">
  import { sanitizePlotA11y } from '$lib/presentations/plot-a11y';
  // Episteme × topic-evolution cell (Phase 121).
  //
  // Renders an Observable Plot stream graph (stacked area) of BERTopic
  // topic volume over time. The Phase 120 BFF endpoint
  // (`GET /api/v1/topics/distribution`) returns one aggregate per query;
  // this cell composes a temporal series by issuing one query per
  // sub-window across the active range. Bucket count is bounded so the
  // request fan-out stays predictable; @tanstack/svelte-query caches
  // each sub-window independently so adjacent windows share work.
  //
  // Colour: Viridis ramp keyed by topic id. Per Phase 121 line 2764, the
  // palette is arbitrary — topic colour carries no valence. The outlier
  // class (`topicId == -1`) is rendered grey and labelled "uncategorised".
  //
  // Reduced motion: `prefers-reduced-motion: reduce` collapses the
  // stream-graph silhouette into a static stacked bar chart so animation
  // and large-area motion don't fire.
  import { createQueries } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    topicDistributionQuery,
    type TopicDistributionResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { useProbeLabels } from '$lib/presentations/use-probe-labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';
  import { JOINT_CORPUS_MIN_SOURCES } from '$lib/config/topic-thresholds';
  import {
    OUTLIER_COLOUR,
    computeBuckets,
    seriesId,
    seriesMetaValues,
    buildSeriesMeta,
    selectTopSeriesIds,
    collectLanguages,
    buildPlotRows,
    buildLegendEntries,
    type Bucket,
    type PlotRow
  } from '$lib/presentations/topic-evolution-internals';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    probeIds = [],
    sources = []
  }: PresentationCellProps = $props();

  // Phase 122i / ADR-034 — Joint-corpus methodology note when the
  // evolution is computed over ≥ JOINT_CORPUS_MIN_SOURCES sources.
  // Stream graphs over a joint corpus aggregate source-specific
  // framings into shared streams; the banner cites WP-005 §6.2.
  const isJointCorpus = $derived(sources.length >= JOINT_CORPUS_MIN_SOURCES);

  // Bucket boundaries — split [windowStart, windowEnd) into N equal sub-windows
  // (computeBuckets; falls back to the last 30 days when the panel is unbounded).
  // Date.now() is read at the call site so the derived recomputes correctly.
  let buckets = $derived.by<Bucket[]>(() => computeBuckets(windowStart, windowEnd, Date.now()));

  // createQueries is variadic by tuple in svelte-query's typings, which
  // does not express dynamic-length parallel queries cleanly. We cast
  // the result through `unknown` to a homogeneous result shape so the
  // call sites below can pattern-match on the discriminated outcome.
  type SubQueryResult = {
    isPending: boolean;
    isError: boolean;
    data: QueryOutcome<TopicDistributionResponseDto> | undefined;
  };
  const queries = createQueries(() => ({
    queries: buckets.map((b) => {
      const o = topicDistributionQuery(ctx, {
        scope,
        scopeId,
        start: b.start,
        end: b.end,
        probeIds: probeIds.length > 0 ? probeIds : undefined,
        includeOutlier: true
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime
      };
    })
  })) as unknown as SubQueryResult[];

  let isPending = $derived(queries.some((q) => q.isPending));
  // Refusal surfaces from any sub-window — they all hit the same gate.
  let refusalData = $derived.by(() => {
    for (const q of queries) {
      if (q.data?.kind === 'refusal') return q.data;
    }
    return null;
  });
  let isNetworkError = $derived(queries.some((q) => q.isError || q.data?.kind === 'network-error'));

  // Align each successful sub-window payload with its bucket (non-success
  // buckets dropped). `queries` is `buckets.map(...)`, so index i pairs the
  // i-th bucket with its query — preserving the original bucket→payload map.
  let alignedBuckets = $derived.by<{ bucket: Bucket; payload: TopicDistributionResponseDto }[]>(
    () => {
      const out: { bucket: Bucket; payload: TopicDistributionResponseDto }[] = [];
      for (let i = 0; i < queries.length; i++) {
        const q = queries[i]!;
        if (q.data?.kind !== 'success') continue;
        out.push({ bucket: buckets[i]!, payload: q.data.data });
      }
      return out;
    }
  );

  // Aggregate (language, topic) totals across all buckets, pick the stable
  // top-K named series + outlier, and enumerate the language partitions.
  let seriesMeta = $derived.by(() => buildSeriesMeta(alignedBuckets.map((a) => a.payload)));
  let topSeriesIds = $derived.by(() => selectTopSeriesIds(seriesMeta));
  let languages = $derived.by(() => collectLanguages(seriesMeta));
  let multiLang = $derived(languages.length > 1);

  // Per-bucket per-series stream rows (top-K + folded "other" + zero-backfill).
  let plotRows = $derived.by<PlotRow[]>(() =>
    buildPlotRows(alignedBuckets, topSeriesIds, seriesMeta)
  );

  // Detect prefers-reduced-motion (default false on SSR).
  let reducedMotion = $state(false);
  $effect(() => {
    if (typeof window === 'undefined') return;
    const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
    reducedMotion = mq.matches;
    const onChange = () => (reducedMotion = mq.matches);
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  });

  // Phase 148e — unified cell title. Topics subject; Tier 2 tier chip plus a
  // conditional reduced-motion badge. Placed after `reducedMotion` so the
  // $derived reads it without a use-before-declare warning.
  const probeLabels = useProbeLabels(() => ctx);
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_topic_evolution_label(),
    subject: { kind: 'topics', label: m.cells_topicevo_title() },
    scope: { kind: 'single', label: probeLabels.labelFor(scopeId) },
    qualifiers: [
      {
        label: m.cells_topicevo_tier_badge(),
        tone: 'tier',
        title: m.cells_topicevo_tier_badge_title()
      },
      ...(reducedMotion
        ? [
            {
              label: m.cells_topicevo_reduced_motion_badge(),
              tone: 'layer' as const,
              title: m.cells_topicevo_reduced_motion_badge_title()
            }
          ]
        : [])
    ],
    idSeed: 'topic-evo-title'
  });

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const data = plotRows;
    if (!host || data.length === 0) return;
    const token = ++renderToken;
    const useStaticBars = reducedMotion;
    const facetMulti = multiLang;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;

      // Stable category order: outlier last, "other" just above outlier,
      // remaining top-K by total volume descending. Drives stack order
      // and legend order; visually pins the outlier ridge to the bottom.
      const orderEntries = seriesMetaValues(seriesMeta).filter((e) => topSeriesIds[seriesId(e)]);
      orderEntries.sort((a, b) => {
        if (a.isOutlier !== b.isOutlier) return a.isOutlier ? 1 : -1;
        return b.totalArticles - a.totalArticles;
      });
      const seriesOrder = [
        ...orderEntries.map((e) => seriesId(e)),
        ...languages.map((l) => `${l}::other`)
      ];

      // Viridis-ish hand-rolled palette so we don't pull a colour-scale
      // dependency. 8 bins + grey for outlier + light slate for "other".
      const VIRIDIS = [
        '#440154',
        '#482677',
        '#3F4A8A',
        '#31678E',
        '#25828E',
        '#1FA088',
        '#3FBC73',
        '#7AD151'
      ];
      const colourFor = (row: PlotRow): string => {
        if (row.isOutlier) return OUTLIER_COLOUR;
        if (row.series.endsWith('::other')) return '#5b6677';
        const named = orderEntries.filter((e) => !e.isOutlier);
        const idx = named.findIndex((e) => seriesId(e) === row.series);
        if (idx < 0) return VIRIDIS[0]!;
        return VIRIDIS[idx % VIRIDIS.length]!;
      };

      const labelFor = (row: PlotRow): string =>
        row.label.length > 32 ? `${row.label.slice(0, 29)}…` : row.label;

      const height = facetMulti ? Math.max(160 * languages.length, 300) : 300;

      const commonChannels = {
        x: 'bucket' as const,
        y: 'articleCount' as const,
        z: 'series' as const,
        fill: (d: PlotRow) => colourFor(d),
        stroke: (d: PlotRow) => colourFor(d),
        title: (d: PlotRow) =>
          [
            labelFor(d),
            m.cells_topicevo_tooltip_language({ language: d.language }),
            m.cells_topicevo_tooltip_articles({ count: d.articleCount }),
            d.isOutlier
              ? m.cells_topicevo_tooltip_topic_id_outlier({ topicId: d.topicId })
              : m.cells_topicevo_tooltip_topic_id({ topicId: d.topicId })
          ].join('\n'),
        // Phase 132 — exact-value hover readout. Stacked + ordered (+ optional
        // facet) marks reorder the DOM, so Plot's data-bound `tip` is the
        // correct readout here (the index-mapped CellReadout is for marks in
        // input order). No click handler to conflict with.
        tip: true
      };

      const stackedMark = useStaticBars
        ? Plot.barY(data, {
            ...commonChannels,
            order: seriesOrder,
            ...(facetMulti ? { fy: 'language' } : {})
          })
        : Plot.areaY(data, {
            ...commonChannels,
            order: seriesOrder,
            curve: 'catmull-rom',
            offset: 'wiggle',
            ...(facetMulti ? { fy: 'language' } : {})
          });

      const next = Plot.plot({
        width: host.clientWidth,
        height,
        marginLeft: 64,
        marginRight: 16,
        marginBottom: 36,
        marginTop: facetMulti ? 24 : 12,
        x: { type: 'time', label: m.cells_topicevo_axis_time(), grid: false, nice: true },
        y: {
          // `labelAnchor: 'center'` rotates the y-label vertically along the
          // axis instead of Plot's default top-left horizontal placement,
          // which overlapped the plot/facet header. A shorter label + wider
          // left margin keep it clear of the tick numbers.
          label: useStaticBars
            ? m.cells_topicevo_axis_y_static()
            : m.cells_topicevo_axis_y_stream(),
          labelAnchor: 'center',
          grid: true
        },
        ...(facetMulti
          ? {
              fy: { label: m.cells_topicevo_facet_language(), domain: languages }
            }
          : {}),
        marks: [stackedMark]
      });

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

  // Custom legend — colours match the on-chart stacked mark exactly (same
  // orderEntries sort + Viridis index as the render effect's colourFor).
  let legendEntries = $derived.by(() => buildLegendEntries(seriesMeta, topSeriesIds, languages));

  // Phase 131 (BUG5) — export.
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'topic_evolution',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      languages: languages.join('+')
    },
    summary: { buckets: buckets.length, series: legendEntries.length },
    howToRead: composeHowToRead('topic_evolution', { renderedCount: legendEntries.length }),
    rows: plotRows.map((r) => ({
      bucket: r.bucket.toISOString(),
      topicId: r.topicId,
      label: r.label,
      language: r.language,
      articleCount: r.articleCount
    })),
    columns: ['bucket', 'topicId', 'label', 'language', 'articleCount']
  });
  const exportFilenameParts = $derived(['topic-evolution', scope === 'source' ? scopeId : 'probe']);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="topic-cell" aria-labelledby="topic-evo-title" bind:this={cellEl}>
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if plotRows.length > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>

  {#if buckets.length === 0}
    <p class="muted">{m.cells_topicevo_no_window()}</p>
  {:else if isPending}
    <p class="muted" aria-busy="true">{m.cells_topicevo_loading()}</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_topicevo_error()}</p>
  {:else if plotRows.length === 0}
    <CellEmptyState />
  {:else}
    {#if isJointCorpus}
      {@const note = methodologyNotes.epistemeJointCorpusEvolution(sources.length)}
      <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="{m.cells_topicevo_plot_aria()}{multiLang
        ? m.cells_topicevo_plot_aria_faceted({ languages: languages.join(', ') })
        : ''}"
    ></div>

    <ul class="legend" aria-label={m.cells_topicevo_legend_aria()}>
      {#each legendEntries as e (e.id)}
        <li class="legend-item" class:outlier={e.isOutlier} class:other={e.isOther}>
          <span class="swatch" style="background: {e.colour};" aria-hidden="true"></span>
          <span class="legend-label">{e.label}</span>
          {#if multiLang}
            <span class="legend-lang">{m.cells_topicevo_legend_lang({ language: e.language })}</span
            >
          {/if}
        </li>
      {/each}
    </ul>

    <footer
      class="provenance"
      aria-label={m.cells_topicevo_prov_aria()}
      data-export-exclude="provenance"
    >
      <dl>
        <div>
          <dt>{m.cells_topicevo_prov_buckets()}</dt>
          <dd>
            {m.cells_topicevo_prov_buckets_value({
              buckets: buckets.length,
              series: legendEntries.length
            })}
          </dd>
        </div>
        <div>
          <dt>{m.cells_topicevo_prov_embeddings()}</dt>
          <dd><code>intfloat/multilingual-e5-large</code></dd>
        </div>
        <div>
          <dt>{m.cells_topicevo_prov_tier()}</dt>
          <dd>{m.cells_topicevo_prov_tier_value()}</dd>
        </div>
        <div>
          <dt>{m.cells_topicevo_prov_partition()}</dt>
          <dd>
            {multiLang
              ? m.cells_topicevo_prov_partition_multi()
              : m.cells_topicevo_prov_partition_single({ language: languages[0] ?? '' })}
          </dd>
        </div>
        <div>
          <dt>{m.cells_topicevo_prov_palette()}</dt>
          <dd>{m.cells_topicevo_prov_palette_value()}</dd>
        </div>
      </dl>
    </footer>
  {/if}
</section>

<style>
  .topic-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .plot-host {
    width: 100%;
  }

  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
  }

  .plot-host :global(svg) {
    background: transparent;
  }

  .legend {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2) var(--space-4);
    list-style: none;
    margin: 0;
    padding: 0;
  }

  .legend-item {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }

  .legend-item.outlier .legend-label,
  .legend-item.other .legend-label {
    font-style: italic;
    color: var(--color-fg-subtle);
  }

  .swatch {
    display: inline-block;
    width: 12px;
    height: 12px;
    border-radius: 2px;
    border: 1px solid color-mix(in srgb, currentColor 30%, transparent);
  }

  .legend-lang {
    font-size: 10px;
    color: var(--color-fg-subtle);
  }

  .provenance {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-3);
  }

  .provenance dl {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: var(--space-2) var(--space-4);
    margin: 0;
  }

  .provenance dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .provenance dd {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .provenance code {
    font-family: var(--font-mono);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
