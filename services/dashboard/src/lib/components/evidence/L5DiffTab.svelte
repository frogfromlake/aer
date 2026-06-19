<script lang="ts">
  // Phase 122d.1 — Article-body Diff tab for the L5 Evidence Reader (Phase 141
  // extraction from L5EvidenceReader.svelte). Steps over EDITORIAL versions
  // only (Phase 133): the slider walks the non-identical consecutive pairs
  // (`walkSteps`), oldest → newest; the chain-head pair (current article body
  // vs the NEWEST snapshot) is the separate "vs. current article" view. The
  // full per-capture list stays in the Revision-history section.
  //
  // The component is always mounted (parent toggles visibility via CSS) so the
  // diff query fires eagerly on modal open (BUG-9) and the tab switch is
  // instant. The chain derivation + selectors are pure (l5-evidence-internals,
  // unit-tested); this file is render glue + the diff query.
  import { createQuery } from '@tanstack/svelte-query';
  import {
    articleRevisionDiffQuery,
    type ArticleRevisionDiffDto,
    type ArticleRevisionEntryDto,
    type FetchContext,
    type QueryOutcome
  } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import {
    deriveDiffChain,
    selectedDiffPairIndex,
    wordDiff,
    walkStepLabel,
    cumulativeLabel,
    type DiffView
  } from './l5-evidence-internals';

  let {
    open,
    articleId,
    ctx,
    revisionList
  }: {
    open: boolean;
    articleId: string | null;
    ctx: FetchContext;
    revisionList: ArticleRevisionEntryDto[];
  } = $props();

  // Which diff the panel shows: a step in the editorial walk, or the
  // chain-head (current article body vs the NEWEST snapshot).
  let diffView = $state<DiffView>('walk');
  // Position WITHIN `walkSteps` (not a revisionIndex). 0 = oldest step.
  let walkPos = $state<number>(0);

  const chain = $derived(deriveDiffChain(revisionList));
  const chainHead = $derived(chain.chainHead);
  const walkSteps = $derived(chain.walkSteps);
  const changedCount = $derived(chain.changedCount);
  const pendingCount = $derived(chain.pendingCount);
  const identicalCount = $derived(chain.identicalCount);
  const cumulativeAvailable = $derived(chain.cumulativeAvailable);
  const hasEditorialContent = $derived(chain.hasEditorialContent);
  // The revisionIndex the diff query actually fetches.
  const diffPairIndex = $derived(selectedDiffPairIndex(chain, diffView, walkPos));

  function walkStepLabelFor(step: ArticleRevisionEntryDto | undefined): string {
    return walkStepLabel(chain.lookupByIndex, step);
  }

  // BUG-9 + BUG-6 — the diff query fires EAGERLY on modal open (not only on
  // Diff-tab click) so the tab switch is instant.
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

  // Re-seed at the oldest editorial step whenever the article changes. With no
  // editorial steps but a chain-head present, fall back to the cumulative view
  // so the tab is never blank.
  $effect(() => {
    void articleId;
    void revisionList.length;
    walkPos = 0;
    diffView = walkSteps.length > 0 ? 'walk' : 'cumulative';
  });
</script>

