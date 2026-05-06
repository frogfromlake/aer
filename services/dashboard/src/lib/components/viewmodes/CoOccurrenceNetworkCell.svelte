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
  import ArticlePreviewList from '$lib/components/lanes/ArticlePreviewList.svelte';
  import { wikidataHref, wikipediaHref } from './cooccurrence-network-internals';
  import type { ViewModeCellProps } from '$lib/viewmodes';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    sources,
    dataLayer = 'gold'
  }: ViewModeCellProps = $props();

  const TOP_N = 60;
  const WIDTH = 720;
  const HEIGHT = 500;

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
    wikidataQid?: string | null;
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
      radius: 4 + 14 * Math.sqrt(n.totalCount / maxTotal),
      wikidataQid: n.wikidataQid ?? null
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
            .distance(70)
            .strength(0.5)
        )
        // Reduced repulsion: -80 instead of -120 — tight enough to separate
        // labels but not strong enough to fling isolated nodes to infinity.
        .force('charge', d3.forceManyBody<SimNode>().strength(-80))
        .force('center', d3.forceCenter(WIDTH / 2, HEIGHT / 2).strength(0.05))
        // Weak gravitational well toward center: prevents disconnected
        // sub-graphs from drifting out of the viewport entirely.
        .force('gravX', d3.forceX<SimNode>(WIDTH / 2).strength(0.04))
        .force('gravY', d3.forceY<SimNode>(HEIGHT / 2).strength(0.04))
        .force(
          'collide',
          d3.forceCollide<SimNode>().radius((n: SimNode) => n.radius + 3)
        );
      simulation.on('tick', () => {
        if (t !== token) return;
        nodes = [...seedNodes];
        edges = [...seedEdges];
      });
      // Reset viewport to default when new data loads.
      resetView();
    })();
  });

  onDestroy(() => {
    simulation?.stop();
    simulation = null;
  });

  // ── Zoom / pan ──────────────────────────────────────────────────────────────

  let tx = $state(0);
  let ty = $state(0);
  let scale = $state(1);

  let svgEl: SVGSVGElement | null = $state(null);

  function resetView() {
    tx = 0;
    ty = 0;
    scale = 1;
  }

  // Convert client coords → SVG viewBox coords.
  function clientToSvg(cx: number, cy: number): { x: number; y: number } {
    if (!svgEl) return { x: cx, y: cy };
    const r = svgEl.getBoundingClientRect();
    return { x: ((cx - r.left) / r.width) * WIDTH, y: ((cy - r.top) / r.height) * HEIGHT };
  }

  // Convert SVG viewBox coords → simulation (inner-group) coords.
  function svgToSim(sx: number, sy: number): { x: number; y: number } {
    return { x: (sx - tx) / scale, y: (sy - ty) / scale };
  }

  // Attach wheel listener as non-passive so we can preventDefault.
  $effect(() => {
    if (!svgEl) return;
    function onWheel(e: WheelEvent) {
      e.preventDefault();
      const { x: svgX, y: svgY } = clientToSvg(e.clientX, e.clientY);
      const factor = e.deltaY < 0 ? 1.18 : 1 / 1.18;
      const newScale = Math.max(0.15, Math.min(12, scale * factor));
      tx = svgX - (svgX - tx) * (newScale / scale);
      ty = svgY - (svgY - ty) * (newScale / scale);
      scale = newScale;
    }
    svgEl.addEventListener('wheel', onWheel, { passive: false });
    return () => svgEl?.removeEventListener('wheel', onWheel);
  });

  // ── Pan ─────────────────────────────────────────────────────────────────────

  let panning = $state(false);
  let panOrigin = { cx: 0, cy: 0, tx: 0, ty: 0, rw: 1, rh: 1 };

  function onBgPointerDown(e: PointerEvent) {
    if (e.button !== 0 || !svgEl) return;
    e.stopPropagation();
    panning = true;
    const r = svgEl.getBoundingClientRect();
    panOrigin = { cx: e.clientX, cy: e.clientY, tx, ty, rw: r.width, rh: r.height };
    (e.currentTarget as SVGElement).setPointerCapture(e.pointerId);
  }

  function onBgPointerMove(e: PointerEvent) {
    if (!panning) return;
    tx = panOrigin.tx + ((e.clientX - panOrigin.cx) / panOrigin.rw) * WIDTH;
    ty = panOrigin.ty + ((e.clientY - panOrigin.cy) / panOrigin.rh) * HEIGHT;
  }

  function onBgPointerUp() {
    panning = false;
  }

  // ── Selected entity (click-to-browse-articles) ───────────────────────────────

  interface SelectedEntity {
    text: string;
    label: string;
    coOccursWith: string[];
    wikidataQid?: string | null;
  }
  let selectedEntity = $state<SelectedEntity | null>(null);

  // Phase 118 / 121b: external Wikidata + Wikipedia links for entity nodes
  // whose Wikidata QID was resolved by the Phase-118 alias index. Helpers
  // live in cooccurrence-network-internals.ts so vitest can pin the URL
  // shape without a Svelte compiler pass.

  // ── Node drag + click distinction ────────────────────────────────────────────

  let draggingNode: SimNode | null = null;
  let dragMoved = false;
  let dragDownClient = { x: 0, y: 0 };

  function onNodePointerDown(e: PointerEvent, n: SimNode) {
    e.stopPropagation(); // don't trigger pan
    draggingNode = n;
    dragMoved = false;
    dragDownClient = { x: e.clientX, y: e.clientY };
    n.fx = n.x ?? 0;
    n.fy = n.y ?? 0;
    simulation?.alphaTarget(0.3).restart();
    (e.currentTarget as SVGElement).setPointerCapture(e.pointerId);
  }

  function onNodePointerMove(e: PointerEvent) {
    if (!draggingNode) return;
    const dx = e.clientX - dragDownClient.x;
    const dy = e.clientY - dragDownClient.y;
    if (dx * dx + dy * dy > 25) dragMoved = true; // >5px = real drag
    const { x: sx, y: sy } = clientToSvg(e.clientX, e.clientY);
    const { x: simX, y: simY } = svgToSim(sx, sy);
    draggingNode.fx = simX;
    draggingNode.fy = simY;
  }

  function onNodePointerUp(e: PointerEvent, n: SimNode) {
    if (!draggingNode) return;
    if (!dragMoved) {
      // Click: select entity and show article panel.
      if (selectedEntity?.text === n.id) {
        selectedEntity = null; // toggle off
      } else {
        const coOccursWith = edges
          .filter((ed) => nodeId(ed.source) === n.id || nodeId(ed.target) === n.id)
          .map((ed) => (nodeId(ed.source) === n.id ? nodeId(ed.target) : nodeId(ed.source)))
          .sort();
        selectedEntity = {
          text: n.id,
          label: n.label,
          coOccursWith,
          wikidataQid: n.wikidataQid ?? null
        };
      }
    }
    draggingNode.fx = null;
    draggingNode.fy = null;
    simulation?.alphaTarget(0);
    draggingNode = null;
    dragMoved = false;
  }

  // ── Helpers ──────────────────────────────────────────────────────────────────

  function labelColor(label: string): string {
    const palette = ['#5283b8', '#b87a52', '#52b885', '#a058b8', '#b85265', '#888888'];
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
    {#if nodes.length > 0}
      <button class="reset-btn" onclick={resetView} title="Reset zoom">⊙</button>
    {/if}
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
      bind:this={svgEl}
      class="graph"
      class:panning
      viewBox="0 0 {WIDTH} {HEIGHT}"
      role="img"
      aria-label="Force-directed entity co-occurrence graph. Scroll to zoom, drag to pan, drag nodes to reposition."
    >
      <!-- Transparent hit-target for pan -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <rect
        width={WIDTH}
        height={HEIGHT}
        fill="transparent"
        onpointerdown={onBgPointerDown}
        onpointermove={onBgPointerMove}
        onpointerup={onBgPointerUp}
        onpointercancel={onBgPointerUp}
      />
      <g transform="translate({tx},{ty}) scale({scale})">
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
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <g
              transform="translate({n.x ?? 0},{n.y ?? 0})"
              class="node"
              onpointerdown={(e) => onNodePointerDown(e, n)}
              onpointermove={onNodePointerMove}
              onpointerup={(e) => onNodePointerUp(e, n)}
              onpointercancel={(e) => onNodePointerUp(e, n)}
            >
              <circle
                r={n.radius}
                fill={labelColor(n.label)}
                fill-opacity="0.85"
                stroke={selectedEntity?.text === n.id ? 'var(--color-fg)' : 'none'}
                stroke-width="1.5"
              />
              <title>{n.id} ({n.label})</title>
              <text
                x={n.radius + 3}
                y={3}
                font-size={n.radius > 8 ? '10' : '8'}
                fill="var(--color-fg)"
                fill-opacity={n.radius > 8 ? '1' : '0.7'}
                font-family="var(--font-mono)">{n.id}</text
              >
            </g>
          {/each}
        </g>
      </g>
    </svg>
    <p class="hint">
      Scroll to zoom · drag background to pan · drag nodes to reposition · click node for articles
    </p>
  {:else}
    <p class="muted" aria-busy="true">Laying out…</p>
  {/if}
  {#if selectedEntity}
    <div class="entity-panel">
      <header class="entity-panel-header">
        <span class="entity-name">{selectedEntity.text}</span>
        <span class="entity-label-badge">{selectedEntity.label}</span>
        {#if selectedEntity.wikidataQid}
          <span class="external-links" data-testid="entity-external-links">
            <!-- eslint-disable svelte/no-navigation-without-resolve -->
            <a
              class="ext-link"
              href={wikidataHref(selectedEntity.wikidataQid)}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={`Wikidata page for ${selectedEntity.text} (external, opens in new tab)`}
              title="Open Wikidata page"
            >
              <span aria-hidden="true">↗ Wikidata</span>
            </a>
            <a
              class="ext-link"
              href={wikipediaHref(selectedEntity.wikidataQid)}
              target="_blank"
              rel="noopener noreferrer"
              aria-label={`Wikipedia article for ${selectedEntity.text} (external, opens in new tab)`}
              title="Open Wikipedia article"
            >
              <span aria-hidden="true">↗ Wikipedia</span>
            </a>
            <!-- eslint-enable svelte/no-navigation-without-resolve -->
          </span>
        {/if}
        {#if selectedEntity.coOccursWith.length > 0}
          <span class="cooccurs-hint">
            co-occurs with:
            {#each selectedEntity.coOccursWith.slice(0, 6) as peer (peer)}
              <button
                class="peer-chip"
                onclick={() => {
                  const n = nodes.find((nd) => nd.id === peer);
                  if (n) {
                    const coOccursWith = edges
                      .filter((ed) => nodeId(ed.source) === peer || nodeId(ed.target) === peer)
                      .map((ed) =>
                        nodeId(ed.source) === peer ? nodeId(ed.target) : nodeId(ed.source)
                      )
                      .sort();
                    selectedEntity = {
                      text: peer,
                      label: n.label,
                      coOccursWith,
                      wikidataQid: n.wikidataQid ?? null
                    };
                  }
                }}>{peer}</button
              >
            {/each}
            {#if selectedEntity.coOccursWith.length > 6}
              <span class="muted">+{selectedEntity.coOccursWith.length - 6} more</span>
            {/if}
          </span>
        {/if}
        <button
          class="close-btn"
          onclick={() => (selectedEntity = null)}
          aria-label="Close entity panel">✕</button
        >
      </header>
      <div class="source-lists">
        {#each sources as src (src.name)}
          <div class="source-section">
            <h4 class="source-section-title">{src.emicDesignation ?? src.name}</h4>
            <ArticlePreviewList
              sourceId={src.name}
              {ctx}
              {windowStart}
              {windowEnd}
              entityMatch={selectedEntity.text}
            />
          </div>
        {/each}
      </div>
    </div>
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

  .reset-btn {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    background: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 1px 6px;
    cursor: pointer;
    line-height: 1.6;
  }
  .reset-btn:hover {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .graph {
    width: 100%;
    height: auto;
    max-height: 65vh;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    cursor: grab;
    touch-action: none;
  }

  .graph.panning {
    cursor: grabbing;
  }

  .node {
    cursor: grab;
  }

  .node:active {
    cursor: grabbing;
  }

  .hint {
    font-size: 10px;
    color: var(--color-fg-subtle);
    margin: 0;
    text-align: center;
    letter-spacing: 0.02em;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .entity-panel {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    background: var(--color-bg-elevated);
    overflow: hidden;
  }

  .entity-panel-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg);
  }

  .entity-name {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
    font-family: var(--font-mono);
  }

  .entity-label-badge {
    font-size: 10px;
    color: var(--color-fg-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 5px;
    line-height: 1.6;
  }

  .external-links {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
  }

  .ext-link {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 5px;
    line-height: 1.6;
    text-decoration: none;
  }
  .ext-link:hover,
  .ext-link:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: none;
  }

  .cooccurs-hint {
    font-size: 10px;
    color: var(--color-fg-subtle);
    display: flex;
    align-items: center;
    gap: var(--space-1);
    flex-wrap: wrap;
  }

  .peer-chip {
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 5px;
    cursor: pointer;
    line-height: 1.6;
  }
  .peer-chip:hover {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .close-btn {
    margin-left: auto;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    background: none;
    border: none;
    cursor: pointer;
    padding: 2px 4px;
    line-height: 1;
  }
  .close-btn:hover {
    color: var(--color-fg);
  }

  .source-lists {
    display: flex;
    flex-direction: column;
    gap: 0;
  }

  .source-section {
    padding: var(--space-3) var(--space-4);
    border-bottom: 1px solid var(--color-border);
  }
  .source-section:last-child {
    border-bottom: none;
  }

  .source-section-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-2);
    text-transform: uppercase;
    letter-spacing: 0.06em;
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
