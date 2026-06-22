<script lang="ts">
  // L5 Evidence Reader (Phase 106).
  // Modal overlay for individual article detail — cleaned text, Silver metadata,
  // per-extractor provenance. Subject to k-anonymity gate (WP-006 §7):
  // HTTP 403 surfaces as a methodological RefusalSurface.
  //
  // The reader does not trap the rest of the surface — it floats over without
  // replacing it (Design Brief §4.1 rule 2 "no layer replaces").
  //
  // Phase 141 — decomposed into per-region children (L5MetaGrid /
  // L5NegativeSpaceSection / L5DiffTab / L5RevisionHistory) + pure logic in
  // l5-evidence-internals.ts (unit-tested). This file is now the thin
  // orchestrator: the two independent article queries (detail + revisions) and
  // the Article-body tab.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    articleDetailQuery,
    articleRevisionsQuery,
    type ArticleDetailDto,
    type ArticleRevisionsResponseDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import { Dialog } from '$lib/components/base';
  import { m } from '$lib/paraglide/messages.js';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import L5MetaGrid from './L5MetaGrid.svelte';
  import L5NegativeSpaceSection from './L5NegativeSpaceSection.svelte';
  import L5DiffTab from './L5DiffTab.svelte';
  import L5RevisionHistory from './L5RevisionHistory.svelte';

  interface Props {
    open: boolean;
    articleId: string | null;
    ctx: FetchContext;
    onClose: () => void;
  }

  let { open, articleId, ctx, onClose }: Props = $props();

  let provenanceExpanded = $state(false);
  let showRaw = $state(false);
  // Phase 122d.0 — Silent-Edit Observability. The L5 reader surfaces the
  // per-article revision chain inline so the operator can scrub the silent-edit
  // timeline beside the article body. Folded by default; the summary surfaces
  // the count + lookup status so a closed section is still informative.
  let revisionsExpanded = $state(false);
  // Phase 122d.1 — Article body / Diff tabs. Both tab contents stay mounted
  // (toggled via CSS .hidden) so the tab switch is instant (BUG-6) and the diff
  // query inside L5DiffTab fires eagerly on modal open (BUG-9).
  let activeTab = $state<'article' | 'diff'>('article');

  const detailQ = createQuery<
    QueryOutcome<ArticleDetailDto>,
    Error,
    QueryOutcome<ArticleDetailDto>
  >(() => {
    const enabled = open && articleId !== null;
    if (!enabled || !articleId) {
      return {
        queryKey: ['aer', 'article-detail', null],
        queryFn: () =>
          Promise.resolve({
            kind: 'success',
            data: null
          } as unknown as QueryOutcome<ArticleDetailDto>),
        staleTime: 0,
        enabled: false
      };
    }
    const o = articleDetailQuery(ctx, articleId);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled
    };
  });

  // Phase 122d.0 — Silent-Edit Observability per-article chain. Fires alongside
  // the detail query but is independent: a refusal on the detail (k-anonymity)
  // does not block the revisions query (which has its own Silver-eligibility
  // gate); a refusal on revisions does not block the body render. Both surfaces
  // degrade independently to keep the L5 reader resilient.
  const revisionsQ = createQuery<
    QueryOutcome<ArticleRevisionsResponseDto>,
    Error,
    QueryOutcome<ArticleRevisionsResponseDto>
  >(() => {
    const enabled = open && articleId !== null;
    if (!enabled || !articleId) {
      return {
        queryKey: ['aer', 'article-revisions', null],
        queryFn: () =>
          Promise.resolve({
            kind: 'success',
            data: null
          } as unknown as QueryOutcome<ArticleRevisionsResponseDto>),
        staleTime: 0,
        enabled: false
      };
    }
    const o = articleRevisionsQuery(ctx, articleId);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled
    };
  });

  const revisionList = $derived(
    revisionsQ.data?.kind === 'success' ? (revisionsQ.data.data.revisions ?? []) : []
  );
  const revisionStatus = $derived(
    revisionsQ.data?.kind === 'success' ? revisionsQ.data.data.lookupStatus : ''
  );

  let title = $derived.by(() => {
    if (!articleId) return m.evidence_article();
    if (detailQ.data?.kind === 'success' && detailQ.data.data) {
      const d = detailQ.data.data;
      return `${d.source} · ${new Date(d.timestamp).toLocaleDateString('en-CA')}`;
    }
    return articleId.slice(0, 16) + '…';
  });

  function onDialogClose() {
    provenanceExpanded = false;
    revisionsExpanded = false;
    showRaw = false;
    activeTab = 'article';
    onClose();
  }
