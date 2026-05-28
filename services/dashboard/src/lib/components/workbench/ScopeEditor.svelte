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
  import { probesQuery } from '$lib/api/queries';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import { loadDraft, saveDraft, clearDraft } from '$lib/workbench/scope-editor-draft';
  import { createQuery } from '@tanstack/svelte-query';
  import { onMount, onDestroy } from 'svelte';

  interface Props {
    /** When provided, the editor edits this Panel's scope (edit mode).
     *  When omitted, the editor opens in create mode. */
    panel?: Panel;
    /** Primary dossier — the source list for the production probe.
     *  Multi-probe support: when Phase 123 lands additional probes the
     *  editor will fetch their dossiers internally; today only the
     *  primary dossier is reachable, and non-primary probes surface a
     *  "Phase 123 pending" hint. */
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

  const DISCOURSE_FUNCTIONS: ReadonlyArray<{
    id: DiscourseFunction;
    label: string;
    color: string;
  }> = [
    { id: 'epistemic_authority', label: 'Epistemic Authority', color: '#7dc7e5' },
    { id: 'power_legitimation', label: 'Power Legitimation', color: '#e8a25c' },
    { id: 'cohesion_identity', label: 'Cohesion & Identity', color: '#a3c984' },
    { id: 'subversion_friction', label: 'Subversion & Friction', color: '#d97a7a' }
  ];

  function effectiveLockForGroup(groupIndex: number): DiscourseFunction | null {
    return perGroupLock[groupIndex] ?? null;
  }

  function sourceMatchesDf(
    source: ProbeDossierDto['sources'][number],
    df: DiscourseFunction | null
  ): boolean {
    if (df === null) return true;
    // Phase 122k §11 finding — DF-lock matches on PRIMARY only. The
    // secondary-function tag is a softer signal (Bundesregierung's
    // secondary EA exists because policy reads as authoritative, but the
    // source's structural role is PL). Locking on EA should NOT auto-
    // include PL-primary sources just because they carry a secondary EA.
    return source.primaryFunction === df;
  }

  // Source-resolver: today only the primary dossier carries actual
  // source data. For multi-probe, this returns a placeholder until
  // Phase 123 wires per-probe dossier loading.
  function sourcesForProbe(probeId: string): ProbeDossierDto['sources'] {
    if (probeId === dossier.probeId) return dossier.sources;
    return [];
  }

  function toggleProbe(groupIndex: number, probeId: string) {
    const group = draftScopes[groupIndex];
    if (!group) return;
    const probeIds = group.probeIds.includes(probeId)
      ? group.probeIds.filter((p) => p !== probeId)
      : [...group.probeIds, probeId];
    // Source-IDs orphaned by probe deselection are dropped automatically:
    // sources from a probe that's no longer in the group are filtered out.
    const remainingProbes = new Set(probeIds);
    const liveSourceIds = group.sourceIds.filter((sid) => {
      for (const pid of remainingProbes) {
        if (sourcesForProbe(pid).some((s) => s.name === sid)) return true;
      }
      return false;
    });
    draftScopes = draftScopes.map((g, i) =>
      i === groupIndex ? { probeIds, sourceIds: liveSourceIds } : g
    );
  }

  function toggleSource(groupIndex: number, sourceName: string) {
    const group = draftScopes[groupIndex];
    if (!group) return;
    const sourceIds = group.sourceIds.includes(sourceName)
      ? group.sourceIds.filter((s) => s !== sourceName)
      : [...group.sourceIds, sourceName];
    draftScopes = draftScopes.map((g, i) =>
      i === groupIndex ? { probeIds: [...group.probeIds], sourceIds } : g
    );
  }

  function selectAllSourcesForProbe(groupIndex: number, probeId: string) {
    const group = draftScopes[groupIndex];
    if (!group) return;
    const lock = effectiveLockForGroup(groupIndex);
    const matching = sourcesForProbe(probeId)
      .filter((s) => sourceMatchesDf(s, lock))
      .map((s) => s.name);
    const otherProbeSources = group.sourceIds.filter(
      (sid) => !sourcesForProbe(probeId).some((s) => s.name === sid)
    );
    draftScopes = draftScopes.map((g, i) =>
      i === groupIndex
        ? { probeIds: [...group.probeIds], sourceIds: [...otherProbeSources, ...matching] }
        : g
    );
  }

  function clearAllSources(groupIndex: number) {
    const group = draftScopes[groupIndex];
    if (!group) return;
    draftScopes = draftScopes.map((g, i) =>
      i === groupIndex ? { probeIds: [...group.probeIds], sourceIds: [] } : g
    );
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
    const group = draftScopes[groupIndex];
    if (group && df) {
      // Prune sources that no longer match the new lock.
      const filtered = group.sourceIds.filter((name) => {
        for (const pid of group.probeIds) {
          const src = sourcesForProbe(pid).find((s) => s.name === name);
          if (src && sourceMatchesDf(src, df)) return true;
        }
        return false;
      });
      if (filtered.length !== group.sourceIds.length) {
        draftScopes = draftScopes.map((g, i) =>
          i === groupIndex ? { probeIds: [...group.probeIds], sourceIds: filtered } : g
        );
      }
    }
  }

  function apply() {
    // Single panel-level lock (today): when ALL groups share the same
    // restriction, surface it as panel.lockedFunction; otherwise leave
    // the panel unlocked. A future Phase 122k.2 lifts the lock into
    // per-ScopeGroup schema so multi-group DF mixes round-trip.
    const uniqueLocks = new Set(perGroupLock);
    const resolvedLock =
      uniqueLocks.size === 1
        ? ((Array.from(uniqueLocks)[0] as DiscourseFunction | null | undefined) ?? null)
        : null;
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
        {@const lock = effectiveLockForGroup(groupIndex)}
        {@const lockMeta = lock ? DISCOURSE_FUNCTIONS.find((d) => d.id === lock) : null}
        <article
          class="group"
          aria-label="Scope group {groupIndex + 1}"
          style:--lock-color={lockMeta?.color ?? 'var(--color-accent)'}
        >
          <header class="group-header">
            <div class="group-title-line">
              <span class="group-eyebrow">Group {groupIndex + 1}</span>
              <span class="group-summary">
                {group.probeIds.length} probe{group.probeIds.length === 1 ? '' : 's'} ·
                {group.sourceIds.length} source{group.sourceIds.length === 1 ? '' : 's'}
                {#if lockMeta}· locked to <strong>{lockMeta.label}</strong>{/if}
              </span>
              {#if draftScopes.length > 1}
                <button
                  type="button"
                  class="group-remove-btn"
                  onclick={() => removeGroup(groupIndex)}
                  aria-label="Remove this scope group"
                  title="Remove this scope group"
                >
                  × Remove group
                </button>
              {/if}
            </div>
          </header>

          <!-- 1. Probes -->
          <section class="step" data-step="1">
            <header class="step-header">
              <span class="step-num" aria-hidden="true">1</span>
              <h3 class="step-title">Probes</h3>
              <span class="step-hint">Pick one or more probes for this scope group.</span>
            </header>
            <div class="probe-grid">
              {#if probesQ.isPending}
                <p class="muted" aria-busy="true">Loading probes…</p>
              {:else if probeList.length === 0}
                <p class="muted">No probes available.</p>
              {:else}
                {#each probeList as probe (probe.probeId)}
                  {@const checked = group.probeIds.includes(probe.probeId)}
                  {@const isPrimary = probe.probeId === dossier.probeId}
                  <label class="probe-chip" class:checked>
                    <input
                      type="checkbox"
                      {checked}
                      onchange={() => toggleProbe(groupIndex, probe.probeId)}
                      aria-label="Include {probe.probeId}"
                    />
                    <span class="probe-name">{probe.probeId}</span>
                    <span class="probe-lang">{probe.language.toUpperCase()}</span>
                    {#if !isPrimary}
                      <span class="probe-hint" title="Multi-probe source loading pends Phase 123">
                        Phase 123
                      </span>
                    {/if}
                  </label>
                {/each}
              {/if}
            </div>
          </section>

          <!-- 2. DF restriction -->
          <section class="step" data-step="2">
            <header class="step-header">
              <span class="step-num" aria-hidden="true">2</span>
              <h3 class="step-title">Discourse function</h3>
              <span class="step-hint"
                >Optional. Dim sources that don't carry the chosen function.</span
              >
            </header>
            <div class="df-row">
              <button
                type="button"
                class="df-chip df-chip-none"
                class:active={lock === null}
                onclick={() => setGroupLock(groupIndex, null)}
              >
                None — all functions
              </button>
              {#each DISCOURSE_FUNCTIONS as df (df.id)}
                <button
                  type="button"
                  class="df-chip"
                  class:active={lock === df.id}
                  style:--chip-color={df.color}
                  onclick={() => setGroupLock(groupIndex, df.id)}
                >
                  {df.label}
                </button>
              {/each}
            </div>
          </section>

          <!-- 3. Sources -->
          <section class="step" data-step="3">
            <header class="step-header">
              <span class="step-num" aria-hidden="true">3</span>
              <h3 class="step-title">Sources</h3>
              <span class="step-hint">
                Within the selected probes. Leave empty to include the whole probe.
              </span>
            </header>
            {#if group.probeIds.length === 0}
              <p class="muted-large">Pick at least one probe in step 1 to see its sources here.</p>
            {:else}
              {#each group.probeIds as probeId (probeId)}
                {@const probeSources = sourcesForProbe(probeId)}
                <div class="source-section">
                  <header class="source-section-header">
                    <span class="probe-section-label">{probeId}</span>
                    {#if probeSources.length > 0}
                      <span class="source-actions">
                        <button
                          type="button"
                          class="source-action"
                          onclick={() => selectAllSourcesForProbe(groupIndex, probeId)}
                          title="Include all matching sources of this probe"
                        >
                          Select all
                        </button>
                        <button
                          type="button"
                          class="source-action"
                          onclick={() => clearAllSources(groupIndex)}
                          title="Whole-probe scope (no source narrowing)"
                        >
                          Clear all
                        </button>
                      </span>
                    {/if}
                  </header>
                  {#if probeSources.length === 0}
                    <p class="muted">
                      Source list for <code>{probeId}</code> is not yet wired (Phase 123 will load multi-probe
                      dossiers in the editor).
                    </p>
                  {:else}
                    <ul class="source-list" role="list">
                      {#each probeSources as source (source.name)}
                        {@const checked = group.sourceIds.includes(source.name)}
                        {@const dim = !sourceMatchesDf(source, lock)}
                        <li>
                          <label class="source-row" class:checked class:dim>
                            <input
                              type="checkbox"
                              {checked}
                              disabled={dim}
                              onchange={() => toggleSource(groupIndex, source.name)}
                              aria-label="Include {source.emicDesignation ?? source.name}"
                            />
                            <span class="source-label">
                              <span class="source-name">
                                {source.emicDesignation ?? source.name}
                              </span>
                              {#if source.emicDesignation && source.emicDesignation !== source.name}
                                <code class="source-id">{source.name}</code>
                              {/if}
                              {#if source.primaryFunction}
                                {@const fnMeta = DISCOURSE_FUNCTIONS.find(
                                  (d) => d.id === source.primaryFunction
                                )}
                                <span
                                  class="source-df-tag"
                                  style:--tag-color={fnMeta?.color ?? 'var(--color-fg-subtle)'}
                                  title="Primary discourse function (provisional, source-level; per-article Phase 122a deferred)"
                                >
                                  {(source.primaryFunction ?? '').replace('_', ' ')}
                                </span>
                              {/if}
                            </span>
                            {#if dim}
                              <span class="source-dim-hint">not matching {lockMeta?.label}</span>
                            {/if}
                          </label>
                        </li>
                      {/each}
                    </ul>
                  {/if}
                </div>
              {/each}
            {/if}
          </section>
        </article>
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

  .group {
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4) var(--space-5);
    background: var(--color-surface);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
    box-shadow:
      0 1px 2px rgba(0, 0, 0, 0.05),
      inset 3px 0 0 var(--lock-color);
  }

  .group-header {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
  }

  .group-title-line {
    display: flex;
    align-items: baseline;
    gap: var(--space-3);
    flex-wrap: wrap;
  }

  .group-eyebrow {
    text-transform: uppercase;
    font-size: 11px;
    letter-spacing: 0.1em;
    color: var(--color-fg-subtle);
    font-family: var(--font-mono);
  }

  .group-summary {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
  }

  .group-remove-btn {
    margin-left: auto;
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 4px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .group-remove-btn:hover,
  .group-remove-btn:focus-visible {
    color: #d97a7a;
    border-color: #d97a7a;
  }

  /* ---------- Steps (numbered sections) ---------- */
  .step {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding: var(--space-3) var(--space-4);
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }
  /* Phase 122k §10 finding — accent dimmed and pulled to a single
     consistent border-left tint. The dimmed accents read as visual
     organisation without competing with the DF chips and source rows. */
  .step[data-step='1'] {
    border-left: 2px solid color-mix(in srgb, #7dc7e5 50%, var(--color-border));
  }
  .step[data-step='2'] {
    border-left: 2px solid color-mix(in srgb, #e8a25c 50%, var(--color-border));
  }
  .step[data-step='3'] {
    border-left: 2px solid color-mix(in srgb, #a3c984 50%, var(--color-border));
  }

  .step-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .step-num {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 1.5rem;
    height: 1.5rem;
    border-radius: 999px;
    background: var(--color-bg-elevated);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-weight: 700;
    font-size: var(--font-size-sm);
    border: 1px solid var(--color-border);
    flex-shrink: 0;
  }
  /* Number badges stay neutral — the dim border-left does the
     section-tinting work. */

  .step-title {
    margin: 0;
    font-size: var(--font-size-lg);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
  }

  .step-hint {
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  /* ---------- Step 1 — probe grid ---------- */
  .probe-grid {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .probe-chip {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    cursor: pointer;
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .probe-chip:hover {
    border-color: #7dc7e5;
  }
  .probe-chip.checked {
    background: color-mix(in srgb, #7dc7e5 18%, var(--color-bg-elevated));
    border-color: #7dc7e5;
  }
  .probe-chip input {
    accent-color: #7dc7e5;
  }

  .probe-name {
    font-family: var(--font-mono);
  }

  .probe-lang {
    font-family: var(--font-mono);
    font-size: 10.5px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 5px;
    background: var(--color-bg);
    border-radius: var(--radius-sm);
    color: var(--color-fg-subtle);
  }

  .probe-hint {
    font-size: 10px;
    color: var(--color-fg-subtle);
    font-style: italic;
    cursor: help;
  }

  /* ---------- Step 2 — DF chips ---------- */
  .df-row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
  }

  .df-chip {
    appearance: none;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    padding: var(--space-2) var(--space-3);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .df-chip:hover,
  .df-chip:focus-visible {
    color: var(--color-fg);
    border-color: var(--chip-color, var(--color-border-strong));
  }
  .df-chip.active {
    background: color-mix(
      in srgb,
      var(--chip-color, var(--color-accent)) 20%,
      var(--color-bg-elevated)
    );
    border-color: var(--chip-color, var(--color-accent));
    color: var(--color-fg);
    font-weight: 600;
  }
  .df-chip-none.active {
    background: color-mix(in srgb, var(--color-fg-subtle) 12%, var(--color-bg-elevated));
    border-color: var(--color-fg-subtle);
  }

  /* ---------- Step 3 — Sources ---------- */
  .source-section {
    display: flex;
    flex-direction: column;
    gap: var(--space-2);
    padding-top: var(--space-2);
  }
  .source-section + .source-section {
    border-top: 1px dashed var(--color-border);
    margin-top: var(--space-2);
    padding-top: var(--space-3);
  }

  .source-section-header {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
  }

  .probe-section-label {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
    background: color-mix(in srgb, #a3c984 14%, var(--color-bg-elevated));
    border: 1px solid color-mix(in srgb, #a3c984 40%, transparent);
    padding: 2px var(--space-2);
    border-radius: var(--radius-sm);
  }

  .source-actions {
    margin-left: auto;
    display: flex;
    gap: var(--space-2);
  }

  .source-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  .source-action:hover,
  .source-action:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }

  .source-list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .source-row {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-3);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition: background-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .source-row:hover {
    background: var(--color-bg-elevated);
  }
  .source-row.checked {
    background: color-mix(in srgb, #a3c984 12%, var(--color-surface));
  }
  .source-row.dim {
    opacity: 0.45;
    cursor: not-allowed;
  }
  .source-row.dim:hover {
    background: transparent;
  }
  .source-row input {
    accent-color: #a3c984;
  }

  .source-label {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
    flex-wrap: wrap;
  }

  .source-name {
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .source-id {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
  }

  .source-df-tag {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    padding: 1px 6px;
    border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--tag-color, var(--color-fg-subtle)) 14%, transparent);
    color: var(--tag-color, var(--color-fg-subtle));
    border: 1px solid color-mix(in srgb, var(--tag-color, var(--color-border)) 40%, transparent);
  }

  .source-dim-hint {
    font-size: var(--font-size-xs);
    color: var(--color-fg-subtle);
    font-style: italic;
  }

  .muted {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-subtle);
  }
  .muted-large {
    margin: 0;
    font-size: var(--font-size-md);
    color: var(--color-fg-subtle);
    font-style: italic;
    padding: var(--space-2) 0;
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
