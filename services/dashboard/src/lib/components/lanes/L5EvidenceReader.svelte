<script lang="ts">
  // L5 Evidence Reader (Phase 106).
  // Modal overlay for individual article detail — cleaned text, Silver metadata,
  // per-extractor provenance. Subject to k-anonymity gate (WP-006 §7):
  // HTTP 403 surfaces as a methodological RefusalSurface.
  //
  // The reader does not trap the rest of the surface — it floats over without
  // replacing it (Design Brief §4.1 rule 2 "no layer replaces").
  import { createQuery } from '@tanstack/svelte-query';
  import { diffWordsWithSpace, type Change } from 'diff';
  import {
    articleDetailQuery,
    articleRevisionsQuery,
    articleRevisionDiffQuery,
    type ArticleDetailDto,
    type ArticleRevisionsResponseDto,
    type ArticleRevisionEntryDto,
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

  // Phase 133 — Diff tab reworked to step over EDITORIAL versions only.
  // The Wayback chain is captured at raw-HTML granularity, so most
  // adjacent pairs are "identical after extraction" (a re-archival with
  // no editorial change). Stepping through those is noise. The slider
  // therefore walks only the non-identical consecutive pairs
  // (`walkSteps`), oldest → newest; the chain-head pair (current article
  // body vs the NEWEST snapshot — the latest editorial change) is shown
  // as a separate "vs current article" view. The full per-capture list
  // stays in the Revision history section below (the home for the
  // re-archival frequency signal). `diffStatus` (from the revisions
  // endpoint) classifies every pair WITHOUT fetching each diff up front.
  let activeTab = $state<'article' | 'diff'>('article');
  // Which diff the panel shows: a step in the editorial walk, or the
  // chain-head (current article body vs the NEWEST snapshot).
  let diffView = $state<'walk' | 'cumulative'>('walk');
  // Position WITHIN `walkSteps` (not a revisionIndex). 0 = oldest step.
  let walkPos = $state<number>(0);

  // Chain head = the row carrying the head diff (Phase 133: current article
  // body vs the NEWEST snapshot = the latest editorial change). It is the
  // OLDEST row by snapshot_at (`revisionList[0]`) — the only row with no
  // mid-chain predecessor, so its diff slot holds this head transition.
  // `revision_index` is contiguous per article but NOT guaranteed to start
  // at 0 (ADR-036 rebuilds can offset it), so key off array position.
  const chainHead = $derived<ArticleRevisionEntryDto | null>(revisionList[0] ?? null);
  // Editorial walk = every row after the head (each has a valid
  // predecessor in the contiguous chain) minus the proven-identical
  // re-archivals. `pending` stays in the walk — it MIGHT be a real change
  // once the sweep computes it; we never silently hide a potentially-
  // editorial step, only proven re-archivals are dropped.
  const walkSteps = $derived(revisionList.slice(1).filter((r) => r.diffStatus !== 'identical'));
  const lookupByIndex = $derived(new Map(revisionList.map((r) => [r.revisionIndex, r])));
  // Summary counts span ALL rows (mid-chain steps + the head's
  // newest-snapshot→current transition) so the disclosure matches the
  // `revision_count` metric exactly (Phase 133 — the head carries a real
  // editorial transition, not a cumulative double-count).
  const changedCount = $derived(revisionList.filter((r) => r.diffStatus === 'changed').length);
  const pendingCount = $derived(revisionList.filter((r) => r.diffStatus === 'pending').length);
  const identicalCount = $derived(revisionList.filter((r) => r.diffStatus === 'identical').length);
  const cumulativeAvailable = $derived(chainHead !== null);

  // Whether the Diff tab has anything worth showing: at least one pair is
  // an editorial change or still computing. When every capture is an
  // identical re-archival (common for low-edit articles, and the whole
  // story for single-snapshot ones), the cumulative "vs current article"
  // view would just restate "nothing changed" — so we show one clear
  // line instead of an empty slider (Issue 3).
  const hasEditorialContent = $derived(
    revisionList.some((r) => r.diffStatus === 'changed' || r.diffStatus === 'pending')
  );

  // The revisionIndex the diff query actually fetches.
  const diffPairIndex = $derived(
    diffView === 'cumulative'
      ? (chainHead?.revisionIndex ?? -1)
      : (walkSteps[walkPos]?.revisionIndex ?? -1)
  );

  // BUG-9 + BUG-6 — the diff query fires EAGERLY on modal open (not only
  // on Diff-tab click) so the tab switch is instant.
  const diffQ = createQuery<
    QueryOutcome<ArticleRevisionDiffDto>,
    Error,
    QueryOutcome<ArticleRevisionDiffDto>
  >(() => {
    const enabled =
      open && articleId !== null && diffPairIndex >= 0 && diffPairIndex < revisionList.length;
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

  // Re-seed at the oldest editorial step whenever the article changes.
  // With no editorial steps but a chain-head present, fall back to the
  // cumulative view so the tab is never blank.
  $effect(() => {
    void articleId;
    void revisionList.length;
    walkPos = 0;
    diffView = walkSteps.length > 0 ? 'walk' : 'cumulative';
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

  // Phase 122d.1 BUG-8 — word-level inline diff for `mod` ops via
  // jsdiff. Returns an array of Change records the template renders
  // with rote/green spans. `add` / `del` ops keep their full-paragraph
  // styling (the user wants the whole new/removed paragraph
  // highlighted, not word-by-word).
  function wordDiff(before: string, after: string): Change[] {
    return diffWordsWithSpace(before ?? '', after ?? '');
  }

  // Walk-step label — the (before → after) snapshot dates for a
  // consecutive editorial pair. `before` is the preceding chain entry
  // (revisionIndex − 1), `after` is this step's snapshot.
  function walkStepLabel(step: ArticleRevisionEntryDto | undefined): string {
    if (!step) return '—';
    const before = lookupByIndex.get(step.revisionIndex - 1);
    const b = before ? new Date(before.snapshotAt).toLocaleDateString('en-CA') : '?';
    const a = new Date(step.snapshotAt).toLocaleDateString('en-CA');
    return `${b} → ${a}`;
  }

  // Cumulative label — Phase 133: the chain-head now compares the current
  // article body to the NEWEST snapshot (the latest archived state), i.e.
  // "what the publisher changed since the last archive". Read newest →
  // current.
  function cumulativeLabel(): string {
    const newest = revisionList[revisionList.length - 1]?.snapshotAt;
    if (!newest) return 'Latest snapshot → current article';
    return `Latest snapshot ${new Date(newest).toLocaleDateString('en-CA')} → current article`;
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

    <!-- Phase 122d.1 — Article body / Diff tabs. With BUG-11 the
         Diff tab is enabled at chainLength >= 1 (chain-head pair =
         current Silver vs Wayback[0]). The Diff tab is dimmed only
         for non-web articles. BUG-6 fix: both tab contents stay
         mounted (toggled via CSS .hidden) so the tab switch is
         instant — no remount, no "modal disappeared" perception. -->
    <div class="text-controls" role="tablist" aria-label="Article view tabs">
      <button
        type="button"
        class="toggle-btn"
        class:active={activeTab === 'article'}
        role="tab"
        aria-selected={activeTab === 'article'}
        onclick={(e) => {
          e.stopPropagation();
          activeTab = 'article';
        }}>Article body</button
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
          ? 'Diff view is only meaningful for web articles with a canonical URL.'
          : revisionList.length < 1
            ? 'No Wayback snapshots — nothing to diff against.'
            : 'View paragraph diff between current and archived snapshots'}
        onclick={(e) => {
          e.stopPropagation();
          activeTab = 'diff';
        }}>Diff{revisionList.length >= 1 ? ` (${revisionList.length})` : ''}</button
      >
    </div>

    <!-- Article body tab — always mounted, toggled via CSS so the
         tab switch keeps scroll position + state. -->
    <div class="tab-panel" class:hidden={activeTab !== 'article'}>
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
    </div>

    <!-- Diff tab — also always mounted so the diff query result
         persists across tab switches. The diff fires eagerly on
         modal open (BUG-9 prefetch) so by the time the user clicks
         the tab, content is usually already loaded. -->
    <div class="tab-panel" class:hidden={activeTab !== 'diff'}>
      {#if revisionList.length >= 1}
        {#if !hasEditorialContent}
          <!-- No editorial change anywhere in the chain — show one clear
               line rather than a cumulative view that just restates
               "unchanged" (Issue 3). The full capture list stays in the
               Revision history section below. -->
          <p class="muted">
            No editorial changes detected — unchanged across {revisionList.length} archived capture{revisionList.length ===
            1
              ? ''
              : 's'}{chainHead
              ? ` since ${new Date(chainHead.snapshotAt).toLocaleDateString('en-CA')}`
              : ''}. Wayback re-archived the page, but the article body never changed after
            extraction.
          </p>
        {:else}
          <!-- Disclosure — how the editorial walk relates to the raw
             captures. The walk steps over editorial versions only;
             identical re-archivals are summarised here, never silently
             hidden (their full list is the Revision history below). -->
          <p class="diff-summary">
            {changedCount} editorial {changedCount === 1 ? 'change' : 'changes'}
            {#if identicalCount > 0}
              · {identicalCount} identical re-archival{identicalCount === 1 ? '' : 's'} skipped
            {/if}
            {#if pendingCount > 0}
              · {pendingCount} still computing
            {/if}
          </p>

          <!-- View toggle — only when both an editorial walk and the
             cumulative chain-head comparison are available. -->
          {#if cumulativeAvailable && walkSteps.length > 0}
            <div class="diff-view-toggle" role="tablist" aria-label="Diff view">
              <button
                type="button"
                class="toggle-btn small"
                class:active={diffView === 'walk'}
                role="tab"
                aria-selected={diffView === 'walk'}
                onclick={() => (diffView = 'walk')}>Editorial walk</button
              >
              <button
                type="button"
                class="toggle-btn small"
                class:active={diffView === 'cumulative'}
                role="tab"
                aria-selected={diffView === 'cumulative'}
                onclick={() => (diffView = 'cumulative')}>vs. current article</button
              >
            </div>
          {/if}

          {#if diffView === 'cumulative' && cumulativeAvailable}
            <div class="diff-controls">
              <span class="label-xs">Cumulative</span>
              <span class="diff-pair-readout">{cumulativeLabel()}</span>
            </div>
          {:else if walkSteps.length > 0}
            <div class="diff-controls">
              <label class="diff-pair-label">
                <span class="label-xs">Editorial version</span>
                {#if walkSteps.length > 1}
                  <input
                    type="range"
                    min={0}
                    max={walkSteps.length - 1}
                    step={1}
                    bind:value={walkPos}
                    aria-label="Editorial version selector"
                  />
                {/if}
                <span class="diff-pair-readout">
                  {walkPos + 1} / {walkSteps.length} · {walkStepLabel(walkSteps[walkPos])}
                </span>
              </label>
            </div>
          {:else}
            <p class="muted">
              No editorial changes to step through — every archived capture parsed to identical
              content after extraction.
            </p>
          {/if}

          {#if diffPairIndex < 0}
            <!-- No valid pair selected; the controls above explain why. -->
          {:else if diffQ.isPending}
            <p class="muted" aria-busy="true">Loading diff…</p>
          {:else if diffQ.data?.kind === 'refusal'}
            <RefusalSurface refusal={diffQ.data} {ctx} />
          {:else if diffQ.isError || diffQ.data?.kind === 'network-error'}
            {@const err = (diffQ.error ?? null) as { httpStatus?: number; message?: string } | null}
            {#if err?.httpStatus === 404 || (diffQ.data?.kind === 'network-error' && diffQ.data.httpStatus === 404)}
              <!-- BUG-7 fix — operator info removed. -->
              <p class="muted">Diff is being computed; check back in a few minutes.</p>
            {:else}
              <p class="error">Could not load diff.</p>
            {/if}
          {:else if diffQ.data?.kind === 'success' && diffQ.data.data}
            {@const diff = diffQ.data.data}
            {#if diff.pairKind === 'chain_head'}
              <p class="diff-kind-note">
                <span class="diff-kind-pill">latest vs. current</span>
                Comparing the current article body to the most recent Wayback snapshot — answers "what
                has the publisher changed since the IA last archived this URL".
              </p>
            {/if}
            {#if diff.headlineChanged}
              <div class="headline-change">
                <p class="headline-label">Headline changed</p>
                <p class="headline-before">
                  <span class="op-mark op-del">−</span>
                  {diff.headlineBefore || '(empty)'}
                </p>
                <p class="headline-after">
                  <span class="op-mark op-add">+</span>
                  {diff.headlineAfter || '(empty)'}
                </p>
              </div>
            {/if}
            {#if diff.identical}
              <!-- BUG-10 fix — distinct surface for "computed but
                 identical" (vs. BUG-7's "pending" state). -->
              <p class="muted">
                Both snapshots parse to identical content after extraction. The Wayback Machine
                archived two captures with different file hashes but trafilatura recovers the same
                article body from each — likely just an HTTP re-fetch without an editorial change.
              </p>
            {:else if diff.diffParagraphs.length === 0}
              <p class="muted">No paragraph-level changes detected between these snapshots.</p>
            {:else}
              <ol class="diff-list" aria-label="Paragraph diff">
                {#each diff.diffParagraphs as op, idx (idx)}
                  <li class="diff-item op-{op.op}">
                    {#if op.op === 'add'}
                      <span class="op-mark op-add">+</span>
                      <span class="diff-text diff-add-block">{op.after}</span>
                    {:else if op.op === 'del'}
                      <span class="op-mark op-del">−</span>
                      <span class="diff-text diff-del-block">{op.before}</span>
                    {:else if op.op === 'mod'}
                      <!-- BUG-8 fix — word-level inline diff. Each
                         token is wrapped in a span: added text gets
                         a green background, removed text gets a red
                         background with strike-through, unchanged
                         text stays plain. Reads like a GitHub PR
                         diff inline within the paragraph. -->
                      {@const wordOps = wordDiff(op.before ?? '', op.after ?? '')}
                      <div class="diff-mod">
                        <p class="diff-mod-line">
                          <span class="op-mark op-mod">~</span>
                          <span class="diff-text">
                            {#each wordOps as token, tIdx (tIdx)}
                              {#if token.added}
                                <span class="word-add">{token.value}</span>
                              {:else if token.removed}
                                <span class="word-del">{token.value}</span>
                              {:else}
                                <span class="word-eq">{token.value}</span>
                              {/if}
                            {/each}
                          </span>
                        </p>
                      </div>
                    {:else}
                      <!-- Defence-in-depth: an op outside the add/del/mod
                         vocabulary (e.g. a sentinel leaking through a
                         writer/reader mismatch) must never render as a
                         blank row — surface it explicitly instead. -->
                      <span class="op-mark">·</span>
                      <span class="diff-text muted"
                        >Unrecognised change (<code>{op.op}</code>).</span
                      >
                    {/if}
                  </li>
                {/each}
              </ol>
            {/if}
          {/if}
        {/if}
      {:else}
        <p class="muted">
          No Wayback snapshots are available for this article yet. The diff view becomes available
          once Wayback CDX captures the first snapshot.
        </p>
      {/if}
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
                  <span
                    class="rev-diff-tag rev-diff-{rev.diffStatus}"
                    title={rev.diffStatus === 'changed'
                      ? 'Editorial change detected at this capture'
                      : rev.diffStatus === 'identical'
                        ? 'Re-archival with no editorial change after extraction'
                        : 'Diff not yet computed'}
                  >
                    {rev.diffStatus}
                  </span>
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

  .diff-summary {
    margin: 0 0 var(--space-2);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
  }

  .diff-view-toggle {
    display: flex;
    gap: var(--space-2);
    margin: 0 0 var(--space-2);
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
    background: transparent;
    border-left: 2px solid rgba(232, 168, 80, 0.55);
    padding: var(--space-2);
  }

  .diff-mod {
    width: 100%;
  }

  .diff-mod-line {
    margin: 0;
    display: flex;
    gap: var(--space-2);
    align-items: flex-start;
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

  .op-mod {
    color: rgba(232, 168, 80, 0.95);
  }

  .diff-text {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    line-height: var(--line-height-loose);
    white-space: pre-wrap;
    flex: 1;
  }

  /* BUG-8 — Git-style block highlighting for full-paragraph add/del. */
  .diff-add-block {
    background: rgba(126, 196, 160, 0.18);
    border-radius: var(--radius-sm);
    padding: 2px 4px;
  }

  .diff-del-block {
    background: rgba(196, 126, 126, 0.18);
    border-radius: var(--radius-sm);
    padding: 2px 4px;
    text-decoration: line-through;
    text-decoration-color: rgba(196, 126, 126, 0.55);
  }

  /* BUG-8 — word-level inline diff spans for `mod` ops. Tight
     padding so adjacent words stay visually connected; subtle
     border so the span boundary is legible. */
  .word-add {
    background: rgba(126, 196, 160, 0.25);
    color: var(--color-fg);
    border-radius: 2px;
    padding: 0 2px;
    border-bottom: 1px solid rgba(126, 196, 160, 0.6);
  }

  .word-del {
    background: rgba(196, 126, 126, 0.22);
    color: rgba(196, 126, 126, 0.95);
    border-radius: 2px;
    padding: 0 2px;
    text-decoration: line-through;
    text-decoration-color: rgba(196, 126, 126, 0.7);
    border-bottom: 1px solid rgba(196, 126, 126, 0.6);
  }

  .word-eq {
    color: var(--color-fg);
  }

  /* BUG-6 — tab panels always mounted; visibility toggled via class. */
  .tab-panel.hidden {
    display: none;
  }

  /* BUG-11 — pair-kind banner for chain-head pairs. */
  .diff-kind-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    margin: 0 0 var(--space-3);
    line-height: var(--line-height-loose);
    display: flex;
    gap: var(--space-2);
    align-items: baseline;
    flex-wrap: wrap;
  }

  .diff-kind-pill {
    display: inline-block;
    padding: 1px var(--space-2);
    background: rgba(82, 131, 184, 0.15);
    border: 1px solid rgba(82, 131, 184, 0.5);
    border-radius: var(--radius-pill);
    font-family: var(--font-mono);
    font-size: 10px;
    color: rgba(82, 131, 184, 0.95);
    white-space: nowrap;
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

  .rev-diff-tag {
    font-size: 9px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    padding: 0 var(--space-2);
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border);
    font-family: var(--font-mono);
  }

  .rev-diff-changed {
    color: rgba(126, 196, 160, 0.95);
    border-color: rgba(126, 196, 160, 0.5);
  }

  .rev-diff-identical {
    color: var(--color-fg-subtle);
  }

  .rev-diff-pending {
    color: rgba(232, 168, 80, 0.95);
    border-color: rgba(232, 168, 80, 0.45);
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