</script>

<Dialog {open} {title} onClose={onDialogClose}>
  {#if !open || !articleId}
    <p class="muted">{m.evidence_no_article_selected()}</p>
  {:else if detailQ.isPending}
    <p class="muted" aria-busy="true">{m.evidence_loading_article()}</p>
  {:else if detailQ.isError}
    {@const err = detailQ.error as { httpStatus?: number; message?: string } | null}
    {#if err?.httpStatus === 404}
      <p class="error">{m.evidence_metadata_not_found({ id: articleId })}</p>
    {:else}
      <p class="error">
        {err?.httpStatus
          ? m.evidence_failed_to_load_with_status({ status: err.httpStatus })
          : m.evidence_failed_to_load()}
      </p>
    {/if}
  {:else if detailQ.data?.kind === 'refusal'}
    <!-- k-anonymity or silver-eligibility gate -->
    <RefusalSurface refusal={detailQ.data} {ctx} />
    <p class="refusal-note">{m.evidence_k_anonymity_note()}</p>
  {:else if detailQ.data?.kind === 'network-error'}
    <p class="error">{m.evidence_network_error({ message: detailQ.data.message })}</p>
  {:else if detailQ.data?.kind === 'success'}
    {@const article = detailQ.data.data}
    <L5MetaGrid {article} />

    <L5NegativeSpaceSection {revisionList} {revisionStatus} {articleId} />

    <!-- Article body / Diff tabs. The Diff tab is enabled at chainLength >= 1
         (BUG-11) and dimmed only for non-web articles. -->
    <div class="text-controls" role="tablist" aria-label={m.evidence_article_view_tabs()}>
      <button
        type="button"
        class="toggle-btn"
        class:active={activeTab === 'article'}
        role="tab"
        aria-selected={activeTab === 'article'}
        onclick={(e) => {
          e.stopPropagation();
          activeTab = 'article';
        }}>{m.evidence_article_body()}</button
      >
      <button
        type="button"
        class="toggle-btn"
        class:active={activeTab === 'diff'}
        class:disabled-tab={article.sourceType !== 'web' || revisionList.length < 1}
        role="tab"
        aria-selected={activeTab === 'diff'}
        disabled={article.sourceType !== 'web' || revisionList.length < 1}
        title={article.sourceType !== 'web'
          ? m.evidence_diff_tab_title_non_web()
          : revisionList.length < 1
            ? m.evidence_diff_tab_title_no_snapshots()
            : m.evidence_diff_tab_title_view()}
        onclick={(e) => {
          e.stopPropagation();
          activeTab = 'diff';
        }}>{m.evidence_diff()}{revisionList.length >= 1 ? ` (${revisionList.length})` : ''}</button
      >
    </div>

    <!-- Article body tab — always mounted, toggled via CSS so the tab switch
         keeps scroll position + state. -->
    <div class="tab-panel" class:hidden={activeTab !== 'article'}>
      <div class="text-subcontrols">
        <button
          type="button"
          class="toggle-btn small"
          class:active={!showRaw}
          aria-pressed={!showRaw}
          onclick={() => (showRaw = false)}>{m.evidence_cleaned()}</button
        >
        {#if article.rawText}
          <button
            type="button"
            class="toggle-btn small"
            class:active={showRaw}
            aria-pressed={showRaw}
            onclick={() => (showRaw = true)}>{m.evidence_raw()}</button
          >
          <!-- Phase 148c — raw HTML is sourced from Bronze on-demand (no longer
               stored in Silver); Bronze has a 90-day TTL, so the Raw view is
               time-limited. Disclose it so an absent Raw toggle on older
               articles reads as expected, not broken. -->
          <span class="raw-note">{m.evidence_raw_note()}</span>
        {/if}
      </div>
      <div class="article-text" lang={article.language ?? undefined}>
        {showRaw && article.rawText ? article.rawText : article.cleanedText}
      </div>
    </div>

    <!-- Diff tab — also always mounted so the diff query result persists across
         tab switches and prefetches on modal open. -->
    <div class="tab-panel" class:hidden={activeTab !== 'diff'}>
      <L5DiffTab {open} {articleId} {ctx} {revisionList} />
    </div>

    <!-- Silent-Edit Observability — per-article revision chain (Phase 122d.0). -->
    <!-- Only meaningful for `source_type='web'` articles: the Wayback CDX signal -->
    <!-- needs a canonical URL the IA can archive, which RSS / legacy envelopes -->
    <!-- do not carry. Hiding the section entirely for non-web sources keeps the -->
    <!-- four-state lookup-status vocabulary from collapsing into a blank. -->
    {#if article.sourceType === 'web'}
      <L5RevisionHistory
        revisionsRefusal={revisionsQ.data?.kind === 'refusal' ? revisionsQ.data : null}
        revisionsSuccess={revisionsQ.data?.kind === 'success'}
        {revisionList}
        {revisionStatus}
        {ctx}
        bind:expanded={revisionsExpanded}
      />
    {/if}

    <!-- Extraction provenance (expandable) -->
    {#if article.extractionProvenance && Object.keys(article.extractionProvenance).length > 0}
      <details class="provenance-section" bind:open={provenanceExpanded}>
        <summary class="provenance-summary">
          {m.evidence_extraction_provenance({
            count: Object.keys(article.extractionProvenance).length
          })}
        </summary>
        <dl class="provenance-list">
          {#each Object.entries(article.extractionProvenance) as [extractor, version] (extractor)}
            <div class="prov-row">
              <dt><code>{extractor}</code></dt>
              <dd><code>{version}</code></dd>
            </div>
          {/each}
        </dl>
      </details>
    {/if}

    <!-- SilverMeta (if present) -->
    {#if article.meta && Object.keys(article.meta).length > 0}
      <details class="meta-section">
        <summary class="provenance-summary">SilverMeta</summary>
        <pre class="meta-pre">{JSON.stringify(article.meta, null, 2)}</pre>
      </details>
    {/if}
  {/if}
</Dialog>

<style>
  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }

  .error {
    font-size: var(--font-size-sm);
    color: var(--color-status-expired);
    margin: 0;
  }

  .refusal-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    line-height: var(--line-height-loose);
    margin: var(--space-3) 0 0;
    font-style: italic;
  }

  .text-controls {
    display: flex;
    gap: var(--space-2);
    margin-bottom: var(--space-3);
  }

  .toggle-btn {
    padding: 3px var(--space-3);
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    cursor: pointer;
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .toggle-btn:hover {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .toggle-btn.active {
    color: var(--color-fg);
    background: var(--color-surface);
    border-color: var(--color-accent-muted);
  }

  .toggle-btn.disabled-tab,
  .toggle-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .toggle-btn.small {
    padding: 2px var(--space-2);
    font-size: 10px;
  }

  .toggle-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .text-subcontrols {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--space-2);
    margin: var(--space-2) 0 var(--space-3);
  }

  /* Phase 148c — discloses the 90-day Bronze TTL on the on-demand Raw view. */
  .raw-note {
    font-size: var(--font-size-xs, 0.75rem);
    color: var(--color-fg-subtle);
    line-height: 1.3;
  }

  /* BUG-6 — tab panels always mounted; visibility toggled via class. */
  .tab-panel.hidden {
    display: none;
  }

  .article-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    white-space: pre-wrap;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    max-height: 28rem;
    overflow-y: auto;
    margin-bottom: var(--space-4);
    font-family: var(--font-ui);
  }

  .provenance-section,
  .meta-section {
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    margin-bottom: var(--space-3);
    background: var(--color-surface-2, var(--color-surface));
  }

  .provenance-summary {
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    cursor: pointer;
    user-select: none;
    list-style: none;
    background: var(--color-bg-elevated);
  }

  .provenance-summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .provenance-list {
    margin: 0;
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .prov-row {
    display: flex;
    gap: var(--space-3);
    font-size: var(--font-size-xs);
  }

  .prov-row dt {
    color: var(--color-fg-muted);
    flex-shrink: 0;
  }

  .prov-row dd {
    color: var(--color-fg-subtle);
    margin: 0;
  }

  .meta-pre {
    padding: var(--space-3);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    white-space: pre-wrap;
    overflow-x: auto;
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .toggle-btn {
      transition: none;
    }
  }
</style>
