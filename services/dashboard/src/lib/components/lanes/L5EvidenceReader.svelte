<script lang="ts">
  // L5 Evidence Reader (Phase 106).
  // Modal overlay for individual article detail — cleaned text, Silver metadata,
  // per-extractor provenance. Subject to k-anonymity gate (WP-006 §7):
  // HTTP 403 surfaces as a methodological RefusalSurface.
  //
  // The reader does not trap the rest of the surface — it floats over without
  // replacing it (Design Brief §4.1 rule 2 "no layer replaces").
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
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';

  interface Props {
    open: boolean;
    articleId: string | null;
    ctx: FetchContext;
    onClose: () => void;
  }

  let { open, articleId, ctx, onClose }: Props = $props();

  let provenanceExpanded = $state(false);
  let showRaw = $state(false);
  // Phase 122d.0 — Silent-Edit Observability. The L5 reader surfaces
  // the per-article revision chain inline so the operator can scrub
  // the silent-edit timeline beside the article body. Folded by
  // default; the summary surfaces the count + lookup status so a
  // closed section is still informative.
  let revisionsExpanded = $state(false);

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

  // Phase 122d.0 — Silent-Edit Observability per-article chain. Fires
  // alongside the detail query but is independent: a refusal on the
  // detail (k-anonymity) does not block the revisions query (which
  // has its own Silver-eligibility gate); a refusal on revisions
  // does not block the body render. Both surfaces degrade
  // independently to keep the L5 reader resilient.
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
    if (!articleId) return 'Article';
    if (detailQ.data?.kind === 'success' && detailQ.data.data) {
      const d = detailQ.data.data;
      return `${d.source} · ${new Date(d.timestamp).toLocaleDateString('en-CA')}`;
    }
    return articleId.slice(0, 16) + '…';
  });

  function formatTs(iso: string): string {
    try {
      return new Date(iso).toLocaleString('en-CA', {
        dateStyle: 'medium',
        timeStyle: 'short'
      });
    } catch {
      return iso;
    }
  }

  function onDialogClose() {
    provenanceExpanded = false;
    revisionsExpanded = false;
    showRaw = false;
    onClose();
  }

  function lookupStatusLabel(status: string): string {
    switch (status) {
      case 'ok':
        return 'Wayback CDX returned snapshots.';
      case 'no_snapshots':
        return 'No Wayback snapshots — the URL is not yet archived.';
      case 'failed':
        return 'Wayback lookup failed (timeout or rate-limit).';
      case 'skipped':
        return 'Lookup skipped (canonical URL missing).';
      case 'disabled':
        return 'Wayback integration is disabled in this deployment.';
      default:
        return 'No revision metadata recorded for this article.';
    }
  }
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- source.url is an external link opened in a new tab -->
<Dialog {open} {title} onClose={onDialogClose}>
  {#if !open || !articleId}
    <p class="muted">No article selected.</p>
  {:else if detailQ.isPending}
    <p class="muted" aria-busy="true">Loading article…</p>
  {:else if detailQ.isError}
    {@const err = detailQ.error as { httpStatus?: number; message?: string } | null}
    {#if err?.httpStatus === 404}
      <p class="error">
        Article metadata not found in the metadata store. The article still exists in the analytical
        layer (Gold) and aggregates remain valid, but the full text cannot be retrieved — most
        likely because PostgreSQL retention has pruned the document record while ClickHouse and
        MinIO retain longer. Article ID: <code>{articleId}</code>.
      </p>
    {:else}
      <p class="error">
        Failed to load article{err?.httpStatus ? ` (HTTP ${err.httpStatus})` : ''}. Check network
        connectivity.
      </p>
    {/if}
  {:else if detailQ.data?.kind === 'refusal'}
    <!-- k-anonymity or silver-eligibility gate -->
    <RefusalSurface refusal={detailQ.data} {ctx} />
    <p class="refusal-note">
      The article is not accessible below the k-anonymity threshold (WP-006 §7). The observation
      exists in the Gold layer; the raw text is withheld to protect source-level attribution.
    </p>
  {:else if detailQ.data?.kind === 'network-error'}
    <p class="error">Network error: {detailQ.data.message}</p>
  {:else if detailQ.data?.kind === 'success'}
    {@const article = detailQ.data.data}
    <!-- Article metadata header -->
    <dl class="meta-grid">
      <div class="meta-item">
        <dt>Source</dt>
        <dd><code>{article.source}</code></dd>
      </div>
      <div class="meta-item">
        <dt>Published</dt>
        <dd><time datetime={article.timestamp}>{formatTs(article.timestamp)}</time></dd>
      </div>
      {#if article.language}
        <div class="meta-item">
          <dt>Language</dt>
          <dd><code>{article.language}</code></dd>
        </div>
      {/if}
      <div class="meta-item">
        <dt>Words</dt>
        <dd>{article.wordCount.toLocaleString()}</dd>
      </div>
      {#if article.url}
        <div class="meta-item">
          <dt>URL</dt>
          <dd>
            <a href={article.url} target="_blank" rel="noopener noreferrer" class="source-link">
              {article.url}
            </a>
          </dd>
        </div>
      {/if}
      <div class="meta-item">
        <dt>Schema</dt>
        <dd><code>{article.schemaVersion}</code></dd>
      </div>
    </dl>

    <!-- Text toggle: cleaned vs. raw -->
    <div class="text-controls">
      <button
        type="button"
        class="toggle-btn"
        class:active={!showRaw}
        aria-pressed={!showRaw}
        onclick={() => (showRaw = false)}>Cleaned text</button
      >
      {#if article.rawText}
        <button
          type="button"
          class="toggle-btn"
          class:active={showRaw}
          aria-pressed={showRaw}
          onclick={() => (showRaw = true)}>Raw text</button
        >
      {/if}
    </div>

    <!-- Article body -->
    <div class="article-text" lang={article.language ?? undefined}>
      {showRaw && article.rawText ? article.rawText : article.cleanedText}
    </div>

    <!-- Silent-Edit Observability — per-article revision chain (Phase 122d.0). -->
    <!-- Only meaningful for `source_type='web'` articles: the Wayback CDX -->
    <!-- signal needs a canonical URL the IA can archive, which RSS / legacy -->
    <!-- envelopes do not carry. Rendering the panel for non-web sources -->
    <!-- would collapse the "we did not look because RSS articles have no -->
    <!-- canonical_url chain" state into the same blank surface as "we -->
    <!-- looked, found nothing" — the four-state lookup-status vocabulary -->
    <!-- exists precisely to keep those apart. Hide the section entirely -->
    <!-- when the article is not a web article; restore it transparently -->
    <!-- when WebMeta is the envelope. -->
    {#if article.sourceType !== 'web'}
      <!-- Section intentionally omitted for non-web articles. -->
    {:else if revisionsQ.data?.kind === 'refusal'}
      <details class="revisions-section">
        <summary class="provenance-summary">Revision history (refused)</summary>
        <div class="revisions-body">
          <RefusalSurface refusal={revisionsQ.data} {ctx} />
        </div>
      </details>
    {:else if revisionsQ.data?.kind === 'success'}
      <details class="revisions-section" bind:open={revisionsExpanded}>
        <summary class="provenance-summary">
          Revision history ({revisionList.length}
          {revisionList.length === 1 ? 'revision' : 'revisions'})
          {#if revisionStatus && revisionStatus !== 'ok'}
            <span class="status-badge"><code>{revisionStatus}</code></span>
          {/if}
        </summary>
        <div class="revisions-body">
          <p class="status-line">{lookupStatusLabel(revisionStatus)}</p>
          {#if revisionList.length > 0}
            <ol class="revisions-list">
              {#each revisionList as rev, idx (rev.contentHash + idx)}
                <li class="rev-row">
                  <time datetime={rev.snapshotAt}>{formatTs(rev.snapshotAt)}</time>
                  <span class="rev-trigger"><code>{rev.trigger}</code></span>
                  <code class="rev-hash" title={rev.contentHash}>
                    {rev.contentHash.slice(0, 12)}…
                  </code>
                  {#if rev.archiveUrl}
                    <a
                      href={rev.archiveUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="rev-link"
                    >
                      view snapshot ↗
                    </a>
                  {/if}
                </li>
              {/each}
            </ol>
          {/if}
        </div>
      </details>
    {/if}

    <!-- Extraction provenance (expandable) -->
    {#if article.extractionProvenance && Object.keys(article.extractionProvenance).length > 0}
      <details class="provenance-section" bind:open={provenanceExpanded}>
        <summary class="provenance-summary">
          Extraction provenance ({Object.keys(article.extractionProvenance).length} extractors)
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

  .meta-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(10rem, 1fr));
    gap: var(--space-3);
    margin: 0 0 var(--space-4) 0;
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border-radius: var(--radius-md);
    border: 1px solid var(--color-border);
  }

  .meta-item {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .meta-item dt {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .meta-item dd {
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    margin: 0;
  }

  .meta-item code {
    font-family: var(--font-mono);
  }

  .source-link {
    color: var(--color-accent-muted);
    text-decoration: none;
    overflow-wrap: break-word;
    word-break: break-all;
    font-size: var(--font-size-xs);
  }

  .source-link:hover {
    color: var(--color-accent);
    text-decoration: underline;
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

  .toggle-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
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

  .refusal-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    line-height: var(--line-height-loose);
    margin: var(--space-3) 0 0;
    font-style: italic;
  }

  .provenance-section,
  .meta-section,
  .revisions-section {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    overflow: hidden;
    margin-bottom: var(--space-3);
  }

  .revisions-body {
    padding: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }

  .status-line {
    margin: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
  }

  .status-badge {
    margin-left: var(--space-2);
    padding: 0 var(--space-2);
    border-radius: var(--radius-pill);
    background: rgba(154, 143, 184, 0.18);
    border: 1px solid var(--color-border);
    font-size: 10px;
  }

  .revisions-list {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }

  .rev-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    align-items: baseline;
    font-size: var(--font-size-xs);
    padding: var(--space-2);
    border-bottom: 1px dashed var(--color-border);
  }

  .rev-row time {
    font-family: var(--font-mono);
    color: var(--color-fg);
    flex-shrink: 0;
  }

  .rev-trigger code {
    font-family: var(--font-mono);
    color: var(--color-accent-muted);
  }

  .rev-hash {
    font-family: var(--font-mono);
    color: var(--color-fg-subtle);
  }

  .rev-link {
    color: var(--color-accent-muted);
    text-decoration: none;
    font-size: var(--font-size-xs);
    margin-left: auto;
  }

  .rev-link:hover {
    color: var(--color-accent);
    text-decoration: underline;
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
