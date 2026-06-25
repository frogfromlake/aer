<script lang="ts">
  // Phase 122i / ADR-034 — Multi-Panel Workbench: Window renderer.
  //
  // A Window holds 1..8 Panels arranged side-by-side in a CSS-grid raster (C1):
  // up to 4 columns, wrapping to the next row at panel 5. Each panel is
  // focusable; every panel hosts a (collapsed-by-default) PanelControls strip.
  //
  // Phase 149 (Zen) — any panel can open full-screen in **Zen mode**: a CSS
  // fixed overlay above everything (incl. the SideRail) holding just that panel,
  // still fully editable. Zen is transient view state (`zenPanelIndex`, NOT
  // URL-persisted); `Esc` or the Leave button returns to the grid. This replaced
  // the former Maximize mode (URL-state `maximizedPanelIndex` + minimised tray),
  // which was removed in Phase 149.
  //
  // Per-window max panels is enforced at the URL-state layer
  // (MAX_PANELS_PER_WINDOW = 8).
  import { onMount } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { SvelteSet } from 'svelte/reactivity';
  import type { FetchContext, ProbeDossierDto } from '$lib/api/queries';
  import { MAX_PANELS_PER_WINDOW, type PillarState, type PillarId } from '$lib/state/url-internals';
  import { addPanel, setPanelsPerRow } from '$lib/workbench/panel-mutators';
  import { openOverlay, pushUrl } from '$lib/state/url.svelte';
  import { clearDraft } from '$lib/workbench/scope-editor-draft';
  import { buildPanelFromScopes } from '$lib/workbench/panel-queries';
  import { defaultPresentationForPillar } from '$lib/presentations';
  import type { ScopeGroup } from '$lib/state/url-internals';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import PanelHost from './PanelHost.svelte';
  import ZenOverlay from './ZenOverlay.svelte';
  import ScopeEditor from './ScopeEditor.svelte';
  // Phase 122k §14b finding 2 — WorkbenchDatasetShape retired here; the
  // per-panel `PanelMetaStrip` surfaces scope info inside each panel.

  interface Props {
    pillar: PillarId;
    pillarState: PillarState;
    dossier: ProbeDossierDto;
    ctx: FetchContext;
    windowStart: string | undefined;
    windowEnd: string | undefined;
  }

  let { pillar, pillarState, dossier, ctx, windowStart, windowEnd }: Props = $props();

  // Resolve the active window from the PillarState. Out-of-range
  // activeWindowIndex (e.g. after URL surgery) falls back to the first
  // window so the user is never stranded.
  const activeWindow = $derived(
    pillarState.windows[pillarState.activeWindowIndex] ?? pillarState.windows[0] ?? null
  );
  const activeWindowIndex = $derived(
    pillarState.windows[pillarState.activeWindowIndex] ? pillarState.activeWindowIndex : 0
  );

  const panelCount = $derived(activeWindow?.panels.length ?? 0);
  const canAddPanel = $derived(panelCount < MAX_PANELS_PER_WINDOW);

  // Phase 125b — cross-panel linked brushing. A Window-scoped, transient set of
  // selected article ids shared across the panels of this Window: clicking a
  // per-article mark (scatter point / parallel line) in one panel highlights the
  // same articles in another panel's per-article cell. Deliberately NOT
  // URL-persisted (ephemeral interaction, not saved configuration; Brief §5.5).
  // The `selection` object reference is stable; `ids` is the reactive SvelteSet.
  const selectedArticleIds = new SvelteSet<string>();
  const selection = {
    get ids(): ReadonlySet<string> {
      return selectedArticleIds;
    },
    toggle(articleId: string) {
      if (selectedArticleIds.has(articleId)) selectedArticleIds.delete(articleId);
      else selectedArticleIds.add(articleId);
    },
    clear() {
      selectedArticleIds.clear();
    }
  };
  // Phase 149 (Zen) — which panel (if any) is open full-screen in Zen mode.
  // Transient view state, deliberately NOT URL-persisted (an ephemeral reading
  // posture, not saved configuration — mirrors `selection`).
  let zenPanelIndex = $state<number | null>(null);
  // Phase 149 (Zen) — whole-window Zen: the entire multi-panel grid full-screen.
  // The three Zen levels (cell / panel / window) are transient and never
  // URL-persisted. Window- and panel-Zen are mutually exclusive (entering one
  // clears the other), so the Esc/exit semantics stay unambiguous.
  let windowZen = $state(false);
  const zenPanel = $derived(
    zenPanelIndex !== null ? (activeWindow?.panels[zenPanelIndex] ?? null) : null
  );

  // Clear the selection AND leave every Zen level when the active window changes
  // (a new comparison frame); stale ids/zen state from a prior frame start clean.
  $effect(() => {
    void activeWindowIndex;
    selectedArticleIds.clear();
    zenPanelIndex = null;
    windowZen = false;
  });

  // Defensive — if the zoomed target disappears (URL surgery / panels removed),
  // drop back to the grid so we never hold a stale index or an empty overlay.
  $effect(() => {
    if (zenPanelIndex !== null && (!activeWindow || zenPanelIndex >= activeWindow.panels.length)) {
      zenPanelIndex = null;
    }
    if (windowZen && (!activeWindow || activeWindow.panels.length === 0)) {
      windowZen = false;
    }
  });

  function enterZen(i: number) {
    windowZen = false;
    zenPanelIndex = i;
  }
  function exitZen() {
    zenPanelIndex = null;
  }
  function toggleWindowZen() {
    zenPanelIndex = null;
    windowZen = !windowZen;
  }
  function exitWindowZen() {
    windowZen = false;
  }

  // Phase 122k F3 — `+ Panel` opens the ScopeEditor in create mode. On
  // Apply the editor's draft becomes a fresh Panel which is appended to
  // the active window via the addPanel mutator (with the template arg).
  let addPanelEditorOpen = $state(false);

  function onAddPanel() {
    addPanelEditorOpen = true;
  }

  // Phase 127 — analysis-lifecycle actions, relocated here next to `+ Panel`
  // (they were cramped beside the PillarSwitch). "New analysis" wipes the whole
  // Workbench; the page auto-opens the create editor when pillar state is null.
  // `clearDraft` so that editor starts blank, not from a resumed draft.
  function newAnalysis() {
    clearDraft();
    pushUrl({ pillars: null, activePillar: null });
  }

  function applyNewPanel(scopes: ScopeGroup[], lockedFunction: DiscourseFunction | null) {
    const template = buildPanelFromScopes(scopes, {
      view: defaultPresentationForPillar(pillar),
      lockedFunction: lockedFunction ?? undefined
    });
    addPanel(pillar, template);
    addPanelEditorOpen = false;
  }

  // Phase 149 (Zen) — Escape leaves Zen mode. Window-scoped key handler — only
  // acts while a panel is open full-screen.
  onMount(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Escape') return;
      // Avoid firing while a field is focused (typing in a panel-control input
      // or the caption editor); those own their own Escape semantics.
      if (e.target instanceof HTMLInputElement) return;
      if (e.target instanceof HTMLTextAreaElement) return;
      // Exit the active Zen level; panel-Zen takes precedence over window-Zen.
      if (zenPanelIndex !== null) {
        exitZen();
        return;
      }
      if (windowZen) {
        exitWindowZen();
        return;
      }
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });
</script>

