<script lang="ts">
  // Phase 122d.0 — Silent-Edit Observability: per-article revision chain for the
  // L5 Evidence Reader (Phase 141 extraction). Lists every Wayback capture with
  // its editorial diffStatus + Phase 122d.3 discourse-shift deltas. Only
  // rendered for `source_type='web'` articles (the parent gates that) — the
  // Wayback CDX signal needs a canonical URL the IA can archive.
  import type { ArticleRevisionEntryDto, FetchContext, RefusalOutcome } from '$lib/api/queries';
  import RefusalSurface from '$lib/components/RefusalSurface.svelte';
  import { formatTs, sentimentArrow, fmtDelta, lookupStatusLabel } from './l5-evidence-internals';

  let {
    revisionsRefusal,
    revisionsSuccess,
    revisionList,
    revisionStatus,
    ctx,
    expanded = $bindable(false)
  }: {
    revisionsRefusal: RefusalOutcome | null;
    revisionsSuccess: boolean;
    revisionList: ArticleRevisionEntryDto[];
    revisionStatus: string;
    ctx: FetchContext;
    expanded?: boolean;
  } = $props();
</script>

<!-- eslint-disable svelte/no-navigation-without-resolve -- rev.archiveUrl is an external link opened in a new tab -->
{#if revisionsRefusal}
  <details class="revisions-section">
    <summary class="provenance-summary">Revision history (refused)</summary>
    <div class="revisions-body">
      <RefusalSurface refusal={revisionsRefusal} {ctx} />
    </div>
  </details>
{:else if revisionsSuccess}
  <details class="revisions-section" bind:open={expanded}>
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
                <a href={rev.archiveUrl} target="_blank" rel="noopener noreferrer" class="rev-link">
                  view snapshot ↗
                </a>
              {/if}
              <!-- Phase 122d.3 — discourse-shift deltas for this edit (later −
                   earlier). Only shown when computed; pending / identical /
                   language-unknown rows carry no measurement. -->
              {#if rev.deltasComputed}
                <span class="rev-deltas">
                  <span
                    class="rev-delta-sent"
                    class:pos={(rev.sentimentDelta ?? 0) > 0}
                    class:neg={(rev.sentimentDelta ?? 0) < 0}
                    title="Sentiment change across this edit (later − earlier), multilingual backbone"
                  >
                    {sentimentArrow(rev.sentimentDelta ?? 0)} sentiment {fmtDelta(
                      rev.sentimentDelta ?? 0
                    )}
                  </span>
                  <span
                    class="rev-delta-topic"
                    title="Semantic shift — E5 cosine distance between the two snapshot texts (0 = identical meaning)"
                  >
                    ⤳ shift {(rev.topicShiftScore ?? 0).toFixed(3)}
                  </span>
                  {#if (rev.entitiesAdded?.length ?? 0) > 0 || (rev.entitiesRemoved?.length ?? 0) > 0}
                    <span
                      class="rev-delta-ent"
                      title={`Entities added: ${(rev.entitiesAdded ?? []).join(', ') || '—'}\nEntities removed: ${(rev.entitiesRemoved ?? []).join(', ') || '—'}`}
                    >
                      +{rev.entitiesAdded?.length ?? 0} / −{rev.entitiesRemoved?.length ?? 0}
                      entities
                    </span>
                  {/if}
                </span>
              {:else if rev.diffStatus === 'changed'}
                <span
                  class="rev-deltas rev-deltas-pending"
                  title="Discourse-shift deltas are still being computed for this edit"
                >
                  shift computing…
                </span>
              {/if}
            </li>
          {/each}
        </ol>
      {/if}
    </div>
  </details>
{/if}

<style>
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

  /* Phase 122d.3 — discourse-shift deltas wrap onto their own line under the
     revision row (flex-basis:100% forces the wrap regardless of the auto-margin
     on .rev-link). */
  .rev-deltas {
    flex-basis: 100%;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-3);
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--color-fg-muted);
    padding-left: var(--space-2);
  }

  .rev-deltas-pending {
    color: rgba(232, 168, 80, 0.9);
    font-style: italic;
  }

  .rev-delta-sent.pos {
    color: rgba(126, 196, 160, 0.95);
  }

  .rev-delta-sent.neg {
    color: rgba(214, 122, 122, 0.95);
  }

  .rev-delta-ent {
    color: var(--color-fg-subtle);
    cursor: help;
  }

  /* Section summary (shared visual with the parent's provenance sections;
     duplicated here because Svelte scopes <style> per component). */
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
</style>
