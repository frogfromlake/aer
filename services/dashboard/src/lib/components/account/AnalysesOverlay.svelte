<script lang="ts">
  // Saved analyses as a global overlay (Phase 135 / ADR-040). Same model as the
  // Dossier / account overlays: a dimmed scrim over the persistent globe + a
  // solid panel, driven by `?analyses=open`.
  //
  // A saved analysis IS a Workbench deep link — the full Workbench state already
  // round-trips through the URL grammar (`?activePillar=&aleph=/episteme=/…`),
  // so we persist the current relative URL (path + search minus overlay params)
  // and restore it with a plain navigation. No bespoke (de)serialisation.
  //
  // Phase 141 — this component is now a thin orchestrator: it owns the list +
  // save `$state` and the API calls, and delegates the two big rendered regions
  // to children (the sortable list → AnalysisTable, the detail/share/delete pane
  // → AnalysisDrawer) and the client-side filter/sort/deep-link logic to the
  // tested `analyses-overlay-internals` module.
  import * as api from '$lib/api/analyses';
  import { urlState, setUrl } from '$lib/state/url.svelte';
  import { setCleanBaseline } from '$lib/workbench/dirty.svelte';
  import AuthNotice from '$lib/components/auth/AuthNotice.svelte';
  import { m } from '$lib/paraglide/messages.js';
  import AnalysisTable from './AnalysisTable.svelte';
  import AnalysisDrawer from './AnalysisDrawer.svelte';
  import AnalysisSavePanel from './AnalysisSavePanel.svelte';
  import {
    filterAnalyses,
    sortAnalyses,
    nextSort,
    stripDeepLink,
    isSaveableWorkbenchUrl,
    findEditableLoaded,
    NON_STATE_PARAMS,
    type SortKey,
    type SortDir
  } from './analyses-overlay-internals';

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

  let sortKey = $state<SortKey>('updatedAt');
  let sortDir = $state<SortDir>('desc');

  function toggleSort(key: SortKey) {
    const n = nextSort({ key: sortKey, dir: sortDir }, key);
    sortKey = n.key;
    sortDir = n.dir;
  }

  const filtered = $derived(
    sortAnalyses(
      filterAnalyses(items, {
        search,
        showOwned,
        showShared,
        showEditable,
        showReadable,
        createdFrom,
        createdTo
      }),
      sortKey,
      sortDir
    )
  );

  // --- load list ------------------------------------------------------------
  async function loadList() {
    loading = true;
    loadError = null;
    const res = await api.listAnalyses();
    loading = false;
    if (res.ok) items = res.data.analyses ?? [];
    else loadError = m.account_analyses_load_failed();
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

  const currentDeepLink = () => stripDeepLink(window.location.href, NON_STATE_PARAMS);

  // "Save current view" only makes sense on a *configured* Workbench. Re-evaluates
  // whenever the URL state changes (opening the overlay toggles it).
  const canSaveCurrent = $derived.by(() => {
    void url; // track url-state so this re-runs when the overlay opens
    if (typeof window === 'undefined') return false;
    return isSaveableWorkbenchUrl(window.location.href);
  });

  // The saved analysis the current view was opened from (set by "Open in
  // Workbench"). If it's still ours-and-editable, "Save" can update it in place.
  const loadedAnalysis = $derived(findEditableLoaded(items, url.savedAnalysis));

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
      // Phase 127 — the saved state is now the clean leave-guard baseline.
      setCleanBaseline(currentDeepLink());
      saveStep = 'closed';
      saveMsg = {
        kind: 'success',
        text: m.account_analyses_updated_notice({ name: loadedAnalysis.name })
      };
      await loadList();
    } else {
      saveMsg = { kind: 'error', text: res.message || m.account_analyses_update_failed() };
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
      // Phase 127 — the freshly-saved state is now the clean leave-guard baseline.
      setCleanBaseline(currentDeepLink());
      saveName = saveDescription = '';
      saveStep = 'closed';
      saveMsg = { kind: 'success', text: m.account_analyses_saved_notice() };
      await loadList();
    } else {
      saveMsg = { kind: 'error', text: res.message || m.account_analyses_save_failed() };
    }
  }

  // --- detail drawer (AnalysisDrawer owns its own edit/share/delete state) ---
  let selectedId = $state<string | null>(null);
  const selected = $derived(items.find((a) => a.id === selectedId) ?? null);

  // Toggle: clicking the already-open row closes the drawer.
  function onOpenRow(a: api.AnalysisListItem) {
    selectedId = selectedId === a.id ? null : a.id;
  }
  function closeDrawer() {
    selectedId = null;
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
        aria-label={m.account_analyses_title()}
        tabindex="-1"
      >
        <header class="head">
          <h2>{m.account_analyses_title()}</h2>
          <button type="button" class="close" aria-label={m.common_close()} onclick={close}
            >×</button
          >
        </header>

        <!-- toolbar — search only. Saving is initiated from the Workbench's
             "Save current view" action (which opens this overlay in save mode);
             a second save trigger here was redundant + confusing (operator). -->
        <div class="toolbar">
          <input
            class="search"
            type="search"
            placeholder={m.account_analyses_search_placeholder()}
            bind:value={search}
            aria-label={m.account_analyses_search_label()}
          />
        </div>

        <!-- Errors stay inline, anchored to the action; success is a floating
             toast (below) so it never reflows the table. -->
        {#if saveMsg?.kind === 'error'}
          <AuthNotice variant="error">{saveMsg.text}</AuthNotice>
        {/if}

        {#if saveStep === 'open'}
          <AnalysisSavePanel
            showChoice={showSaveChoice}
            {loadedAnalysis}
            bind:saveName
            bind:saveDescription
            {saving}
            onUpdate={updateLoaded}
            onSaveAsNewChoice={beginSaveAsNew}
            onSubmit={saveAsNew}
            onCancel={cancelSave}
          />
        {/if}

        <!-- filters -->
        <div class="filters">
          <fieldset>
            <legend>{m.account_analyses_filter_show()}</legend>
            <label
              ><input type="checkbox" bind:checked={showOwned} />
              {m.account_analyses_filter_owned()}</label
            >
            <label
              ><input type="checkbox" bind:checked={showShared} />
              {m.account_analyses_filter_shared()}</label
            >
          </fieldset>
          <fieldset>
            <legend>{m.account_analyses_filter_permission()}</legend>
            <label
              ><input type="checkbox" bind:checked={showEditable} />
              {m.account_analyses_filter_editable()}</label
            >
            <label
              ><input type="checkbox" bind:checked={showReadable} />
              {m.account_analyses_filter_readonly()}</label
            >
          </fieldset>
          <fieldset class="dates">
            <legend>{m.account_analyses_filter_created()}</legend>
            <input
              type="date"
              bind:value={createdFrom}
              aria-label={m.account_analyses_filter_created_from()}
            />
            <span aria-hidden="true">→</span>
            <input
              type="date"
              bind:value={createdTo}
              aria-label={m.account_analyses_filter_created_to()}
            />
          </fieldset>
        </div>

        <!-- body: list (the drawer is a sibling card, so the list never reflows) -->
        <div class="body">
          <AnalysisTable
            rows={filtered}
            {loading}
            {loadError}
            totalCount={items.length}
            {selectedId}
            {sortKey}
            {sortDir}
            onToggleSort={toggleSort}
            {onOpenRow}
          />
        </div>

        {#if saveMsg?.kind === 'success'}
          <div class="save-toast" role="status" aria-live="polite">{saveMsg.text}</div>
        {/if}
      </section>

      <AnalysisDrawer analysis={selected} onClose={closeDrawer} onChanged={loadList} />
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
  /* The panel + drawer form one fixed-size unit. A FIXED height (not max-height)
     on the group means the panel never grows/shrinks when the drawer opens or
     closes — both children stretch to this same height, so the drawer always
     matches the panel and the table just scrolls inside. The detail pane is
     ALWAYS reserved (master-detail), so selecting a row never shifts the panel. */
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
    /* Anchor for the floating success toast. */
    position: relative;
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
  .search {
    flex: 1;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-ui);
  }
  .search:focus-visible {
    outline: none;
    border-color: var(--color-accent);
    box-shadow: 0 0 0 var(--focus-ring-width)
      color-mix(in oklab, var(--color-accent) 40%, transparent);
  }
  /* Floating success toast — bottom-centre inside the panel, out of layout flow
     so appearing/dismissing never reflows the table (operator finding). */
  .save-toast {
    position: absolute;
    bottom: var(--space-4);
    left: 50%;
    transform: translateX(-50%);
    z-index: 5;
    max-width: calc(100% - 2 * var(--space-6));
    padding: var(--space-2) var(--space-4);
    background: var(--color-bg-elevated);
    border: 1px solid color-mix(in srgb, var(--color-status-validated) 55%, var(--color-border));
    border-radius: var(--radius-md);
    box-shadow: var(--elevation-2);
    color: var(--color-status-validated);
    font-size: var(--font-size-sm);
    text-align: center;
    animation: save-toast-in var(--motion-duration-fast) var(--motion-ease-standard);
  }
  @keyframes save-toast-in {
    from {
      opacity: 0;
      transform: translate(-50%, var(--space-2));
    }
    to {
      opacity: 1;
      transform: translate(-50%, 0);
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .save-toast {
      animation: none;
    }
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
  .filters label {
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
</style>
