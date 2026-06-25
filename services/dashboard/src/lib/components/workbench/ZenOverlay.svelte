<script lang="ts">
  // Phase 149 (Zen) — the shared full-screen Zen chrome, used by BOTH the panel
  // Zen (WindowHost) and the per-cell Zen (PanelCell). A CSS fixed overlay
  // portalled to <body> so its z-index beats the SideRail (450) without an
  // ancestor stacking-context trap; a dimmed scrim backgrounds the workbench so
  // the zoomed content is the sole focus. The caller supplies the content via the
  // `children` snippet and the exit handler; `framed` wraps the content in an
  // elevated card (cells, which have no surface of their own) — panels bring
  // their own surface, so they stay unframed.
  import { m } from '$lib/paraglide/messages.js';
  import { portal } from '$lib/actions/portal';
  import type { Snippet } from 'svelte';

  let {
    onExit,
    framed = false,
    children
  }: { onExit: () => void; framed?: boolean; children: Snippet } = $props();
</script>

<div
  class="zen-overlay"
  use:portal
  role="dialog"
  aria-modal="true"
  aria-label={m.workbench_zen_aria_label()}
>
  <div class="zen-bar">
    <span class="zen-eyebrow">{m.workbench_zen_eyebrow()}</span>
    <button type="button" class="zen-exit" onclick={onExit} title={m.workbench_zen_exit_title()}>
      {m.workbench_zen_exit()}
    </button>
  </div>
  <div class="zen-stage" class:framed>
    {@render children()}
  </div>
</div>

<style>
  .zen-overlay {
    position: fixed;
    inset: 0;
    /* Above the SideRail (450) and every in-app modal; portalled to <body> so
       this actually wins. */
    z-index: 2000;
    display: flex;
    flex-direction: column;
    /* Dimmed scrim — the workbench shell behind reads as backgrounded, the zoomed
       content (its own elevated surface) is the sole focus. */
    background: color-mix(in srgb, var(--color-bg) 78%, rgba(0, 0, 0, 0.92));
    backdrop-filter: blur(3px);
    -webkit-backdrop-filter: blur(3px);
    animation: zen-fade-in var(--motion-duration-base, 180ms) var(--motion-ease-standard, ease-out);
  }
  @keyframes zen-fade-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .zen-overlay {
      animation: none;
    }
  }
  .zen-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--space-3);
    padding: var(--space-2) var(--space-4);
    border-bottom: 1px solid var(--color-border);
    background: var(--color-bg-elevated);
  }
  .zen-eyebrow {
    font-family: var(--font-mono);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-fg-subtle);
    font-weight: var(--font-weight-semibold);
  }
  .zen-exit {
    appearance: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: var(--space-1) var(--space-3);
    color: var(--color-fg);
    font-family: var(--font-ui);
    font-size: var(--font-size-sm);
    cursor: pointer;
  }
  .zen-exit:hover,
  .zen-exit:focus-visible {
    background: color-mix(in srgb, var(--color-accent) 12%, var(--color-surface));
    border-color: var(--color-accent);
  }
  /* The content fills the remaining height; the stage scrolls if it overflows
     (e.g. a tall co-occurrence map). The scrim shows in the stage padding. */
  .zen-stage {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--space-4);
  }
  /* `framed` — an elevated card for content without a surface of its own (cells).
     `position: relative` so the cell's absolutely-positioned affordances (config
     popover, reading-guide pull-tab) anchor to the card. */
  .zen-stage.framed {
    position: relative;
    margin: var(--space-4);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
</style>
