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
  import ArticleListModal from '$lib/components/lanes/ArticleListModal.svelte';
  import { wikidataHref, wikipediaHref } from './cooccurrence-network-internals';
  import { viewerLabelLanguage } from '$lib/viewmodes/viewer-language';
  import { negativeSpaceActive } from '$lib/state/tray.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import type { ExportPayload } from '$lib/viewmodes/cell-export';
  import {
    computeMetricExtent,
    computeMetricColorExtent,
    computeCommunities,
    resolvedSourceCount as resolvedSourceCountOf,
    buildNetworkNodes,
    buildNetworkEdges,
    nodeRadius,
    buildSourceColorMap,
    nodeFillColor,
    nodeStrokeColor as nodeStrokeColorShared,
    nodeStrokeWidth as nodeStrokeWidthShared,
    edgeStrokeColor,
    buildHowToReadFacts,
    buildExportPayload,
    SHARED_COLOR,
    UNKNOWN_PROVENANCE_COLOR,
    type MetricExtent,
    type NodeColorContext
  } from '$lib/viewmodes/cooccurrence-network-shared';
  import {
    fmtValue,
    HIDDEN_READOUT,
    type ReadoutRow,
    type ReadoutState
  } from '$lib/viewmodes/cell-readout';
  import CellExport from './CellExport.svelte';
  import CellReadout from './CellReadout.svelte';
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
    forceStrength,
    showEdges,
    displayLanguage,
    configOverridden
  }: ViewModeCellProps = $props();

  // Phase 123b — cross-lingual relabel. When the panel toggle is on 'viewer',
  // request the app-language label per QID-linked node; 'source' (default)
  // sends nothing and every node keeps its source surface form. The language is
  // the app content language clamped to the index's label languages.
  const viewerLang = $derived(displayLanguage === 'viewer' ? viewerLabelLanguage() : undefined);

  // Phase 131 — configurable top-edge cap (default 60, BFF-clamped to [1,500])
  // and visual-channel binding (node size, node colour).
  const TOP_N = $derived(topN ?? 60);
  const netSize = $derived(channels?.netSize ?? 'total_count');
  // Phase 125 — per-article metric(s) aggregated onto nodes. Size and colour can
  // bind to DIFFERENT metrics (ISSUE 7): `netMetric` drives the size channel,
  // `netColorMetric` the colour channel (falling back to `netMetric` when unset).
  const netMetric = $derived(channels?.netMetric);
  const netColorMetric = $derived(channels?.netColorMetric);
  // The metric requested per channel. Reads the RAW `channels?.netColor` (NOT the
  // defaulted `netColor` below) to avoid a query→result→query TDZ cycle on
  // `graphQ`: the default only ever resolves to 'source_overlay'/'label', so
  // netColor === 'metric' is always an explicit selection.
  const sizeMetricReq = $derived(netSize === 'metric' ? netMetric : undefined);
  const colorMetricReq = $derived(
    channels?.netColor === 'metric' ? (netColorMetric ?? netMetric) : undefined
  );
  // Phase 131 (BUG1.7) — force-layout spread (0..100 → node repulsion).
  // Higher spreads a crowded graph apart so it stays readable.
  const spread = $derived(forceStrength ?? 50);
  // Co-occurrence redesign — show/hide edge (connection) lines; default HIDDEN
  // (nodes-only reads cleaner; clustering carries the structure). The force
  // layout still uses the edges — only rendering is toggled.
  const edgesShown = $derived(showEdges ?? false);
  const WIDTH = 720;
  const HEIGHT = 500;
  // Phase 122d.2 — when the NS toggle is on, request the overlay so each edge
  // carries `nsSupport` (contributing articles with no real publication date);
  // edges with nsSupport>0 render dashed as a disclosure (never filtered).
  const negSpaceOn = $derived(negativeSpaceActive());

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
      topN: TOP_N,
      ...(viewerLang ? { viewerLanguage: viewerLang } : {}),
      ...(negSpaceOn ? { negativeSpaceOverlay: 'ghost' as const } : {}),
      ...(sizeMetricReq ? { nodeMetric: sizeMetricReq } : {}),
      ...(colorMetricReq ? { nodeColorMetric: colorMetricReq } : {})
    });
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  // Phase 131a — "merged scope" is decided from the BFF response (the
  // union of distinct source names across edge.presence / node.presence),
  // NOT the `sources` prop. A single-probe scope often resolves to
  // multiple underlying sources on the BFF side ("probe-as-aggregator"),
  // and the `sources` prop may carry only the probe's primary entry.
  // The BFF-resolved set is the SoT for the overlay. Declared AFTER `graphQ`
  // (it reads `graphQ.data`).
  const resolvedSourceCount = $derived.by(() => {
    const data = graphQ.data?.kind === 'success' ? graphQ.data.data : null;
    return data ? resolvedSourceCountOf(data) : 0;
  });
  const isMergedScope = $derived(resolvedSourceCount > 1 || sources.length > 1);
  // Kriesel default — colour by detected theme cluster (Louvain community).
  const netColor = $derived(channels?.netColor ?? 'community');
  // Theme-cluster (Louvain) map, loaded async when community colouring is active.
  let communities = $state<Map<string, number> | null>(null);
  $effect(() => {
    const want = netColor === 'community';
    const d = graphQ.data?.kind === 'success' ? graphQ.data.data : null;
    if (!want || !d) {
      communities = null;
      return;
    }
    let cancelled = false;
    void computeCommunities(d).then((m) => {
      if (!cancelled) communities = m;
    });
    return () => {
      cancelled = true;
    };
  });
  // Phase 125 — min/max of the per-node SIZE metric across the returned graph,
  // for normalising the metric size channel. Null when no node carries it.
  const metricExtent = $derived.by<MetricExtent | null>(() => {
    const d = graphQ.data?.kind === 'success' ? graphQ.data.data : null;
    return d ? computeMetricExtent(d) : null;
  });
  // Phase 125 / ISSUE 7 — separate extent for the COLOUR metric (may differ from
  // the size metric). Used by the colour ramp + the colour legend.
  const metricColorExtent = $derived.by<MetricExtent | null>(() => {
    const d = graphQ.data?.kind === 'success' ? graphQ.data.data : null;
    return d ? computeMetricColorExtent(d) : null;
  });

  interface SimNode {
    id: string;
    label: string;
    /** Source-language surface form (the node's original text). */
    sourceText: string;
    /** Phase 123b — viewer-language Wikidata label, or null when the node is
     *  unlinked / has no label in the viewer language. */
    viewerLabel: string | null;
    /** The name actually rendered: viewerLabel when relabelling is on and one
     *  exists, otherwise the source surface form. */
    displayName: string;
    /** True when displayName differs from the source form (relabelled). */
    relabeled: boolean;
    radius: number;
    /** Raw channel values for the tooltip. */
    totalCount: number;
    degree: number;
    /** Number of distinct sources the entity appears in (colour channel
     *  `presence`); 0 when the BFF did not return a presence array. */
    presenceCount: number;
    /** Source names this node appears in (Phase 131a — copied from
     *  `node.presence` so the source-overlay palette can colour the node
     *  directly without an additional lookup). */
    presence: string[];
    wikidataQid?: string | null;
    /** Phase 125 — mean per-article SIZE metric; null when no size metric was
     *  requested or the entity has no such article. */
    metricValue?: number | null;
    /** Phase 125 / ISSUE 7 — mean per-article COLOUR metric (falls back to the
     *  size metric when colour reuses it); null when neither applies. */
    metricColorValue?: number | null;
    /** Louvain community id (theme cluster) for the 'community' colour channel. */
    community?: number | null;
    x?: number;
    y?: number;
    fx?: number | null;
    fy?: number | null;
  }
  interface SimEdge {
    source: string | SimNode;
    target: string | SimNode;
    weight: number;
    /** Co-occurrence article support (Phase 132 — surfaced in the edge
     *  hover readout; 0 when the BFF did not return it). */
    articleCount: number;
    /** Source names this edge appears in (Phase 131a — copied from
     *  `edge.presence`). Empty array when the BFF did not return per-
     *  edge presence (single-source scope). */
    presence: string[];
    /** Phase 122d.2 — contributing articles with no real publication date
     *  (>0 only under the NS overlay). Edges with nsSupport>0 render dashed. */
    nsSupport: number;
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
    // Phase 125b — node/edge models come from the shared module (size channel,
    // relabel, presence) so the SVG cell and the at-scale renderer stay in
    // lock-step. The radius range (4..26) is this renderer's mapping of the
    // shared 0..1 `sizeNorm`.
    const seedNodes: SimNode[] = buildNetworkNodes(data, netSize, metricExtent, communities).map(
      (b) => ({
        ...b,
        radius: nodeRadius(b.sizeNorm)
      })
    );
    const seedEdges: SimEdge[] = buildNetworkEdges(data);
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
            // Phase 131 (BUG2.1) — link distance also scales with Spread so
            // connected nodes WITHIN a cluster move apart too, not just whole
            // clusters. 0 → tight (40), 100 → loose (200).
            .distance(40 + spread * 1.6)
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
      // Phase 131a — at 500 nodes the default d3 simulation runs for
      // several seconds with ~300 ticks. We raise alphaDecay so the
      // layout stabilises in ~1 s (default 0.0228 → 0.05, halving the
      // tick count) and lower the minimum-alpha threshold so the
      // simulation hard-stops once it's settled enough. Both reduce
      // total Svelte re-render work without throttling the per-tick
      // render path — the previous deferred-rAF throttle caused
      // pointer events to mis-target nodes because the rendered DOM
      // position lagged the simulation's internal positions by up to
      // 33 ms, making clicks land on the transparent pan-rect instead
      // of the visible node.
      simulation.alphaDecay(0.05).alphaMin(0.01);
      simulation.on('tick', () => {
        if (t !== token) return;
        nodes = [...seedNodes];
        edges = [...seedEdges];
      });
      // Belt-and-braces: simulation.on('end') guarantees a final
      // render at the stabilised positions, in case the last `tick`
      // raced with a token bump on data refresh.
      simulation.on('end', () => {
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
    readout = HIDDEN_READOUT;
    const r = svgEl.getBoundingClientRect();
    panOrigin = { cx: e.clientX, cy: e.clientY, tx, ty, rw: r.width, rh: r.height };
    (e.currentTarget as SVGElement).setPointerCapture(e.pointerId);
  }

  function onBgPointerMove(e: PointerEvent) {
    // Phase 132 — moving over empty background dismisses the hover readout.
    if (!panning) {
      readout = HIDDEN_READOUT;
      return;
    }
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

  // Phase 132 — exact-value hover readout for nodes AND edges. Nodes had a
  // native <title> (kept for accessibility); edges had no readout at all.
  // The box is suppressed during pan / node-drag so it never competes with
  // those gestures.
  let readout = $state<ReadoutState>(HIDDEN_READOUT);
  function showNodeReadout(e: PointerEvent, n: SimNode): void {
    const rows: ReadoutRow[] = [
      { label: 'weight', value: fmtValue(n.totalCount) },
      { label: 'degree', value: fmtValue(n.degree) }
    ];
    if (n.presenceCount > 0) {
      rows.push({ label: 'in sources', value: fmtValue(n.presenceCount) });
    }
    // Phase 125 (ISSUE 8 + ISSUE 7) — when a metric channel is bound, surface
    // that node's metric value so the tooltip reflects the chosen channel(s)
    // (was always just weight/degree). Size and colour may bind to different
    // metrics; a node with no such article reads 'no data', never 0.
    if (sizeMetricReq) {
      rows.push({
        label: `size · ${sizeMetricReq}`,
        value: n.metricValue != null ? fmtValue(n.metricValue) : 'no data'
      });
    }
    if (colorMetricReq && colorMetricReq !== sizeMetricReq) {
      rows.push({
        label: `colour · ${colorMetricReq}`,
        value: n.metricColorValue != null ? fmtValue(n.metricColorValue) : 'no data'
      });
    }
    readout = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      title: `${n.id} · ${n.label}`,
      rows,
      hint: 'Click to see articles'
    };
  }
  function onEdgeHover(e: PointerEvent, edge: SimEdge): void {
    if (panning || draggingNode) return;
    const rows: ReadoutRow[] = [{ label: 'weight', value: fmtValue(edge.weight) }];
    if (edge.articleCount > 0) {
      rows.push({ label: 'articles', value: fmtValue(edge.articleCount) });
    }
    if (isMergedScope && edge.presence.length > 0) {
      rows.push({ label: 'sources', value: edge.presence.join(', ') });
    }
    readout = {
      visible: true,
      x: e.clientX,
      y: e.clientY,
      title: `${nodeId(edge.source)} — ${nodeId(edge.target)}`,
      rows
    };
  }

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
    readout = HIDDEN_READOUT;
    dragDownClient = { x: e.clientX, y: e.clientY };
    n.fx = n.x ?? 0;
    n.fy = n.y ?? 0;
    simulation?.alphaTarget(0.3).restart();
    (e.currentTarget as SVGElement).setPointerCapture(e.pointerId);
  }

  function onNodePointerMove(e: PointerEvent, n: SimNode) {
    // Dragging: update the captured node's fixed position (Phase 132 — the
    // hover readout is suppressed while a drag is in flight).
    if (draggingNode) {
      const dx = e.clientX - dragDownClient.x;
      const dy = e.clientY - dragDownClient.y;
      if (dx * dx + dy * dy > 25) dragMoved = true; // >5px = real drag
      const { x: sx, y: sy } = clientToSvg(e.clientX, e.clientY);
      const { x: simX, y: simY } = svgToSim(sx, sy);
      draggingNode.fx = simX;
      draggingNode.fy = simY;
      return;
    }
    if (panning) return;
    showNodeReadout(e, n);
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

  // Phase 125b — colour/size logic lives in the shared module
  // (`cooccurrence-network-shared.ts`); these are thin reactive wrappers so the
  // SVG template is unchanged and the at-scale renderer computes identical
  // visuals. The source palette / label hash / blue→amber ramps live there.
  const sourceColorMap = $derived(buildSourceColorMap(sources.map((s) => s.name)));
  const maxPresence = $derived(nodes.reduce((m, n) => Math.max(m, n.presenceCount), 1));
  const nodeColorCtx = $derived<NodeColorContext>({
    netColor,
    // ISSUE 7 — the colour channel uses the COLOUR metric's extent (may differ
    // from the size metric); nodeFillColor reads each node's metricColorValue.
    metricExtent: metricColorExtent,
    maxPresence,
    sourceColorMap
  });
  function nodeFill(n: SimNode): string {
    return nodeFillColor(n, nodeColorCtx);
  }
  function nodeStrokeColor(n: SimNode, isSelected: boolean): string | 'none' {
    return nodeStrokeColorShared(n, isMergedScope, sourceColorMap, isSelected);
  }
  function nodeStrokeWidth(n: SimNode, isSelected: boolean): number {
    return nodeStrokeWidthShared(n.radius, isMergedScope, isSelected);
  }
  function edgeStroke(e: SimEdge): string {
    return edgeStrokeColor(e, isMergedScope, sourceColorMap);
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

  // Phase 123b — cross-lingual coverage. linkedNodeCount = nodes carrying a
  // QID (the relabel-eligible subset); labeledNodeCount = those that actually
  // got a viewer-language label. Surfaced so the reader sees how much of a
  // foreign-language graph the toggle can relabel (no silent gaps, WP-006).
  const graphData = $derived(graphQ.data?.kind === 'success' ? graphQ.data.data : null);
  const totalNodeCount = $derived(graphData?.nodes.length ?? 0);
  const linkedNodeCount = $derived(graphData?.linkedNodeCount ?? 0);
  const labeledNodeCount = $derived(graphData?.labeledNodeCount ?? 0);
  const relabelActive = $derived(displayLanguage === 'viewer' && !!viewerLang);
  // Phase 125b — how-to-read facts + export payload come from the shared module
  // (same contract as the at-scale renderer). `howToReadFacts` still feeds the
  // <HowToRead> component below.
  const howToReadFacts = $derived(
    buildHowToReadFacts({
      topN: TOP_N,
      netSize,
      netColor,
      renderedCount: nodes.length,
      displayLanguage: displayLanguage ?? 'source',
      viewerLanguage: viewerLang,
      linkedNodeCount,
      labeledNodeCount,
      configOverridden
    })
  );
  const exportPayload = $derived<ExportPayload>(
    buildExportPayload({
      scope,
      scopeId,
      windowStart,
      windowEnd,
      topN: TOP_N,
      netSize,
      netColor,
      nodeCount: nodes.length,
      edgeCount: edges.length,
      howToReadFacts,
      data: graphData
    })
  );
  const exportFilenameParts = $derived([
    'cooccurrence-network',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="net-cell" aria-labelledby="net-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="net-title" class="cell-title">
      Entity co-occurrence
      <span class="muted">— top {TOP_N} pairs · <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if nodes.length > 0}
      <div class="header-actions">
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
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
    {@const articlesInScope = graphQ.data.data.articlesInScope ?? 0}
    {#if articlesInScope > 0}
      <!-- Phase 131a pipeline-gap hint (BUG 1.5). The BFF reports
           `articlesInScope > 0` while the co-occurrence sweep emitted
           no rows — i.e. the data exists in `aer_gold.entities` but
           the corpus extractor missed it. Distinct from sparse data;
           the worker also logs `corpus.sweep.pipeline_gap` per source. -->
      <aside class="pipeline-gap" role="alert" aria-label="Pipeline gap warning">
        <strong>Pipeline gap detected.</strong>
        {articlesInScope}
        {articlesInScope === 1 ? 'article' : 'articles'} in this window have ≥2 entities, but the co-occurrence
        sweep produced zero rows. The data exists in
        <code>aer_gold.entities</code>; the worker's corpus extractor is missing it. Check
        <code>corpus.sweep.pipeline_gap</code> warnings in the analysis-worker logs.
      </aside>
    {:else}
      <p class="muted">No entity co-occurrences in this window.</p>
    {/if}
  {:else if nodes.length > 0}
    {#if isMergedScope}
      <!-- Phase 131a merged-provenance note (BUG 1.5). Surfaces what is
           merged so the reader does not mistake an overlay graph for a
           single-source one. Mirrors the methodology-banner pattern
           the other pillars already use. -->
      <aside class="methodology-merged" role="note" aria-label="Merged provenance">
        <strong>Merged graph</strong> — co-occurrences from {sources.length} sources are pooled into one
        network. Source provenance is carried by the <em>border ring</em> on each node and the
        <em>edge stroke</em>, so a metric channel (entity type, source presence, …) on the node
        <em>fill</em> stays readable. Nodes and edges observed in more than one source render in
        grey. Picking <em>Source overlay</em> as the Colour channel makes the fill match the border for
        a single-encoding view.
      </aside>
    {/if}
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
          {#if edgesShown}
            {#each edges as e (edgeKey(e))}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <line
                x1={nodeX(e.source)}
                y1={nodeY(e.source)}
                x2={nodeX(e.target)}
                y2={nodeY(e.target)}
                stroke={edgeStroke(e)}
                stroke-width={0.4 + 2.4 * (e.weight / maxEdgeWeight)}
                stroke-dasharray={negSpaceOn && e.nsSupport > 0 ? '3 3' : undefined}
                stroke-opacity={negSpaceOn && e.nsSupport > 0 ? 0.45 : undefined}
                onpointermove={(ev) => onEdgeHover(ev, e)}
                onpointerleave={() => (readout = HIDDEN_READOUT)}
              />
            {/each}
          {/if}
        </g>
        <g class="nodes">
          {#each nodes as n (n.id)}
            <!-- svelte-ignore a11y_no_static_element_interactions -->
            <g
              transform="translate({n.x ?? 0},{n.y ?? 0})"
              class="node"
              onpointerdown={(e) => onNodePointerDown(e, n)}
              onpointermove={(e) => onNodePointerMove(e, n)}
              onpointerup={(e) => onNodePointerUp(e, n)}
              onpointercancel={(e) => onNodePointerUp(e, n)}
              onpointerleave={() => (readout = HIDDEN_READOUT)}
            >
              <circle
                r={n.radius}
                fill={nodeFill(n)}
                fill-opacity="0.85"
                stroke={nodeStrokeColor(n, selectedEntity?.text === n.id)}
                stroke-width={nodeStrokeWidth(n, selectedEntity?.text === n.id)}
              />
              <text
                x={n.radius + 3}
                y={3}
                font-size={n.radius > 8 ? '10' : '8'}
                fill="var(--color-fg)"
                fill-opacity={n.radius > 8 ? '1' : '0.7'}
                font-family="var(--font-mono)"
                >{n.displayName}{#if n.relabeled}<tspan class="relabel-mark" dx="3"
                    >↺<title>Relabelled from “{n.sourceText}” ({viewerLang})</title></tspan
                  >{/if}</text
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
    {#if totalNodeCount > 0}
      <p class="link-coverage" role="status">
        {#if relabelActive}
          <strong>{labeledNodeCount}</strong> of {linkedNodeCount} Wikidata-linked
          {linkedNodeCount === 1 ? 'node' : 'nodes'} shown in the app language (<strong
            >{viewerLang}</strong
          >); ↺ marks those whose label differs from the source form. The remaining {totalNodeCount -
            labeledNodeCount} keep their source form
          {#if linkedNodeCount > labeledNodeCount}(incl. {linkedNodeCount - labeledNodeCount} linked with
            no {viewerLang} label){/if}.
        {:else}
          {linkedNodeCount} of {totalNodeCount}
          {totalNodeCount === 1 ? 'node' : 'nodes'} link to Wikidata — switch <em>Labels</em> to the app
          language to relabel that subset; unlinked nodes stay on their source form.
        {/if}
      </p>
    {/if}
    {#if isMergedScope}
      <!-- Phase 131a — source-coloured overlay legend.
           Shown whenever the scope is merged, regardless of the active
           Colour channel, because source provenance is always carried
           by the node border ring + edge stroke in merged scopes (the
           Colour channel controls the *fill* metric). When the user
           picks `Source overlay` as the Colour channel, the fill ALSO
           matches the legend — the legend is consistent either way. -->
      <ul class="source-legend" aria-label="Source overlay legend">
        {#each sources as src (src.name)}
          <li>
            <span
              class="legend-swatch"
              style="background:{sourceColorMap[src.name] ?? UNKNOWN_PROVENANCE_COLOR}"
              aria-hidden="true"
            ></span>
            <span class="legend-label">{src.emicDesignation ?? src.name}</span>
          </li>
        {/each}
        <li>
          <span class="legend-swatch" style="background:{SHARED_COLOR}" aria-hidden="true"></span>
          <span class="legend-label">shared (≥2 sources)</span>
        </li>
        <li>
          <span
            class="legend-swatch"
            style="background:{UNKNOWN_PROVENANCE_COLOR}"
            aria-hidden="true"
          ></span>
          <span class="legend-label">provenance unavailable</span>
        </li>
      </ul>
    {/if}
    {#if sizeMetricReq || colorMetricReq}
      <!-- Phase 125 (ISSUE 8 + ISSUE 7) — metric-channel legend. Size and colour
           can bind to different metrics, each with its own extent; this is the
           visual key for the size ramp and/or colour ramp, with an honest note
           about grey (no-data) nodes. -->
      <div class="metric-legend" aria-label="Metric channel legend">
        {#if sizeMetricReq}
          {#if metricExtent}
            <div class="metric-legend-row">
              <span class="metric-legend-title">Size · {sizeMetricReq}</span>
              <span class="metric-legend-min">{fmtValue(metricExtent.min)}</span>
              <span class="metric-legend-sizes" aria-hidden="true">
                <span class="size-dot size-dot-sm"></span>
                <span class="size-dot size-dot-lg"></span>
              </span>
              <span class="metric-legend-max">{fmtValue(metricExtent.max)}</span>
            </div>
          {:else}
            <span class="metric-legend-note"
              >No node carries “{sizeMetricReq}” (size) in this scope — channel inactive (honest
              absence).</span
            >
          {/if}
        {/if}
        {#if colorMetricReq}
          {#if metricColorExtent}
            <div class="metric-legend-row">
              <span class="metric-legend-title">Colour · {colorMetricReq}</span>
              <span class="metric-legend-min">{fmtValue(metricColorExtent.min)}</span>
              <span class="metric-legend-ramp" aria-hidden="true"></span>
              <span class="metric-legend-max">{fmtValue(metricColorExtent.max)}</span>
            </div>
          {:else}
            <span class="metric-legend-note"
              >No node carries “{colorMetricReq}” (colour) in this scope — channel inactive (honest
              absence).</span
            >
          {/if}
        {/if}
        <span class="metric-legend-note">Grey nodes have no such article (never a coerced 0).</span>
      </div>
    {/if}
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
    </div>
  {/if}
  <CellReadout {readout} />
</section>

<!-- Phase 122d.1 refactor — the article list for a selected entity
     opens in a modal stacked above the graph. Pre-122d.1 this was
     N source-lists stacked inline under the graph; the graph
     scrolled out of view on entity click. The modal keeps the
     graph visible behind it and pre-fills source tabs from the
     active scope. -->
<ArticleListModal
  open={selectedEntity !== null}
  title={selectedEntity ? `Articles mentioning "${selectedEntity.text}"` : 'Articles'}
  {ctx}
  {windowStart}
  {windowEnd}
  onClose={() => (selectedEntity = null)}
  config={{
    mode: 'source-articles',
    sources: sources.map((s) => ({
      name: s.name,
      label: s.emicDesignation ?? s.name
    })),
    entityMatch: selectedEntity?.text
  }}
/>

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

  /* Phase 123b — cross-lingual relabel coverage caption + per-node marker. */
  .link-coverage {
    font-size: 11px;
    color: var(--color-fg-muted);
    margin: 0.25rem 0 0;
    text-align: center;
  }
  .link-coverage strong {
    color: var(--color-fg);
  }
  .relabel-mark {
    fill: var(--color-accent, var(--color-fg-subtle));
    font-size: 9px;
    cursor: help;
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

  /* Phase 131a — merged-provenance note (informational). */
  .methodology-merged {
    display: block;
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: 1.4;
    background: color-mix(in srgb, var(--color-info, #5283b8) 8%, var(--color-surface));
    border-left: 3px solid var(--color-info, #5283b8);
    margin-bottom: var(--space-2);
  }

  /* Phase 131a — pipeline-gap warning (actionable / alarm). */
  .pipeline-gap {
    display: block;
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    line-height: 1.5;
    background: color-mix(in srgb, var(--color-status-expired) 14%, var(--color-surface));
    border-left: 3px solid var(--color-status-expired);
  }
  .pipeline-gap code {
    font-family: var(--font-mono);
    font-size: 11px;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0 4px;
  }

  /* Phase 131a — source-overlay legend. */
  .source-legend {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    font-size: 10px;
    color: var(--color-fg-muted);
  }
  .source-legend li {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .legend-swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
    display: inline-block;
    border: 1px solid var(--color-border);
  }
  .legend-label {
    font-family: var(--font-mono);
  }

  /* Phase 125 (ISSUE 8) — metric-channel legend (size and/or colour ramp). */
  .metric-legend {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 10px;
    color: var(--color-fg-muted);
  }
  .metric-legend-row {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-wrap: wrap;
  }
  .metric-legend-title {
    font-family: var(--font-mono);
    color: var(--color-fg);
  }
  .metric-legend-min,
  .metric-legend-max {
    font-family: var(--font-mono);
  }
  .metric-legend-ramp {
    width: 90px;
    height: 10px;
    border-radius: 2px;
    border: 1px solid var(--color-border);
    /* matches the shared METRIC_LO→METRIC_HI blue→amber ramp */
    background: linear-gradient(to right, rgb(82, 131, 184), rgb(224, 160, 80));
  }
  .metric-legend-sizes {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .size-dot {
    display: inline-block;
    border-radius: 50%;
    background: var(--color-fg-muted);
  }
  .size-dot-sm {
    width: 5px;
    height: 5px;
  }
  .size-dot-lg {
    width: 13px;
    height: 13px;
  }
  .metric-legend-note {
    font-style: italic;
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

  /* Pre-Phase-122d.1 N-stacked-source-list styles removed —
   * the entity-click article list now opens in `ArticleListModal`
   * with source tabs, so the graph stays visible behind it. */

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
