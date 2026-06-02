<script lang="ts">
  // Episteme × topic-distribution cell (Phase 121).
  //
  // Backed by `GET /api/v1/topics/distribution`. The cell renders one
  // horizontal "ridge" (bar) per BERTopic topic in the resolved scope,
  // sorted by article count descending. When the active scope spans
  // multiple language partitions, the cell facets by language so each
  // partition gets its own ridge stack — cross-language `topic_id`
  // comparisons are explicitly meaningless under WP-004 §3.4 and we
  // never merge them at the rendering layer.
  //
  // The outlier class (`topic_id == -1`) is rendered as a greyed
  // "uncategorised" ridge rather than hidden — an outlier is an
  // observation, not an error (Brief §7.7 / absence-as-data).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    topicDistributionQuery,
    type TopicDistributionResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import HowToRead from './HowToRead.svelte';
  import CellExport from './CellExport.svelte';
  import {
    normaliseTopics,
    languagesOf,
    type NormalisedTopic
  } from '$lib/viewmodes/topic-internals';
  import { JOINT_CORPUS_MIN_SOURCES, TOPIC_MIN_DOCS } from '$lib/config/topic-thresholds';
  import MethodologyBanner from '$lib/components/base/MethodologyBanner.svelte';
  import { methodologyNotes } from '$lib/methodology-copy';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    probeIds = [],
    sources = []
  }: ViewModeCellProps = $props();

  const OUTLIER_FILL = 'rgba(150, 150, 150, 0.45)';
  const OUTLIER_STROKE = 'rgba(150, 150, 150, 0.85)';
  const TOPIC_FILL = 'rgba(82, 131, 184, 0.55)';
  const TOPIC_STROKE = 'rgba(82, 131, 184, 0.95)';

  const topicQ = createQuery<
    QueryOutcome<TopicDistributionResponseDto>,
    Error,
    QueryOutcome<TopicDistributionResponseDto>
  >(() => {
    const params = {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      probeIds: probeIds.length > 0 ? probeIds : undefined,
      includeOutlier: true
    };
    const o = topicDistributionQuery(ctx, params);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  let isPending = $derived(topicQ.isPending);
  let refusalData = $derived(topicQ.data?.kind === 'refusal' ? topicQ.data : null);
  let isNetworkError = $derived(topicQ.isError || topicQ.data?.kind === 'network-error');
  let payload = $derived<TopicDistributionResponseDto | null>(
    topicQ.data?.kind === 'success' ? topicQ.data.data : null
  );

  // When no window is set (the whole-dataset default), topic_distribution is
  // synchronic: the BFF returns the single NEWEST BERTopic sweep (one coherent
  // model — topic_ids are sweep-local, so it cannot aggregate across sweeps).
  // We surface the sweep's own coverage window so the reader knows these topics
  // describe that span, not the whole 365-day corpus.
  let isLatestSweep = $derived(windowStart === undefined && windowEnd === undefined);
  let sweepRange = $derived(
    payload && payload.windowStart && payload.windowEnd
      ? `${payload.windowStart.slice(0, 10)} – ${payload.windowEnd.slice(0, 10)}`
      : null
  );

  let rows = $derived<NormalisedTopic[]>(payload ? normaliseTopics(payload.topics) : []);
  let languages = $derived(languagesOf(rows));
  let multiLang = $derived(languages.length > 1);

  // Phase 122i / ADR-034 — Methodology gates surfaced as banners (not
  // refusals): joint-corpus when the scope spans ≥ JOINT_CORPUS_MIN_SOURCES
  // sources, small-corpus when the total article count is below
  // TOPIC_MIN_DOCS. Both banners cite WP-005 §6.2 so the reader can
  // follow the reasoning into the methodology surface.
  let totalArticles = $derived(rows.reduce((sum, r) => sum + r.articleCount, 0));
  let isJointCorpus = $derived(sources.length >= JOINT_CORPUS_MIN_SOURCES);
  let isSmallCorpus = $derived(totalArticles > 0 && totalArticles < TOPIC_MIN_DOCS);

  // Phase 131 (BUG5) — export.
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'topic_distribution',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      languages: languages.join('+')
    },
    summary: { topics: rows.length, totalArticles },
    howToRead: composeHowToRead('topic_distribution', { renderedCount: rows.length }),
    rows: rows.map((r) => ({
      topicId: r.topicId,
      label: r.label,
      articleCount: r.articleCount,
      avgConfidence: r.avgConfidence,
      language: r.language
    })),
    columns: ['topicId', 'label', 'articleCount', 'avgConfidence', 'language']
  });
  const exportFilenameParts = $derived([
    'topic-distribution',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
  let modelHashes = $derived(
    Array.from(
      new Set(
        (payload?.topics ?? [])
          .map((t) => t.modelHash)
          .filter((h): h is string => typeof h === 'string' && h.length > 0)
      )
    )
  );

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const data = rows;
    if (!host || data.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;

      // Render one row per (language, topic) — y-axis is the topic
      // label; faceted by language so partitions never share a y-scale.
      // Bar length = articleCount. The visual is a horizontal ridge
      // stack (ridgeline-as-bar) — keeps the mark inventory consistent
      // with the existing DistributionCell (Plot.rectY → Plot.barX).
      // Raw label, lightly capped — only for building a unique y-domain key.
      const labelOf = (d: NormalisedTopic): string =>
        d.label.length > 64 ? `${d.label.slice(0, 61)}…` : d.label;
      // Human-readable axis label: BERTopic labels arrive as
      // "<id>_word_word_word_word" (a c-TF-IDF fingerprint, not a title).
      // Strip the leading numeric id (it is not a percentage — see the
      // "how to read" note) and join the words with middots so the axis
      // reads "word · word · word"; truncate to fit the left margin with
      // the full label preserved in the hover tip.
      const prettyLabel = (d: NormalisedTopic): string => {
        if (d.isOutlier) return 'uncategorised';
        const words = d.label.replace(/^-?\d+_/, '').split('_').filter(Boolean);
        const joined = words.join(' · ');
        return joined.length > 32 ? `${joined.slice(0, 31)}…` : joined || d.label;
      };
      // Prepare row keys; pad short labels with zero-width chars per
      // ridge index so identically-labeled topics across languages
      // sort deterministically inside Plot's domain inference.
      const keyed = data.map((d) => ({
        ...d,
        key: `${d.language}::${d.ridge}::${labelOf(d)}`
      }));
      // Map each composite y-domain key to its human topic label. Read by
      // the y-axis `tickFormat` below so the axis shows the readable label,
      // not the composite sort key. Built before plot() on purpose.
      const labelMap: Record<string, string> = Object.create(null);
      for (const r of keyed) labelMap[r.key] = prettyLabel(r);

      const baseHeight = 28; // px per ridge
      const partitions: Record<string, number> = Object.create(null);
      for (const r of data) {
        partitions[r.language] = Math.max(partitions[r.language] ?? 0, r.ridge + 1);
      }
      // Total content height: stacked partitions + facet header gutter.
      const contentH = Object.values(partitions).reduce((acc, n) => acc + n * baseHeight, 0);
      const height = Math.max(220, contentH + (multiLang ? 36 : 12));

      const next = Plot.plot({
        width: host.clientWidth,
        height,
        marginLeft: 220,
        marginRight: 64,
        marginTop: multiLang ? 20 : 8,
        marginBottom: 36,
        x: { label: 'articles', grid: true, nice: true },
        // `tickFormat` maps the composite sort key back to the human topic
        // label. Doing it here (not via a post-render DOM patch) means Plot
        // measures the real label for fit, so a lone long-labelled topic
        // (N=1) still renders its tick instead of dropping it.
        y: { label: null, axis: 'left', tickFormat: (k: string) => labelMap[k] ?? k },
        ...(multiLang
          ? {
              fy: {
                label: 'language',
                domain: languages
              }
            }
          : {}),
        marks: [
          Plot.barX(keyed, {
            x: 'articleCount',
            y: 'key',
            ...(multiLang ? { fy: 'language' } : {}),
            sort: { y: 'x', reverse: true },
            fill: (d: NormalisedTopic) => (d.isOutlier ? OUTLIER_FILL : TOPIC_FILL),
            stroke: (d: NormalisedTopic) => (d.isOutlier ? OUTLIER_STROKE : TOPIC_STROKE),
            // Methodological-register tooltip: fold the Tier-2 caveat,
            // model hash, and average confidence into the hover. The
            // semantic register is the topic label (already on y-axis).
            title: (d: NormalisedTopic) =>
              [
                `${d.label}${d.isOutlier ? ' (outlier class)' : ''}`,
                `language: ${d.language}`,
                `articles: ${d.articleCount}`,
                `mean confidence: ${d.avgConfidence.toFixed(3)}`,
                d.isOutlier ? 'BERTopic outlier — no coherent cluster.' : null
              ]
                .filter((s): s is string => s !== null)
                .join('\n'),
            // Phase 132 — exact-value hover readout. This cell sorts +
            // facets its bars, so the DOM order does not match input order;
            // the shared index-mapped CellReadout would mislabel. Plot's own
            // data-bound `tip` is correct here and safe (no click handler to
            // conflict with).
            tip: true
          }),
          Plot.text(keyed, {
            x: 'articleCount',
            y: 'key',
            ...(multiLang ? { fy: 'language' } : {}),
            text: (d: NormalisedTopic) => `${d.articleCount}`,
            dx: 6,
            textAnchor: 'start',
            fill: 'currentColor',
            fontSize: 10
          }),
          Plot.ruleX([0])
        ]
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
</script>

<section class="topic-cell" aria-labelledby="topic-dist-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="topic-dist-title" class="cell-title">
      <span class="primary">Topics</span>
      <span class="muted"
        >— BERTopic distribution · <strong class="scope-name">{scopeId}</strong></span
      >
      <span
        class="tier-badge"
        title="Tier 2 (Phase 120) — reproducible with pinned model, not bit-for-bit deterministic across platforms."
        >Tier 2</span
      >
    </h3>
    {#if rows.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if isLatestSweep && sweepRange && rows.length > 0}
    <p
      class="sweep-caption"
      title="Topics are computed per background sweep; this cell shows the single latest sweep."
    >
      Latest topic sweep · {sweepRange}
    </p>
  {/if}

  {#if isPending}
    <p class="muted" aria-busy="true">Loading topic distribution…</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load topic distribution.</p>
  {:else if rows.length === 0}
    <p class="muted">
      No topic assignments in this window. The BERTopic sweep runs in the background — if this probe
      was just enabled, the first sweep may not have completed yet.
    </p>
  {:else}
    {#if isJointCorpus}
      {@const note = methodologyNotes.epistemeJointCorpusDistribution(sources.length)}
      <MethodologyBanner anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    {#if isSmallCorpus}
      {@const note = methodologyNotes.epistemeSmallCorpus(totalArticles, TOPIC_MIN_DOCS)}
      <MethodologyBanner variant="warn" anchorHref={note.anchorHref} anchorLabel={note.anchorLabel}>
        <strong>{note.headline}</strong> — {note.body}
      </MethodologyBanner>
    {/if}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Horizontal ridges of topics, one per BERTopic cluster, sorted by article count{multiLang
        ? `, faceted by language partition: ${languages.join(', ')}`
        : ''}"
    ></div>
    <footer class="provenance" aria-label="Methodology provenance" data-export-exclude="provenance">
      <dl>
        <div>
          <dt>Method</dt>
          <dd>BERTopic over {rows.length} topics</dd>
        </div>
        <div>
          <dt>Embeddings</dt>
          <dd><code>intfloat/multilingual-e5-large</code></dd>
        </div>
        <div>
          <dt>Tier</dt>
          <dd>2 — pinned model + seeded UMAP/HDBSCAN (WP-002 §8.1 Option C)</dd>
        </div>
        <div>
          <dt>Partition</dt>
          <dd>
            {multiLang
              ? 'per-language (no cross-language alignment — WP-004 §3.4)'
              : `single language (${languages[0]})`}
          </dd>
        </div>
        {#if modelHashes.length === 1}
          <div>
            <dt>model_hash</dt>
            <dd><code class="hash">{modelHashes[0]}</code></dd>
          </div>
        {:else if modelHashes.length > 1}
          <div>
            <dt>model_hash</dt>
            <dd>
              <span class="warn" title="Topic identity may have rotated within this window."
                >{modelHashes.length} hashes ⚠</span
              >
            </dd>
          </div>
        {/if}
      </dl>
    </footer>
    <HowToRead presentation="topic_distribution" facts={{ renderedCount: rows.length }} />
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

  .provenance code.hash {
    font-size: 10px;
    color: var(--color-fg-muted);
    overflow: hidden;
    text-overflow: ellipsis;
    display: inline-block;
    max-width: 28ch;
    vertical-align: bottom;
  }

  .provenance .warn {
    color: #e0a050;
    font-family: var(--font-mono);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .sweep-caption {
    margin: calc(-1 * var(--space-2)) 0 0;
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
  }

  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
