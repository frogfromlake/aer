<script lang="ts">
  // Network Science × force-directed graph cell (Phase 107).
  // Per-scope entity co-occurrence subgraph backed by
  // `GET /api/v1/entities/cooccurrence`. The d3-force simulation is
  // dynamically imported so its chunk only ships when this cell is
  // selected (Brief §7 bundle budget).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    entityCoOccurrenceQuery,
    type CoOccurrenceGraphDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  // metricName is part of the cell id so a future "colour by sentiment"
  // mode can lift it; for the MVP we colour by entity label and ignore it.
  // Phase 111: Silver layer has no entity co-occurrence data; show a notice.
  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    dataLayer = 'gold'
  }: ViewModeCellProps = $props();

  const TOP_N = 60;
  const WIDTH = 720;
  const HEIGHT = 420;

  const graphQ = createQuery<
    QueryOutcome<CoOccurrenceGraphDto>,
    Error,
    QueryOutcome<CoOccurrenceGraphDto>
  >(() => {
    const o = entityCoOccurrenceQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      topN: TOP_N
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  interface SimNode {
    id: string;
    label: string;
    radius: number;
    x?: number;
    y?: number;
    fx?: number | null;
    fy?: number | null;
  }
  interface SimEdge {
    source: string | SimNode;
    target: string | SimNode;
    weight: number;
  }

  let nodes: SimNode[] = $state([]);
  let edges: SimEdge[] = $state([]);
  let token = 0;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let simulation: any = null;

  $effect(() => {
    const data = graphQ.data?.kind === 'success' ? graphQ.data.data : null;
    if (!data) return;
    const t = ++token;
    const maxTotal = data.nodes.reduce((m, n) => Math.max(m, n.totalCount), 1);
    const seedNodes: SimNode[] = data.nodes.map((n) => ({
      id: n.text,
      label: n.label,
      radius: 4 + 14 * Math.sqrt(n.totalCount / maxTotal)
    }));
    const seedEdges: SimEdge[] = data.edges.map((e) => ({
      source: e.a,
      target: e.b,
      weight: e.weight
    }));
    (async () => {
      const d3 = await import('d3-force');
      if (t !== token) return;
      simulation?.stop();
      simulation = d3
        .forceSimulation<SimNode>(seedNodes)
        .force(
          'link',
          d3
            .forceLink<SimNode, SimEdge>(seedEdges)
            .id((n: SimNode) => n.id)
            .distance(60)
            .strength(0.4)
        )
        .force('charge', d3.forceManyBody<SimNode>().strength(-120))
        .force('center', d3.forceCenter(WIDTH / 2, HEIGHT / 2))
        .force(
          'collide',
          d3.forceCollide<SimNode>().radius((n: SimNode) => n.radius + 2)
        );
      simulation.on('tick', () => {
        if (t !== token) return;
        nodes = [...seedNodes];
        edges = [...seedEdges];
      });
    })();
  });

  onDestroy(() => {
    simulation?.stop();
    simulation = null;
  });

  // Stable label-keyed colour. The set of NER labels is small (PER, ORG, LOC, MISC, …).
  function labelColor(label: string): string {
    const palette = [
      '#5283b8', // blue
      '#b87a52', // amber
      '#52b885', // green
      '#a058b8', // purple
      '#b85265', // red
      '#888888'
    ];
    let h = 0;
    for (let i = 0; i < label.length; i++) h = (h * 31 + label.charCodeAt(i)) | 0;
    return palette[Math.abs(h) % palette.length] ?? palette[0]!;
  }

  function nodeX(n: SimNode | string): number {
    return typeof n === 'string' ? 0 : (n.x ?? 0);
  }
  function nodeY(n: SimNode | string): number {
    return typeof n === 'string' ? 0 : (n.y ?? 0);
  }
  function nodeId(n: SimNode | string): string {
    return typeof n === 'string' ? n : n.id;
  }
  function edgeKey(e: SimEdge): string {
    return nodeId(e.source) + '|' + nodeId(e.target);
  }

  let maxEdgeWeight = $derived(edges.reduce((m, e) => Math.max(m, e.weight), 1));
</script>

<section class="net-cell" aria-labelledby="net-title">
  <header class="cell-header">
    <h3 id="net-title" class="cell-title">
      Entity co-occurrence
      <span class="muted">— top {TOP_N} pairs ({scope})</span>
    </h3>
  </header>

  {#if dataLayer === 'silver'}
    <p class="notice">
      Co-occurrence network is not available for Silver-layer data. Co-occurrence analysis operates
      on Gold-layer entity extractions. Switch to Distribution to explore Silver-layer document
      characteristics.
    </p>
  {:else if graphQ.isPending}
    <p class="muted" aria-busy="true">Loading co-occurrence graph…</p>
  {:else if graphQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={graphQ.data} {ctx} />
  {:else if graphQ.isError || graphQ.data?.kind === 'network-error'}
    <p class="muted">Could not load co-occurrence graph.</p>
  {:else if graphQ.data?.kind === 'success' && graphQ.data.data.edges.length === 0}
    <p class="muted">No entity co-occurrences in this window.</p>
  {:else if nodes.length > 0}
    <svg
      class="graph"
      viewBox="0 0 {WIDTH} {HEIGHT}"
      role="img"
      aria-label="Force-directed entity co-occurrence graph"
    >
      <g class="edges">
        {#each edges as e (edgeKey(e))}
          <line
            x1={nodeX(e.source)}
            y1={nodeY(e.source)}
            x2={nodeX(e.target)}
            y2={nodeY(e.target)}
            stroke="rgba(180, 200, 220, 0.5)"
            stroke-width={0.4 + 2.4 * (e.weight / maxEdgeWeight)}
          />
        {/each}
      </g>
      <g class="nodes">
        {#each nodes as n (n.id)}
          <g transform="translate({n.x ?? 0},{n.y ?? 0})">
            <circle r={n.radius} fill={labelColor(n.label)} fill-opacity="0.85" />
            <title>{n.id} ({n.label})</title>
            {#if n.radius > 8}
              <text
                x={n.radius + 3}
                y={3}
                font-size="10"
                fill="var(--color-fg)"
                font-family="var(--font-mono)">{n.id}</text
              >
            {/if}
          </g>
        {/each}
      </g>
    </svg>
  {:else}
    <p class="muted" aria-busy="true">Laying out…</p>
  {/if}
</section>

<style>
  .net-cell {
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

  .graph {
    width: 100%;
    height: auto;
    max-height: 60vh;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .notice {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    padding: var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px dashed var(--color-border-strong);
    border-radius: var(--radius-md);
    max-width: 36rem;
  }
</style>
