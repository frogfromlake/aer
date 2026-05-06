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
  import {
    normaliseTopics,
    languagesOf,
    type NormalisedTopic
  } from '$lib/viewmodes/topic-internals';

  let { ctx, scope, scopeId, windowStart, windowEnd, probeIds = [] }: ViewModeCellProps = $props();

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

  let rows = $derived<NormalisedTopic[]>(payload ? normaliseTopics(payload.topics) : []);
  let languages = $derived(languagesOf(rows));
  let multiLang = $derived(languages.length > 1);
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
      const labelOf = (d: NormalisedTopic): string =>
        // Truncate long c-TF-IDF labels so the y-axis stays legible.
        // The full label is in the tip mark.
        d.label.length > 64 ? `${d.label.slice(0, 61)}…` : d.label;
      // Prepare row keys; pad short labels with zero-width chars per
      // ridge index so identically-labeled topics across languages
      // sort deterministically inside Plot's domain inference.
      const keyed = data.map((d) => ({
        ...d,
        key: `${d.language}::${d.ridge}::${labelOf(d)}`
      }));

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
        y: { label: null, axis: 'left' },
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
                .join('\n')
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
      // Replace the y-axis tick labels with the human topic label.
      // Plot's default `text` mark on the categorical y axis prints the
      // composed `key` string — we override after render using the
      // canonical pattern `tickFormat` does not support.
      const labelMap: Record<string, string> = Object.create(null);
      for (const r of keyed) labelMap[r.key] = labelOf(r);
      const ticks = (next as HTMLElement).querySelectorAll('[aria-label="y-axis tick label"]');
      ticks.forEach((t) => {
        const raw = t.textContent ?? '';
        const human = labelMap[raw];
        if (human) t.textContent = human;
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

<section class="topic-cell" aria-labelledby="topic-dist-title">
  <header class="cell-header">
    <h3 id="topic-dist-title" class="cell-title">
      <span class="primary">Topics</span>
      <span class="muted">— BERTopic distribution ({scope})</span>
      <span
        class="tier-badge"
        title="Tier 2 (Phase 120) — reproducible with pinned model, not bit-for-bit deterministic across platforms."
        >Tier 2</span
      >
    </h3>
  </header>

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
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Horizontal ridges of topics, one per BERTopic cluster, sorted by article count{multiLang
        ? `, faceted by language partition: ${languages.join(', ')}`
        : ''}"
    ></div>
    <footer class="provenance" aria-label="Methodology provenance">
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
</style>