{#if revisionList.length >= 1}
  {#if !hasEditorialContent}
    <!-- No editorial change anywhere in the chain — show one clear line rather
         than a cumulative view that just restates "unchanged" (Issue 3). The
         full capture list stays in the Revision history section below. -->
    {@const sincePart = chainHead
      ? m.evidence_no_editorial_changes_since({
          date: new Date(chainHead.snapshotAt).toLocaleDateString('en-CA')
        })
      : ''}
    <p class="muted">
      {revisionList.length === 1
        ? m.evidence_no_editorial_changes_one({ count: revisionList.length, since: sincePart })
        : m.evidence_no_editorial_changes_other({ count: revisionList.length, since: sincePart })}
    </p>
  {:else}
    <!-- Disclosure — how the editorial walk relates to the raw captures. The
         walk steps over editorial versions only; identical re-archivals are
         summarised here, never silently hidden (their full list is the
         Revision history below). -->
    <p class="diff-summary">
      {changedCount === 1
        ? m.evidence_editorial_change_count_one({ count: changedCount })
        : m.evidence_editorial_change_count_other({ count: changedCount })}
      {#if identicalCount > 0}
        {identicalCount === 1
          ? m.evidence_identical_rearchival_one({ count: identicalCount })
          : m.evidence_identical_rearchival_other({ count: identicalCount })}
      {/if}
      {#if pendingCount > 0}
        {m.evidence_still_computing({ count: pendingCount })}
      {/if}
    </p>

    <!-- View toggle — only when both an editorial walk and the cumulative
         chain-head comparison are available. -->
    {#if cumulativeAvailable && walkSteps.length > 0}
      <div class="diff-view-toggle" role="tablist" aria-label={m.evidence_diff_view()}>
        <button
          type="button"
          class="toggle-btn small"
          class:active={diffView === 'walk'}
          role="tab"
          aria-selected={diffView === 'walk'}
          onclick={() => (diffView = 'walk')}>{m.evidence_editorial_walk()}</button
        >
        <button
          type="button"
          class="toggle-btn small"
          class:active={diffView === 'cumulative'}
          role="tab"
          aria-selected={diffView === 'cumulative'}
          onclick={() => (diffView = 'cumulative')}>{m.evidence_vs_current_article()}</button
        >
      </div>
    {/if}

    {#if diffView === 'cumulative' && cumulativeAvailable}
      <div class="diff-controls">
        <span class="label-xs">{m.evidence_cumulative()}</span>
        <span class="diff-pair-readout">{cumulativeLabel(revisionList)}</span>
      </div>
    {:else if walkSteps.length > 0}
      <div class="diff-controls">
        <label class="diff-pair-label">
          <span class="label-xs">{m.evidence_editorial_version()}</span>
          {#if walkSteps.length > 1}
            <input
              type="range"
              min={0}
              max={walkSteps.length - 1}
              step={1}
              bind:value={walkPos}
              aria-label={m.evidence_editorial_version_selector()}
            />
          {/if}
          <span class="diff-pair-readout">
            {walkPos + 1} / {walkSteps.length} · {walkStepLabelFor(walkSteps[walkPos])}
          </span>
        </label>
      </div>
    {:else}
      <p class="muted">{m.evidence_no_editorial_steps()}</p>
    {/if}

    {#if diffPairIndex < 0}
      <!-- No valid pair selected; the controls above explain why. -->
    {:else if diffQ.isPending}
      <p class="muted" aria-busy="true">{m.evidence_loading_diff()}</p>
    {:else if diffQ.data?.kind === 'refusal'}
      <RefusalSurface refusal={diffQ.data} {ctx} />
    {:else if diffQ.isError || diffQ.data?.kind === 'network-error'}
      {@const err = (diffQ.error ?? null) as { httpStatus?: number; message?: string } | null}
      {#if err?.httpStatus === 404 || (diffQ.data?.kind === 'network-error' && diffQ.data.httpStatus === 404)}
        <!-- BUG-7 fix — operator info removed. -->
        <p class="muted">{m.evidence_diff_being_computed()}</p>
      {:else}
        <p class="error">{m.evidence_could_not_load_diff()}</p>
      {/if}
    {:else if diffQ.data?.kind === 'success' && diffQ.data.data}
      {@const diff = diffQ.data.data}
      {#if diff.pairKind === 'chain_head'}
        <p class="diff-kind-note">
          <span class="diff-kind-pill">{m.evidence_latest_vs_current()}</span>
          {m.evidence_chain_head_note()}
        </p>
      {/if}
      {#if diff.headlineChanged}
        <div class="headline-change">
          <p class="headline-label">{m.evidence_headline_changed()}</p>
          <p class="headline-before">
            <span class="op-mark op-del">−</span>
            {diff.headlineBefore || m.evidence_empty()}
          </p>
          <p class="headline-after">
            <span class="op-mark op-add">+</span>
            {diff.headlineAfter || m.evidence_empty()}
          </p>
        </div>
      {/if}
      {#if diff.identical}
        <!-- BUG-10 fix — distinct surface for "computed but identical" (vs.
             BUG-7's "pending" state). -->
        <p class="muted">{m.evidence_both_identical()}</p>
      {:else if diff.diffParagraphs.length === 0}
        <p class="muted">{m.evidence_no_paragraph_changes()}</p>
      {:else}
        <ol class="diff-list" aria-label={m.evidence_paragraph_diff()}>
          {#each diff.diffParagraphs as op, idx (idx)}
            <li class="diff-item op-{op.op}">
              {#if op.op === 'add'}
                <span class="op-mark op-add">+</span>
                <span class="diff-text diff-add-block">{op.after}</span>
              {:else if op.op === 'del'}
                <span class="op-mark op-del">−</span>
                <span class="diff-text diff-del-block">{op.before}</span>
              {:else if op.op === 'mod'}
                <!-- BUG-8 fix — word-level inline diff. Each token is wrapped in
                     a span: added text gets a green background, removed text
                     gets a red background with strike-through, unchanged text
                     stays plain. Reads like a GitHub PR diff inline within the
                     paragraph. -->
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
                <!-- Defence-in-depth: an op outside the add/del/mod vocabulary
                     (e.g. a sentinel leaking through a writer/reader mismatch)
                     must never render as a blank row — surface it explicitly
                     instead. -->
                <span class="op-mark">·</span>
                <span class="diff-text muted">{m.evidence_unrecognised_change({ op: op.op })}</span>
              {/if}
            </li>
          {/each}
        </ol>
      {/if}
    {/if}
  {/if}
{:else}
  <p class="muted">{m.evidence_no_wayback_snapshots()}</p>
{/if}

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

  /* Toggle buttons (shared visual with the parent's tab strip; duplicated here
     because Svelte scopes <style> per component). */
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

  .toggle-btn.small {
    padding: 2px var(--space-2);
    font-size: 10px;
  }

  .toggle-btn:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
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

  /* BUG-8 — word-level inline diff spans for `mod` ops. Tight padding so
     adjacent words stay visually connected; subtle border so the span boundary
     is legible. */
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

  @media (prefers-reduced-motion: reduce) {
    .toggle-btn {
      transition: none;
    }
  }
</style>
