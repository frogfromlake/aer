<script lang="ts">
  // ScopeEditor — Phase 122k K1 rebuild (§10 revision).
  //
  // The Workbench's single configuration surface. Every Panel scope on
  // the Workbench is configured through this modal. The user picks
  // probes, optionally restricts to a discourse function, then picks
  // sources within the selected probes. All combinations of probes ×
  // sources × discourse-function-lock are supported.
  //
  // Mode-aware:
  //   * panel provided → edit mode (snapshot of existing Panel scope)
  //   * panel omitted  → create mode (used by Workbench-page first
  //                      Panel + WindowHost `+Panel`).
  //
  // The editor is re-openable, draft-state internal, never closes on
  // backdrop-click or focus-loss (only Esc / Cancel / Apply close).
  // ScopeGroups are first-class Cards in a stack; multi-group
  // composition is explicit and discoverable.
  import type { Panel, ScopeGroup } from '$lib/state/url-internals';
  import type { ProbeDossierDto, ProbeDto, FetchContext, QueryOutcome } from '$lib/api/queries';
  import { probesQuery, probeDossierQuery } from '$lib/api/queries';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import { loadDraft, saveDraft, clearDraft } from '$lib/workbench/scope-editor-draft';
  import {
    toggleProbeInGroup,
    toggleSourceInGroup,
    selectAllSourcesInGroup,
    clearSourcesInGroup,
    pruneSourcesToLock,
    resolvePanelLock
  } from '$lib/workbench/scope-editor-internals';
  import ScopeGroupCard from './ScopeGroupCard.svelte';
  import { createQuery } from '@tanstack/svelte-query';
  import { onMount, onDestroy } from 'svelte';

  interface Props {
    /** When provided, the editor edits this Panel's scope (edit mode).
     *  When omitted, the editor opens in create mode. */
    panel?: Panel;
    /** Seed dossier for the primary probe. Phase 123c: every other probe a
     *  ScopeGroup selects has its dossier fetched internally (see
     *  `dossierSourcesByProbe`), so cross-probe scopes list all sources. */
    dossier: ProbeDossierDto;
    /** TanStack fetch context for the internal probesQuery. */
    ctx: FetchContext;
    /** Initial probes to seed Group 1 with when opening in create mode.
     *  Defaults to empty — the user must consciously pick. */
    seedProbes?: readonly string[];
    /** Phase 122k §11 — when true, the editor participates in the
     *  one-shot draft persistence: read sessionStorage on mount, save
     *  on Apply. Every other mount path clears the saved draft as a
     *  side effect so a stale draft from a previous Apply never leaks
     *  into a +Panel or Edit-Scope flow. Only the Workbench-page
     *  auto-open passes this flag. */
    enableDraftPersistence?: boolean;
    /** Commits the user's draft scope on Apply. */
    onApply: (scopes: ScopeGroup[], lockedFunction: DiscourseFunction | null) => void;
    /** Discards the draft and closes. */
    onCancel: () => void;
  }

  let {
    panel,
    dossier,
    ctx,
    seedProbes,
    enableDraftPersistence = false,
    onApply,
    onCancel
  }: Props = $props();

  // svelte-ignore state_referenced_locally
  const initialSeed: string[] = seedProbes && seedProbes.length > 0 ? [...seedProbes] : [];
  // svelte-ignore state_referenced_locally
  const isCreateMode = panel === undefined;
  const headerTitle = isCreateMode ? 'Configure new panel' : 'Configure panel scope';
  const applyLabel = isCreateMode ? 'Create panel' : 'Apply changes';

  // Phase 122k §11 — one-shot draft persistence. ONLY honoured when
  // `enableDraftPersistence` is true (Workbench-page auto-open path).
  // Other mounts (`+Panel` / Edit-Scope / edit mode) skip the read and
  // clear the saved draft via the onMount side effect below.
  // svelte-ignore state_referenced_locally
  const restoredDraft = enableDraftPersistence && panel === undefined ? loadDraft() : null;

  function initialDraftScopes(): ScopeGroup[] {
    if (panel) {
      return panel.scopes.map((g) => ({
        probeIds: [...g.probeIds],
        sourceIds: [...g.sourceIds]
      }));
    }
    // Create mode — prefer the explicit seedProbes; fall back to the
    // last Apply draft if and only if persistence was opted into.
    if (initialSeed.length > 0) {
      return [{ probeIds: initialSeed, sourceIds: [] }];
    }
    if (restoredDraft && restoredDraft.scopes.length > 0) {
      return restoredDraft.scopes.map((g) => ({
        probeIds: [...g.probeIds],
        sourceIds: [...g.sourceIds]
      }));
    }
    return [{ probeIds: [], sourceIds: [] }];
  }

  function initialPerGroupLock(scopesLen: number): (DiscourseFunction | null)[] {
    if (panel) return panel.scopes.map(() => null);
    if (restoredDraft && restoredDraft.perGroupLock.length === scopesLen) {
      return [...restoredDraft.perGroupLock];
    }
    return Array.from({ length: scopesLen }, () => null);
  }

  let draftScopes = $state<ScopeGroup[]>(initialDraftScopes());
  // svelte-ignore state_referenced_locally
  let perGroupLock = $state<(DiscourseFunction | null)[]>(initialPerGroupLock(draftScopes.length));

  // Internal probesQuery — fuels the probe-multiselect inside each
  // ScopeGroup. Today: returns just Probe 0; the editor architecture
  // already handles multi-probe selection cleanly.
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);

  function effectiveLockForGroup(groupIndex: number): DiscourseFunction | null {
    return perGroupLock[groupIndex] ?? null;
  }

  // Phase 123c — per-probe dossier source cache. The `dossier` prop seeds the
  // primary probe; every other probe that enters any ScopeGroup has its
  // dossier fetched imperatively into this map so Step-3 lists the sources of
  // ALL selected probes (cross-probe scopes). Keyed by probeId.
  // svelte-ignore state_referenced_locally
  let dossierSourcesByProbe = $state<Record<string, ProbeDossierDto['sources']>>({
    [dossier.probeId]: dossier.sources
  });
  // probeIds whose fetch is in flight, so the effect does not re-issue. Plain
  // object (not reactive) — it is a dedup guard, never rendered.
  const dossierFetchInFlight: Record<string, true> = {};

  // Whenever a probe appears in any draft group and is not yet cached, fetch
  // its dossier. Fail-silent: a failed fetch leaves the probe absent from the
  // map (its source list renders empty with a retry-on-next-open semantics).
  $effect(() => {
    const wanted: string[] = [];
    for (const g of draftScopes)
      for (const p of g.probeIds) if (!wanted.includes(p)) wanted.push(p);
    for (const probeId of wanted) {
      if (probeId in dossierSourcesByProbe || dossierFetchInFlight[probeId]) continue;
      dossierFetchInFlight[probeId] = true;
      const o = probeDossierQuery(ctx, probeId);
      Promise.resolve(o.queryFn())
        .then((outcome) => {
          if (outcome.kind === 'success') {
            dossierSourcesByProbe = {
              ...dossierSourcesByProbe,
              [probeId]: outcome.data.sources
            };
          }
          // Non-success (refusal/network) leaves the probe uncached → empty
          // source list with retry-on-next-open; the editor stays usable.
        })
        .catch(() => {
          // Defensive: queryFn is fail-typed, but never let a rejection throw.
        })
        .finally(() => {
          delete dossierFetchInFlight[probeId];
        });
    }
  });

  // Source-resolver: returns the cached sources for a probe, or [] while its
  // dossier is still loading (the UI shows a transient "loading sources" note).
  function sourcesForProbe(probeId: string): ProbeDossierDto['sources'] {
    return dossierSourcesByProbe[probeId] ?? [];
  }

  function sourcesLoading(probeId: string): boolean {
    return !(probeId in dossierSourcesByProbe);
  }

  // The per-group draft mutations delegate to the pure cores in
  // `scope-editor-internals.ts` (unit-tested); this component only owns the
  // `$state` assignment.
  function toggleProbe(groupIndex: number, probeId: string) {
    draftScopes = toggleProbeInGroup(draftScopes, groupIndex, probeId, sourcesForProbe);
  }

  function toggleSource(groupIndex: number, sourceName: string) {
    draftScopes = toggleSourceInGroup(draftScopes, groupIndex, sourceName);
  }

  function selectAllSourcesForProbe(groupIndex: number, probeId: string) {
    draftScopes = selectAllSourcesInGroup(
      draftScopes,
      groupIndex,
      probeId,
      effectiveLockForGroup(groupIndex),
      sourcesForProbe
    );
  }

  function clearAllSources(groupIndex: number) {
    draftScopes = clearSourcesInGroup(draftScopes, groupIndex);
  }

  function addGroup() {
    draftScopes = [...draftScopes, { probeIds: [], sourceIds: [] }];
    perGroupLock = [...perGroupLock, null];
  }

  function removeGroup(groupIndex: number) {
    if (draftScopes.length <= 1) return;
    draftScopes = draftScopes.filter((_, i) => i !== groupIndex);
    perGroupLock = perGroupLock.filter((_, i) => i !== groupIndex);
  }

  function setGroupLock(groupIndex: number, df: DiscourseFunction | null) {
    perGroupLock = perGroupLock.map((v, i) => (i === groupIndex ? df : v));
    // Prune sources that no longer match the new lock (no-op when df is null).
    if (df) draftScopes = pruneSourcesToLock(draftScopes, groupIndex, df, sourcesForProbe);
  }

  function apply() {
    // Single panel-level lock (today): when ALL groups share the same
    // restriction, surface it as panel.lockedFunction; otherwise leave
    // the panel unlocked. A future Phase 122k.2 lifts the lock into
    // per-ScopeGroup schema so multi-group DF mixes round-trip.
    const resolvedLock = resolvePanelLock(perGroupLock);
    // Phase 122k §11 — only the Workbench-page auto-open path
    // participates in draft persistence. Other Apply paths (+Panel,
    // Edit-Scope) deliberately don't save, so a +Panel apply never
    // leaks a draft into a subsequent back-nav restore.
    if (enableDraftPersistence) {
      saveDraft({ scopes: draftScopes, perGroupLock });
    }
    onApply(draftScopes, resolvedLock);
  }

  function cancel() {
    onCancel();
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      e.preventDefault();
      cancel();
    }
  }

  onMount(() => {
    window.addEventListener('keydown', onKeydown);
    // Phase 122k §11 — one-shot draft consumption. Whatever the
    // restore-eligible path read (or didn't read), clearing here makes
    // sure no stale draft survives this mount. The Apply path will
    // write a fresh draft if the user proceeds.
    clearDraft();
  });
  onDestroy(() => {
    window.removeEventListener('keydown', onKeydown);
  });

  // Validation: Apply is enabled when EVERY group has at least one
  // probe selected (otherwise the panel would have no scope at all).
  const canApply = $derived(draftScopes.every((g) => g.probeIds.length > 0));
