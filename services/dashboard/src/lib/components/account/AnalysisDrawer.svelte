<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- opens an internal saved deep link */
  // Phase 141 — the saved-analysis detail drawer, extracted from
  // AnalysesOverlay. A self-contained master-detail pane: it owns the edit /
  // share / delete state + the API calls for the currently `analysis`, and asks
  // the parent (which owns `selectedId` + the list) only to close or to refresh
  // after a mutation. The pane is always reserved, so selecting a row never
  // reflows the list — it just fills here.
  import { goto } from '$app/navigation';
  import { fly } from 'svelte/transition';
  import * as api from '$lib/api/analyses';
  import Button from '$lib/components/base/Button.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import { fmtDate } from './analyses-overlay-internals';

  interface Props {
    analysis: api.AnalysisListItem | null;
    onClose: () => void;
    onChanged: () => void | Promise<void>;
  }

  let { analysis, onClose, onChanged }: Props = $props();

  let editName = $state('');
  let editDescription = $state('');
  let editing = $state(false);
  let drawerBusy = $state(false);
  let drawerMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  // shares
  let shares = $state<api.AnalysisShare[]>([]);
  let shareEmail = $state('');
  let shareCanEdit = $state(false);
  let shareBusy = $state(false);
  let shareMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  // Re-seed the editable fields + (re)load shares whenever the selected analysis
  // changes (a row click, or a list refresh after a save). Mirrors the old
  // openDrawer reset; the parent no longer touches this transient state.
  $effect(() => {
    const a = analysis;
    editing = false;
    editName = a?.name ?? '';
    editDescription = a?.description ?? '';
    drawerMsg = null;
    shareMsg = null;
    shares = [];
    if (a?.owned) void loadShares(a.id);
  });

  async function loadShares(id: string) {
    const res = await api.listShares(id);
    if (res.ok) shares = res.data.shares ?? [];
  }

  async function saveEdit() {
    if (!analysis || drawerBusy || !editName.trim()) return;
    drawerBusy = true;
    drawerMsg = null;
    const res = await api.updateAnalysis(analysis.id, {
      name: editName.trim(),
      description: editDescription.trim()
    });
    drawerBusy = false;
    if (res.ok) {
      editing = false;
      await onChanged();
    } else {
      drawerMsg = { kind: 'error', text: res.message || 'Could not save changes.' };
    }
  }

  async function removeAnalysis() {
    if (!analysis || drawerBusy) return;
    drawerBusy = true;
    const res = await api.deleteAnalysis(analysis.id);
    drawerBusy = false;
    if (res.ok) {
      onClose();
      await onChanged();
    } else {
      drawerMsg = { kind: 'error', text: 'Could not delete.' };
    }
  }

  async function addShareSubmit(event: SubmitEvent) {
    event.preventDefault();
    if (!analysis || !shareEmail.trim() || shareBusy) return;
    shareBusy = true;
    shareMsg = null;
    const res = await api.addShare(analysis.id, shareEmail.trim(), shareCanEdit);
    shareBusy = false;
    if (res.ok) {
      shareEmail = '';
      shareCanEdit = false;
      await loadShares(analysis.id);
    } else {
      shareMsg = {
        kind: 'error',
        text:
          res.code === 'grantee_not_found'
            ? 'No user with that email.'
            : res.code === 'cannot_share_with_self'
              ? 'You already own this analysis.'
              : res.message || 'Could not share.'
      };
    }
  }
  async function removeShareEntry(granteeId: string) {
    if (!analysis) return;
    if ((await api.removeShare(analysis.id, granteeId)).ok) await loadShares(analysis.id);
  }

  // Open in Workbench: a saved analysis IS a Workbench deep link, so we just
  // navigate to it, tagging `?savedAnalysis=<id>` so a later "Save" can offer to
  // update THIS analysis in place rather than only ever creating a new one.
  async function loadAnalysis(id: string) {
    const res = await api.getAnalysis(id);
    if (!res.ok) {
      drawerMsg = { kind: 'error', text: 'Could not open this analysis.' };
      return;
    }
    const state = res.data.state;
    const target = `${state}${state.includes('?') ? '&' : '?'}savedAnalysis=${encodeURIComponent(id)}`;
    await goto(target);
  }
</script>

