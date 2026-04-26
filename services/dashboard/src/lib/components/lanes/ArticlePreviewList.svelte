<script lang="ts">
  // Paginated article list for a source (Phase 106).
  // Feeds from /api/v1/sources/{id}/articles with cursor-based pagination.
  // Each row opens L5EvidenceReader on click.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    sourceArticlesQuery,
    type ArticlesPageDto,
    type FetchContext,
    type QueryOutcome,
    type ArticleListParams
  } from '$lib/api/queries';
  import L5EvidenceReader from './L5EvidenceReader.svelte';

  interface Props {
    sourceId: string;
    ctx: FetchContext;
    windowStart?: string;
    windowEnd?: string;
  }

  let { sourceId, ctx, windowStart, windowEnd }: Props = $props();

  const PAGE_SIZE = 20;

  let cursor = $state<string | null | undefined>(undefined);
  let allItems = $state<NonNullable<ArticlesPageDto>['items']>([]);
  let hasMore = $state(false);

  // Selected article for L5
  let openArticleId = $state<string | null>(null);

  // Filters (local state, resets pagination on change)
  let filterLang = $state('');
  let filterSentiment = $state<'' | 'negative' | 'neutral' | 'positive'>('');

  let queryParams = $derived.by<ArticleListParams>(() => {
    const p: ArticleListParams = { limit: PAGE_SIZE };
    if (windowStart) p.start = windowStart;
    if (windowEnd) p.end = windowEnd;
    if (filterLang) p.language = filterLang;
    if (filterSentiment) p.sentimentBand = filterSentiment as 'negative' | 'neutral' | 'positive';
    if (cursor) p.cursor = cursor;
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
        // Append next page
        allItems = [...allItems, ...page.items];
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

  function formatTs(iso: string): string {
    try {
      return new Date(iso).toLocaleString('en-CA', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
      });
    } catch {
      return iso.slice(0, 16).replace('T', ' ');
    }
  }

  function sentimentClass(score: number | null | undefined): string {
    if (score === null || score === undefined) return '';
    if (score >= 0.05) return 'sentiment-pos';
    if (score <= -0.05) return 'sentiment-neg';
    return 'sentiment-neu';
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
            <th scope="col"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each allItems as item (item.articleId)}
            <tr class="article-row">
              <td>
                <time datetime={item.timestamp} class="ts-cell">
                  {formatTs(item.timestamp)}
                </time>
              </td>
              <td>
                <span class="lang-cell">{item.language ?? '—'}</span>
              </td>
              <td class="right">
                <span class="num-cell">{item.wordCount?.toLocaleString() ?? '—'}</span>
              </td>
              <td class="right">
                {#if item.sentimentScore !== null && item.sentimentScore !== undefined}
                  <span class="sentiment-cell {sentimentClass(item.sentimentScore)}">
                    {item.sentimentScore.toFixed(3)}
                  </span>
                {:else}
                  <span class="num-cell muted">—</span>
                {/if}
              </td>
              <td>
                <button
                  type="button"
                  class="view-btn"
                  aria-label="View article detail: {formatTs(item.timestamp)}"
                  onclick={() => (openArticleId = item.articleId)}>View</button
                >
              </td>
            </tr>
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

  .article-table th.right,
  .article-table td.right {
    text-align: right;
  }

  .article-row {
    border-bottom: 1px solid var(--color-border);
    transition: background var(--motion-duration-instant) var(--motion-ease-standard);
  }

  .article-row:hover {
    background: var(--color-surface-hover);
  }

  .article-row td {
    padding: var(--space-2) var(--space-3);
    vertical-align: middle;
  }

  .ts-cell {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    white-space: nowrap;
  }

  .lang-cell {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    text-transform: uppercase;
  }

  .num-cell {
    font-family: var(--font-mono);
    color: var(--color-fg-muted);
  }

  .sentiment-cell {
    font-family: var(--font-mono);
  }

  .sentiment-pos {
    color: #7ec4a0;
  }
  .sentiment-neg {
    color: #c47e7e;
  }
  .sentiment-neu {
    color: var(--color-fg-muted);
  }

  .view-btn {
    padding: 2px var(--space-2);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
    white-space: nowrap;
  }

  .view-btn:hover,
  .view-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent-muted);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

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

  @media (prefers-reduced-motion: reduce) {
    .article-row {
      transition: none;
    }
  }
</style>
