<script lang="ts">
  // Workbench-side article-list modal — Phase 122d.1.
  //
  // The Dossier inline pattern (ArticlePreviewList embedded under a
  // SourceCard) works well for "reading a source's catalogue". The
  // Workbench use case is different: the user is looking at a cell
  // visualisation, drills into an article-cohort, wants to scrub
  // the cohort, then return to the cell without scrolling the
  // visualisation out of view. The N-stacked-lists pattern in the
  // pre-122d.1 CoOccurrenceNetworkCell scrolled the graph away on
  // entity-click; this modal solves that.
  //
  // Same `ArticleRow` primitive as the inline list — the badges,
  // filter behaviour, and row-click L5 handoff are identical. Two
  // hosts, one row.

  import { untrack } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import {
    sourceArticlesQuery,
    revisionsArticlesQuery,
    type ArticlesPageDto,
    type RevisionsArticlesPageDto,
    type FetchContext,
    type QueryOutcome,
    type ArticleListParams,
    type RevisionsArticlesParams
  } from '$lib/api/queries';
  import { Dialog } from '$lib/components/base';
  import L5EvidenceReader from './L5EvidenceReader.svelte';
  import ArticleRow from './ArticleRow.svelte';

  // Two modes:
  //
  //   mode='source-articles' — drives `GET /sources/{id}/articles`.
  //     Used by the Cooccurrence drilldown (entity-click → article
  //     list filtered by entityMatch).
  //
  //   mode='revisions-articles' — drives `GET /revisions/articles`.
  //     Used by the Phase-122d.0 revision-cell drilldown (bar/
  //     bucket-click → articles with revisions in that source+window).
  //
  // Both modes consume the same row component and the same L5
  // handoff; the difference is only in the BFF endpoint.

  type ModeSourceArticles = {
    mode: 'source-articles';
    sources: ReadonlyArray<{ name: string; label?: string }>;
    /** Optional entity-match filter (used by cooccurrence drilldown). */
    entityMatch?: string | undefined;
  };

  type ModeRevisionsArticles = {
    mode: 'revisions-articles';
    /** `scope=probe`: a probe id. `scope=source`: a source name. */
    scope: 'probe' | 'source';
    scopeId: string;
    hasHeadlineChange?: boolean | undefined;
    minChainLength?: number | undefined;
  };

  interface Props {
    open: boolean;
    title: string;
    ctx: FetchContext;
    windowStart?: string | undefined;
    windowEnd?: string | undefined;
    onClose: () => void;
    config: ModeSourceArticles | ModeRevisionsArticles;
  }

  let { open, title, ctx, windowStart, windowEnd, onClose, config }: Props = $props();

  const PAGE_SIZE = 20;

  let openArticleId = $state<string | null>(null);
  // One pagination state per source-group when in source-articles
  // mode (cooccurrence picks N sources at once); a single cursor for
  // revisions-articles mode (one scoped list).
  let cursor = $state<string | null | undefined>(undefined);
  let allItems = $state<Array<RowItem>>([]);
  let hasMore = $state(false);
  let activeSourceIdx = $state(0);

  // Unified row shape — both endpoints normalise into this.
  type RowItem = {
    articleId: string;
    source: string;
    timestamp: string;
    language?: string | null | undefined;
    wordCount?: number | null | undefined;
    sentimentScore?: number | null | undefined;
    chainLength?: number | null | undefined;
    hasHeadlineChange?: boolean | null | undefined;
    latestRevisionAt?: string | null | undefined;
  };

  // Reset pagination when the active source / config / window changes.
  $effect.pre(() => {
    void open;
    void windowStart;
    void windowEnd;
    void activeSourceIdx;
    if (config.mode === 'source-articles') {
      void config.entityMatch;
    } else {
      void config.scope;
      void config.scopeId;
      void config.hasHeadlineChange;
      void config.minChainLength;
    }
    cursor = undefined;
    allItems = [];
  });

  // --- source-articles mode query ----------------------------------------
  const sourceArticlesParams = $derived.by<ArticleListParams>(() => {
    const p: ArticleListParams = { limit: PAGE_SIZE, includeRevisions: true };
    if (windowStart) p.start = windowStart;
    if (windowEnd) p.end = windowEnd;
    if (config.mode === 'source-articles' && config.entityMatch) {
      p.entityMatch = config.entityMatch;
    }
    if (cursor) p.cursor = cursor;
    return p;
  });

  const sourceArticlesQ = createQuery<
    QueryOutcome<ArticlesPageDto>,
    Error,
    QueryOutcome<ArticlesPageDto>
  >(() => {
    const isSourceMode = config.mode === 'source-articles';
    const activeSourceName =
      isSourceMode && config.sources[activeSourceIdx] ? config.sources[activeSourceIdx]!.name : '';
    const enabled = open && isSourceMode && activeSourceName !== '';
    if (!enabled) {
      return {
        queryKey: ['aer', 'article-list-modal-source', null, null] as const,
        queryFn: () =>
          Promise.resolve({
            kind: 'success',
            data: { items: [], hasMore: false }
          } as QueryOutcome<ArticlesPageDto>),
        staleTime: 0,
        enabled: false
      };
    }
    const o = sourceArticlesQuery(ctx, activeSourceName, sourceArticlesParams);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  // --- revisions-articles mode query -------------------------------------
  const revisionsArticlesParams = $derived.by<RevisionsArticlesParams | null>(() => {
    if (config.mode !== 'revisions-articles') return null;
    if (!windowStart || !windowEnd) return null;
    const p: RevisionsArticlesParams = {
      scope: config.scope,
      scopeId: config.scopeId,
      start: windowStart,
      end: windowEnd,
      limit: PAGE_SIZE
    };
    if (config.hasHeadlineChange) p.hasHeadlineChange = true;
    if (config.minChainLength && config.minChainLength > 1)
      p.minChainLength = config.minChainLength;
    if (cursor) p.cursor = cursor;
    return p;
  });

  const revisionsArticlesQ = createQuery<
    QueryOutcome<RevisionsArticlesPageDto>,
    Error,
    QueryOutcome<RevisionsArticlesPageDto>
  >(() => {
    const enabled =
      open && config.mode === 'revisions-articles' && revisionsArticlesParams !== null;
    if (!enabled || !revisionsArticlesParams) {
      return {
        queryKey: ['aer', 'article-list-modal-revisions', null] as const,
        queryFn: () =>
          Promise.resolve({
            kind: 'success',
            data: { items: [], hasMore: false }
          } as QueryOutcome<RevisionsArticlesPageDto>),
        staleTime: 0,
        enabled: false
      };
    }
    const o = revisionsArticlesQuery(ctx, revisionsArticlesParams);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled: true
    };
  });

  // --- merge incoming pages into allItems --------------------------------
  $effect(() => {
    const out = config.mode === 'source-articles' ? sourceArticlesQ : revisionsArticlesQ;
    if (out.data?.kind === 'success' && out.data.data) {
      const page = out.data.data;
      const items = (page.items ?? []).map(normaliseRow);
      if (cursor === undefined || cursor === null) {
        allItems = items;
      } else {
        const existing = untrack(() => allItems);
        const seen = new Set(existing.map((a) => a.articleId));
        const fresh = items.filter((a) => !seen.has(a.articleId));
        allItems = [...existing, ...fresh];
      }
      hasMore = page.hasMore ?? false;
    }
  });

  function normaliseRow(
    item:
      | NonNullable<ArticlesPageDto>['items'][number]
      | NonNullable<RevisionsArticlesPageDto>['items'][number]
  ): RowItem {
    return {
      articleId: item.articleId,
      source: item.source,
      timestamp: item.timestamp,
      language: item.language,
      wordCount: item.wordCount,
      sentimentScore: 'sentimentScore' in item ? item.sentimentScore : null,
      chainLength: item.chainLength,
      hasHeadlineChange: item.hasHeadlineChange,
      latestRevisionAt:
        'latestRevisionAt' in item && item.latestRevisionAt ? item.latestRevisionAt : null
    };
  }

  const activeQuery = $derived(
    config.mode === 'source-articles' ? sourceArticlesQ : revisionsArticlesQ
  );

  const activeNextCursor = $derived(
    activeQuery.data?.kind === 'success' && activeQuery.data.data
      ? activeQuery.data.data.nextCursor
      : undefined
  );

  function loadMore(): void {
    if (activeNextCursor) cursor = activeNextCursor;
  }

  function openArticle(articleId: string): void {
    openArticleId = articleId;
  }

  function onDialogClose(): void {
    openArticleId = null;
    onClose();
  }

  function selectSource(idx: number): void {
    activeSourceIdx = idx;
  }
</script>

<Dialog {open} {title} onClose={onDialogClose}>
  {#if config.mode === 'source-articles' && config.sources.length > 1}
    <!-- Source tabs for multi-source contexts (cooccurrence). -->
    <div class="source-tabs" role="tablist" aria-label="Source tabs">
      {#each config.sources as src, idx (src.name)}
        <button
          type="button"
          role="tab"
          aria-selected={idx === activeSourceIdx}
          class="source-tab"
          class:active={idx === activeSourceIdx}
          onclick={() => selectSource(idx)}
        >
          {src.label ?? src.name}
        </button>
      {/each}
    </div>
  {/if}

  {#if activeQuery.isPending && allItems.length === 0}
    <p class="state-msg" aria-busy="true">Loading articles…</p>
  {:else if activeQuery.isError || activeQuery.data?.kind === 'network-error'}
    <p class="state-msg error">Failed to load articles.</p>
  {:else if allItems.length === 0}
    <p class="state-msg muted">No articles match the current filter.</p>
  {:else}
    <div class="table-wrap" role="region" aria-label="Article list">
      <table class="article-table">
        <thead>
          <tr>
            <th scope="col">Published</th>
            {#if config.mode === 'revisions-articles'}
              <th scope="col">Source</th>
            {/if}
            <th scope="col">Lang</th>
            <th scope="col" class="right">Words</th>
            <th scope="col" class="right">Sentiment</th>
            <th scope="col"><span class="sr-only">Revision badges</span></th>
            <th scope="col"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each allItems as item (item.articleId)}
            <ArticleRow
              {item}
              onOpen={openArticle}
              showSourceCol={config.mode === 'revisions-articles'}
            />
          {/each}
        </tbody>
      </table>
    </div>

    {#if hasMore}
      <button
        type="button"
        class="load-more-btn"
        aria-label="Load more articles"
        disabled={activeQuery.isFetching}
        onclick={loadMore}
      >
        {activeQuery.isFetching ? 'Loading…' : 'Load more'}
      </button>
    {:else}
      <p class="end-note">All {allItems.length} article{allItems.length !== 1 ? 's' : ''} shown.</p>
    {/if}
  {/if}
</Dialog>

<!-- L5 Evidence Reader stacked over the modal. -->
<L5EvidenceReader
  open={openArticleId !== null}
  articleId={openArticleId}
  {ctx}
  onClose={() => (openArticleId = null)}
/>

<style>
  .source-tabs {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
    margin-bottom: var(--space-3);
    padding-bottom: var(--space-2);
    border-bottom: 1px solid var(--color-border);
  }

  .source-tab {
    padding: 3px var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
    cursor: pointer;
  }

  .source-tab.active {
    color: var(--color-fg);
    background: var(--color-surface);
    border-color: var(--color-accent-muted);
  }

  .source-tab:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .state-msg {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
    padding: var(--space-3) 0;
  }

  .state-msg.error {
    color: var(--color-status-expired);
  }

  .table-wrap {
    overflow-x: auto;
    max-height: 60vh;
    overflow-y: auto;
  }

  .article-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-xs);
  }

  .article-table th {
    position: sticky;
    top: 0;
    padding: var(--space-2) var(--space-3);
    text-align: left;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
    white-space: nowrap;
    font-weight: var(--font-weight-medium);
    z-index: 1;
  }

  .article-table th.right {
    text-align: right;
  }

  .load-more-btn {
    align-self: flex-start;
    margin-top: var(--space-3);
    padding: var(--space-2) var(--space-4);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .load-more-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .load-more-btn:not(:disabled):hover {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .end-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    margin: var(--space-3) 0 0;
    font-family: var(--font-mono);
  }

  .muted {
    color: var(--color-fg-muted);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }
</style>
