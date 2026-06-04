<script lang="ts">
  // Pure row-renderer for any article-list context — Phase 122d.1 refactor
  // extract from ArticlePreviewList.svelte.
  //
  // Three hosts consume this row:
  //   1. ArticlePreviewList (Dossier inline-expand — `SourceCard`)
  //   2. ArticleListModal (Workbench modal — cooccurrence + revision-cell drill-downs)
  //   3. Future contexts (revision-spec lists, scatter point click, etc.)
  //
  // The row carries optional Phase-122d.1 fields (chainLength, hasHeadlineChange,
  // latestRevisionAt) — present when the parent fetched with
  // ?includeRevisions=true (Phase 122d.1 BFF extension) and on the
  // /revisions/articles endpoint. Absent fields collapse to the
  // standard row without badges.

  interface ArticleRowItem {
    articleId: string;
    source?: string;
    timestamp: string;
    language?: string | null | undefined;
    wordCount?: number | null | undefined;
    sentimentScore?: number | null | undefined;
    // Phase 122d.1 — optional revision fields.
    chainLength?: number | null | undefined;
    // Phase 133 — editorial changes (how often the source actually revised
    // the article), as opposed to chainLength (Wayback captures, mostly
    // identical re-archivals). When present, the pill leads with this.
    editorialChangeCount?: number | null | undefined;
    hasHeadlineChange?: boolean | null | undefined;
    latestRevisionAt?: string | null | undefined;
  }

  interface Props {
    item: ArticleRowItem;
    /** Called with the articleId when the row's "View" button is clicked. */
    onOpen: (articleId: string) => void;
    /** Whether to render the source column. Default true; the Dossier
     *  inline-list hides it (the source is the card heading) while the
     *  Workbench modal shows it (the list spans multiple sources). */
    showSourceCol?: boolean;
  }

  let { item, onOpen, showSourceCol = false }: Props = $props();

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

<tr class="article-row">
  <td>
    <time datetime={item.timestamp} class="ts-cell">
      {formatTs(item.timestamp)}
    </time>
  </td>
  {#if showSourceCol}
    <td>
      <span class="source-cell"><code>{item.source ?? '—'}</code></span>
    </td>
  {/if}
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
  <!-- Phase 133 — REVISION badge = editorial edits ONLY. A revision is a
       confirmed editorial change; raw Wayback captures (mostly identical
       re-archivals) are NEVER a revision and get NO badge. The badge shows
       only when editorialChangeCount ≥ 1; the capture count is disclosed
       solely in the tooltip, clearly labelled "captures". -->
  <td class="badges">
    {#if item.editorialChangeCount !== null && item.editorialChangeCount !== undefined && item.editorialChangeCount > 0}
      <span
        class="badge chain-badge"
        title="{item.editorialChangeCount} editorial change{item.editorialChangeCount === 1
          ? ''
          : 's'}{item.chainLength
          ? ` · ${item.chainLength} archived capture${item.chainLength === 1 ? '' : 's'} (mostly identical re-archivals)`
          : ''}{item.latestRevisionAt ? ` · latest ${formatTs(item.latestRevisionAt)}` : ''}"
        aria-label="Editorial changes: {item.editorialChangeCount}"
      >
        ✎ {item.editorialChangeCount}
      </span>
    {/if}
    {#if item.hasHeadlineChange}
      <span
        class="badge headline-badge"
        title="Headline changed at least once in this revision chain"
        aria-label="Headline changed"
      >
        ✎ headline
      </span>
    {/if}
  </td>
  <td>
    <button
      type="button"
      class="view-btn"
      aria-label="View article detail: {formatTs(item.timestamp)}"
      onclick={() => onOpen(item.articleId)}>View</button
    >
  </td>
</tr>

<style>
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

  .article-row td.right {
    text-align: right;
  }

  .ts-cell {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    white-space: nowrap;
  }

  .source-cell code {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
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

  .muted {
    color: var(--color-fg-muted);
  }

  .badges {
    display: flex;
    gap: var(--space-1);
    flex-wrap: wrap;
    align-items: center;
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
    font-size: 10px;
    line-height: 1.4;
    border: 1px solid var(--color-border);
    background: rgba(154, 143, 184, 0.12);
    color: var(--color-fg-muted);
  }

  .chain-badge {
    border-color: rgba(154, 143, 184, 0.5);
    color: rgba(154, 143, 184, 0.95);
  }

  .headline-badge {
    border-color: rgba(232, 168, 80, 0.55);
    background: rgba(232, 168, 80, 0.15);
    color: rgba(232, 168, 80, 0.95);
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

  @media (prefers-reduced-motion: reduce) {
    .article-row {
      transition: none;
    }
  }
</style>
