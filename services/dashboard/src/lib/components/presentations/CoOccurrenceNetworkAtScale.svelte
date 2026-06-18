<script lang="ts">
  // Phase 125b + co-occurrence redesign — large-scale co-occurrence renderer
  // (sigma.js / WebGL). PanelHost auto-routes here on a SINGLE cell once the
  // Top-N lever crosses ~500 edges (no Maximize dependency); below that the
  // default small/capped d3-force SVG cell (CoOccurrenceNetworkCell) renders.
  // Both compute identical node sizing / colour / relabel / export via the
  // SHARED module — this renderer adds only the WebGL rendering +
  // ForceAtlas2-in-a-worker layout + pan/zoom; it copies no logic.
  //
  // sigma + graphology + the FA2 worker are lazy-imported so their chunk ships
  // only when a user actually opens the at-scale view (Brief §7 bundle budget).
  import { untrack } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import {
    entityCoOccurrenceQuery,
    type CoOccurrenceGraphDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/article/ArticleListModal.svelte';
  import { wikidataHref, wikipediaHref } from './cooccurrence-network-internals';
  import { pickViewerLabelLanguage } from '$lib/presentations/viewer-language';
  import { locale } from '$lib/state/locale.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import type { ExportPayload } from '$lib/presentations/cell-export';
  import { HIDDEN_READOUT, fmtValue, type ReadoutState } from '$lib/presentations/cell-readout';
  import {
    computeMetricExtent,
    computeMetricColorExtent,
    resolvedSourceCount as resolvedSourceCountOf,
    buildNetworkNodes,
    buildNetworkEdges,
    buildSourceColorMap,
    computeCommunities,
    communityHeads,
    nodeFillColor,
    edgeStrokeColor,
    nodeRadius,
    buildHowToReadFacts,
    buildExportPayload,
    type MetricExtent,
    type NodeColorContext
  } from '$lib/presentations/cooccurrence-network-shared';
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
    forceStrength,
    showEdges,
    settleSeconds,
    channels,
    displayLanguage,
    configOverridden
  }: PresentationCellProps = $props();

  // Top-N is the single density lever (PanelControls), continuous with the SVG
  // cell and capped at the BFF ceiling — the renderer auto-switches into this
  // WebGL view once Top-N crosses ~500. (Min-weight was retired: it overlapped
  // Top-N and confused the density model.)
  const AT_SCALE_CEILING = 6000;
  const AT_SCALE_TOP_N = $derived(Math.min(AT_SCALE_CEILING, topN ?? AT_SCALE_CEILING));
  // Spread (forceStrength 0..100) — shared lever with the SVG cell; mapped to FA2
  // repulsion/gravity below so the layout responds the same way in both.
  const spread = $derived(forceStrength ?? 50);
  // Connection lines on/off — default HIDDEN (a nodes-only theme map reads far
  // cleaner; clustering + colour carry the structure). Toggle in PanelControls.
  const edgesShown = $derived(showEdges ?? false);
  // Layout settle time (seconds) — user-controlled (PanelControls). The FA2
  // worker runs this long, then freezes. Default 12 s; large maps benefit from
  // more (the lever goes to 60 s).
  const settleSec = $derived(settleSeconds ?? 12);

  const viewerLang = $derived(
    displayLanguage === 'viewer' ? pickViewerLabelLanguage(locale()) : undefined
  );
  const netSize = $derived(channels?.netSize ?? 'total_count');
  // Phase 125 / ISSUE 7 — size + colour can bind to different metrics.
  const netMetric = $derived(channels?.netMetric);
  const netColorMetric = $derived(channels?.netColorMetric);
  const sizeMetricReq = $derived(netSize === 'metric' ? netMetric : undefined);
  const colorMetricReq = $derived(
    (channels?.netColor ?? '') === 'metric' ? (netColorMetric ?? netMetric) : undefined
  );
  // Phase 122d.2 — NS overlay: always request per-edge nsSupport and dim
  // NS-supported edges (the former Negative-Space toggle is gone).

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
      topN: AT_SCALE_TOP_N,
      ...(viewerLang ? { viewerLanguage: viewerLang } : {}),
      negativeSpaceOverlay: 'ghost' as const,
      ...(sizeMetricReq ? { nodeMetric: sizeMetricReq } : {}),
      ...(colorMetricReq ? { nodeColorMetric: colorMetricReq } : {})
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver'
    };
  });

  const data = $derived(graphQ.data?.kind === 'success' ? graphQ.data.data : null);
  const refusal = $derived(graphQ.data?.kind === 'refusal' ? graphQ.data : null);
  const isNetworkError = $derived(graphQ.isError || graphQ.data?.kind === 'network-error');

  const resolvedSourceCount = $derived(data ? resolvedSourceCountOf(data) : 0);
  const isMergedScope = $derived(resolvedSourceCount > 1 || sources.length > 1);
  // Kriesel default — colour by detected theme cluster (Louvain community).
  const netColor = $derived(channels?.netColor ?? 'community');
  const metricExtent = $derived.by<MetricExtent | null>(() =>
    data ? computeMetricExtent(data) : null
  );
  // Phase 125 / ISSUE 7 — separate colour-metric extent.
  const metricColorExtent = $derived.by<MetricExtent | null>(() =>
    data ? computeMetricColorExtent(data) : null
  );

  // ── sigma / graphology render ─────────────────────────────────────────────
  let host: HTMLDivElement | undefined = $state();
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let sigmaInstance: any = null;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let fa2: any = null;
  let renderedNodeCount = $state(0);

  function teardown() {
    try {
      fa2?.kill();
    } catch {
      /* layout already stopped */
    }
    try {
      sigmaInstance?.kill();
    } catch {
      /* renderer already disposed */
    }
    fa2 = null;
    sigmaInstance = null;
  }

  let pointer = $state({ x: 0, y: 0 });
  let readout = $state<ReadoutState>(HIDDEN_READOUT);

  interface SelectedEntity {
    text: string;
    label: string;
    coOccursWith: string[];
    wikidataQid?: string | null;
  }
  let selectedEntity = $state<SelectedEntity | null>(null);

  $effect(() => {
    // Hoist EVERY reactive read above the await so they register as effect
    // dependencies (Svelte 5 only tracks reads before the first await). This
    // includes isMergedScope + the size/colour metric requests, which are
    // otherwise used only deep in the async closure / event handlers.
    const d = data;
    const ns = netSize;
    const mExt = metricExtent;
    const mColorExt = metricColorExtent;
    const merged = isMergedScope;
    const sizeM = sizeMetricReq;
    const colorM = colorMetricReq;
    const nsOn = true;
    // Spread drives FA2 repulsion → re-layout on change (NOT edgesShown: that is
    // a pure render toggle handled by the edge reducer + a refresh effect, so
    // hiding lines never reshuffles the map).
    const spr = spread;
    const settle = settleSec;
    const nc = netColor;
    // `sources` is a FRESH array on every parent re-render (PanelHost rebuilds it
    // via `sourcesForUnit`), so reading it TRACKED would re-run this expensive
    // FA2 re-layout on unrelated panel mutations — e.g. collapsing/expanding
    // PanelControls. Read it UNTRACKED: a real source-set change always arrives
    // WITH a `data` change (the scope re-query), which is tracked above.
    const srcNames = untrack(() => sources.map((s) => s.name));
    const colorCtx: NodeColorContext = {
      netColor,
      // ISSUE 7 — colour channel uses the COLOUR metric's extent.
      metricExtent: mColorExt,
      maxPresence: 1,
      sourceColorMap: buildSourceColorMap(srcNames)
    };
    if (!host || !d || d.nodes.length === 0) {
      teardown();
      renderedNodeCount = 0;
      return;
    }
    let cancelled = false;
    let stopTimer: ReturnType<typeof setTimeout> | undefined;
    let convergeTimer: ReturnType<typeof setInterval> | undefined;
    let detachWheel: (() => void) | undefined;
    const container = host;
    (async () => {
      const [{ default: Graph }, { default: Sigma }, { default: FA2Layout }, fa2base] =
        await Promise.all([
          import('graphology'),
          import('sigma'),
          import('graphology-layout-forceatlas2/worker'),
          import('graphology-layout-forceatlas2')
        ]);
      if (cancelled || !container) return;

      // Kriesel effect — detect theme clusters (Louvain) when colouring by
      // community (the default), so each topic region gets its own colour + a
      // labelled head node. Best-effort; empty map → grey nodes.
      const communities = nc === 'community' ? await computeCommunities(d) : null;
      if (cancelled || !container) return;
      teardown();

      const nodes = buildNetworkNodes(d, ns, mExt, communities);
      const edges = buildNetworkEdges(d);
      const heads = communities ? communityHeads(nodes) : null;
      colorCtx.maxPresence = nodes.reduce((m, n) => Math.max(m, n.presenceCount), 1);
      const maxWeight = edges.reduce((m, e) => Math.max(m, e.weight), 1);

      const graph = new Graph({ multi: false, type: 'undirected' });
      const n = nodes.length;
      // RANDOM seed in a box scaled by √n. A symmetric (spiral/ring) seed tends
      // to be *preserved* by FA2 as a ring; random starts let the layout break
      // symmetry and relax into genuine theme clusters.
      const seedR = Math.max(150, 16 * Math.sqrt(n));
      nodes.forEach((node) => {
        graph.addNode(node.id, {
          x: (Math.random() - 0.5) * 2 * seedR,
          y: (Math.random() - 0.5) * 2 * seedR,
          size: nodeRadius(node.sizeNorm, 2, 14),
          color: nodeFillColor(node, colorCtx),
          label: node.displayName,
          // Kriesel effect — always label each theme cluster's representative
          // (largest) node so the topic regions read as named areas.
          forceLabel: heads?.has(node.id) ?? false,
          // carried for the click → article modal + hover readout
          nerLabel: node.label,
          wikidataQid: node.wikidataQid,
          relabeled: node.relabeled,
          totalCount: node.totalCount,
          degree: node.degree,
          community: node.community,
          metricValue: node.metricValue,
          metricColorValue: node.metricColorValue
        });
      });
      for (const e of edges) {
        if (!graph.hasNode(e.source) || !graph.hasNode(e.target)) continue;
        if (graph.hasEdge(e.source, e.target)) continue;
        graph.addEdge(e.source, e.target, {
          size: 0.4 + 2 * (e.weight / maxWeight),
          // Phase 122d.2 — dim edges supported by undated articles (disclosure).
          color:
            nsOn && e.nsSupport > 0
              ? 'rgba(154,143,184,0.25)'
              : edgeStrokeColor(e, merged, colorCtx.sourceColorMap),
          // LAYOUT weight is LOG-DAMPENED. graphology FA2 derives node mass as
          // 1 + Σ(edge weight), and repulsion ∝ mass_i · mass_j — so raw, skewed
          // co-occurrence weights gave hub nodes enormous mass → big empty voids
          // around them ("clusters within clusters"). log() flattens that mass so
          // small nodes sit next to their big hub, while still keeping a weighted
          // signal for attraction. (Edge THICKNESS above still uses raw weight.)
          weight: 1 + Math.log(Math.max(1, e.weight)),
          articleCount: e.articleCount
        });
      }
      renderedNodeCount = graph.order;

      sigmaInstance = new Sigma(graph, container, {
        // Semantic LOD: only label the bigger nodes until the user zooms in,
        // so a thousand-node map is not a wall of text.
        labelRenderedSizeThreshold: 8,
        defaultEdgeColor: 'rgba(180,200,220,0.4)',
        minCameraRatio: 0.05,
        maxCameraRatio: 12,
        // Connections toggle — edges stay in the graph for the FA2 layout but are
        // hidden from rendering when `edgesShown` is off. Reading the reactive
        // value inside the reducer + a refresh effect (below) toggles lines
        // WITHOUT a rebuild/re-layout.
        edgeReducer: (_edge: string, attrs: Record<string, unknown>) =>
          edgesShown ? attrs : { ...attrs, hidden: true }
      });

      // Zoom only with Ctrl/⌘ held (mirrors the SVG cell) so a plain wheel
      // scrolls the page instead of being swallowed by the map. Capture-phase on
      // the container: without the modifier we stop the event before sigma's
      // canvas handler sees it (page scrolls); with it we let sigma zoom and
      // block the browser's ctrl-wheel page-zoom.
      const onWheelCapture = (e: WheelEvent) => {
        if (e.ctrlKey || e.metaKey) {
          e.preventDefault();
          return;
        }
        e.stopPropagation();
      };
      container.addEventListener('wheel', onWheelCapture, { capture: true, passive: false });
      detachWheel = () => container.removeEventListener('wheel', onWheelCapture, { capture: true });

      // Hover readout (cursor-anchored via the container's own pointer position,
      // decoupled from sigma's event coordinate shape for robustness).
      sigmaInstance.on('enterNode', ({ node }: { node: string }) => {
        const a = graph.getNodeAttributes(node);
        readout = {
          visible: true,
          x: pointer.x,
          y: pointer.y,
          title: String(a.label ?? node),
          rows: [
            { label: 'Type', value: String(a.nerLabel ?? '') },
            { label: 'Co-occurrences', value: String(a.totalCount ?? 0) },
            { label: 'Degree', value: String(a.degree ?? 0) },
            ...(sizeM && a.metricValue != null
              ? [{ label: `size · ${sizeM}`, value: Number(a.metricValue).toFixed(3) }]
              : []),
            ...(colorM && colorM !== sizeM && a.metricColorValue != null
              ? [{ label: `colour · ${colorM}`, value: Number(a.metricColorValue).toFixed(3) }]
              : [])
          ]
        };
      });
      sigmaInstance.on('leaveNode', () => {
        readout = HIDDEN_READOUT;
      });
      sigmaInstance.on('clickNode', ({ node }: { node: string }) => {
        if (selectedEntity?.text === node) {
          selectedEntity = null;
          return;
        }
        const a = graph.getNodeAttributes(node);
        selectedEntity = {
          text: node,
          label: String(a.nerLabel ?? ''),
          coOccursWith: graph.neighbors(node).sort(),
          wikidataQid: (a.wikidataQid as string | null) ?? null
        };
      });

      // ForceAtlas2 in a Web Worker (UI stays responsive). LinLog dense-cluster
      // recipe — the strongest lever for tight theme clusters with no hub voids:
      //  • linLogMode: true — LOGARITHMIC attraction: cluster members pull very
      //    close together (dense topic blobs) and small nodes hug their big hub
      //    (fills the voids that mass-weighted repulsion otherwise leaves). The
      //    earlier "ring/line" came from gravity 0.25, not from linLog itself.
      //  • outboundAttractionDistribution: false — hubs attract at FULL strength
      //    so same-theme neighbours sit close to them.
      //  • adjustSizes: false — no collision jitter; the layout settles.
      //  • edgeWeightInfluence: 0.65 — co-occurrence strength tightens themes.
      //  • gravity: 1 — in linLog this compacts clusters without collapsing the
      //    whole graph (log attraction balances it); Spread sets the (small,
      //    linLog-scale) repulsion that holds clusters apart.
      //  • barnesHut from inferSettings (size-aware) for 6k-node performance.
      const inferred = fa2base.inferSettings(graph);
      fa2 = new FA2Layout(graph, {
        settings: {
          barnesHutOptimize: inferred.barnesHutOptimize ?? n > 1000,
          barnesHutTheta: inferred.barnesHutTheta ?? 0.5,
          linLogMode: true,
          outboundAttractionDistribution: false,
          adjustSizes: false,
          edgeWeightInfluence: 0.65,
          // gravity 1.5 — pulls cluster centroids toward the middle so themes sit
          // CLOSER together (linLog keeps each cluster internally dense, so this
          // closes the inter-cluster gaps without re-collapsing into one blob).
          gravity: 1.5,
          scalingRatio: 0.5 + spr / 100,
          // Half the damping of before → bigger steps → linLog reaches its rest
          // point much faster (it converges slower than linear; the auto-stop
          // then fires sooner). Settle lever stays the safety cap.
          slowDown: 1 + Math.log(Math.max(2, n)) / 2
        }
      });

      // Stop the worker + both timers (idempotent).
      const stopLayout = () => {
        if (stopTimer) {
          clearTimeout(stopTimer);
          stopTimer = undefined;
        }
        if (convergeTimer) {
          clearInterval(convergeTimer);
          convergeTimer = undefined;
        }
        try {
          fa2?.stop();
        } catch {
          /* already stopped */
        }
      };

      fa2.start();
      // AUTO-STOP at the rest point: poll mean node movement; once it falls below
      // a small fraction of the layout scale for two consecutive checks, the map
      // has settled — stop immediately (no waiting). The Settle lever is the
      // SAFETY CAP ("run until settled, but at most N seconds").
      const capMs = Math.max(2000, Math.min(120000, settle * 1000));
      stopTimer = setTimeout(stopLayout, capMs);
      const convEps = 0.004 * seedR;
      // Plain object (not Map) — a non-reactive local position cache; the Svelte
      // reactivity lint rule rightly forbids a mutable Map in component scope.
      const prevPos: Record<string, [number, number]> = {};
      let calmStreak = 0;
      convergeTimer = setInterval(() => {
        let sum = 0;
        let count = 0;
        graph.forEachNode((id: string, attrs: { x?: number; y?: number }) => {
          const x = attrs.x ?? 0;
          const y = attrs.y ?? 0;
          const p = prevPos[id];
          if (p) {
            sum += Math.hypot(x - p[0], y - p[1]);
            count++;
          }
          prevPos[id] = [x, y];
        });
        const meanMove = count > 0 ? sum / count : Infinity;
        if (meanMove < convEps) {
          if (++calmStreak >= 2) stopLayout();
        } else {
          calmStreak = 0;
        }
      }, 600);
    })();
    // Effect cleanup — runs on every re-run AND on destroy: cancel the in-flight
    // async build, clear the FA2 stop-timer (no timer-after-destroy retention),
    // and tear down the sigma instance + worker.
    return () => {
      cancelled = true;
      if (stopTimer) clearTimeout(stopTimer);
      if (convergeTimer) clearInterval(convergeTimer);
      detachWheel?.();
      teardown();
    };
  });

  // Connections toggle — re-render sigma (re-runs the edge reducer) when
  // `edgesShown` flips, WITHOUT rebuilding the graph or restarting FA2.
  $effect(() => {
    void edgesShown;
    sigmaInstance?.refresh();
  });

  // ── how-to-read + export (shared contract) ────────────────────────────────
  const linkedNodeCount = $derived(data?.linkedNodeCount ?? 0);
  const labeledNodeCount = $derived(data?.labeledNodeCount ?? 0);
  const totalNodeCount = $derived(data?.nodes.length ?? 0);
  const relabelActive = $derived(displayLanguage === 'viewer' && !!viewerLang);
  const howToReadFacts = $derived(
    buildHowToReadFacts({
      topN: AT_SCALE_TOP_N,
      netSize,
      netColor,
      renderedCount: renderedNodeCount,
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
      topN: AT_SCALE_TOP_N,
      netSize,
      netColor,
      nodeCount: renderedNodeCount,
      edgeCount: data?.edges.length ?? 0,
      howToReadFacts,
      data
    })
  );
  const exportFilenameParts = $derived([
    'cooccurrence-at-scale',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="atscale-cell" aria-labelledby="atscale-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="atscale-title" class="cell-title">
      Entity co-occurrence
      <span class="muted">— large-scale view · <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if data && renderedNodeCount > 0}
      <div class="header-actions">
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      </div>
    {/if}
  </header>

  <p class="atscale-hint" role="note">
    Large-scale WebGL map. <strong>Ctrl/⌘ + scroll to zoom</strong> · drag to pan · click a node for
    its articles. Colour = detected <strong>theme cluster</strong> (the labelled node names each
    region). The layout settles automatically (Settle = the max wait); Top N sets how many
    connections appear, Spread how far they fan out.
    {#if data}
      <span class="muted density-count"
        >· {renderedNodeCount} nodes · {data.edges.length} edges</span
      >
    {/if}
  </p>

  {#if dataLayer === 'silver'}
    <p class="muted">Co-occurrence is a Gold-layer artefact — not available on the Silver layer.</p>
  {:else if graphQ.isPending}
    <p class="muted" aria-busy="true">Loading the large-scale graph…</p>
  {:else if refusal}
    <RefusalSurface {refusal} {ctx} />
  {:else if isNetworkError}
    <p class="muted">Could not load the co-occurrence graph (network error).</p>
  {:else if data && data.nodes.length === 0}
    <p class="muted">No co-occurring entities in the active scope and window.</p>
  {:else}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div
      class="sigma-host"
      bind:this={host}
      onpointermove={(e) => {
        const r = (e.currentTarget as HTMLElement).getBoundingClientRect();
        pointer = { x: e.clientX - r.left, y: e.clientY - r.top };
      }}
    ></div>
    {#if relabelActive}
      <p class="muted coverage-note">
        Labels: {labeledNodeCount} of {totalNodeCount} nodes relabelled to the app language ({linkedNodeCount}
        link to Wikidata); the rest keep their source form.
      </p>
    {/if}
  {/if}

  <CellReadout {readout} />

  {#if sizeMetricReq || colorMetricReq}
    <!-- Metric-channel legend — parity with the SVG cell (ISSUE 8). -->
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
            >No node carries “{sizeMetricReq}” (size) in this scope — channel inactive.</span
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
            >No node carries “{colorMetricReq}” (colour) in this scope — channel inactive.</span
          >
        {/if}
      {/if}
      <span class="metric-legend-note">Grey nodes have no such article (never a coerced 0).</span>
    </div>
  {/if}

  <HowToRead presentation="cooccurrence_network" facts={howToReadFacts} />
</section>

<ArticleListModal
  open={selectedEntity !== null}
  title={selectedEntity ? `Articles mentioning "${selectedEntity.text}"` : 'Articles'}
  {ctx}
  {windowStart}
  {windowEnd}
  onClose={() => (selectedEntity = null)}
  config={{
    mode: 'source-articles',
    sources: sources.map((s) => ({ name: s.name, label: s.emicDesignation ?? s.name })),
    entityMatch: selectedEntity?.text
  }}
/>

{#if selectedEntity}
  <aside class="entity-panel" aria-label="Selected entity">
    <div class="entity-head">
      <span class="entity-name">{selectedEntity.text}</span>
      <span class="entity-label-badge">{selectedEntity.label}</span>
      {#if selectedEntity.wikidataQid}
        <!-- eslint-disable svelte/no-navigation-without-resolve -->
        <a
          class="ext-link"
          href={wikidataHref(selectedEntity.wikidataQid)}
          target="_blank"
          rel="noopener noreferrer">Wikidata ↗</a
        >
        <a
          class="ext-link"
          href={wikipediaHref(selectedEntity.wikidataQid)}
          target="_blank"
          rel="noopener noreferrer">Wikipedia ↗</a
        >
        <!-- eslint-enable svelte/no-navigation-without-resolve -->
      {/if}
      <button class="entity-close" onclick={() => (selectedEntity = null)}>×</button>
    </div>
  </aside>
{/if}

<style>
  .atscale-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    height: 100%;
    min-height: 70vh;
  }
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
  }
  .cell-title {
    margin: 0;
    font-size: var(--font-size-md);
  }
  .muted {
    color: var(--color-fg-muted, #888);
    font-size: var(--font-size-xs);
  }
  .scope-name {
    color: var(--color-fg);
  }
  .atscale-hint {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted, #888);
  }
  .density-count {
    white-space: nowrap;
  }
  /* Metric-channel legend — parity with the SVG cell. */
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
  .sigma-host {
    flex: 1;
    min-height: 60vh;
    width: 100%;
    background: var(--color-surface, #0e1116);
    border: 1px solid var(--color-border, #2a2f37);
    border-radius: var(--radius-sm);
  }
  .coverage-note {
    margin: 0;
  }
  .entity-panel {
    margin-top: var(--space-2);
    padding: var(--space-2) var(--space-3);
    border: 1px solid var(--color-border, #2a2f37);
    border-radius: var(--radius-sm);
    background: var(--color-surface-2, var(--color-surface));
  }
  .entity-head {
    display: flex;
    align-items: center;
    gap: var(--space-2);
  }
  .entity-name {
    font-weight: 600;
  }
  .entity-label-badge {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted, #888);
    border: 1px solid var(--color-border, #2a2f37);
    border-radius: var(--radius-sm);
    padding: 0 var(--space-1);
  }
  .ext-link {
    font-size: var(--font-size-xs);
  }
  .entity-close {
    margin-left: auto;
    background: none;
    border: none;
    color: var(--color-fg-muted, #888);
    cursor: pointer;
    font-size: var(--font-size-md);
  }
</style>
