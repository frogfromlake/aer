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
    articleRevisionDiffQuery,
    type ArticleDetailDto,
    type ArticleRevisionsResponseDto,
    type ArticleRevisionDiffDto,
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

  // Phase 122d.1 — Diff tab. The tab is only meaningful when the
  // article has ≥ 2 revisions (one to diff against another). The
  // `diffPairIndex` is the LATER revision's `revisionIndex` (the
  // endpoint diffs `index-1` → `index`); we default to the most
  // recent pair on open and let the user scrub backwards.
  let activeTab = $state<'article' | 'diff'>('article');
  let diffPairIndex = $state<number>(1);

  // Diff query — only fires when we are on the diff tab AND the
  // article has at least 2 revisions, so we don't hit the BFF
  // for a 404 in the common case.
  const diffQ = createQuery<
    QueryOutcome<ArticleRevisionDiffDto>,
    Error,
    QueryOutcome<ArticleRevisionDiffDto>
  >(() => {
    const enabled =
      open &&
      articleId !== null &&
      activeTab === 'diff' &&
      revisionList.length >= 2 &&
      diffPairIndex >= 1 &&
      diffPairIndex < revisionList.length;
    if (!enabled || !articleId) {
      return {
        queryKey: ['aer', 'article-revision-diff', null, 0],
        queryFn: () =>
          Promise.resolve({
            kind: 'success',
            data: null
          } as unknown as QueryOutcome<ArticleRevisionDiffDto>),
        staleTime: 0,
        enabled: false
      };
    }
    const o = articleRevisionDiffQuery(ctx, articleId, diffPairIndex);
    return {
      queryKey: [...o.queryKey],
      queryFn: o.queryFn,
      staleTime: o.staleTime,
      enabled
    };
  });

  // Auto-pick the most recent pair when the revision list becomes
  // available (or when the article changes). The most recent pair
  // is `revisionIndex = revisionList.length - 1`.
  $effect(() => {
    if (revisionList.length >= 2) {
      diffPairIndex = revisionList.length - 1;
    } else {
      diffPairIndex = 1;
    }
  });

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
    activeTab = 'article';
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

    <!-- Phase 122d.1 — Article body / Diff tabs. The Diff tab is
         only meaningful when ≥ 2 revisions exist; we still render it
         dimmed when chainLength is 0/1 so the absence is legible
         (per WP-006 §4.2 "what AĒR does not observe is as important
         as what it does"). -->
    <div class="text-controls" role="tablist" aria-label="Article view tabs">
      <button
        type="button"
        class="toggle-btn"
        class:active={activeTab === 'article'}
        role="tab"
        aria-selected={activeTab === 'article'}
        onclick={() => (activeTab = 'article')}>Article body</button
      >
      <button
        type="button"
        class="toggle-btn"
        class:active={activeTab === 'diff'}
        class:disabled-tab={article.sourceType !== 'web' || revisionList.length < 2}
        role="tab"
        aria-selected={activeTab === 'diff'}
        disabled={article.sourceType !== 'web' || revisionList.length < 2}
        title={article.sourceType !== 'web'
          ? 'Diff view is only meaningful for web articles with a canonical URL.'
          : revisionList.length < 2
            ? 'Need at least two revisions to diff. This article has ' + revisionList.length + '.'
            : 'View paragraph diff between consecutive snapshots'}
        onclick={() => (activeTab = 'diff')}
        >Diff{revisionList.length >= 2 ? ` (${revisionList.length - 1})` : ''}</button
      >
    </div>

    {#if activeTab === 'article'}
      <!-- Cleaned/raw sub-toggle within the article tab -->
      <div class="text-subcontrols">
        <button
          type="button"
          class="toggle-btn small"
          class:active={!showRaw}
          aria-pressed={!showRaw}
          onclick={() => (showRaw = false)}>Cleaned</button
        >
        {#if article.rawText}
          <button
            type="button"
            class="toggle-btn small"
            class:active={showRaw}
            aria-pressed={showRaw}
            onclick={() => (showRaw = true)}>Raw</button
          >
        {/if}
      </div>
      <div class="article-text" lang={article.language ?? undefined}>
        {showRaw && article.rawText ? article.rawText : article.cleanedText}
      </div>
    {:else}
      <!-- Phase 122d.1 — Diff view between consecutive snapshots. -->
      {@const pairCount = revisionList.length - 1}
      {#if pairCount >= 1}
        <div class="diff-controls">
          <label class="diff-pair-label">
            <span class="label-xs">Snapshot pair</span>
            <input
              type="range"
              min={1}
              max={pairCount}
              step={1}
              bind:value={diffPairIndex}
              aria-label="Snapshot pair selector"
            />
            <span class="diff-pair-readout"
              >{diffPairIndex} / {pairCount} (revision {diffPairIndex - 1} → {diffPairIndex})</span
            >
          </label>
        </div>
        {#if diffQ.isPending}
          <p class="muted" aria-busy="true">Loading diff…</p>
        {:else if diffQ.data?.kind === 'refusal'}
          <RefusalSurface refusal={diffQ.data} {ctx} />
        {:else if diffQ.isError || diffQ.data?.kind === 'network-error'}
          {@const err = (diffQ.error ?? null) as { httpStatus?: number; message?: string } | null}
          {#if err?.httpStatus === 404 || (diffQ.data?.kind === 'network-error' && diffQ.data.httpStatus === 404)}
            <p class="muted">
              Diff for this snapshot pair has not yet been computed by the worker sweep. The diff
              loop processes pairs in batches; check back in a few minutes, or operator can lower
              <code>REVISION_DIFF_EXTRACTION_INTERVAL_SECONDS</code> for faster turnaround.
            </p>
          {:else}
            <p class="error">Could not load diff.</p>
          {/if}
        {:else if diffQ.data?.kind === 'success' && diffQ.data.data}
          {@const diff = diffQ.data.data}
          {#if diff.headlineChanged}
            <div class="headline-change">
              <p class="headline-label">Headline changed</p>
              <p class="headline-before">
                <span class="op-mark op-del">—</span>
                {diff.headlineBefore || '(empty)'}
              </p>
              <p class="headline-after">
                <span class="op-mark op-add">+</span>
                {diff.headlineAfter || '(empty)'}
              </p>
            </div>
          {/if}
          {#if diff.diffParagraphs.length === 0}
            <p class="muted">
              No paragraph-level changes between these snapshots. The article was archived at two
              different points but the body (after trafilatura re-extraction) is identical.
            </p>
          {:else}
            <ol class="diff-list" aria-label="Paragraph diff">
              {#each diff.diffParagraphs as op, idx (idx)}
                <li class="diff-item op-{op.op}">
                  {#if op.op === 'add'}
                    <span class="op-mark op-add">+</span>
                    <span class="diff-text">{op.after}</span>
                  {:else if op.op === 'del'}
                    <span class="op-mark op-del">−</span>
                    <span class="diff-text strike">{op.before}</span>
                  {:else if op.op === 'mod'}
                    <div class="diff-mod">
                      <p class="diff-mod-before">
                        <span class="op-mark op-del">−</span>
                        <span class="diff-text strike">{op.before}</span>
                      </p>
                      <p class="diff-mod-after">
                        <span class="op-mark op-add">+</span>
                        <span class="diff-text">{op.after}</span>
                      </p>
                    </div>
                  {/if}
                </li>
              {/each}
            </ol>
          {/if}
        {/if}
      {/if}
    {/if}

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

  .toggle-btn.disabled-tab,
  .toggle-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }

  .toggle-btn.small {
    padding: 2px var(--space-2);
    font-size: 10px;
  }

  .text-subcontrols {
    display: flex;
    gap: var(--space-2);
    margin: var(--space-2) 0 var(--space-3);
  }

  .diff-controls {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    margin: var(--space-2) 0 var(--space-3);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .diff-pair-label {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .label-xs {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .diff-pair-readout {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }

  .diff-pair-label input[type='range'] {
    flex: 1;
    min-width: 12rem;
  }

  .headline-change {
    padding: var(--space-3);
    background: rgba(232, 168, 80, 0.08);
    border: 1px solid rgba(232, 168, 80, 0.4);
    border-radius: var(--radius-md);
    margin: 0 0 var(--space-3);
  }

  .headline-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: rgba(232, 168, 80, 0.95);
    margin: 0 0 var(--space-2);
    font-weight: var(--font-weight-medium);
  }

  .headline-before,
  .headline-after {
    margin: 0;
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    display: flex;
    gap: var(--space-2);
  }

  .headline-before .diff-text {
    color: var(--color-fg-muted);
    text-decoration: line-through;
  }

  .headline-after .diff-text {
    color: var(--color-fg);
  }

  .diff-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    max-height: 24rem;
    overflow-y: auto;
  }

  .diff-item {
    padding: var(--space-2);
    border-radius: var(--radius-sm);
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
  }

  .diff-item.op-add {
    background: rgba(126, 196, 160, 0.08);
    border-left: 2px solid rgba(126, 196, 160, 0.85);
  }

  .diff-item.op-del {
    background: rgba(196, 126, 126, 0.06);
    border-left: 2px solid rgba(196, 126, 126, 0.85);
  }

  .diff-item.op-mod {
    background: rgba(232, 168, 80, 0.06);
    border-left: 2px solid rgba(232, 168, 80, 0.85);
    padding: 0;
  }

  .diff-mod {
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  .diff-mod-before,
  .diff-mod-after {
    margin: 0;
    padding: var(--space-2);
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
  }

  .diff-mod-before {
    border-bottom: 1px dashed rgba(232, 168, 80, 0.3);
  }

  .op-mark {
    font-family: var(--font-mono);
    font-weight: var(--font-weight-medium);
    font-size: var(--font-size-sm);
    flex-shrink: 0;
    width: 1.25rem;
    text-align: center;
  }

  .op-add {
    color: rgba(126, 196, 160, 0.95);
  }

  .op-del {
    color: rgba(196, 126, 126, 0.95);
  }

  .diff-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    white-space: pre-wrap;
  }

  .diff-text.strike {
    text-decoration: line-through;
    color: var(--color-fg-muted);
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
