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
  import { createQuery } from '@tanstack/svelte-query';
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
  import { HIDDEN_READOUT, type ReadoutState } from '$lib/viewmodes/cell-readout';
  import {
    computeMetricExtent,
    computeMetricColorExtent,
    resolvedSourceCount as resolvedSourceCountOf,
    buildNetworkNodes,
    buildNetworkEdges,
    buildSourceColorMap,
    nodeFillColor,
    edgeStrokeColor,
    nodeRadius,
    buildHowToReadFacts,
    buildExportPayload,
    type MetricExtent,
    type NodeColorContext
  } from '$lib/viewmodes/cooccurrence-network-shared';
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
    displayLanguage,
    configOverridden
  }: ViewModeCellProps = $props();

  // The at-scale view honours the Top-N lever (the auto-switch into this renderer
  // happens once Top-N crosses ~500), capped at the BFF ceiling; the user further
  // thins the graph with the minWeight density slider (a visible lever — the real
  // density control). Kept as LOCAL transient state: it is an
  // exploration knob, not panel configuration (not URL-persisted).
  const AT_SCALE_CEILING = 6000;
  // Honour the Top-N lever (continuous with the SVG cell), capped at the ceiling.
  const AT_SCALE_TOP_N = $derived(Math.min(AT_SCALE_CEILING, topN ?? AT_SCALE_CEILING));
  let minWeight = $state(0);

  const viewerLang = $derived(displayLanguage === 'viewer' ? viewerLabelLanguage() : undefined);
  const netSize = $derived(channels?.netSize ?? 'total_count');
  // Phase 125 / ISSUE 7 — size + colour can bind to different metrics.
  const netMetric = $derived(channels?.netMetric);
  const netColorMetric = $derived(channels?.netColorMetric);
  const sizeMetricReq = $derived(netSize === 'metric' ? netMetric : undefined);
  const colorMetricReq = $derived(
    (channels?.netColor ?? '') === 'metric' ? (netColorMetric ?? netMetric) : undefined
  );
  // Phase 122d.2 — NS overlay: request per-edge nsSupport; dim NS-supported edges.
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
      topN: AT_SCALE_TOP_N,
      minWeight,
      ...(viewerLang ? { viewerLanguage: viewerLang } : {}),
      ...(negSpaceOn ? { negativeSpaceOverlay: 'ghost' as const } : {}),
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
  const netColor = $derived(channels?.netColor ?? (isMergedScope ? 'source_overlay' : 'label'));
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
    const nsOn = negSpaceOn;
    const colorCtx: NodeColorContext = {
      netColor,
      // ISSUE 7 — colour channel uses the COLOUR metric's extent.
      metricExtent: mColorExt,
      maxPresence: 1,
      sourceColorMap: buildSourceColorMap(sources.map((s) => s.name))
    };
    if (!host || !d || d.nodes.length === 0) {
      teardown();
      renderedNodeCount = 0;
      return;
    }
    let cancelled = false;
    let stopTimer: ReturnType<typeof setTimeout> | undefined;
    const container = host;
    (async () => {
      const [{ default: Graph }, { default: Sigma }, { default: FA2Layout }] = await Promise.all([
        import('graphology'),
        import('sigma'),
        import('graphology-layout-forceatlas2/worker')
      ]);
      if (cancelled || !container) return;
      teardown();

      const nodes = buildNetworkNodes(d, ns, mExt);
      const edges = buildNetworkEdges(d);
      colorCtx.maxPresence = nodes.reduce((m, n) => Math.max(m, n.presenceCount), 1);
      const maxWeight = edges.reduce((m, e) => Math.max(m, e.weight), 1);

      const graph = new Graph({ multi: false, type: 'undirected' });
      const n = nodes.length;
      nodes.forEach((node, i) => {
        // Circular seed positions — FA2 needs distinct, non-coincident starts.
        const angle = (2 * Math.PI * i) / n;
        graph.addNode(node.id, {
          x: Math.cos(angle) * 100,
          y: Math.sin(angle) * 100,
          size: nodeRadius(node.sizeNorm, 2, 14),
          color: nodeFillColor(node, colorCtx),
          label: node.displayName,
          // carried for the click → article modal + hover readout
          nerLabel: node.label,
          wikidataQid: node.wikidataQid,
          relabeled: node.relabeled,
          totalCount: node.totalCount,
          degree: node.degree,
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
          weight: e.weight,
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
        maxCameraRatio: 12
      });

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

      // ForceAtlas2 layout in a Web Worker — keeps the UI thread responsive.
      // Run bounded, then stop (a settled map; the user pans/zooms, not waits).
      fa2 = new FA2Layout(graph, {
        settings: { gravity: 1, scalingRatio: 10, slowDown: 1 + Math.log(Math.max(2, n)) }
      });
      fa2.start();
      const stopAfterMs = Math.min(6000, 1500 + n * 2);
      stopTimer = setTimeout(() => {
        if (!cancelled) fa2?.stop();
      }, stopAfterMs);
    })();
    // Effect cleanup — runs on every re-run AND on destroy: cancel the in-flight
    // async build, clear the FA2 stop-timer (no timer-after-destroy retention),
    // and tear down the sigma instance + worker.
    return () => {
      cancelled = true;
      if (stopTimer) clearTimeout(stopTimer);
      teardown();
    };
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
    Large-scale WebGL map (Top-N &gt; 500). Up to {AT_SCALE_TOP_N} strongest edges; drag to pan, scroll
    to zoom. Lower Top-N for the interactive small-graph view. Raise <em>min weight</em> to thin a dense
    graph.
  </p>

  <div class="density-row">
    <label class="density-label" for="minweight">Min weight</label>
    <input
      id="minweight"
      type="range"
      min="0"
      max="50"
      step="1"
      bind:value={minWeight}
      aria-label="Minimum co-occurrence weight"
    />
    <span class="density-value">{minWeight}</span>
    {#if data}
      <span class="muted density-count">{renderedNodeCount} nodes · {data.edges.length} edges</span>
    {/if}
  </div>

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
  .density-row {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--font-size-xs);
  }
  .density-row input[type='range'] {
    flex: 0 0 180px;
  }
  .density-count {
    margin-left: auto;
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
