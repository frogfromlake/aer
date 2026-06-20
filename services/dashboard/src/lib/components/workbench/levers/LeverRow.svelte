<script lang="ts">
  // LeverRow — Phase 141. Shared control-strip row primitive extracted from
  // PanelControls: the `<div class="ctrl-row">` + `.ctrl-eyebrow` label that
  // every lever repeats. Owns the row structure CSS plus the dashed inter-row
  // divider, and (via :global, since they arrive through the slot) the shared
  // select / slider / value / empty-note styles so per-lever children never
  // re-declare them.
  //
  // `options` content is wrapped in `.ctrl-options` (the radio button cluster);
  // `children` content renders raw after the eyebrow (selects, sliders, notes).
  import type { Snippet } from 'svelte';

  interface Props {
    eyebrow: string;
    role?: 'radiogroup' | 'group' | 'note' | 'presentation';
    ariaLabel?: string;
    /** Extra row classes (e.g. 'config-row', 'partial-hint', 'ctrl-row-split'). */
    rowClass?: string;
    /** Radio/toggle cluster — wrapped in `.ctrl-options`. */
    options?: Snippet;
    /** Raw row content after the eyebrow (selects, sliders, notes). */
    children?: Snippet;
  }

  let { eyebrow, role, ariaLabel, rowClass = '', options, children }: Props = $props();
</script>

<div class={`ctrl-row ${rowClass}`} {role} aria-label={ariaLabel}>
  <span class="ctrl-eyebrow">{eyebrow}</span>
  {#if options}
    <div class="ctrl-options">{@render options()}</div>
  {/if}
  {@render children?.()}
</div>

<style>
  /* All strip-shared classes are :global so both LeverRow's own template AND
     the custom rows in CompositionControls / LayerCompareControls (which cannot
     use this single-eyebrow primitive) are styled from one place — no
     per-child duplication. Every class is scoped under the strip by name. */
  :global(.ctrl-row) {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    flex-wrap: wrap;
    padding: var(--space-2) 0;
  }
  /* Phase 122k §14 finding 5 — dashed separators between adjacent rows so the
     control groups read as discrete blocks. :global so Svelte does not prune it
     (each LeverRow template holds a single `.ctrl-row`; adjacency only happens at
     runtime across sibling instances) and so it matches across component
     boundaries. Scoped under `.ctrl-row`, which exists only in the strip. */
  :global(.ctrl-row + .ctrl-row) {
    border-top: 1px dashed color-mix(in srgb, var(--color-border) 50%, transparent);
  }

  /* Eyebrow + options cluster — :global so custom rows (composition, layer/
     compare) reuse them without re-declaring. Scoped under `.ctrl-row`. */
  :global(.ctrl-row .ctrl-eyebrow) {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--color-accent);
    font-weight: var(--font-weight-semibold);
    min-width: 3.5rem;
    flex-shrink: 0;
  }

  :global(.ctrl-row .ctrl-options) {
    display: inline-flex;
    flex-wrap: wrap;
    gap: var(--space-1);
  }

  /* Phase 131 — per-cell config rows. */
  :global(.ctrl-row.config-row) {
    align-items: center;
  }

  /* Shared slotted controls (Phase 141) — styled :global because the slot
     content carries the child component's scope hash, not this one. Scoped
     under `.ctrl-row`, which exists only inside the control strip. */
  :global(.ctrl-row .config-inline) {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    flex: 1 1 auto;
    min-width: 0;
  }
  :global(.ctrl-row .config-inline input[type='range']) {
    flex: 1 1 auto;
    max-width: 16rem;
    /* Phase 128 — WCAG 2.2 (2.5.8) 24px minimum target height for the slider. */
    min-height: 24px;
    accent-color: var(--color-accent);
    cursor: pointer;
  }
  :global(.ctrl-row .config-value) {
    font-family: var(--font-mono);
    font-size: var(--font-size-xs);
    color: var(--color-fg);
    min-width: 2.5ch;
    text-align: right;
  }
  :global(.ctrl-row .config-select) {
    appearance: none;
    background: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    padding: 3px var(--space-2);
    /* Phase 128 — WCAG 2.2 (2.5.8) 24px minimum target height. */
    min-height: 24px;
    font-size: var(--font-size-xs);
    /* Task B — friendly display labels (e.g. "Sentiment Score · BERT
       Multilingual"), so a proportional font reads better and fits more than the
       old monospace. Widen the closed select to show long labels in full; the
       native open list auto-sizes to the option text regardless. Ellipsis is a
       graceful last resort so an extreme label can never break the strip. */
    font-family: inherit;
    cursor: pointer;
    max-width: 22rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  :global(.ctrl-row .config-select:hover),
  :global(.ctrl-row .config-select:focus-visible) {
    border-color: var(--color-accent);
  }
  :global(.ctrl-row .field-empty) {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-style: italic;
  }
</style>
