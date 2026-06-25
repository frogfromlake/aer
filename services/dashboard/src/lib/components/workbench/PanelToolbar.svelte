<script lang="ts">
  // Phase 141 — PanelHost header, extracted from PanelHost.svelte. Presentation
  // eyebrow + bound metric + lock state + the interactive panel actions
  // (Edit scope / Zen / Remove). Purely presentational: the action handlers
  // (which mutate panel state / toggle the scope editor and own their own
  // stopPropagation) are passed in by PanelHost.
  import { m } from '$lib/paraglide/messages.js';
  import type { PresentationDefinition } from '$lib/presentations';
  import type { Panel } from '$lib/state/url-internals';

  interface Props {
    presentation: PresentationDefinition;
    panel: Panel;
    /** Interactive editing path resolved (focus on click, action buttons). */
    isInteractive: boolean;
    /** Phase 149 (Zen) — whether this panel is currently the full-screen Zen
     *  panel; flips the Zen action between enter and the (overlay-side) exit. */
    isZen: boolean;
    canRemove: boolean;
    onEditScope: (e: MouseEvent) => void;
    onToggleZen: (e: MouseEvent) => void;
    onRemove: (e: MouseEvent) => void;
    /** Phase 149 — commit a new panel caption (empty string clears it). Absent
     *  for non-interactive (unfocused / read-only) panels — the label still
     *  renders, but without the edit affordance. */
    onRenameLabel?: (label: string) => void;
    /** Phase 149 — the colour of the pillar this panel lives in; tints the
     *  caption text (matches the PillarSwitch title colour). */
    pillarColor?: string;
  }

  let {
    presentation,
    panel,
    isInteractive,
    isZen,
    canRemove,
    onEditScope,
    onToggleZen,
    onRemove,
    onRenameLabel,
    pillarColor
  }: Props = $props();

  // Phase 149 — inline caption editor (pencil → input → diskette). Local UI state;
  // the committed value flows up via onRenameLabel and persists in the URL state.
  let editing = $state(false);
  let draft = $state('');
  function startEdit(e: MouseEvent) {
    e.stopPropagation();
    draft = panel.label ?? '';
    editing = true;
  }
  function commit(e: Event) {
    e.stopPropagation();
    onRenameLabel?.(draft.trim());
    editing = false;
  }
  function cancel() {
    editing = false;
  }
  function onKeydown(e: KeyboardEvent) {
    e.stopPropagation();
    if (e.key === 'Enter') commit(e);
    else if (e.key === 'Escape') cancel();
  }
</script>

<!-- Phase 149 — 3-column grid (1fr · auto · 1fr) so the caption is centred relative
     to the FULL panel width, independent of the left title / right actions. -->
