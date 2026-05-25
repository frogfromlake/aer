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
  import type { ExportRow, ExportPayload } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import CellExport from './CellExport.svelte';
  import HowToRead from './HowToRead.svelte';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    sources,
    dataLayer = 'gold',
    topN,
    channels,
    forceStrength
  }: ViewModeCellProps = $props();

  // Phase 131 — configurable top-edge cap (default 60, BFF-clamped to [1,500])
  // and visual-channel binding (node size, node colour).
  const TOP_N = $derived(topN ?? 60);
  const netSize = $derived(channels?.netSize ?? 'total_count');
  const netColor = $derived(channels?.netColor ?? 'label');
  // Phase 131 (BUG1.7) — force-layout spread (0..100 → node repulsion).
  // Higher spreads a crowded graph apart so it stays readable.
  const spread = $derived(forceStrength ?? 50);
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
    /** Raw channel values for the tooltip. */
    totalCount: number;
    degree: number;
    /** Number of distinct sources the entity appears in (colour channel
     *  `presence`); 0 when the BFF did not return a presence array. */
    presenceCount: number;
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
    // Phase 131 — node size bound to the selected channel: total co-occurrence
    // weight (default) or node degree.
    const sizeOf = (n: (typeof data.nodes)[number]) =>
      netSize === 'degree' ? (n.degree ?? 0) : n.totalCount;
    const maxSize = data.nodes.reduce((m, n) => Math.max(m, sizeOf(n)), 1);
    const seedNodes: SimNode[] = data.nodes.map((n) => ({
      id: n.text,
      label: n.label,
      radius: 4 + 14 * Math.sqrt(sizeOf(n) / maxSize),
      totalCount: n.totalCount,
      degree: n.degree ?? 0,
      presenceCount: n.presence?.length ?? 0,
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
        // Phase 131 (BUG1.7) — repulsion is user-tunable via the Spread
        // slider (0..100). Maps to charge magnitude so a crowded single
        // cluster can be pulled apart for readability. Default 50 → -150.
        .force('charge', d3.forceManyBody<SimNode>().strength(-(20 + spread * 2.6)))
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
      // Phase 131 (BUG1.6) — only zoom when Ctrl/Cmd is held, so a plain
      // mouse-wheel scrolls the page instead of being swallowed by the graph.
      if (!(e.ctrlKey || e.metaKey)) return;
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

  // Phase 131 — node colour bound to the selected channel.
  let maxPresence = $derived(nodes.reduce((m, n) => Math.max(m, n.presenceCount), 1));
  function nodeFill(n: SimNode): string {
    if (netColor === 'uniform') return '#5283b8';
    if (netColor === 'presence') {
      const t = maxPresence > 1 ? (n.presenceCount - 1) / (maxPresence - 1) : 0;
      const lo = [82, 131, 184];
      const hi = [224, 160, 80];
      const c = lo.map((l, i) => Math.round(l + ((hi[i] ?? l) - l) * t));
      return `rgb(${c[0]}, ${c[1]}, ${c[2]})`;
    }
    return labelColor(n.label);
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

  // Phase 131 — export (the full edge list) + how-to-read facts.
  const exportRows = $derived<ExportRow[]>(
    (graphQ.data?.kind === 'success' ? graphQ.data.data.edges : []).map((e) => ({
      entityA: e.a,
      entityB: e.b,
      weight: e.weight,
      articleCount: e.articleCount ?? ''
    }))
  );
  const howToReadFacts = $derived({
    topN: TOP_N,
    netSize,
    netColor,
    renderedCount: nodes.length
  });
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'cooccurrence_network',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      topN: TOP_N,
      sizeChannel: netSize,
      colorChannel: netColor
    },
    summary: { nodes: nodes.length, edges: edges.length },
    howToRead: composeHowToRead('cooccurrence_network', howToReadFacts),
    rows: exportRows,
    columns: ['entityA', 'entityB', 'weight', 'articleCount']
  });
  const exportFilenameParts = $derived([
    'cooccurrence-network',
    scope === 'source' ? scopeId : 'probe'
  ]);
  function getSvg(): SVGSVGElement | null {
    return svgEl;
  }
</script>

<section class="net-cell" aria-labelledby="net-title">
  <header class="cell-header">
    <h3 id="net-title" class="cell-title">
      Entity co-occurrence
      <span class="muted">— top {TOP_N} pairs · <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if nodes.length > 0}
      <div class="header-actions">
        <CellExport {getSvg} payload={exportPayload} filenameParts={exportFilenameParts} />
        <button class="reset-btn" onclick={resetView} title="Reset zoom">⊙</button>
      </div>
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
    {#if nodes.length < 8}
      <!-- Phase 122i revision (A6). When the graph collapses to a handful
           of nodes, surface a methodology note so the reader does not
           mistake corpus sparseness for an analytical conclusion. The
           BFF logs the same statistic in `op=GetEntityCoOccurrence` /
           `op=PostEntityCoOccurrenceQuery` for operator-side diagnosis. -->
      <aside class="methodology-note" role="note" aria-label="Sparse-corpus note">
        <strong>{nodes.length} nodes · {edges.length} edges</strong> — the entity-co-occurrence graph
        collapsed to a small set. Possible causes: short scope window, few articles in the active source
        set, NER pipeline conservative on this corpus, or a few dominant entities crowding out the rest.
        Widen the window or pick a richer source subset.
      </aside>
    {/if}
    <svg
      bind:this={svgEl}
      class="graph"
      class:panning
      viewBox="0 0 {WIDTH} {HEIGHT}"
      role="img"
      aria-label="Force-directed entity co-occurrence graph. Ctrl or Cmd plus scroll to zoom, drag to pan, drag nodes to reposition."
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
                fill={nodeFill(n)}
                fill-opacity="0.85"
                stroke={selectedEntity?.text === n.id ? 'var(--color-fg)' : 'none'}
                stroke-width="1.5"
              />
              <title
                >{n.id} · {n.label} — weight {n.totalCount}, {n.degree} neighbour{n.degree === 1
                  ? ''
                  : 's'}{n.presenceCount > 0
                  ? `, in ${n.presenceCount} source${n.presenceCount === 1 ? '' : 's'}`
                  : ''} · click to see articles</title
              >
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
      Ctrl/⌘ + scroll to zoom · drag background to pan · drag nodes to reposition · click node for
      articles
    </p>
    <HowToRead presentation="cooccurrence_network" facts={howToReadFacts} />
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

  .header-actions {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
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

  .methodology-note {
    display: block;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: 1.4;
    background: color-mix(in srgb, var(--color-status-expired) 10%, var(--color-surface));
    border-left: 3px solid var(--color-status-expired);
    margin-bottom: var(--space-2);
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

  .scope-name {
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
    font-family: var(--font-mono);
  }
</style>