<section
  class="window-host"
  aria-label={m.workbench_window_aria_label()}
  data-panel-count={panelCount}
>
  {#if !activeWindow || panelCount === 0}
    <p class="muted">{m.workbench_window_no_panels()}</p>
  {:else if zenPanelIndex !== null || windowZen}
    <!-- Phase 149 (Zen) — a panel OR the whole window is open full-screen; the
         grid is unmounted here so heavy cells (sigma/WebGL) are not double-
         rendered. The fixed overlay (top-level sibling below) is the surface. -->
  {:else}
    {@render windowBody()}
  {/if}
</section>

<!-- Phase 149 (Zen) — the multi-panel grid + its actions strip, rendered EITHER
     inline (grid mode) OR inside the window-Zen overlay (never both, so the heavy
     panels mount once). `{#if activeWindow}` narrows the type inside the snippet
     closure (the section's guard does not reach in here). -->
{#snippet windowBody()}
  {#if activeWindow}
    {@const ppr = activeWindow.panelsPerRow ?? null}
    <!-- Phase 122k §14b finding 3 — actions strip ABOVE the panels.
         `+ Panel` is the most-used affordance after panel configuration;
         placing it on top keeps it visible without scrolling. -->
    <div class="window-actions window-actions-top">
      <!-- Phase 127 — analysis-lifecycle actions on the far left (Save keeps its
           accent/primary look, New its ghost look); the +Panel action sits to
           the right of the spacer, just left of the panels-per-row group. -->
      <button
        type="button"
        class="window-action window-action-primary"
        onclick={() => openOverlay('analyses', 'save')}
        title={m.workbench_page_save_analysis_title()}
      >
        <span aria-hidden="true">★</span>
        {m.workbench_page_save_analysis()}
      </button>
      <button
        type="button"
        class="window-action"
        onclick={newAnalysis}
        title={m.workbench_page_new_analysis_title()}
      >
        <span aria-hidden="true">＋</span>
        {m.workbench_page_new_analysis()}
      </button>
      <span class="window-action-spacer"></span>
      <button
        type="button"
        class="window-action window-action-primary panel-action-trailing"
        onclick={onAddPanel}
        disabled={!canAddPanel}
        title={canAddPanel
          ? m.workbench_window_add_panel_title()
          : m.workbench_window_add_panel_max_title({ max: MAX_PANELS_PER_WINDOW })}
      >
        ＋ {m.workbench_window_add_panel()}
      </button>
      <!-- Phase 149 (Zen) — open the WHOLE multi-panel view full-screen; flips to
           an exit while open. The third Zen level (cell · panel · window). -->
      <button
        type="button"
        class="window-action"
        onclick={toggleWindowZen}
        aria-pressed={windowZen}
        title={windowZen ? m.workbench_zen_exit_title() : m.workbench_window_zen_title()}
      >
        {windowZen ? m.workbench_zen_exit() : m.workbench_window_zen()}
      </button>
      <span class="ppr-eyebrow">{m.workbench_window_panels_per_row()}</span>
      <div class="ppr-row" role="radiogroup" aria-label={m.workbench_window_panels_per_row()}>
        <button
          type="button"
          role="radio"
          aria-checked={ppr === null}
          class="ppr-btn"
          class:active={ppr === null}
          onclick={() => setPanelsPerRow(pillar, activeWindowIndex, null)}
          title={m.workbench_window_ppr_auto_title()}
        >
          {m.workbench_window_ppr_auto()}
        </button>
        {#each [1, 2, 3, 4] as n (n)}
          <button
            type="button"
            role="radio"
            aria-checked={ppr === n}
            class="ppr-btn"
            class:active={ppr === n}
            onclick={() => setPanelsPerRow(pillar, activeWindowIndex, n)}
            title={n === 1
              ? m.workbench_window_ppr_n_title_one({ n })
              : m.workbench_window_ppr_n_title_other({ n })}
          >
            {n}
          </button>
        {/each}
      </div>
    </div>
    <div class="panel-grid" class:fixed-cols={ppr !== null} style:--cols={ppr ?? ''}>
      {#each activeWindow.panels as panel, i (i)}
        <PanelHost
          {panel}
          {dossier}
          {ctx}
          {windowStart}
          {windowEnd}
          {pillar}
          windowIndex={activeWindowIndex}
          panelIndex={i}
          focused={i === activeWindow.focusedPanelIndex}
          canRemove={panelCount > 1}
          isZen={false}
          onToggleZen={() => enterZen(i)}
          {selection}
        />
      {/each}
    </div>
  {/if}
{/snippet}

{#if windowZen && activeWindow && panelCount > 0}
  <!-- Phase 149 (Zen) — the WHOLE multi-panel view full-screen in the shared
       ZenOverlay (portalled above the SideRail, dimmed scrim). Unframed — the
       panels bring their own surfaces. Esc / Leave / the actions-strip toggle
       return to the grid. -->
  <ZenOverlay onExit={exitWindowZen}>
    {@render windowBody()}
  </ZenOverlay>
{/if}

{#if zenPanel && zenPanelIndex !== null}
  <!-- Phase 149 (Zen) — the panel opens full-screen in the shared ZenOverlay
       (portalled above the SideRail, dimmed scrim). The panel stays fully
       interactive/editable; Esc / Leave / the toolbar's Zen toggle return to the
       grid. The panel brings its own surface, so the stage stays unframed. -->
  {@const zenIdx = zenPanelIndex ?? 0}
  <ZenOverlay onExit={exitZen}>
    <PanelHost
      panel={zenPanel}
      {dossier}
      {ctx}
      {windowStart}
      {windowEnd}
      {pillar}
      windowIndex={activeWindowIndex}
      panelIndex={zenIdx}
      focused
      canRemove={false}
      isZen
      onToggleZen={exitZen}
      {selection}
    />
  </ZenOverlay>
{/if}

{#if addPanelEditorOpen}
  <ScopeEditor
    {dossier}
    {ctx}
    onApply={applyNewPanel}
    onCancel={() => (addPanelEditorOpen = false)}
  />
{/if}

<style>
  .window-host {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
    flex: 1;
    min-height: 0;
  }

  .panel-grid {
    /* Phase 122i revision (C1). Raster layout: up to 4 panels per row,
       wrap to next row at panel 5. No horizontal scroll — vertical
       scroll is the page's natural overflow. `repeat(auto-fit, …)` lets
       the browser fit as many columns as the viewport allows; for the
       typical ~1440-px desktop (rail consumes ~184 px → ~1256 px
       canvas) this yields 4 columns of ~28 rem each, then wraps. */
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(28rem, 1fr));
    gap: var(--space-4);
    align-items: stretch;
  }

  /* Phase 122k §14 finding 6 — fixed columns. The user picks N panels
     per row via the `Panels per row` toggle; CSS picks up the value via
     the `--cols` CSS custom property. */
  .panel-grid.fixed-cols {
    grid-template-columns: repeat(var(--cols), minmax(0, 1fr));
  }

  /* Phase 149 (Zen) — the full-screen Zen chrome lives in the shared ZenOverlay
     component (overlay + scrim + bar + Leave + portal); WindowHost only supplies
     the panel as its content. */

  /* Phase 122k §14c finding 3 — Panels-per-row toggle: prominenter so
     it reads as a peer to the `+ Panel` primary button. */
  .ppr-eyebrow {
    font-family: var(--font-mono);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-muted);
  }
  .ppr-row {
    display: inline-flex;
    gap: 4px;
  }
  .ppr-btn {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg-muted);
    padding: 6px var(--space-3);
    font-size: var(--font-size-sm);
    font-family: var(--font-mono);
    font-weight: 600;
    cursor: pointer;
    min-width: 2.25rem;
  }
  .ppr-btn:hover,
  .ppr-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  .ppr-btn.active {
    color: var(--color-accent);
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
  }
  .window-action-spacer {
    flex: 1 1 auto;
  }

  .window-actions {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    margin-top: var(--space-2);
  }

  /* Phase 122k §14b finding 3 — top action strip above the panel grid. */
  .window-actions-top {
    margin-top: 0;
    margin-bottom: var(--space-3);
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }

  /* Phase 122k §14c — `+ Panel` is the user's most common next-step.
     Subtly highlighted with the AĒR-türkis accent, solid border, slight
     shadow + font-weight bump so it reads unambiguously as a button. */
  .window-action-primary {
    background: var(--color-accent);
    color: var(--color-bg);
    border: 1px solid var(--color-accent);
    border-style: solid;
    font-weight: 700;
    padding: var(--space-2) var(--space-4);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      box-shadow var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .window-action-primary:hover:not(:disabled),
  .window-action-primary:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 85%, var(--color-fg));
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.25);
  }

  .window-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius-sm);
    padding: var(--space-2) var(--space-3);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }

  /* Phase 127 — slight gap left of the panels-per-row group. */
  .panel-action-trailing {
    margin-right: var(--space-2);
  }

  .window-action:hover:not(:disabled),
  .window-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-color: var(--color-accent);
    border-style: solid;
  }

  .window-action:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .muted {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    margin: 0;
  }
</style>