<header class="panel-header">
  <div class="panel-header-left">
    <span class="panel-eyebrow">{presentation.label}</span>
    {#if presentation.usesMetric !== false}
      <span class="panel-sep" aria-hidden="true">·</span>
      <code class="panel-metric">{panel.metric}</code>
    {/if}
  </div>
  <!-- Phase 149 — editable panel caption, CENTERED in the header, tinted in the
       pillar colour. Click the label (or the pencil) to edit; save with the
       diskette. Icons are monochrome inline SVGs so they take the SideRail accent
       colour (the floppy emoji would ignore `color`). -->
  <span class="panel-caption">
    {#if isInteractive && onRenameLabel}
      {#if editing}
        <span class="panel-label-editor" role="presentation" onclick={(e) => e.stopPropagation()}>
          <!-- svelte-ignore a11y_autofocus -->
          <input
            class="panel-label-input"
            style:color={pillarColor}
            bind:value={draft}
            onkeydown={onKeydown}
            placeholder={m.workbench_panel_label_placeholder()}
            aria-label={m.workbench_panel_label_input_aria()}
            autofocus
          />
          <button
            type="button"
            class="panel-label-btn"
            onclick={commit}
            title={m.workbench_panel_label_save_title()}
            aria-label={m.workbench_panel_label_save_title()}
          >
            <svg viewBox="0 0 24 24" aria-hidden="true">
              <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2Z" />
              <path d="M17 21v-8H7v8" />
              <path d="M7 3v5h8" />
            </svg>
          </button>
        </span>
      {:else}
        {#if panel.label}
          <button
            type="button"
            class="panel-label panel-label-button"
            style:color={pillarColor}
            onclick={startEdit}
            title={m.workbench_panel_label_edit_title()}>{panel.label}</button
          >
        {/if}
        <button
          type="button"
          class="panel-label-btn"
          onclick={startEdit}
          title={panel.label
            ? m.workbench_panel_label_edit_title()
            : m.workbench_panel_label_add_title()}
          aria-label={panel.label
            ? m.workbench_panel_label_edit_title()
            : m.workbench_panel_label_add_title()}
        >
          <svg viewBox="0 0 24 24" aria-hidden="true">
            <path d="M12 20h9" />
            <path d="M16.5 3.5a2.12 2.12 0 0 1 3 3L7 19l-4 1 1-4Z" />
          </svg>
        </button>
      {/if}
    {:else if panel.label}
      <span class="panel-label" style:color={pillarColor}>{panel.label}</span>
    {/if}
  </span>
  <div class="panel-header-right">
    {#if panel.locked === true && panel.lockedFunction}
      <span class="panel-lock" title={m.workbench_panel_lock_title()}>
        {m.workbench_panel_locked_to({ function: panel.lockedFunction })}
      </span>
    {/if}
    {#if isInteractive}
      <!-- Each action button stops click + keydown propagation in its own
           handler so the surrounding `<article>`'s focus handler does not also
           fire. Phase 122i revision (B1): `locked` is scope-only; `Edit scope`
           (scope mutation) is gated downstream when locked, `×Remove` and the
           other panel-level actions remain available. -->
      <div class="panel-actions">
        <button
          type="button"
          class="panel-action"
          onclick={onEditScope}
          title={m.workbench_panel_edit_scope_title()}
        >
          {m.workbench_panel_edit_scope()}
        </button>
        <!-- Phase 149 (Zen) — Zen is UI state, not scope editing, so it stays
             enabled on locked panels. Always available (a single panel can go
             full-screen too); flips to an exit affordance while Zen is open. -->
        <button
          type="button"
          class="panel-action"
          onclick={onToggleZen}
          title={isZen ? m.workbench_zen_exit_title() : m.workbench_panel_zen_title()}
          aria-pressed={isZen}
        >
          {isZen ? m.workbench_zen_exit() : m.workbench_panel_zen()}
        </button>
        {#if canRemove}
          <button
            type="button"
            class="panel-action panel-action-remove"
            onclick={onRemove}
            title={m.workbench_panel_remove_title()}
          >
            ×
          </button>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  .panel-header {
    display: grid;
    grid-template-columns: 1fr auto 1fr;
    align-items: center;
    gap: var(--space-2);
  }
  /* Equal 1fr side columns → the centre `auto` column (caption) is centred
     relative to the FULL header width, regardless of left/right content. */
  .panel-header-left {
    justify-self: start;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }
  .panel-header-right {
    justify-self: end;
    display: flex;
    align-items: center;
    gap: var(--space-2);
    min-width: 0;
  }

  .panel-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
  }

  .panel-sep {
    color: var(--color-fg-subtle);
  }

  .panel-metric {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
  }

  .panel-lock {
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-style: italic;
  }

  /* Phase 149 — panel caption: centered in the header, tinted in the pillar colour.
     `flex: 1` lets the empty/short caption push the lock + actions to the right
     edge without a `margin-left: auto` race. */
  .panel-caption {
    justify-self: center;
    min-width: 0;
    max-width: 100%;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-1);
  }

  .panel-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    color: var(--color-fg);
    max-width: 28rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* The label doubles as an edit affordance (click to rename). */
  .panel-label-button {
    appearance: none;
    border: none;
    background: transparent;
    padding: 0;
    cursor: pointer;
    font-family: inherit;
  }
  .panel-label-button:hover,
  .panel-label-button:focus-visible {
    text-decoration: underline;
    text-underline-offset: 3px;
    outline: none;
  }

  .panel-label-editor {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    min-width: 0;
  }

  .panel-label-input {
    min-width: 10rem;
    max-width: 28rem;
    padding: 1px var(--space-2);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-family: inherit;
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-semibold);
    text-align: center;
  }
  .panel-label-input:focus-visible {
    outline: none;
    border-color: var(--color-accent);
  }

  /* Pencil / diskette — same accent colour as the SideRail icons, sized to read. */
  .panel-label-btn {
    appearance: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-width: 26px;
    min-height: 26px;
    padding: 0 4px;
    background: transparent;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--color-accent);
    cursor: pointer;
  }
  .panel-label-btn svg {
    width: 16px;
    height: 16px;
    fill: none;
    stroke: currentColor;
    stroke-width: 2;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .panel-label-btn:hover,
  .panel-label-btn:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 14%, var(--color-surface));
    border-color: color-mix(in srgb, var(--color-accent) 40%, transparent);
    outline: none;
  }

  .panel-actions {
    display: flex;
    gap: var(--space-1);
  }

  .panel-action {
    appearance: none;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24×24px minimum target size. */
    display: inline-flex;
    align-items: center;
    min-height: 24px;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 2px var(--space-2);
    color: var(--color-fg);
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }

  .panel-action:hover,
  .panel-action:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 10%, var(--color-surface));
    border-color: var(--color-accent);
  }

  .panel-action-remove {
    color: var(--color-status-expired);
  }

  .panel-action-remove:hover,
  .panel-action-remove:focus-visible {
    background: color-mix(in srgb, var(--color-status-expired) 12%, var(--color-surface));
    border-color: var(--color-status-expired);
  }
</style>
