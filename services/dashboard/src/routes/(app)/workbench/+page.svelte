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
  import { m } from '$lib/paraglide/messages.js';
  import { beforeNavigate, goto } from '$app/navigation';
  import { pushUrl, urlState } from '$lib/state/url.svelte';
  import { defaultPresentationForPillar, getPillar } from '$lib/presentations';
  import { clearDraft } from '$lib/workbench/scope-editor-draft';
  import { setCleanBaseline, isWorkbenchDirty } from '$lib/workbench/dirty.svelte';
  import {
    stripDeepLink,
    NON_STATE_PARAMS
  } from '$lib/components/account/analyses-overlay-internals';
  import WorkbenchLeaveGuard from '$lib/components/workbench/WorkbenchLeaveGuard.svelte';
  import {
    probeDossierQuery,
    probesQuery,
    type FetchContext,
    type ProbeDossierDto,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';
  import {
    type PillarState,
    type ScopeGroup,
    type PillarId,
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

  // Phase 127 — leave-guard baseline. The analysis is fully URL-encoded; the
  // current deep-link (overlay params stripped) is compared against a clean
  // baseline only when the user navigates away. The baseline is (re)set to
  // "clean" whenever the loaded-analysis id changes — i.e. on entry, when a
  // saved analysis is opened, and on Save-as-new. In-place updates reset it
  // from the AnalysesOverlay. Keyed on the memoised `loadedAnalysisId`, so this
  // effect runs only on those transitions, NEVER on a panel tweak.
  const currentDeepLink = () => stripDeepLink(window.location.href, NON_STATE_PARAMS);
  const loadedAnalysisId = $derived(url.savedAnalysis);
  $effect(() => {
    void loadedAnalysisId;
    setCleanBaseline(currentDeepLink());
  });

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

  // Default window = the WHOLE dataset (undefined ⇒ no time filter). The seed
  // dossier preview then reports whole-corpus numbers, matching the Dossier
  // overlay; time-limiting engages only when the URL carries from/to.
  const windowMs = $derived.by<{ start: string | undefined; end: string | undefined }>(() => {
    const fromMs = url.from ? Date.parse(url.from) : NaN;
    const toMs = url.to ? Date.parse(url.to) : NaN;
    return {
      start: Number.isFinite(fromMs) ? new Date(fromMs).toISOString() : undefined,
      end: Number.isFinite(toMs)
        ? new Date(toMs).toISOString()
        : url.from
          ? new Date().toISOString()
          : undefined
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
    const pillarId: PillarId = activePillar.id;
    const panel = buildPanelFromScopes(scopes, {
      view: defaultPresentationForPillar(pillarId),
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
    // Cancelling the create-mode editor (the initial, no-scope entry) abandons
    // the Workbench → fall back to the Atmosphere, rather than stranding the
    // user on a bare "No scope configured yet" placeholder.
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal back-to-globe
    void goto('/');
  }

  function reopenCreateEditor() {
    editorDismissed = false;
  }

  // Phase 127 — leave-guard. Navigating away from the Workbench (SideRail
  // surface clicks, browser back/forward, any route change) with a CHANGED,
  // unsaved analysis pops a confirm modal: leave without saving · save as a new
  // analysis · update the loaded saved analysis. A clean or unconfigured
  // Workbench leaves immediately (and clears the scope-editor draft as before).
  // Same-pathname navigations (the back-from-Apply search-only change) are not a
  // leave. The compare runs only here, at navigation time — no standing effect.
  let pendingTo = $state<URL | null>(null);
  let confirmedLeave = false;

  beforeNavigate((nav) => {
    if (confirmedLeave) return; // our own post-confirm navigation
    if (!nav.to) return;
    const from = nav.from?.url.pathname;
    const to = nav.to.url.pathname;
    if (from === to) return; // in-Workbench state change, not a leave
    if (!hasScope || !isWorkbenchDirty(currentDeepLink())) {
      clearDraft();
      return;
    }
    nav.cancel();
    pendingTo = nav.to.url;
  });

  function leaveNow() {
    const dest = pendingTo;
    pendingTo = null;
    if (!dest) return;
    clearDraft();
    confirmedLeave = true;
    // eslint-disable-next-line svelte/no-navigation-without-resolve -- internal confirmed leave
    void goto(`${dest.pathname}${dest.search}${dest.hash}`);
  }
  function stayHere() {
    pendingTo = null;
  }
</script>

<svelte:head>
  <title>{m.workbench_page_title({ pillar: activePillar.label })}</title>
</svelte:head>

<!-- Phase 135 — the initial (no-scope) Workbench is transparent so the
     layout's persistent globe shows through (the create-mode ScopeEditor's own
     scrim dims it); a configured scope switches to the opaque work area. -->
<main class="workbench-main" id="main-workbench" class:scope-empty={!hasScope}>
  {#if !hasScope}
    <!-- While the create-mode editor is on its way (showCreateEditor), render
         NOTHING behind it — otherwise the placeholder text flashes for a frame
         before the ScopeEditor mounts. The placeholder only appears if the
         editor is genuinely not opening (a defensive fallback). -->
    {#if !showCreateEditor}
      <div class="empty-scope">
        <h1>{m.workbench_page_empty_heading()}</h1>
        <p class="muted">{m.workbench_page_empty_no_scope()}</p>
        <button type="button" class="reopen-btn" onclick={reopenCreateEditor}>
          {m.workbench_page_configure_scope()}
        </button>
      </div>
    {/if}
  {:else}
    <!-- Phase 127 — the Save/New-analysis actions moved into the WindowHost
         action strip (next to `+ Panel`); the header now holds just the switch. -->
    <div class="workbench-header">
      <PillarSwitch />
    </div>
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

<WorkbenchLeaveGuard
  open={pendingTo !== null}
  loadedAnalysisId={url.savedAnalysis}
  currentState={pendingTo !== null ? currentDeepLink() : ''}
  onLeave={leaveNow}
  onCancel={stayHere}
/>

<style>
  .workbench-main {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: 0;
    right: 0;
    z-index: 1;
    overflow-y: auto;
    background: var(--color-bg);
    display: flex;
    flex-direction: column;
    padding: var(--space-5);
    gap: var(--space-4);
  }
  /* Initial (no-scope) entry: let the layout's persistent globe show through;
     the ScopeEditor's own scrim provides the glassy dim. */
  .workbench-main.scope-empty {
    background: transparent;
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

  /* The header now holds just the PillarSwitch (the analysis actions moved to
     the WindowHost action strip in Phase 127). */
  .workbench-header {
    display: flex;
    align-items: stretch;
    gap: var(--space-4);
  }
  .workbench-header > :global(.pillar-switch) {
    flex: 1 1 auto;
    min-width: 0;
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
