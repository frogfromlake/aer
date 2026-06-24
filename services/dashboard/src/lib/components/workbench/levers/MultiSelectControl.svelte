<script lang="ts">
  // MultiSelectControl — Phase 148g. A compact multi-select rendered as a
  // collapsed dropdown (a <details> disclosure styled like the sibling
  // `.config-select` single-selects) instead of an always-expanded row of
  // checkbox chips: the chips wrapped onto several lines and broke the control
  // strip's rhythm. Closed, it shows the selected count + labels on one line;
  // opened, it reveals the checkbox list. Used for the N-metric set
  // (correlation matrix / parallel coordinates).
  //
  // Kept presentational + label-agnostic: the caller passes the option ids, the
  // current selection, a label resolver, and a per-id toggle, so the same widget
  // serves metrics or fields.
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    /** Option ids in display order. */
    options: string[];
    /** Currently-selected ids. */
    selected: readonly string[];
    /** id → human label. */
    label: (id: string) => string;
    /** Toggle one id in/out of the selection. */
    onToggle: (id: string) => void;
    /** Accessible name for the dropdown. */
    ariaLabel: string;
    /** Shown in the panel when there are no options at all. */
    emptyText?: string;
  }

  let { options, selected, label, onToggle, ariaLabel, emptyText }: Props = $props();

  const selectedLabels = $derived(options.filter((o) => selected.includes(o)).map(label));
</script>

<!-- role="presentation" wrapper carries the stopPropagation (the control strip
     toggles collapse on click) so the interactive <details>/<summary> stays free
     of mouse handlers — mirrors the sibling lever pattern. -->
<div class="ms-wrap" role="presentation" onclick={(e) => e.stopPropagation()}>
  <details class="ms-dropdown">
    <summary class="ms-summary" aria-label={ariaLabel}>
      <span class="ms-summary-text">
        {#if selectedLabels.length > 0}
          <span class="ms-count"
            >{m.levers_multiselect_selected({ count: selectedLabels.length })}</span
          >{selectedLabels.join(' | ')}
        {:else}
          {m.levers_multiselect_none()}
        {/if}
      </span>
      <span class="ms-chevron" aria-hidden="true">▾</span>
    </summary>
    <div class="ms-panel" role="group" aria-label={ariaLabel}>
      {#if options.length === 0}
        <span class="field-empty">{emptyText ?? m.levers_multiselect_none()}</span>
      {/if}
      {#each options as o (o)}
        <label class="ms-option" class:active={selected.includes(o)}>
          <input type="checkbox" checked={selected.includes(o)} onchange={() => onToggle(o)} />
          <code>{label(o)}</code>
        </label>
      {/each}
    </div>
  </details>
</div>

<style>
  /* Sized + bordered like `.config-select` so the dropdown reads as one of the
     standard lever controls. */
  .ms-wrap {
    display: flex;
    min-width: 0;
    flex: 1 1 auto;
  }
  .ms-dropdown {
    min-width: 0;
    flex: 1 1 auto;
  }
  .ms-summary {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    cursor: pointer;
    list-style: none;
    padding: 2px var(--space-2);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    background: var(--color-surface);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    min-height: 1.6rem;
  }
  .ms-summary::-webkit-details-marker {
    display: none;
  }
  .ms-summary:hover {
    border-color: color-mix(in srgb, var(--color-accent) 50%, var(--color-border));
  }
  .ms-summary:focus-visible {
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }
  .ms-summary-text {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .ms-count {
    font-family: var(--font-mono);
    color: var(--color-viridis-50);
    margin-right: var(--space-1);
  }
  .ms-chevron {
    flex-shrink: 0;
    color: var(--color-fg-muted);
    transition: transform var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .ms-dropdown[open] .ms-chevron {
    transform: rotate(180deg);
  }
  @media (prefers-reduced-motion: reduce) {
    .ms-chevron {
      transition: none;
    }
  }
  /* In-flow panel (expands the strip rather than overlaying) — robust against
     any clipping ancestor in the control strip. Collapsed by default, so the
     options no longer occupy vertical space until the reader opens them. */
  .ms-panel {
    display: flex;
    flex-direction: column;
    gap: 2px;
    margin-top: var(--space-1);
    padding: var(--space-1);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    max-height: 14rem;
    overflow-y: auto;
  }
  .ms-option {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: 2px var(--space-1);
    border-radius: var(--radius-sm);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .ms-option:hover {
    background: color-mix(in srgb, var(--color-fg) 5%, transparent);
  }
  .ms-option.active code {
    color: var(--color-accent);
  }
  .ms-option code {
    font-family: var(--font-mono);
  }
  .field-empty {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
</style>
