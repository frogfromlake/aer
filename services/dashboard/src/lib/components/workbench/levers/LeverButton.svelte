<script lang="ts">
  // LeverButton — Phase 141. The single shared control-strip button primitive,
  // extracted from PanelControls so every per-lever child (and the per-cell
  // override popover) renders one styled button instead of re-declaring the
  // `.ctrl-btn` ruleset (it appeared 14× in PanelControls alone). Owns the base
  // button surface plus every variant the strip uses (metric / metadata-metric /
  // layer-silver / partial-toggle), so callers only pass a `variant` string.
  //
  // Behaviour-preserving: the rendered <button> carries the same attributes the
  // inline markup did — `type="button"`, optional role + aria-checked, class
  // toggles, title, disabled, onclick.
  import type { Snippet } from 'svelte';

  interface Props {
    children: Snippet;
    onclick?: (e: MouseEvent) => void;
    /** radio | switch; omitted for plain action buttons (reset, …). */
    role?: 'radio' | 'switch';
    /** Selected state — drives `.active` and (for radio/switch) `aria-checked`. */
    active?: boolean;
    /** Override `aria-checked` independently of `.active` (rare). */
    ariaChecked?: boolean;
    ariaExpanded?: boolean;
    ariaLabel?: string;
    title?: string;
    disabled?: boolean;
    /** Extra classes appended after `ctrl-btn` (e.g. 'metric-btn metadata-metric'). */
    variant?: string;
  }

  let {
    children,
    onclick,
    role,
    active = false,
    ariaChecked,
    ariaExpanded,
    ariaLabel,
    title,
    disabled,
    variant = ''
  }: Props = $props();
</script>

<button
  type="button"
  class={`ctrl-btn ${variant}`}
  class:active
  {role}
  aria-checked={role ? (ariaChecked ?? active) : undefined}
  aria-expanded={ariaExpanded}
  aria-label={ariaLabel}
  {title}
  {disabled}
  {onclick}
>
  {@render children()}
</button>

<style>
  .ctrl-btn {
    appearance: none;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24×24px minimum target size; inline-flex
       keeps the label vertically centred in the now-taller hit area. */
    display: inline-flex;
    align-items: center;
    justify-content: center;
    min-height: 24px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    color: var(--color-fg-muted);
    padding: 4px var(--space-3);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    font-family: var(--font-ui);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .ctrl-btn.metric-btn :global(code) {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: inherit;
  }

  /* Phase 133 (Issue 4) — metadata-metric pill reads as secondary (dashed). */
  .ctrl-btn.metadata-metric {
    border-style: dashed;
  }

  .ctrl-btn:hover,
  .ctrl-btn:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border-strong);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .ctrl-btn.active {
    color: var(--color-fg);
    background: rgba(82, 131, 184, 0.25);
    border-color: var(--color-accent);
  }

  .ctrl-btn.layer-btn.silver.active {
    color: #7ec4a0;
    background: rgba(126, 196, 160, 0.18);
    border-color: #7ec4a0;
  }

  /* Phase 123c (C1) — "show anyway" toggle active accent. */
  .ctrl-btn.partial-toggle {
    align-self: flex-start;
    margin-top: var(--space-1);
  }
  .ctrl-btn.partial-toggle.active {
    color: var(--color-accent);
    border-color: var(--color-accent);
    background: color-mix(in srgb, var(--color-accent) 12%, transparent);
  }

  @media (prefers-reduced-motion: reduce) {
    .ctrl-btn {
      transition: none;
    }
  }
</style>
