<script lang="ts">
  // Phase 125 — Sankey / alluvial (Rhizome). Article flows across an ordered
  // chain of categorical metadata fields (`Panel.fieldChain`). Backed by
  // `GET /metadata/sankey`. d3-sankey is lazy-imported (the only new dependency
  // this phase adds — it ships only when this cell is selected, Brief §7).
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import { sankeyQuery, type SankeyDto, type QueryOutcome } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportRow, type ExportPayload } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import HowToRead from './HowToRead.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { fieldLabel } from '$lib/state/labels.svelte';

  let {
    ctx,
    scope,
    scopeId,
    windowStart,
    windowEnd,
    dataLayer = 'gold',
    fieldChain,
    configOverridden
  }: PresentationCellProps = $props();

  // fieldChain is the ORDERED categorical field chain for the alluvial.
  const fields = $derived<string[]>([...(fieldChain ?? [])]);
  const enoughFields = $derived(fields.length >= 2);

  const skQ = createQuery<QueryOutcome<SankeyDto>, Error, QueryOutcome<SankeyDto>>(() => {
    const o = sankeyQuery(ctx, fields, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      // Phase 125 (ISSUE 6) — fewer, thicker bands read far better than 50
      // hair-thin ones; the long tail is still reachable via hover + export.
      topN: 30
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: dataLayer !== 'silver' && enoughFields
    };
  });

  const data = $derived<SankeyDto | null>(skQ.data?.kind === 'success' ? skQ.data.data : null);
  const refusalData = $derived(skQ.data?.kind === 'refusal' ? skQ.data : null);
  const isNetworkError = $derived(skQ.isError || skQ.data?.kind === 'network-error');
  const isEmpty = $derived(!!data && data.links.length === 0);

  // Layer palette (viridis-ish, distinct per column).
  const LAYER_COLORS = ['#5283b8', '#5ba3a0', '#9a8fb8', '#c8a85a', '#b8728f', '#7ba35b'];

  let host: HTMLDivElement | undefined = $state();
  let renderToken = 0;

  const NS = 'http://www.w3.org/2000/svg';

  $effect(() => {
    const d = data;
    if (!host || !d || d.links.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const d3sankey = await import('d3-sankey');
      if (!host || token !== renderToken) return;
      const width = host.clientWidth || 720;
      // Phase 125 (ISSUE 6) — more vertical room per node so more bands clear
      // the label threshold without crowding.
      const height = Math.max(260, Math.min(680, d.nodes.length * 22 + 80));

      // d3-sankey reserves `value` on a node for the computed flow total, so the
      // category value is carried as `label` in the d3 graph.
      const layout = d3sankey
        .sankey<{ id: string; field: string; label: string; layer: number }, { value: number }>()
        .nodeId((n) => n.id)
        .nodeWidth(12)
        .nodePadding(10)
        .extent([
          [4, 8],
          [width - 4, height - 8]
        ]);

      const graph = layout({
        nodes: d.nodes.map((n) => ({ id: n.id, field: n.field, label: n.value, layer: n.layer })),
        links: d.links.map((l) => ({ source: l.source, target: l.target, value: l.value }))
      });
      const linkPath = d3sankey.sankeyLinkHorizontal();

      const svg = document.createElementNS(NS, 'svg');
      svg.setAttribute('width', String(width));
      svg.setAttribute('height', String(height));
      svg.setAttribute('viewBox', `0 0 ${width} ${height}`);

      // Links.
      const linkG = document.createElementNS(NS, 'g');
      linkG.setAttribute('fill', 'none');
      for (const link of graph.links) {
        const p = document.createElementNS(NS, 'path');
        p.setAttribute('d', linkPath(link) ?? '');
        const srcLayer = (link.source as { layer: number }).layer ?? 0;
        p.setAttribute('stroke', LAYER_COLORS[srcLayer % LAYER_COLORS.length]!);
        p.setAttribute('stroke-width', String(Math.max(1, link.width ?? 1)));
        p.setAttribute('stroke-opacity', '0.4');
        const title = document.createElementNS(NS, 'title');
        const sv = (link.source as { label: string }).label;
        const tv = (link.target as { label: string }).label;
        title.textContent = m.cells_sankey_link_tooltip({
          source: sv,
          target: tv,
          count: link.value
        });
        p.appendChild(title);
        linkG.appendChild(p);
      }
      svg.appendChild(linkG);

      // Nodes + labels.
      const nodeG = document.createElementNS(NS, 'g');
      for (const node of graph.nodes) {
        const x0 = node.x0 ?? 0;
        const x1 = node.x1 ?? 0;
        const y0 = node.y0 ?? 0;
        const y1 = node.y1 ?? 0;
        const rect = document.createElementNS(NS, 'rect');
        rect.setAttribute('x', String(x0));
        rect.setAttribute('y', String(y0));
        rect.setAttribute('width', String(Math.max(1, x1 - x0)));
        rect.setAttribute('height', String(Math.max(1, y1 - y0)));
        rect.setAttribute('fill', LAYER_COLORS[(node.layer ?? 0) % LAYER_COLORS.length]!);
        const title = document.createElementNS(NS, 'title');
        title.textContent = m.cells_sankey_node_tooltip({
          field: fieldLabel(node.field),
          value: node.label
        });
        rect.appendChild(title);
        nodeG.appendChild(rect);

        // Label only when the band is tall enough to avoid clutter; the rest
        // are reachable on hover (the rect carries a <title>). (Phase 125 ISSUE 6)
        if (y1 - y0 >= 7) {
          const text = document.createElementNS(NS, 'text');
          const leftHalf = x0 < width / 2;
          text.setAttribute('x', String(leftHalf ? x1 + 4 : x0 - 4));
          text.setAttribute('y', String((y0 + y1) / 2));
          text.setAttribute('dy', '0.35em');
          text.setAttribute('text-anchor', leftHalf ? 'start' : 'end');
          text.setAttribute('class', 'sankey-label');
          text.textContent = node.label;
          nodeG.appendChild(text);
        }
      }
      svg.appendChild(nodeG);

      // eslint-disable-next-line svelte/no-dom-manipulating
      host.replaceChildren(svg);
    })();
  });

  onDestroy(() => {
    // eslint-disable-next-line svelte/no-dom-manipulating
    if (host) host.replaceChildren();
  });

  const howToReadFacts = $derived({ configOverridden });
  const exportRows = $derived<ExportRow[]>(
    (data?.links ?? []).map((l) => ({ source: l.source, target: l.target, articles: l.value }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'sankey',
      fields: fields.join(' → '),
      scope,
      scopeId,
      windowStart,
      windowEnd
    },
    summary: data ? { nodes: data.nodes.length, links: data.links.length } : undefined,
    howToRead: composeHowToRead('sankey', howToReadFacts),
    rows: exportRows,
    columns: ['source', 'target', 'articles']
  });
  const exportFilenameParts = $derived(['sankey', scope === 'source' ? scopeId : 'probe']);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="sankey-cell" aria-labelledby="sankey-title" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="sankey-title" class="cell-title">
      {m.cells_sankey_title()}
      <span class="muted">— <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if data && data.links.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if dataLayer === 'silver'}
    <p class="muted">{m.cells_sankey_silver()}</p>
  {:else if !enoughFields}
    <p class="muted">{m.cells_sankey_need_fields()}</p>
  {:else if skQ.isPending}
    <p class="muted" aria-busy="true">{m.cells_sankey_loading()}</p>
  {:else if refusalData}
    <RefusalSurface refusal={refusalData} {ctx} />
  {:else if isNetworkError}
    <p class="muted">{m.cells_sankey_error()}</p>
  {:else if isEmpty}
    <CellEmptyState />
  {:else if data}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label={m.cells_sankey_plot_aria({ fields: fields.join(', ') })}
    ></div>
    <HowToRead presentation="sankey" facts={howToReadFacts} />
  {/if}
</section>

<style>
  .sankey-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .cell-header {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-3);
    flex-wrap: wrap;
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
  .plot-host {
    width: 100%;
    min-height: 220px;
  }
  .plot-host :global(.sankey-label) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 10px;
  }
  .plot-host :global(svg) {
    background: transparent;
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