<aside class="drawer drawer-card">
  {#if analysis}
    {@const a = analysis}
    <div class="drawer-content" in:fly={{ x: 16, duration: 160 }}>
      <header class="drawer-head">
        <h3>{a.name}</h3>
        <button type="button" class="close" aria-label="Close details" onclick={onClose}>×</button>
      </header>

      {#if drawerMsg}<AuthNotice variant={drawerMsg.kind}>{drawerMsg.text}</AuthNotice>{/if}

      <div class="drawer-meta">
        <span>Owner: {a.owned ? 'You' : a.ownerEmail}</span>
        <span>Created: {fmtDate(a.createdAt)}</span>
        <span>Updated: {fmtDate(a.updatedAt)}</span>
      </div>

      <div class="row-actions">
        <Button variant="primary" onclick={() => loadAnalysis(a.id)}>Open in Workbench</Button>
      </div>

      {#if a.permission === 'editable'}
        <section class="drawer-block">
          <div class="block-head">
            <h4>Details</h4>
            {#if !editing}
              <button type="button" class="link" onclick={() => (editing = true)}>Edit</button>
            {/if}
          </div>
          {#if editing}
            <input class="field" bind:value={editName} aria-label="Name" />
            <textarea class="field" rows="2" bind:value={editDescription} aria-label="Description"
            ></textarea>
            <div class="row-actions">
              <Button
                variant="primary"
                loading={drawerBusy}
                disabled={!editName.trim()}
                onclick={saveEdit}>Save</Button
              >
              <Button
                variant="secondary"
                onclick={() => {
                  editing = false;
                  editName = a.name;
                  editDescription = a.description;
                }}>Cancel</Button
              >
            </div>
          {:else}
            <p class="muted">{a.description || 'No description.'}</p>
          {/if}
        </section>
      {/if}

      {#if a.owned}
        <section class="drawer-block">
          <h4>Shared with</h4>
          {#if shareMsg}<AuthNotice variant={shareMsg.kind}>{shareMsg.text}</AuthNotice>{/if}
          {#if shares.length === 0}
            <p class="muted">Not shared with anyone yet.</p>
          {:else}
            <ul class="share-list">
              {#each shares as s (s.granteeId)}
                <li>
                  <span>{s.email} · {s.canEdit ? 'can edit' : 'read-only'}</span>
                  <button
                    type="button"
                    class="link-danger"
                    onclick={() => removeShareEntry(s.granteeId)}>Remove</button
                  >
                </li>
              {/each}
            </ul>
          {/if}
          <form class="share-form" onsubmit={addShareSubmit} novalidate>
            <input
              class="field"
              type="email"
              placeholder="Share with email…"
              bind:value={shareEmail}
              aria-label="Recipient email"
            />
            <label class="inline"
              ><input type="checkbox" bind:checked={shareCanEdit} /> Can edit</label
            >
            <Button
              type="submit"
              variant="secondary"
              loading={shareBusy}
              disabled={!shareEmail.trim()}>Share</Button
            >
          </form>
        </section>

        <section class="drawer-block danger">
          <h4>Delete</h4>
          <p class="muted">Removes this analysis for you and everyone it’s shared with.</p>
          <Button variant="secondary" loading={drawerBusy} onclick={removeAnalysis}>Delete</Button>
        </section>
      {/if}
    </div>
  {:else}
    <div class="drawer-empty">
      <p class="muted">
        Select an analysis to see its details, share it, or open it in the Workbench.
      </p>
    </div>
  {/if}
</aside>

<style>
  /* The drawer is a standalone card beside the list panel (not a flex child of
     the table area), so opening it extends the view without reflowing the list. */
  .drawer {
    width: 22rem;
    flex-shrink: 0;
    height: 100%;
    overflow-y: auto;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .drawer-content {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .drawer-empty {
    margin: auto;
    text-align: center;
    max-width: 16rem;
  }
  .drawer-empty .muted {
    line-height: var(--line-height-loose, 1.6);
  }
  .drawer-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: var(--space-2);
  }
  .drawer-head h3 {
    margin: 0;
    font-size: var(--font-size-md);
    color: var(--color-fg);
  }
  .close {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: 0 var(--space-2);
  }
  .close:hover,
  .close:focus-visible {
    color: var(--color-fg);
    outline: none;
  }
  .drawer-meta {
    display: flex;
    flex-direction: column;
    gap: 2px;
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }
  .drawer-block {
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-3);
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .block-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .drawer-block h4 {
    margin: 0;
    font-size: var(--font-size-base);
    color: var(--color-fg);
    font-weight: var(--font-weight-medium);
  }
  .drawer-block.danger h4 {
    color: var(--color-status-expired);
  }
  .field {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-ui);
    width: 100%;
  }
  .field:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  textarea.field {
    resize: vertical;
    font-family: var(--font-ui);
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
  }
  .share-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
  .inline {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }
  .share-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }
  .share-list li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: var(--space-2);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }
  .muted {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    margin: 0;
    line-height: var(--line-height-base);
  }
  .link {
    background: none;
    border: none;
    color: var(--color-accent);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .link:hover {
    text-decoration: underline;
  }
  .link-danger {
    background: none;
    border: none;
    color: var(--color-status-expired);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .link-danger:hover {
    text-decoration: underline;
  }
  @media (max-width: 720px) {
    .drawer {
      display: none;
    }
  }
</style>
