<script lang="ts">
  // Workbench — Phase 122k.
  //
  // AĒR's analytical surface. On empty pillar state the page auto-opens
  // the ScopeEditor in create-mode (F2), seeded from `url.selectedProbes`
  // when the user arrived via the Atmos SHIFT-click flow or the Probe-
  // Filter Modal. Apply seeds the pillar state with a new Panel; Cancel
  // leaves the user on a minimal empty-state placeholder with a re-open
  // affordance.
  import { createQuery } from '@tanstack/svelte-query';
  import { beforeNavigate } from '$app/navigation';
  import { pushUrl, urlState } from '$lib/state/url.svelte';
  import { defaultViewModeForPillar, getPillar } from '$lib/viewmodes';
  import { clearDraft } from '$lib/workbench/scope-editor-draft';
  import {
    probeDossierQuery,
    probesQuery,
    type FetchContext,
    type ProbeDossierDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    DEFAULT_LOOKBACK_MS,
    type PillarState,
    type ScopeGroup,
    type ViewingMode,
    type WorkbenchPillarsState
  } from '$lib/state/url-internals';
  import { buildPanelFromScopes } from '$lib/workbench/panel-queries';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import PillarSwitch from '$lib/components/chrome/PillarSwitch.svelte';
  // Phase 122k §14b finding 2 — global WorkbenchScopeBar retired; the
  // per-panel `PanelMetaStrip` surfaces scope info inside each panel.
  import ScopeEditor from '$lib/components/workbench/ScopeEditor.svelte';
  import AlephShell from '$lib/components/workbench/AlephShell.svelte';
  import EpistemeShell from '$lib/components/workbench/EpistemeShell.svelte';
  import RhizomeShell from '$lib/components/workbench/RhizomeShell.svelte';

  const ctx: FetchContext = { baseUrl: '/api/v1' };
  const url = $derived(urlState());

  const activePillar = $derived(getPillar(url.activePillar));
  const pillarHasState = $derived(
    url.pillars
      ? Boolean(url.pillars[activePillar.id]) &&
          (url.pillars[activePillar.id]?.windows.length ?? 0) > 0
      : false
  );
  const hasScope = $derived(pillarHasState);

  // Phase 122k F2 — auto-open ScopeEditor when no pillar state exists.
  // `editorDismissed` becomes true after the user explicitly cancels the
  // first-open; the empty-state placeholder then exposes a "Configure
  // scope" button to re-open. Apply seeds the pillar state and resets
  // the dismissed flag.
  let editorDismissed = $state(false);
  const showCreateEditor = $derived(!hasScope && !editorDismissed);

  // Load the default probe + dossier for the ScopeEditor's source list.
  // For Probe-0-only production this is deterministic; when Probe 1 lands
  // the editor's probe picker (a K1.2+ feature) will let the user choose
  // a different primary probe.
  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });
  const probeList = $derived<ProbeDto[]>(probesQ.data?.kind === 'success' ? probesQ.data.data : []);
  // Prefer a probe from the selection; fall back to the first known probe.
  const seedProbeId = $derived.by<string>(() => {
    const first = url.selectedProbes[0];
    if (first) return first;
    return probeList[0]?.probeId ?? '';
  });

  const windowMs = $derived.by(() => {
    const now = Date.now();
    const fromMs = url.from ? Date.parse(url.from) : now - DEFAULT_LOOKBACK_MS;
    const toMs = url.to ? Date.parse(url.to) : now;
    return {
      start: new Date(Number.isFinite(fromMs) ? fromMs : now - DEFAULT_LOOKBACK_MS).toISOString(),
      end: new Date(Number.isFinite(toMs) ? toMs : now).toISOString()
    };
  });

  const dossierQ = createQuery<QueryOutcome<ProbeDossierDto>, Error, QueryOutcome<ProbeDossierDto>>(
    () => {
      const o = probeDossierQuery(ctx, seedProbeId, {
        windowStart: windowMs.start,
        windowEnd: windowMs.end
      });
      return {
        queryKey: [...o.queryKey],
        queryFn: o.queryFn,
        staleTime: o.staleTime,
        enabled: seedProbeId !== ''
      };
    }
  );
  const seedDossier = $derived<ProbeDossierDto | null>(
    dossierQ.data?.kind === 'success' ? dossierQ.data.data : null
  );

  function applyNewPanel(scopes: ScopeGroup[], lockedFunction: DiscourseFunction | null) {
    const pillarId: ViewingMode = activePillar.id;
    const panel = buildPanelFromScopes(scopes, {
      view: defaultViewModeForPillar(pillarId),
      lockedFunction: lockedFunction ?? undefined
    });
    const pillarState: PillarState = {
      windows: [{ panels: [panel], focusedPanelIndex: 0 }],
      activeWindowIndex: 0
    };
    const nextPillars: WorkbenchPillarsState = {
      aleph: url.pillars?.aleph ?? null,
      episteme: url.pillars?.episteme ?? null,
      rhizome: url.pillars?.rhizome ?? null,
      [pillarId]: pillarState
    };
    // Phase 122k §11 finding — push, not replace. Browser-back from the
    // populated Workbench restores the pre-Apply URL (no pillars), which
    // re-triggers the auto-open ScopeEditor — the user lands back where
    // they were configuring, not on Atmosphere.
    pushUrl({ pillars: nextPillars, activePillar: pillarId });
    editorDismissed = false;
  }

  function dismissCreateEditor() {
    editorDismissed = true;
  }

  function reopenCreateEditor() {
    editorDismissed = false;
  }

  // Phase 122k §11 — leaving the Workbench (SideRail Atmosphere /
  // Dossier / Reflection clicks, or any other route change) invalidates
  // the draft. Same-pathname navigations (the back-from-Apply case
  // which only changes the search string) are NOT cleared here — that's
  // the explicit one-shot restore path.
  beforeNavigate((nav) => {
    if (!nav.to) return;
    const from = nav.from?.url.pathname;
    const to = nav.to.url.pathname;
    if (from !== to) {
      clearDraft();
    }
  });
