<script lang="ts">
  // Rhizome × Silent-Edit Discourse Shift — Phase 122d.3.
  //
  // The relational reading of silent edits: cross-source temporal
  // coincidences on the same entity. Each cluster is a (time bucket,
  // entity) that ≥2 distinct sources each quietly added or removed in the
  // same window. Rendered as a ranked list — a DISCLOSED coincidence, never
  // a causal/coordination claim (WP-003 §5). The source chips drill into
  // the articles each source edited in that bucket.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    revisionEditClustersQuery,
    type RevisionEditClustersResponseDto,
    type RevisionActivityResolution,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import ArticleListModal from '$lib/components/article/ArticleListModal.svelte';
  import type { PresentationCellProps } from '$lib/presentations';
  import { type ExportPayload, type ExportRow } from '$lib/presentations/cell-export';
  import { composeHowToRead } from '$lib/presentations/how-to-read';
  import { useProbeLabels } from '$lib/presentations/use-probe-labels.svelte';
  import type { CellTitleSpec } from '$lib/presentations/cell-title';
  import CellExport from './CellExport.svelte';
  import CellEmptyState from './CellEmptyState.svelte';
  import CellTitleBar from './CellTitleBar.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import CellLoadingState from '$lib/components/base/CellLoadingState.svelte';
  import { formatDate } from '$lib/localization/format';

  let { ctx, scope, scopeId, windowStart, windowEnd, resolution }: PresentationCellProps = $props();

  const probeLabels = useProbeLabels(() => ctx);

  // Coincidence threshold — the minimum distinct co-editing sources. Held
  // at the BFF default (2) for now; a future PanelControls lever can expose
  // it (the cell already passes it through).
  const MIN_SOURCES = 2;

  const activeResolution = $derived<RevisionActivityResolution>(
    resolution === 'weekly' || resolution === 'monthly' ? resolution : 'daily'
  );

  // Phase 148e — unified cell title. Eyebrow = presentation; no metric subject;
  // scope = resolved probe/source label (this fixes the raw-probe-id leak); the
  // active calendar grain + the ≥N-sources threshold ride the tail as muted
  // qualifiers.
  const titleSpec = $derived<CellTitleSpec>({
    presentation: m.domain_presentation_revision_edit_clusters_label(),
    subject: { kind: 'none' },
    scope: { kind: 'single', label: probeLabels.labelFor(scopeId) },
    qualifiers: [
      { label: activeResolution },
      { label: m.cells_revec_subtitle_sources({ count: MIN_SOURCES }) }
    ],
    idSeed: `rev-ec-title-${scopeId}`
  });

  let drilldown = $state<{ source: string; bucketStart: string; bucketEnd: string } | null>(null);

  const clustersQ = createQuery<
    QueryOutcome<RevisionEditClustersResponseDto>,
    Error,
    QueryOutcome<RevisionEditClustersResponseDto>
  >(() => {
    const o = revisionEditClustersQuery(ctx, {
      scope,
      scopeId,
      start: windowStart,
      end: windowEnd,
      resolution: activeResolution,
      minSources: MIN_SOURCES
    });
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  type Cluster = {
    bucket: Date;
    entity: string;
    sources: string[];
    editCount: number;
    avgTopicShift: number;
  };
  const clusters = $derived<Cluster[]>(
    clustersQ.data?.kind === 'success'
      ? (clustersQ.data.data.clusters ?? []).map((c) => ({
          bucket: new Date(c.bucket),
          entity: c.entity,
          sources: c.sources,
          editCount: c.editCount,
          avgTopicShift: c.avgTopicShift
        }))
      : []
  );

  function bucketDurationMs(res: RevisionActivityResolution): number {
    switch (res) {
      case 'monthly':
        return 30 * 24 * 3_600_000;
      case 'weekly':
        return 7 * 24 * 3_600_000;
      default:
        return 24 * 3_600_000;
    }
  }

  function openSourceDrilldown(source: string, bucket: Date): void {
    const bucketStartMs = bucket.getTime();
    drilldown = {
      source,
      bucketStart: new Date(bucketStartMs).toISOString(),
      bucketEnd: new Date(bucketStartMs + bucketDurationMs(activeResolution)).toISOString()
    };
  }

  function fmtBucket(d: Date): string {
    return formatDate(d.toISOString());
  }

  const exportRows = $derived<ExportRow[]>(
    clusters.map((c) => ({
      bucket: c.bucket.toISOString(),
      entity: c.entity,
      sources: c.sources.join('|'),
      editCount: c.editCount,
      avgTopicShift: c.avgTopicShift
    }))
  );
  const exportPayload = $derived<ExportPayload>({
    meta: {
      viewMode: 'revision_edit_clusters',
      scope,
      scopeId,
      windowStart,
      windowEnd,
      resolution: activeResolution,
      minSources: MIN_SOURCES
    },
    howToRead: composeHowToRead('revision_edit_clusters', {}),
    rows: exportRows,
    columns: ['bucket', 'entity', 'sources', 'editCount', 'avgTopicShift']
  });
  const exportFilenameParts = $derived([
    'edit-clusters',
    scope === 'source' ? scopeId : 'probe',
    activeResolution
  ]);
  let cellEl: HTMLElement | undefined = $state();
  function getNode(): HTMLElement | null {
    return cellEl ?? null;
  }
</script>

<section class="rev-cell" aria-labelledby="rev-ec-title-{scopeId}" bind:this={cellEl}>
  <CellTitleBar spec={titleSpec}>
    {#snippet actions()}
      {#if clusters.length > 0}
        <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
      {/if}
    {/snippet}
  </CellTitleBar>

  {#if clustersQ.isPending}
    <CellLoadingState label={m.cells_revec_loading()} />
  {:else if clustersQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={clustersQ.data} {ctx} />
  {:else if clustersQ.isError || clustersQ.data?.kind === 'network-error'}
    <p class="muted">{m.cells_revec_error()}</p>
  {:else if clusters.length === 0}
    <CellEmptyState />
  {:else}
    <p class="click-hint" aria-hidden="true">
      {m.cells_revec_click_hint({ count: MIN_SOURCES })}
    </p>
    <ul class="cluster-list">
      {#each clusters as c (c.entity + c.bucket.toISOString())}
        <li class="cluster">
          <div class="cluster-head">
            <span class="cluster-entity">{c.entity}</span>
            <time class="cluster-bucket" datetime={c.bucket.toISOString()}
              >{fmtBucket(c.bucket)}</time
            >
          </div>
          <div class="cluster-sources">
            {#each c.sources as src (src)}
              <button
                type="button"
                class="source-chip"
                title={m.cells_revec_chip_title({ source: src })}
                onclick={() => openSourceDrilldown(src, c.bucket)}
              >
                {src}
              </button>
            {/each}
          </div>
          <div class="cluster-meta">
            <span title={m.cells_revec_edits_title()}
              >{m.cells_revec_edits({ count: c.editCount })}</span
            >
            <span class="dot" aria-hidden="true">·</span>
            <span title={m.cells_revec_shift_title()}>
              {m.cells_revec_shift({ value: c.avgTopicShift.toFixed(3) })}
            </span>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>

{#if drilldown}
  <ArticleListModal
    open={drilldown !== null}
    title={m.cells_revec_drilldown_title({ source: drilldown.source })}
    {ctx}
    windowStart={drilldown.bucketStart}
    windowEnd={drilldown.bucketEnd}
    onClose={() => (drilldown = null)}
    config={{
      mode: 'revisions-articles',
      scope: 'source',
      scopeId: drilldown.source
    }}
  />
{/if}

<style>
  .rev-cell {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
  .click-hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    margin: 0;
    font-style: italic;
  }
  .cluster-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .cluster {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
  }
  .cluster-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--space-2);
  }
  .cluster-entity {
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }
  .cluster-bucket {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .cluster-sources {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }
  .source-chip {
    font-family: var(--font-mono);
    font-size: 10px;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    border: 1px solid rgba(154, 143, 184, 0.5);
    color: rgba(154, 143, 184, 0.95);
    background: transparent;
    cursor: pointer;
    transition: all var(--motion-duration-instant) var(--motion-ease-standard);
  }
  .source-chip:hover {
    background: rgba(154, 143, 184, 0.15);
    color: var(--color-fg);
  }
  .cluster-meta {
    display: flex;
    gap: var(--space-2);
    font-size: 10px;
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }
  .cluster-meta .dot {
    color: var(--color-fg-subtle);
  }
</style>
