<script lang="ts">
  // Phase 141 — PanelHost header, extracted from PanelHost.svelte. Presentation
  // eyebrow + bound metric + lock state + the interactive panel actions
  // (Edit scope / Maximize / Remove). Purely presentational: the action
  // handlers (which mutate panel state / toggle the scope editor and own their
  // own stopPropagation) are passed in by PanelHost.
  import { m } from '$lib/paraglide/messages.js';
  import type { PresentationDefinition } from '$lib/presentations';
  import type { Panel } from '$lib/state/url-internals';

  interface Props {
    presentation: PresentationDefinition;
    panel: Panel;
    /** Interactive editing path resolved (focus on click, action buttons). */
    isInteractive: boolean;
    canMaximize: boolean;
    isMaximized: boolean;
    canRemove: boolean;
    onEditScope: (e: MouseEvent) => void;
    onToggleMaximize: (e: MouseEvent) => void;
    onRemove: (e: MouseEvent) => void;
  }

  let {
    presentation,
    panel,
    isInteractive,
    canMaximize,
    isMaximized,
    canRemove,
    onEditScope,
    onToggleMaximize,
    onRemove
  }: Props = $props();
</script>

<header class="panel-header">
  <span class="panel-eyebrow">{presentation.label}</span>
  {#if presentation.usesMetric !== false}
    <span class="panel-sep" aria-hidden="true">·</span>
    <code class="panel-metric">{panel.metric}</code>
  {/if}
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
      {#if canMaximize || isMaximized}
        <!-- Phase 122i revision (C3). Maximize is UI state, not scope editing,
             so it stays enabled on locked panels. Hidden when there is nothing
             else in the window to maximize against. -->
        <button
          type="button"
          class="panel-action"
          onclick={onToggleMaximize}
          title={isMaximized
            ? m.workbench_panel_restore_title()
            : m.workbench_panel_maximize_title()}
          aria-pressed={isMaximized}
        >
          {isMaximized ? m.workbench_panel_restore() : m.workbench_panel_maximize()}
        </button>
      {/if}
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
</header>

<style>
  .panel-header {
    display: flex;
    align-items: baseline;
    gap: var(--space-2);
    flex-wrap: wrap;
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
    margin-left: auto;
    font-size: var(--font-size-xs);
    color: var(--color-accent);
    font-style: italic;
  }

  .panel-actions {
    display: flex;
    gap: var(--space-1);
    margin-left: auto;
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
