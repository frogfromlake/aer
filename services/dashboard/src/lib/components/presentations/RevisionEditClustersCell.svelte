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
  import CellExport from './CellExport.svelte';
  import HowToRead from './HowToRead.svelte';

  let { ctx, scope, scopeId, windowStart, windowEnd, resolution }: PresentationCellProps = $props();

  // Coincidence threshold — the minimum distinct co-editing sources. Held
  // at the BFF default (2) for now; a future PanelControls lever can expose
  // it (the cell already passes it through).
  const MIN_SOURCES = 2;

  const activeResolution = $derived<RevisionActivityResolution>(
    resolution === 'weekly' || resolution === 'monthly' ? resolution : 'daily'
  );

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
    return d.toLocaleDateString('en-CA');
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
  <header class="cell-header">
    <h3 id="rev-ec-title-{scopeId}" class="cell-title">
      <span>Edit clusters</span>
      <span class="muted">
        — <strong class="scope-name">{scopeId}</strong> · <code>{activeResolution}</code> · ≥{MIN_SOURCES}
        sources
      </span>
    </h3>
    {#if clusters.length > 0}
      <CellExport {getNode} payload={exportPayload} filenameParts={exportFilenameParts} />
    {/if}
  </header>

  {#if clustersQ.isPending}
    <p class="muted" aria-busy="true">Loading coordinated edits…</p>
  {:else if clustersQ.data?.kind === 'refusal'}
    <RefusalSurface refusal={clustersQ.data} {ctx} />
  {:else if clustersQ.isError || clustersQ.data?.kind === 'network-error'}
    <p class="muted">Could not load coordinated edits.</p>
  {:else if clusters.length === 0}
    <p class="muted">
      No entity was silently edited by ≥{MIN_SOURCES} sources in the same bucket. Coincidence is the exception,
      not the rule — an empty list is the ordinary case.
    </p>
  {:else}
    <p class="click-hint" aria-hidden="true">
      Each row is an entity ≥{MIN_SOURCES} sources quietly edited in the same bucket — a disclosed coincidence,
      not a causal claim. Click a source to see its edited articles.
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
                title={`Articles ${src} edited in this bucket`}
                onclick={() => openSourceDrilldown(src, c.bucket)}
              >
                {src}
              </button>
            {/each}
          </div>
          <div class="cluster-meta">
            <span title="Total edits touching this entity in the bucket">{c.editCount} edits</span>
            <span class="dot" aria-hidden="true">·</span>
            <span title="Mean semantic shift (E5 cosine distance) of the cluster's edits">
              shift {c.avgTopicShift.toFixed(3)}
            </span>
          </div>
        </li>
      {/each}
    </ul>
    <HowToRead presentation="revision_edit_clusters" facts={{}} />
  {/if}
</section>

{#if drilldown}
  <ArticleListModal
    open={drilldown !== null}
    title={`Articles edited — ${drilldown.source}`}
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
  .cell-title code {
    font-family: var(--font-mono);
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
