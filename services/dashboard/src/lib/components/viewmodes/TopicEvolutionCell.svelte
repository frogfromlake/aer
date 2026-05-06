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
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let { ctx, scope, scopeId, windowStart, windowEnd, probeIds = [] }: ViewModeCellProps = $props();

  const OUTLIER_TOPIC_ID = -1;
  const UNCATEGORISED_LABEL = 'uncategorised';
  const OUTLIER_COLOUR = '#888888';
  // Top-K topics surfaced as named series; everything else folds into
  // an "other" stream so the stack does not explode visually.
  const TOP_K = 8;
  const TARGET_BUCKETS = 10;
  const MIN_BUCKETS = 3;
  const MAX_BUCKETS = 14;

  // Bucket boundaries — split [windowStart, windowEnd) into N equal
  // sub-windows. The total span is in milliseconds; we floor to a
  // reasonable bucket count so each bucket holds at least a few hours
  // (a sub-window narrower than the BERTopic sweep cadence returns
  // identical aggregates and wastes requests).
  interface Bucket {
    start: string;
    end: string;
    midpoint: string;
  }
  let buckets = $derived.by<Bucket[]>(() => {
    const t0 = new Date(windowStart).getTime();
    const t1 = new Date(windowEnd).getTime();
    if (!Number.isFinite(t0) || !Number.isFinite(t1) || t1 <= t0) return [];
    const spanMs = t1 - t0;
    // Target ~1 bucket per 12h, clamped.
    const TWELVE_H = 12 * 60 * 60 * 1000;
    const desired = Math.round(spanMs / TWELVE_H);
    const n = Math.max(MIN_BUCKETS, Math.min(MAX_BUCKETS, desired || TARGET_BUCKETS));
    const step = spanMs / n;
    const out: Bucket[] = [];
    for (let i = 0; i < n; i++) {
      const s = new Date(t0 + i * step);
      const e = new Date(t0 + (i + 1) * step);
      const mid = new Date(t0 + (i + 0.5) * step);
      out.push({
        start: s.toISOString(),
        end: e.toISOString(),
        midpoint: mid.toISOString()
      });
    }
    return out;
  });

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

  // Flatten into per-bucket per-topic rows; first compute the global
  // top-K topics across all buckets so the stream stack is stable.
  interface SeriesKey {
    topicId: number;
    language: string;
    label: string;
    isOutlier: boolean;
  }
  function seriesId(k: SeriesKey): string {
    return `${k.language}::${k.topicId}`;
  }

  type SeriesMetaEntry = SeriesKey & { totalArticles: number };
  let seriesMeta = $derived.by<Record<string, SeriesMetaEntry>>(() => {
    const map: Record<string, SeriesMetaEntry> = Object.create(null);
    for (const q of queries) {
      if (q.data?.kind !== 'success') continue;
      const payload: TopicDistributionResponseDto = q.data.data;
      for (const t of payload.topics) {
        const isOutlier = t.topicId === OUTLIER_TOPIC_ID;
        const key: SeriesKey = {
          topicId: t.topicId,
          language: t.language || 'und',
          label: isOutlier ? UNCATEGORISED_LABEL : t.label,
          isOutlier
        };
        const id = seriesId(key);
        const prior = map[id];
        if (prior) {
          prior.totalArticles += t.articleCount;
        } else {
          map[id] = { ...key, totalArticles: t.articleCount };
        }
      }
    }
    return map;
  });

  function seriesMetaValues(meta: Record<string, SeriesMetaEntry>): SeriesMetaEntry[] {
    return Object.values(meta);
  }

  let topSeriesIds = $derived.by<Record<string, true>>(() => {
    // Outlier always kept; other topics ordered by total volume desc.
    const entries = seriesMetaValues(seriesMeta);
    const outliers = entries.filter((e) => e.isOutlier);
    const named = entries
      .filter((e) => !e.isOutlier)
      .sort((a, b) => b.totalArticles - a.totalArticles)
      .slice(0, TOP_K);
    const out: Record<string, true> = Object.create(null);
    for (const e of [...outliers, ...named]) out[seriesId(e)] = true;
    return out;
  });

  let languages = $derived.by<string[]>(() => {
    const seen: string[] = [];
    for (const e of seriesMetaValues(seriesMeta)) {
      if (!seen.includes(e.language)) seen.push(e.language);
    }
    return seen;
  });
  let multiLang = $derived(languages.length > 1);

  interface PlotRow {
    bucket: Date;
    series: string;
    label: string;
    language: string;
    topicId: number;
    isOutlier: boolean;
    articleCount: number;
  }

  let plotRows = $derived.by<PlotRow[]>(() => {
    const rows: PlotRow[] = [];
    for (let i = 0; i < queries.length; i++) {
      const q = queries[i]!;
      const b = buckets[i]!;
      if (q.data?.kind !== 'success') continue;
      const payload: TopicDistributionResponseDto = q.data.data;
      // Bucket totals — used to fold non-top-K topics into "other".
      const otherByLang: Record<string, number> = Object.create(null);
      const present: Record<string, true> = Object.create(null);
      for (const t of payload.topics) {
        const isOutlier = t.topicId === OUTLIER_TOPIC_ID;
        const lang = t.language || 'und';
        const key: SeriesKey = {
          topicId: t.topicId,
          language: lang,
          label: isOutlier ? UNCATEGORISED_LABEL : t.label,
          isOutlier
        };
        const id = seriesId(key);
        if (topSeriesIds[id]) {
          rows.push({
            bucket: new Date(b.midpoint),
            series: id,
            label: key.label,
            language: lang,
            topicId: t.topicId,
            isOutlier,
            articleCount: t.articleCount
          });
          present[id] = true;
        } else {
          otherByLang[lang] = (otherByLang[lang] ?? 0) + t.articleCount;
        }
      }
      // Backfill explicit zeros for top-K series absent in this bucket
      // so the area mark interpolates cleanly across gaps.
      for (const id of Object.keys(topSeriesIds)) {
        if (present[id]) continue;
        const meta = seriesMeta[id];
        if (!meta) continue;
        rows.push({
          bucket: new Date(b.midpoint),
          series: id,
          label: meta.label,
          language: meta.language,
          topicId: meta.topicId,
          isOutlier: meta.isOutlier,
          articleCount: 0
        });
      }
      // Emit the "other" bucket per language if any non-top-K topics
      // contributed in this bucket.
      for (const [lang, count] of Object.entries(otherByLang)) {
        rows.push({
          bucket: new Date(b.midpoint),
          series: `${lang}::other`,
          label: 'other topics',
          language: lang,
          topicId: -2,
          isOutlier: false,
          articleCount: count
        });
      }
    }
    return rows;
  });

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

      const height = facetMulti ? Math.max(180 * languages.length, 360) : 360;

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
          ].join('\n')
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
        marginLeft: 56,
        marginRight: 16,
        marginBottom: 36,
        marginTop: facetMulti ? 24 : 12,
        x: { type: 'time', label: 'time', grid: false, nice: true },
        y: {
          label: useStaticBars ? 'articles per bucket' : 'stream volume (silhouette)',
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

  // Custom legend — Plot's auto-legend cannot label series with our
  // composed `series` ids directly without a colour scale; we render
  // it manually so the legend matches the on-chart colour exactly.
  interface LegendEntry {
    id: string;
    label: string;
    language: string;
    isOutlier: boolean;
    isOther: boolean;
    colour: string;
  }
  let legendEntries = $derived.by<LegendEntry[]>(() => {
    const orderEntries = seriesMetaValues(seriesMeta).filter((e) => topSeriesIds[seriesId(e)]);
    orderEntries.sort((a, b) => {
      if (a.isOutlier !== b.isOutlier) return a.isOutlier ? 1 : -1;
      return b.totalArticles - a.totalArticles;
    });
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
    const out: LegendEntry[] = [];
    const named = orderEntries.filter((e) => !e.isOutlier);
    named.forEach((e, idx) =>
      out.push({
        id: seriesId(e),
        label: e.label,
        language: e.language,
        isOutlier: false,
        isOther: false,
        colour: VIRIDIS[idx % VIRIDIS.length]!
      })
    );
    for (const lang of languages) {
      out.push({
        id: `${lang}::other`,
        label: 'other topics',
        language: lang,
        isOutlier: false,
        isOther: true,
        colour: '#5b6677'
      });
    }
    for (const e of orderEntries.filter((x) => x.isOutlier)) {
      out.push({
        id: seriesId(e),
        label: UNCATEGORISED_LABEL,
        language: e.language,
        isOutlier: true,
        isOther: false,
        colour: OUTLIER_COLOUR
      });
    }
    return out;
  });
</script>

<section class="topic-cell" aria-labelledby="topic-evo-title">
  <header class="cell-header">
    <h3 id="topic-evo-title" class="cell-title">
      <span class="primary">Topics over time</span>
      <span class="muted">— BERTopic stream graph ({scope})</span>
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

    <footer class="provenance" aria-label="Methodology provenance">
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
</style>