</script>

<!--
  The backdrop does NOT close on click — only Esc / Cancel / Apply close
  the editor. role="presentation" + no onclick handler keeps the
  backdrop purely visual.
-->
<div class="scope-editor-backdrop" role="presentation">
  <!-- svelte-ignore a11y_no_noninteractive_element_to_interactive_role -->
  <section
    class="scope-editor"
    role="dialog"
    aria-modal="true"
    aria-label="Configure panel scope"
    tabindex="-1"
  >
    <header class="editor-header">
      <div class="header-titles">
        <p class="header-eyebrow">Workbench · ScopeEditor</p>
        <h2>{headerTitle}</h2>
        <p class="header-hint">
          Configure the corpus this Panel analyses. Pick probes, optionally restrict to a discourse
          function, then choose sources. Add scope groups to compare scopes side by side.
        </p>
      </div>
      <button type="button" class="close-btn" onclick={cancel} aria-label="Cancel and close">
        ×
      </button>
    </header>

    <div class="groups">
      {#each draftScopes as group, groupIndex (groupIndex)}
        <ScopeGroupCard
          {group}
          {groupIndex}
          lock={effectiveLockForGroup(groupIndex)}
          canRemove={draftScopes.length > 1}
          {probeList}
          probesPending={probesQ.isPending}
          {sourcesForProbe}
          {sourcesLoading}
          onToggleProbe={(probeId) => toggleProbe(groupIndex, probeId)}
          onToggleSource={(sourceName) => toggleSource(groupIndex, sourceName)}
          onSetLock={(df) => setGroupLock(groupIndex, df)}
          onSelectAll={(probeId) => selectAllSourcesForProbe(groupIndex, probeId)}
          onClearAll={() => clearAllSources(groupIndex)}
          onRemove={() => removeGroup(groupIndex)}
        />
      {/each}
    </div>

    <footer class="editor-footer">
      <button
        type="button"
        class="add-group-btn"
        onclick={addGroup}
        title="Add a parallel scope group for side-by-side comparison"
      >
        ＋ Add scope group
      </button>
      <div class="footer-spacer"></div>
      <button type="button" class="cancel-btn" onclick={cancel}>Cancel</button>
      <button
        type="button"
        class="apply-btn"
        onclick={apply}
        disabled={!canApply}
        title={canApply ? '' : 'Each scope group needs at least one probe.'}
      >
        {applyLabel}
      </button>
    </footer>
  </section>
</div>

<style>
  /* ---------- Modal shell ---------- */
  .scope-editor-backdrop {
    position: fixed;
    inset: 0;
    background: color-mix(in srgb, var(--color-bg) 78%, transparent);
    backdrop-filter: blur(3px);
    z-index: 50;
    display: grid;
    place-items: center;
    padding: var(--space-4);
  }

  .scope-editor {
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    /* §10 finding — generous canvas. ~50% larger than before. */
    width: min(84rem, 100%);
    max-height: 92vh;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    padding: var(--space-6);
    box-shadow:
      0 20px 60px rgba(0, 0, 0, 0.4),
      0 0 0 1px color-mix(in srgb, var(--color-accent) 30%, transparent);
  }

  /* ---------- Header ---------- */
  .editor-header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: var(--space-4);
    border-bottom: 1px solid var(--color-border);
    padding-bottom: var(--space-4);
  }

  .header-eyebrow {
    margin: 0 0 var(--space-1) 0;
    font-family: var(--font-mono);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.12em;
    color: var(--color-accent);
  }

  .header-titles h2 {
    margin: 0 0 var(--space-2) 0;
    font-size: var(--font-size-2xl);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    line-height: 1.15;
  }

  .header-hint {
    margin: 0;
    color: var(--color-fg-muted);
    font-size: var(--font-size-md);
    line-height: 1.55;
    max-width: 56rem;
  }

  .close-btn {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    color: var(--color-fg-muted);
    width: 2.5rem;
    height: 2.5rem;
    font-size: 1.5rem;
    cursor: pointer;
    flex-shrink: 0;
    transition:
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  /* ---------- Groups ---------- */
  .groups {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
  }

  /* ---------- Footer ---------- */
  .editor-footer {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    border-top: 1px solid var(--color-border);
    padding-top: var(--space-4);
  }

  .footer-spacer {
    flex: 1 1 auto;
  }

  .add-group-btn,
  .cancel-btn,
  .apply-btn {
    appearance: none;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-3) var(--space-4);
    font-size: var(--font-size-md);
    cursor: pointer;
  }

  .add-group-btn {
    background: transparent;
    border-style: dashed;
    border-color: var(--color-border-strong);
    color: var(--color-fg-muted);
  }
  .add-group-btn:hover,
  .add-group-btn:focus-visible {
    color: var(--color-fg);
    border-color: #a3c984;
    background: color-mix(in srgb, #a3c984 6%, transparent);
  }

  .cancel-btn {
    background: transparent;
    color: var(--color-fg-muted);
  }
  .cancel-btn:hover,
  .cancel-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .apply-btn {
    background: var(--color-accent);
    color: var(--color-bg);
    border-color: var(--color-accent);
    font-weight: 700;
    min-width: 9rem;
  }
  .apply-btn:hover:not(:disabled),
  .apply-btn:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, var(--color-fg));
  }
  .apply-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
