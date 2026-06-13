<script lang="ts">
  /* eslint-disable svelte/no-navigation-without-resolve -- loads an internal saved deep link */
  // Saved analyses as a global overlay (Phase 135 / ADR-040). Same model as the
  // Dossier / account overlays: a dimmed scrim over the persistent globe + a
  // solid panel, driven by `?analyses=open`.
  //
  // A saved analysis IS a Workbench deep link — the full Workbench state already
  // round-trips through the URL grammar (`?activePillar=&aleph=/episteme=/…`),
  // so we persist the current relative URL (path + search minus overlay params)
  // and restore it with a plain navigation. No bespoke (de)serialisation.
  import { goto } from '$app/navigation';
  import { fly } from 'svelte/transition';
  import * as api from '$lib/api/analyses';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import Button from '$lib/components/base/Button.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';

  // Params that are never part of a saved Workbench deep-link: the overlay
  // toggles plus `savedAnalysis` (the "which saved analysis is loaded" marker).
  const NON_STATE_PARAMS = ['analyses', 'account', 'admin', 'dossier', 'savedAnalysis'];

  const url = $derived(urlState());
  const isOpen = $derived(url.analyses === 'open' || url.analyses === 'save');

  let items = $state<api.AnalysisListItem[]>([]);
  let loading = $state(false);
  let loadError = $state<string | null>(null);

  // --- filters / sort (all client-side; live, no refetch) -------------------
  let search = $state('');
  let showOwned = $state(true);
  let showShared = $state(true);
  let showEditable = $state(true);
  let showReadable = $state(true);
  let createdFrom = $state('');
  let createdTo = $state('');

  type SortKey = 'name' | 'ownerEmail' | 'createdAt' | 'updatedAt';
  let sortKey = $state<SortKey>('updatedAt');
  let sortDir = $state<'asc' | 'desc'>('desc');

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = key === 'createdAt' || key === 'updatedAt' ? 'desc' : 'asc';
    }
  }
  const arrow = (key: SortKey) => (sortKey === key ? (sortDir === 'asc' ? '▲' : '▼') : '');

  const filtered = $derived.by(() => {
    const q = search.trim().toLowerCase();
    const from = createdFrom ? new Date(createdFrom).getTime() : null;
    const to = createdTo ? new Date(createdTo).getTime() + 86_400_000 : null; // inclusive day
    const rows = items.filter((a) => {
      if (q && !`${a.name} ${a.description} ${a.ownerEmail}`.toLowerCase().includes(q))
        return false;
      if (!showOwned && a.owned) return false;
      if (!showShared && !a.owned) return false;
      if (!showEditable && a.permission === 'editable') return false;
      if (!showReadable && a.permission === 'readable') return false;
      const t = new Date(a.createdAt).getTime();
      if (from !== null && t < from) return false;
      if (to !== null && t >= to) return false;
      return true;
    });
    const dir = sortDir === 'asc' ? 1 : -1;
    return rows.sort((a, b) => {
      const x = a[sortKey] ?? '';
      const y = b[sortKey] ?? '';
      if (sortKey === 'createdAt' || sortKey === 'updatedAt') {
        return (new Date(x).getTime() - new Date(y).getTime()) * dir;
      }
      return String(x).localeCompare(String(y)) * dir;
    });
  });

  // --- load list ------------------------------------------------------------
  async function loadList() {
    loading = true;
    loadError = null;
    const res = await api.listAnalyses();
    loading = false;
    if (res.ok) items = res.data.analyses ?? [];
    else loadError = 'Could not load your saved analyses.';
  }

  // --- save current view ----------------------------------------------------
  let saving = $state(false);
  // The save panel: closed, or open. When open it shows the new/update *choice*
  // whenever an editable analysis is loaded and the user hasn't opted for a
  // fresh copy (`forceNew`) — kept reactive so the choice still appears if the
  // list finishes loading after the panel opens (the rail "save" shortcut race).
  let saveStep = $state<'closed' | 'open'>('closed');
  let forceNew = $state(false);
  let saveName = $state('');
  let saveDescription = $state('');
  let saveMsg = $state<{ kind: 'error' | 'success'; text: string } | null>(null);

  function currentDeepLink(): string {
    const u = new URL(window.location.href);
    for (const k of NON_STATE_PARAMS) u.searchParams.delete(k);
    return u.pathname + (u.searchParams.toString() ? `?${u.searchParams}` : '');
  }

  // "Save current view" only makes sense on a *configured* Workbench. The
  // Workbench state lives in the URL (`?aleph=/episteme=/rhizome=<base64url>`),
  // so we require the `/workbench` path with at least one non-empty pillar.
  // Re-evaluates whenever the URL state changes (opening the overlay toggles it).
  const canSaveCurrent = $derived.by(() => {
    void url; // track url-state so this re-runs when the overlay opens
    if (typeof window === 'undefined') return false;
    const u = new URL(window.location.href);
    if (!u.pathname.startsWith('/workbench')) return false;
    return ['aleph', 'episteme', 'rhizome'].some((k) => (u.searchParams.get(k) ?? '') !== '');
  });

  // The saved analysis the current view was opened from (set by "Open in
  // Workbench"). If it's still ours-and-editable, "Save" can update it in place.
  const loadedAnalysis = $derived.by(() => {
    const id = url.savedAnalysis;
    if (!id) return null;
    return items.find((a) => a.id === id && a.permission === 'editable') ?? null;
  });

  function beginSave() {
    saveMsg = null;
    forceNew = false;
    saveStep = 'open';
  }
  function beginSaveAsNew() {
    saveName = '';
    saveDescription = '';
    forceNew = true;
  }
  function cancelSave() {
    saveStep = 'closed';
  }
  // Show the new/update choice only while an editable analysis is loaded and the
  // user hasn't chosen a fresh copy.
  const showSaveChoice = $derived(saveStep === 'open' && !!loadedAnalysis && !forceNew);

  async function updateLoaded() {
    if (!loadedAnalysis || saving || !canSaveCurrent) return;
    saving = true;
    saveMsg = null;
    const res = await api.updateAnalysis(loadedAnalysis.id, { state: currentDeepLink() });
    saving = false;
    if (res.ok) {
      saveStep = 'closed';
      saveMsg = { kind: 'success', text: `Updated “${loadedAnalysis.name}”.` };
      await loadList();
    } else {
      saveMsg = { kind: 'error', text: res.message || 'Could not update.' };
    }
  }

  async function saveAsNew(event: SubmitEvent) {
    event.preventDefault();
    if (!saveName.trim() || saving || !canSaveCurrent) return;
    saving = true;
    saveMsg = null;
    const res = await api.createAnalysis(
      saveName.trim(),
      saveDescription.trim(),
      currentDeepLink()
    );
    saving = false;
    if (res.ok) {
      saveName = saveDescription = '';
      saveStep = 'closed';
      saveMsg = { kind: 'success', text: 'Saved.' };
      await loadList();
    } else {
      saveMsg = { kind: 'error', text: res.message || 'Could not save.' };
    }
  }

  // --- side drawer ----------------------------------------------------------
  let selectedId = $state<string | null>(null);
  const selected = $derived(items.find((a) => a.id === selectedId) ?? null);
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

  function openDrawer(a: api.AnalysisListItem) {
    // Toggle: clicking the already-open row closes the drawer.
    if (selectedId === a.id) {
      closeDrawer();
      return;
    }
    selectedId = a.id;
    editing = false;
    editName = a.name;
    editDescription = a.description;
    drawerMsg = null;
    shareMsg = null;
    shares = [];
    if (a.owned) void loadShares(a.id);
  }
  function closeDrawer() {
    selectedId = null;
  }

  async function loadShares(id: string) {
    const res = await api.listShares(id);
    if (res.ok) shares = res.data.shares ?? [];
  }

  async function saveEdit() {
    if (!selected || drawerBusy || !editName.trim()) return;
    drawerBusy = true;
    drawerMsg = null;
    const res = await api.updateAnalysis(selected.id, {
      name: editName.trim(),
      description: editDescription.trim()
    });
    drawerBusy = false;
    if (res.ok) {
      editing = false;
      await loadList();
    } else {
      drawerMsg = { kind: 'error', text: res.message || 'Could not save changes.' };
    }
  }

  async function removeAnalysis() {
    if (!selected || drawerBusy) return;
    drawerBusy = true;
    const res = await api.deleteAnalysis(selected.id);
    drawerBusy = false;
    if (res.ok) {
      closeDrawer();
      await loadList();
    } else {
      drawerMsg = { kind: 'error', text: 'Could not delete.' };
    }
  }

  async function addShareSubmit(event: SubmitEvent) {
    event.preventDefault();
    if (!selected || !shareEmail.trim() || shareBusy) return;
    shareBusy = true;
    shareMsg = null;
    const res = await api.addShare(selected.id, shareEmail.trim(), shareCanEdit);
    shareBusy = false;
    if (res.ok) {
      shareEmail = '';
      shareCanEdit = false;
      await loadShares(selected.id);
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
    if (!selected) return;
    if ((await api.removeShare(selected.id, granteeId)).ok) await loadShares(selected.id);
  }

  // --- load saved analysis (navigate to its deep link) ----------------------
  // We tag the destination with `?savedAnalysis=<id>` (stripped from any future
  // saved deep-link) so a later "Save" can offer to update THIS analysis in
  // place rather than only ever creating a new one.
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

  function close() {
    setUrl({ analyses: null });
  }
  function onKeydown(event: KeyboardEvent) {
    if (!isOpen) return;
    if (event.key === 'Escape') {
      if (selectedId) closeDrawer();
      else close();
    }
  }

  function fmt(iso: string): string {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? '—' : d.toLocaleDateString();
  }

  // Fetch the list the first time the overlay opens.
  let loaded = false;
  $effect(() => {
    if (isOpen && !loaded) {
      loaded = true;
      void loadList();
    }
  });

  // Reset the transient save panel + any flash message whenever the overlay is
  // closed, so a later reopen never shows a stale panel or success notice.
  $effect(() => {
    if (!isOpen) {
      saveStep = 'closed';
      saveMsg = null;
    }
  });

  // Auto-dismiss success notices after a few seconds (errors stay until the
  // next action). The effect cleans up its own timer on re-run/teardown.
  $effect(() => {
    if (saveMsg?.kind === 'success') {
      const t = setTimeout(() => {
        saveMsg = null;
      }, 4000);
      return () => clearTimeout(t);
    }
  });

  // Opened directly in save mode (`?analyses=save`, from the Workbench rail) —
  // jump straight into the save flow. Normalise the URL back to `open` so the
  // panel state isn't re-triggered on the next reactive pass.
  $effect(() => {
    if (url.analyses === 'save') {
      setUrl({ analyses: 'open' });
      if (canSaveCurrent) beginSave();
    }
  });
</script>

<svelte:window onkeydown={onKeydown} />

{#if isOpen}
  <div
    class="overlay-backdrop"
    role="presentation"
    onclick={(e) => {
      if (e.target === e.currentTarget) close();
    }}
  >
    <div class="panel-group">
      <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
      <section
        class="overlay-panel"
        role="dialog"
        aria-modal="true"
        aria-label="Saved analyses"
        tabindex="-1"
      >
        <header class="head">
          <h2>Saved analyses</h2>
          <button type="button" class="close" aria-label="Close" onclick={close}>×</button>
        </header>

        <!-- toolbar -->
        <div class="toolbar">
          <input
            class="search"
            type="search"
            placeholder="Search name, description or owner…"
            bind:value={search}
            aria-label="Search saved analyses"
          />
          {#if canSaveCurrent}
            <Button variant="primary" onclick={beginSave}>Save current view</Button>
          {:else}
            <span class="save-hint" title="Open the Workbench and configure an analysis first"
              >Configure an analysis in the Workbench to save it</span
            >
          {/if}
        </div>

        {#if saveMsg}<AuthNotice variant={saveMsg.kind}>{saveMsg.text}</AuthNotice>{/if}

        {#if saveStep === 'open'}
          <div class="save-form">
            {#if showSaveChoice && loadedAnalysis}
              <p class="muted">
                You opened <strong>{loadedAnalysis.name}</strong>. Update it with the current view,
                or save a separate copy?
              </p>
              <div class="row-actions">
                <Button variant="primary" loading={saving} onclick={updateLoaded}
                  >Update “{loadedAnalysis.name}”</Button
                >
                <Button variant="secondary" onclick={beginSaveAsNew}>Save as new</Button>
                <Button variant="secondary" onclick={cancelSave}>Cancel</Button>
              </div>
            {:else}
              <p class="muted">Saves the current Workbench view as a re-openable deep link.</p>
              <form class="save-fields" onsubmit={saveAsNew} novalidate>
                <input class="field" placeholder="Name" bind:value={saveName} aria-label="Name" />
                <input
                  class="field"
                  placeholder="Description (optional)"
                  bind:value={saveDescription}
                  aria-label="Description"
                />
                <div class="row-actions">
                  <Button
                    type="submit"
                    variant="primary"
                    loading={saving}
                    disabled={!saveName.trim()}>Save</Button
                  >
                  <Button variant="secondary" onclick={cancelSave}>Cancel</Button>
                </div>
              </form>
            {/if}
          </div>
        {/if}

        <!-- filters -->
        <div class="filters">
          <fieldset>
            <legend>Show</legend>
            <label><input type="checkbox" bind:checked={showOwned} /> Owned</label>
            <label><input type="checkbox" bind:checked={showShared} /> Shared with me</label>
          </fieldset>
          <fieldset>
            <legend>Permission</legend>
            <label><input type="checkbox" bind:checked={showEditable} /> Editable</label>
            <label><input type="checkbox" bind:checked={showReadable} /> Read-only</label>
          </fieldset>
          <fieldset class="dates">
            <legend>Created</legend>
            <input type="date" bind:value={createdFrom} aria-label="Created from" />
            <span aria-hidden="true">→</span>
            <input type="date" bind:value={createdTo} aria-label="Created to" />
          </fieldset>
        </div>

        <!-- body: table (the drawer is a sibling card, so the table never reflows) -->
        <div class="body">
          <div class="table-wrap">
            {#if loading}
              <p class="muted pad">Loading…</p>
            {:else if loadError}
              <AuthNotice variant="error">{loadError}</AuthNotice>
            {:else if filtered.length === 0}
              <p class="muted pad">
                {items.length === 0
                  ? 'No saved analyses yet. Open the Workbench and use “Save current view”.'
                  : 'No analyses match these filters.'}
              </p>
            {:else}
              <table>
                <thead>
                  <tr>
                    <th
                      ><button type="button" onclick={() => toggleSort('name')}
                        >Name {arrow('name')}</button
                      ></th
                    >
                    <th class="hide-sm">Description</th>
                    <th
                      ><button type="button" onclick={() => toggleSort('ownerEmail')}
                        >Owner {arrow('ownerEmail')}</button
                      ></th
                    >
                    <th
                      ><button type="button" onclick={() => toggleSort('createdAt')}
                        >Created {arrow('createdAt')}</button
                      ></th
                    >
                    <th
                      ><button type="button" onclick={() => toggleSort('updatedAt')}
                        >Updated {arrow('updatedAt')}</button
                      ></th
                    >
                    <th>Access</th>
                  </tr>
                </thead>
                <tbody>
                  {#each filtered as a (a.id)}
                    <tr
                      class:selected={a.id === selectedId}
                      onclick={() => openDrawer(a)}
                      aria-label="Open {a.name}"
                    >
                      <td class="name">{a.name}</td>
                      <td class="hide-sm desc">{a.description || '—'}</td>
                      <td>{a.owned ? 'You' : a.ownerEmail}</td>
                      <td>{fmt(a.createdAt)}</td>
                      <td>{fmt(a.updatedAt)}</td>
                      <td>
                        <span
                          class="badge"
                          class:own={a.owned}
                          class:edit={a.permission === 'editable'}
                        >
                          {a.owned
                            ? 'Owner'
                            : a.permission === 'editable'
                              ? 'Editable'
                              : 'Read-only'}
                        </span>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            {/if}
          </div>
        </div>
      </section>

      <aside class="drawer drawer-card">
        {#if selected}
          <div class="drawer-content" in:fly={{ x: 16, duration: 160 }}>
            <header class="drawer-head">
              <h3>{selected.name}</h3>
              <button type="button" class="close" aria-label="Close details" onclick={closeDrawer}
                >×</button
              >
            </header>

            {#if drawerMsg}<AuthNotice variant={drawerMsg.kind}>{drawerMsg.text}</AuthNotice>{/if}

            <div class="drawer-meta">
              <span>Owner: {selected.owned ? 'You' : selected.ownerEmail}</span>
              <span>Created: {fmt(selected.createdAt)}</span>
              <span>Updated: {fmt(selected.updatedAt)}</span>
            </div>

            <div class="row-actions">
              <Button variant="primary" onclick={() => loadAnalysis(selected.id)}
                >Open in Workbench</Button
              >
            </div>

            {#if selected.permission === 'editable'}
              <section class="drawer-block">
                <div class="block-head">
                  <h4>Details</h4>
                  {#if !editing}
                    <button type="button" class="link" onclick={() => (editing = true)}>Edit</button
                    >
                  {/if}
                </div>
                {#if editing}
                  <input class="field" bind:value={editName} aria-label="Name" />
                  <textarea
                    class="field"
                    rows="2"
                    bind:value={editDescription}
                    aria-label="Description"
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
                        editName = selected.name;
                        editDescription = selected.description;
                      }}>Cancel</Button
                    >
                  </div>
                {:else}
                  <p class="muted">{selected.description || 'No description.'}</p>
                {/if}
              </section>
            {/if}

            {#if selected.owned}
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
                <Button variant="secondary" loading={drawerBusy} onclick={removeAnalysis}
                  >Delete</Button
                >
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
    </div>
  </div>
{/if}

<style>
  .overlay-backdrop {
    position: fixed;
    inset: 0 0 0 var(--rail-width, 184px);
    background: color-mix(in srgb, var(--color-bg) 70%, transparent);
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
    z-index: 40;
    display: grid;
    place-items: center;
    padding: var(--space-5);
  }
  /* The main panel + the detail drawer sit side by side in a centred group.
     The panel has a fixed width so opening the drawer never reflows the table —
     the drawer is an additional card that simply extends the view. */
  /* The panel + drawer form one fixed-size unit. A FIXED height (not max-height)
     on the group means the panel never grows/shrinks when the drawer opens or
     closes — both children stretch to this same height, so the drawer always
     matches the panel and the table just scrolls inside. */
  /* The detail pane is ALWAYS reserved (master-detail), so selecting a row
     never shifts the panel — it just fills the pane. Fixed group width +
     height → one stable two-pane unit. */
  .panel-group {
    display: flex;
    align-items: stretch;
    gap: var(--space-4);
    width: min(92rem, 100%);
    height: min(86vh, 50rem);
    min-width: 0;
  }
  .overlay-panel {
    /* Fills the group minus the reserved detail pane; height is fixed so the
       table just scrolls inside and nothing resizes on drawer toggle. */
    flex: 1 1 auto;
    min-width: 0;
    height: 100%;
    overflow: hidden;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.4);
    padding: var(--space-6);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .head h2 {
    margin: 0;
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
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
  .toolbar {
    display: flex;
    gap: var(--space-3);
    align-items: center;
  }
  .search,
  .field {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-ui);
  }
  .search {
    flex: 1;
  }
  .save-hint {
    flex-shrink: 0;
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    max-width: 16rem;
    line-height: var(--line-height-base);
  }
  .field {
    width: 100%;
  }
  .search:focus-visible,
  .field:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  .save-form,
  .share-form {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
  .save-fields {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
  }
  .filters {
    display: flex;
    gap: var(--space-4);
    flex-wrap: wrap;
  }
  .filters fieldset {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-2) var(--space-3);
    display: flex;
    gap: var(--space-3);
    align-items: center;
    margin: 0;
  }
  .filters legend {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 0 var(--space-1);
  }
  .filters label,
  .inline {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }
  .filters .dates input {
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 2px var(--space-2);
    font-size: var(--font-size-xs);
  }
  .body {
    flex: 1;
    display: flex;
    gap: var(--space-4);
    overflow: hidden;
    min-height: 0;
  }
  .table-wrap {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }
  .pad {
    padding: var(--space-4);
  }
  table {
    width: 100%;
    border-collapse: collapse;
    font-size: var(--font-size-sm);
  }
  thead th {
    position: sticky;
    top: 0;
    background: var(--color-bg-elevated);
    text-align: left;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-medium);
    border-bottom: 1px solid var(--color-border);
    padding: var(--space-2) var(--space-3);
    white-space: nowrap;
  }
  thead th button {
    background: none;
    border: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    padding: 0;
  }
  thead th button:hover {
    color: var(--color-fg);
  }
  tbody tr {
    cursor: pointer;
    border-bottom: 1px solid var(--color-border);
  }
  tbody tr:hover {
    background: var(--color-surface-hover);
  }
  tbody tr.selected {
    background: var(--color-surface);
  }
  td {
    padding: var(--space-3);
    color: var(--color-fg);
    vertical-align: top;
  }
  td.name {
    font-weight: var(--font-weight-medium);
  }
  td.desc {
    color: var(--color-fg-muted);
    max-width: 18rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .badge {
    font-size: var(--font-size-xs);
    padding: 1px var(--space-2);
    border-radius: var(--radius-pill);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    white-space: nowrap;
  }
  .badge.edit {
    color: var(--color-accent);
    border-color: var(--color-accent-muted);
  }
  .badge.own {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  /* The drawer is a standalone card beside the panel (not a flex child of the
     table area), so opening it extends the view without reflowing the list. */
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
  textarea.field {
    resize: vertical;
    font-family: var(--font-ui);
  }
  .row-actions {
    display: flex;
    gap: var(--space-2);
    flex-wrap: wrap;
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
    .hide-sm {
      display: none;
    }
    .drawer {
      display: none;
    }
  }
</style>
