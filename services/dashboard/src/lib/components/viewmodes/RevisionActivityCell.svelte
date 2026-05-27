<script lang="ts">
  // Aleph × Silent-Edit Observability — Phase 122d.0 (ADR-032).
  //
  // Synchronic per-source bar chart over `aer_gold.article_revisions`.
  // The Aleph cell collapses the active window to one bucket and asks
  // "which source edits most right now"; the diachronic counterpart is
  // `RevisionTimelineCell` (Episteme). One BFF endpoint feeds both.
  import { createQuery } from '@tanstack/svelte-query';
  import { onDestroy } from 'svelte';
  import {
    revisionActivityQuery,
    type RevisionActivityResponseDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/lanes/ArticleListModal.svelte';
  import type { ViewModeCellProps } from '$lib/viewmodes';
  import { type ExportPayload, type ExportRow } from '$lib/viewmodes/cell-export';
  import { composeHowToRead } from '$lib/viewmodes/how-to-read';
  import CellExport from './CellExport.svelte';
  import HowToRead from './HowToRead.svelte';

  let { ctx, scope, scopeId, windowStart, windowEnd }: ViewModeCellProps = $props();

  // Phase 122d.1 — drill-down: click a source bar to open the article
  // list filtered to that source's revisions in the active window.
  let drilldownSource = $state<string | null>(null);

  const revisionQ = createQuery<
    QueryOutcome<RevisionActivityResponseDto>,
    Error,
    QueryOutcome<RevisionActivityResponseDto>
  >(() => {
    const o = revisionActivityQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      resolution: 'snapshot'
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  type Entry = { source: string; revisions: number; articlesAffected: number };
  const entries = $derived<Entry[]>(
    revisionQ.data?.kind === 'success'
      ? (revisionQ.data.data.entries ?? []).map((e) => ({
          source: e.source,
          revisions: e.revisions,
          articlesAffected: e.articlesAffected
        }))
      : []
  );

  let host: HTMLDivElement | undefined = $state();
  let plotEl: HTMLElement | null = null;
  let renderToken = 0;

  $effect(() => {
    const rows = entries;
    if (!host || rows.length === 0) return;
    const token = ++renderToken;
    (async () => {
      const Plot = await import('@observablehq/plot');
      if (!host || token !== renderToken) return;
      const next = Plot.plot({
        width: host.clientWidth,
        height: Math.max(180, rows.length * 32 + 60),
        marginLeft: 140,
        marginBottom: 36,
        x: { label: 'revisions', grid: true },
        y: { label: null, domain: rows.map((r) => r.source) },
        marks: [
          Plot.barX(rows, {
            x: 'revisions',
            y: 'source',
            fill: 'rgba(154, 143, 184, 0.55)',
            stroke: 'rgba(154, 143, 184, 0.95)',
            // Phase 122d.1 — bar click opens the article drill-down
            // modal filtered to that source's revisions in the active
            // window.
            channels: { source: 'source' },
            tip: false
          }),
          Plot.ruleX([0])
        ]
      });
      if (plotEl) plotEl.remove();
      // eslint-disable-next-line svelte/no-dom-manipulating
      host.appendChild(next as unknown as HTMLElement);
      plotEl = next as unknown as HTMLElement;
      // Wire bar clicks to the drilldown — Observable Plot does not
      // emit a typed click; attach a delegated listener on the svg.
      const svg = plotEl?.querySelector('svg');
      if (svg) {
        svg.addEventListener('click', (ev) => {
          const target = ev.target as Element | null;
          const rect = target?.closest('rect');
          if (!rect) return;
          // Plot annotates marks with a `__data__` getter we can read
          // via the d3 datum interface — fall back to bar index by
          // y-coordinate if the datum isn't accessible.
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const datum = (rect as any).__data__ as number | undefined;
          if (typeof datum === 'number' && rows[datum]) {
            drilldownSource = rows[datum].source;
          }
        });
        // Cursor hint that the bars are interactive. SVGRectElement
        // shares the `style` setter with HTMLElement via SVGElement,
        // so a typed widening is sufficient here.
        for (const rect of svg.querySelectorAll('rect')) {
          (rect as SVGRectElement).style.cursor = 'pointer';
        }
      }
    })();
  });

  onDestroy(() => {
    if (plotEl) plotEl.remove();
    plotEl = null;
  });

  const exportRows = $derived<ExportRow[]>(
    entries.map((e) => ({
      source: e.source,
      revisions: e.revisions,
      articles_affected: e.articlesAffected
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'revision_activity',
      scope,
      scopeId,
      windowStart,
      windowEnd
    },
    howToRead: composeHowToRead('revision_activity', {}),
    rows: exportRows,
    columns: ['source', 'revisions', 'articles_affected']
  });
  const exportFilenameParts = $derived([
    'revision-activity',
    scope === 'source' ? scopeId : 'probe'
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="rev-cell" aria-labelledby="rev-title-{scopeId}" bind:this={cellEl}>
  <header class="cell-header">
    <h3 id="rev-title-{scopeId}" class="cell-title">
      <span>Revision activity</span>
      <span class="muted">— <strong class="scope-name">{scopeId}</strong></span>
    </h3>
    {#if entries.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if revisionQ.isPending}
    <p class="muted" aria-busy="true">Loading revision activity…</p>
  {:else if revisionQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={revisionQ.data} {ctx} />
  {:else if revisionQ.isError || revisionQ.data?.kind === 'network-error'}
    <p class="muted">Could not load revision activity.</p>
  {:else if entries.length === 0}
    <p class="muted">
      No silent-edit activity observed in this window. Either Wayback CDX has no snapshots for these
      sources yet, or the publishers have not bumped their sitemap-lastmod inside the window.
    </p>
  {:else}
    <div
      class="plot-host"
      bind:this={host}
      role="img"
      aria-label="Revision counts per source. Click a bar to view the articles."
    ></div>
    <HowToRead presentation="revision_activity" facts={{}} />
  {/if}
</section>

<!-- Phase 122d.1 drill-down — opens articles for the clicked source's
     revisions in the active window. -->
{#if drilldownSource}
  <ArticleListModal
    open={drilldownSource !== null}
    title={`Articles edited — ${drilldownSource}`}
    {ctx}
    {windowStart}
    {windowEnd}
    onClose={() => (drilldownSource = null)}
    config={{
      mode: 'revisions-articles',
      scope: 'source',
      scopeId: drilldownSource
    }}
  />
{/if}

<style>
  .rev-cell {
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
  .plot-host {
    width: 100%;
    min-height: 200px;
  }
  .plot-host :global(text) {
    fill: var(--color-fg-muted);
    font-family: var(--font-mono);
    font-size: 11px;
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
