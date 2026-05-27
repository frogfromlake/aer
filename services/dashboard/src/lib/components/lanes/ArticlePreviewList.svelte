<script lang="ts">
  // Paginated article list for a source (Phase 106).
  // Feeds from /api/v1/sources/{id}/articles with cursor-based pagination.
  // Each row opens L5EvidenceReader on click.
  import { untrack } from 'svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import {
    sourceArticlesQuery,
    type ArticlesPageDto,
    type FetchContext,
    type QueryOutcome,
    type ArticleListParams
  } from '$lib/api/queries';
  import L5EvidenceReader from './L5EvidenceReader.svelte';
  import ArticleRow from './ArticleRow.svelte';

  interface Props {
    sourceId: string;
    ctx: FetchContext;
    // Phase 131a — explicit `| undefined` so callers under
    // `exactOptionalPropertyTypes: true` can pass `windowStart: undefined`
    // for the "whole dataset, no filter" mode.
    windowStart?: string | undefined;
    windowEnd?: string | undefined;
    entityMatch?: string | undefined;
  }

  let { sourceId, ctx, windowStart, windowEnd, entityMatch }: Props = $props();

  const PAGE_SIZE = 20;

  let cursor = $state<string | null | undefined>(undefined);
  let allItems = $state<NonNullable<ArticlesPageDto>['items']>([]);
  let hasMore = $state(false);

  // Selected article for L5
  let openArticleId = $state<string | null>(null);

  // Filters (local state, resets pagination on change)
  let filterLang = $state('');
  let filterSentiment = $state<'' | 'negative' | 'neutral' | 'positive'>('');

  // Phase 131a — reset pagination whenever a filter that changes the
  // result set switches. Without this, the parent (e.g. clicking a
  // different entity on the co-occurrence map switching `entityMatch`)
  // leaves the stale `cursor` + `allItems` from the previous selection
  // in place; the next query result is then APPENDED to a list of
  // articles from a different filter, producing duplicate keys
  // (`each_key_duplicate` Svelte crash) and a broken UI.
  $effect.pre(() => {
    // Track the filter identity. Cursor itself is excluded — cursor
    // movement is the legitimate "Load more" path and must NOT reset.
    void entityMatch;
    void windowStart;
    void windowEnd;
    void filterLang;
    void filterSentiment;
    cursor = undefined;
    allItems = [];
  });

  let queryParams = $derived.by<ArticleListParams>(() => {
    const p: ArticleListParams = { limit: PAGE_SIZE };
    if (windowStart) p.start = windowStart;
    if (windowEnd) p.end = windowEnd;
    if (filterLang) p.language = filterLang;
    if (filterSentiment) p.sentimentBand = filterSentiment as 'negative' | 'neutral' | 'positive';
    if (entityMatch) p.entityMatch = entityMatch;
    if (cursor) p.cursor = cursor;
    // Phase 122d.1 — opt in to the per-row revision fields so the
    // ArticleRow renders the chainLength + headlineChanged badges
    // when applicable. Server-side cost is a thin LEFT JOIN against
    // aer_gold.article_revisions; rows with no revisions simply get
    // null fields and the badges hide.
    p.includeRevisions = true;
    return p;
  });

  const articlesQ = createQuery<
    QueryOutcome<ArticlesPageDto>,
    Error,
    QueryOutcome<ArticlesPageDto>
  >(() => {
    const o = sourceArticlesQuery(ctx, sourceId, queryParams);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  $effect(() => {
    if (articlesQ.data?.kind === 'success') {
      const page = articlesQ.data.data;
      if (cursor === undefined || cursor === null) {
        // First page or reset: replace
        allItems = [...page.items];
      } else {
        // Append next page, deduplicating by articleId as a defensive
        // net against a stray double-fire (svelte-query cache replay,
        // duplicate $effect tick, redelivered NATS row that escaped
        // RMT FINAL). The reset $effect.pre above is the primary
        // guard; this is belt-and-braces.
        //
        // Phase 131a — read `allItems` via `untrack` so this $effect
        // does NOT register itself as a reader of `allItems`. Without
        // that, the read-then-write pattern below registers `allItems`
        // as both a dependency and a write target of the same effect,
        // which Svelte 5 detects as a self-loop and aborts with
        // `effect_update_depth_exceeded`. The reset $effect.pre +
        // svelte-query's cache invalidation are the legitimate
        // triggers; the $effect itself must not re-fire on its own
        // writes.
        const existing = untrack(() => allItems);
        const seen = new Set(existing.map((a) => a.articleId));
        const fresh = page.items.filter((a) => !seen.has(a.articleId));
        allItems = [...existing, ...fresh];
      }
      hasMore = page.hasMore;
    }
  });

  function loadMore() {
    if (articlesQ.data?.kind === 'success' && articlesQ.data.data.nextCursor) {
      cursor = articlesQ.data.data.nextCursor;
    }
  }

  function resetFilters() {
    cursor = undefined;
    allItems = [];
    filterLang = '';
    filterSentiment = '';
  }

  function onFilterChange() {
    cursor = undefined;
    allItems = [];
  }

  function openArticle(articleId: string): void {
    openArticleId = articleId;
  }
</script>

<div class="article-list">
  <!-- Filters -->
  <div class="filters" role="search" aria-label="Filter articles">
    <label class="filter-label">
      <span class="label-xs">Language</span>
      <input
        type="text"
        class="filter-input"
        placeholder="e.g. de"
        bind:value={filterLang}
        oninput={onFilterChange}
        aria-label="Filter by language code"
        maxlength="5"
      />
    </label>
    <label class="filter-label">
      <span class="label-xs">Sentiment</span>
      <select
        class="filter-select"
        bind:value={filterSentiment}
        onchange={onFilterChange}
        aria-label="Filter by sentiment band"
      >
        <option value="">All</option>
        <option value="positive">Positive</option>
        <option value="neutral">Neutral</option>
        <option value="negative">Negative</option>
      </select>
    </label>
    {#if filterLang || filterSentiment}
      <button type="button" class="reset-btn" onclick={resetFilters}>Clear filters</button>
    {/if}
  </div>

  {#if articlesQ.isPending && allItems.length === 0}
    <p class="state-msg" aria-busy="true">Loading articles…</p>
  {:else if articlesQ.isError}
    <p class="state-msg error">Failed to load articles.</p>
  {:else if allItems.length === 0}
    <p class="state-msg muted">No articles found for the current filters.</p>
  {:else}
    <div class="table-wrap" role="region" aria-label="Article list">
      <table class="article-table">
        <thead>
          <tr>
            <th scope="col">Published</th>
            <th scope="col">Lang</th>
            <th scope="col" class="right">Words</th>
            <th scope="col" class="right">Sentiment</th>
            <th scope="col"><span class="sr-only">Revision badges</span></th>
            <th scope="col"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each allItems as item (item.articleId)}
            <ArticleRow {item} onOpen={openArticle} showSourceCol={false} />
          {/each}
        </tbody>
      </table>
    </div>

    {#if hasMore}
      <button
        type="button"
        class="load-more-btn"
        aria-label="Load more articles"
        disabled={articlesQ.isFetching}
        onclick={loadMore}
      >
        {articlesQ.isFetching ? 'Loading…' : 'Load more'}
      </button>
    {:else}
      <p class="end-note">All {allItems.length} article{allItems.length !== 1 ? 's' : ''} shown.</p>
    {/if}
  {/if}
</div>

<!-- L5 Evidence Reader -->
<L5EvidenceReader
  open={openArticleId !== null}
  articleId={openArticleId}
  {ctx}
  onClose={() => (openArticleId = null)}
/>

<style>
  .article-list {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .filters {
    display: flex;
    align-items: flex-end;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .filter-label {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .label-xs {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .filter-input,
  .filter-select {
    padding: 3px var(--space-2);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    font-family: var(--font-mono);
  }

  .filter-input:focus-visible,
  .filter-select:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
    border-color: var(--color-accent);
  }

  .reset-btn {
    padding: 3px var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .reset-btn:hover {
    color: var(--color-fg);
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
  }

  .article-table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-xs);
  }

  .article-table th {
    padding: var(--space-2) var(--space-3);
    text-align: left;
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    border-bottom: 1px solid var(--color-border);
    white-space: nowrap;
    font-weight: var(--font-weight-medium);
  }

  .article-table th.right {
    text-align: right;
  }

  /* Row-level styles (.article-row, .ts-cell, .lang-cell, .num-cell,
   * .sentiment-*, .view-btn) live in `ArticleRow.svelte` — the row is
   * its own component (Phase 122d.1 refactor) so the badges + the
   * row chrome stay co-located with the row markup. */

  .load-more-btn {
    align-self: flex-start;
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
    margin: 0;
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

  /* Reduced-motion preference for row-hover transitions lives in
   * ArticleRow.svelte alongside the row chrome. */
</style>
