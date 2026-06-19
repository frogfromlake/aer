<script lang="ts">
  // Phase 122i / ADR-034 — Multi-Panel Workbench: Window renderer.
  //
  // A Window holds 1..8 Panels arranged side-by-side. WindowHost has two
  // render modes:
  //
  //   - **Grid mode** (default): all panels in a CSS-grid raster (C1).
  //     Up to 4 columns, wraps to next row at panel 5. Each panel is
  //     focusable; the focused panel hosts PanelControls.
  //
  //   - **Maximize mode** (R4): when `window.maximizedPanelIndex` is set
  //     (R2 URL-state field), only the maximized panel renders at full
  //     canvas. A minimised tray sits at the bottom listing the other
  //     panels as small tiles for quick swap. `Esc` un-maximizes;
  //     clicking the maximize button on the active panel toggles back.
  //
  // Per-window max panels is enforced at the URL-state layer
  // (MAX_PANELS_PER_WINDOW = 8).
  import { onMount } from 'svelte';
  import { m } from '$lib/paraglide/messages.js';
  import { SvelteSet } from 'svelte/reactivity';
  import type { FetchContext, ProbeDossierDto } from '$lib/api/queries';
  import { MAX_PANELS_PER_WINDOW, type PillarState, type PillarId } from '$lib/state/url-internals';
  import { addPanel, setMaximizedPanel, setPanelsPerRow } from '$lib/workbench/panel-mutators';
  import { openOverlay, pushUrl } from '$lib/state/url.svelte';
  import { clearDraft } from '$lib/workbench/scope-editor-draft';
  import { buildPanelFromScopes } from '$lib/workbench/panel-queries';
  import { defaultPresentationForPillar } from '$lib/presentations';
  import type { ScopeGroup } from '$lib/state/url-internals';
  import type { DiscourseFunction } from '$lib/discourse-function';
  import PanelHost from './PanelHost.svelte';
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
  // Clear the selection when the active window changes (a new comparison frame);
  // stale ids from a prior scope are harmless but a fresh window starts clean.
  $effect(() => {
    void activeWindowIndex;
    selectedArticleIds.clear();
  });

  // Phase 122i revision (C3). Validate the maximize pointer against the
  // current panel count (defensive — URL surgery could leave a stale
  // value). When valid, we render the maximize branch.
  const maximizedPanelIndex = $derived.by<number | null>(() => {
    if (!activeWindow) return null;
    const raw = activeWindow.maximizedPanelIndex;
    if (raw === undefined || raw === null) return null;
    if (raw < 0 || raw >= activeWindow.panels.length) return null;
    return raw;
  });
  const isMaximizing = $derived(maximizedPanelIndex !== null);

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

  function pickTrayPanel(i: number) {
    // Swap maximize target. Also focuses the panel so PanelControls
    // follows automatically.
    setMaximizedPanel(pillar, activeWindowIndex, i);
  }

  function unmaximize() {
    setMaximizedPanel(pillar, activeWindowIndex, null);
  }

  // Phase 122i revision (C3). Escape un-maximizes. Window-scoped key
  // handler — only active while a panel is maximized.
  onMount(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Escape') return;
      // Avoid firing while a popover / modal is open (the ScopeEditor
      // handles its own Escape internally — its target is the
      // backdrop, not the document — so events that reach `document`
      // mean no modal is active).
      if (e.target instanceof HTMLInputElement) return;
      if (e.target instanceof HTMLTextAreaElement) return;
      // Only act if a panel is actually maximized; otherwise do nothing.
      if (!isMaximizing) return;
      unmaximize();
    }
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  });
</script>

<section
  class="window-host"
  aria-label={m.workbench_window_aria_label()}
  data-panel-count={panelCount}
  class:maximizing={isMaximizing}
>
  {#if !activeWindow || panelCount === 0}
    <p class="muted">{m.workbench_window_no_panels()}</p>
  {:else if isMaximizing && maximizedPanelIndex !== null}
    {@const focusedPanel = activeWindow.panels[maximizedPanelIndex]}
    {#if focusedPanel}
      <div class="panel-maximized">
        <PanelHost
          panel={focusedPanel}
          {dossier}
          {ctx}
          {windowStart}
          {windowEnd}
          {pillar}
          windowIndex={activeWindowIndex}
          panelIndex={maximizedPanelIndex}
          focused
          canRemove={panelCount > 1}
          isMaximized
          canMaximize={true}
          {selection}
        />
      </div>

      {#if panelCount > 1}
        <!-- Phase 122i revision (C3). Minimised tray of sibling panels.
             Each tile is a compact button: clicking swaps the maximize
             target. Provides quick swap without leaving the canvas. -->
        <aside class="panel-tray" aria-label={m.workbench_window_tray_aria_label()}>
          <span class="tray-label">{m.workbench_window_tray_label()}</span>
          <ul class="tray-list" role="list">
            {#each activeWindow.panels as p, i (i)}
              {#if i !== maximizedPanelIndex}
                <li>
                  <button
                    type="button"
                    class="tray-tile"
                    onclick={() => pickTrayPanel(i)}
                    title={m.workbench_window_tray_show_maximized()}
                  >
                    <span class="tray-tile-num">#{i + 1}</span>
                    <span class="tray-tile-meta">
                      <code>{p.view}</code>
                      <span class="tray-tile-metric">{p.metric}</span>
                    </span>
                  </button>
                </li>
              {/if}
            {/each}
          </ul>
          <button
            type="button"
            class="tray-action"
            onclick={unmaximize}
            title={m.workbench_window_tray_restore()}
          >
            {m.workbench_window_tray_restore_label()}
          </button>
        </aside>
      {/if}
    {/if}
  {:else}
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
          isMaximized={false}
          canMaximize={panelCount > 1}
          {selection}
        />
      {/each}
    </div>
  {/if}
</section>

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

  /* Phase 122i revision (C3). Maximize-mode layout. */
  .panel-maximized {
    display: flex;
    flex-direction: column;
    min-height: 28rem;
    flex: 1;
  }

  .panel-tray {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    flex-wrap: wrap;
    padding: var(--space-2) var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }

  .tray-label {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
    min-width: 5rem;
  }

  .tray-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-1);
    flex: 1 1 auto;
  }

  .tray-tile {
    appearance: none;
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: 4px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    cursor: pointer;
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
  }

  .tray-tile:hover,
  .tray-tile:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .tray-tile-num {
    font-weight: var(--font-weight-semibold);
    color: var(--color-accent);
  }

  .tray-tile-meta {
    display: inline-flex;
    align-items: baseline;
    gap: var(--space-1);
    color: var(--color-fg);
  }

  .tray-tile-meta code {
    color: var(--color-fg-subtle);
  }

  .tray-tile-metric {
    color: var(--color-fg-muted);
  }

  .tray-action {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 4px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .tray-action:hover,
  .tray-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
  }

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