</script>

<svelte:head>
  <title>AĒR — Workbench · {activePillar.label}</title>
</svelte:head>

<main class="workbench-main" id="main-workbench">
  {#if !hasScope}
    <div class="empty-scope">
      <h1>Workbench</h1>
      {#if seedProbeId === ''}
        <p class="muted">Loading probe catalogue…</p>
      {:else if !editorDismissed}
        <p class="muted">Configure a scope to begin.</p>
      {:else}
        <p class="muted">No scope configured yet.</p>
        <button type="button" class="reopen-btn" onclick={reopenCreateEditor}>
          Configure scope →
        </button>
      {/if}
    </div>
  {:else}
    <PillarSwitch />
    <div class="pillar-body">
      {#if activePillar.id === 'aleph'}
        <AlephShell probeIds={[]} />
      {:else if activePillar.id === 'episteme'}
        <EpistemeShell probeIds={[]} />
      {:else}
        <RhizomeShell probeIds={[]} />
      {/if}
    </div>
  {/if}
</main>

{#if showCreateEditor && seedDossier}
  <ScopeEditor
    dossier={seedDossier}
    {ctx}
    seedProbes={url.selectedProbes}
    enableDraftPersistence
    onApply={applyNewPanel}
    onCancel={dismissCreateEditor}
  />
{/if}

<style>
  .workbench-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: 0;
    right: 0;
    overflow-y: auto;
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    gap: var(--space-4);
  }

  .empty-scope {
    margin: auto;
    text-align: center;
    max-width: 32rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--space-3);
  }

  .empty-scope h1 {
    font-size: var(--font-size-3xl);
    font-weight: var(--font-weight-semibold);
    margin: 0;
    color: var(--color-fg);
  }

  .reopen-btn {
    appearance: none;
    background: var(--color-accent);
    color: var(--color-bg);
    border: 1px solid var(--color-accent);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-4);
    font-size: var(--font-size-sm);
    font-weight: 600;
    cursor: pointer;
  }
  .reopen-btn:hover,
  .reopen-btn:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, var(--color-fg));
  }

  .pillar-body {
    flex: 1;
    min-height: 24rem;
    display: flex;
    flex-direction: column;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
