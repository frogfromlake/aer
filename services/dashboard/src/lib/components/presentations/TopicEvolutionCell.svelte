<script lang="ts">
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
  import HowToRead from './HowToRead.svelte';
  import CellExport from './CellExport.svelte';
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
            `language: ${d.language}`,
            `articles: ${d.articleCount}`,
            `topic_id: ${d.topicId}${d.isOutlier ? ' (outlier)' : ''}`
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
        x: { type: 'time', label: 'time', grid: false, nice: true },
        y: {
          // `labelAnchor: 'center'` rotates the y-label vertically along the
          // axis instead of Plot's default top-left horizontal placement,
          // which overlapped the plot/facet header. A shorter label + wider
          // left margin keep it clear of the tick numbers.
          label: useStaticBars ? 'articles/bucket' : 'stream volume',
          labelAnchor: 'center',
          grid: true
        },
        ...(facetMulti
          ? {
              fy: { label: 'language', domain: languages }
            }
          : {}),
        marks: [stackedMark]
      });

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
  <header class="cell-header">
    <h3 id="topic-evo-title" class="cell-title">
      <span class="primary">Topics over time</span>
      <span class="muted"
        >— BERTopic stream graph · <strong class="scope-name">{scopeId}</strong></span
      >
      <span
        class="tier-badge"
        title="Tier 2 (Phase 120) — reproducible with pinned model, not bit-for-bit deterministic across platforms."
        >Tier 2</span
      >
      {#if reducedMotion}
        <span class="reduced-motion-badge" title="Static stacked bars per prefers-reduced-motion."
          >reduced motion</span
        >
      {/if}
    </h3>
    {#if plotRows.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if buckets.length === 0}
    <p class="muted">Pick a wider window to see topic evolution over time.</p>
  {:else if isPending}
    <p class="muted" aria-busy="true">Loading topic evolution…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load topic evolution.</p>
  {:else if plotRows.length === 0}
    <p class="muted">
      No topic assignments in this window. The BERTopic sweep loop runs in the background — wait for
      the next sweep or pick a wider time range.
    </p>
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
      aria-label="Stream graph of topic article volume over time, stacked by topic{multiLang
        ? `, faceted by language partition: ${languages.join(', ')}`
        : ''}"
    ></div>

    <ul class="legend" aria-label="Topic colour legend">
      {#each legendEntries as e (e.id)}
        <li class="legend-item" class:outlier={e.isOutlier} class:other={e.isOther}>
          <span class="swatch" style="background: {e.colour};" aria-hidden="true"></span>
          <span class="legend-label">{e.label}</span>
          {#if multiLang}
            <span class="legend-lang">[{e.language}]</span>
          {/if}
        </li>
      {/each}
    </ul>

    <footer class="provenance" aria-label="Methodology provenance" data-export-exclude="provenance">
      <dl>
        <div>
          <dt>Buckets</dt>
          <dd>{buckets.length} sub-windows · {legendEntries.length} series</dd>
        </div>
        <div>
          <dt>Embeddings</dt>
          <dd><code>intfloat/multilingual-e5-large</code></dd>
        </div>
        <div>
          <dt>Tier</dt>
          <dd>2 — pinned BERTopic + seeded UMAP/HDBSCAN (WP-002 §8.1 Option C)</dd>
        </div>
        <div>
          <dt>Partition</dt>
          <dd>
            {multiLang
              ? 'per-language (no cross-language alignment — WP-004 §3.4)'
              : `single language (${languages[0]})`}
          </dd>
        </div>
        <div>
          <dt>Palette</dt>
          <dd>Viridis ramp, arbitrary topic order — colour carries no valence.</dd>
        </div>
      </dl>
    </footer>
    <HowToRead presentation="topic_evolution" facts={{ renderedCount: legendEntries.length }} />
  {/if}
</section>

<style>
  .topic-cell {
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
    flex-wrap: wrap;
  }

  .cell-title .primary {
    font-family: var(--font-mono);
  }

  .tier-badge {
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: var(--font-weight-semibold);
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: rgba(224, 160, 80, 0.15);
    border: 1px solid #e0a050;
    color: #e0a050;
    cursor: help;
  }

  .reduced-motion-badge {
    font-size: 10px;
    font-family: var(--font-mono);
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border-strong);
    color: var(--color-fg-muted);
    cursor: help;
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

  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
