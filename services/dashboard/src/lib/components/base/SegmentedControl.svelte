<script lang="ts" generics="T extends string">
  // Compact, always-visible segmented control. Used where a single
  // choice from a small, stable set should be readable without opening
  // a menu — metric selection at L3, register toggling in
  // ProgressiveSemantics, resolution selection (future). Keeps the
  // chromeline horizontal and the choices legible at a glance, which
  // the Design Brief §4.3 calls for at L3.
  //
  // The component is generic over the option id type so callers get
  // end-to-end type safety (T = 'sentiment_score' | 'word_count' | …).
  interface Option {
    id: T;
    label: string;
    hint?: string;
  }

  interface Props {
    options: readonly Option[];
    value: T;
    onChange: (next: T) => void;
    ariaLabel: string;
    /** Optional label rendered before the segments (e.g. "Metric"). */
    label?: string;
    size?: 'sm' | 'md';
  }

  let { options, value, onChange, ariaLabel, label, size = 'md' }: Props = $props();
</script>

<div class="segmented size-{size}" role="radiogroup" aria-label={ariaLabel}>
  {#if label}
    <span class="group-label">{label}</span>
  {/if}
  <div class="track">
    {#each options as opt (opt.id)}
      <button
        type="button"
        role="radio"
        aria-checked={value === opt.id}
        class:active={value === opt.id}
        title={opt.hint}
        onclick={() => onChange(opt.id)}
      >
        {opt.label}
      </button>
    {/each}
  </div>
</div>

<style>
  .segmented {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
  }
  .group-label {
    color: var(--color-fg-subtle);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-size: 10px;
  }
  .track {
    display: inline-flex;
    align-items: stretch;
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: 2px;
    gap: 2px;
  }
  button {
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px solid transparent;
    border-radius: calc(var(--radius-md) - 2px);
    cursor: pointer;
    font-family: inherit;
    line-height: 1.1;
    letter-spacing: 0.01em;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }
  .size-sm button {
    padding: 3px 8px;
    font-size: 11px;
  }
  .size-md button {
    padding: 6px var(--space-3);
    font-size: var(--font-size-sm);
  }
  button:hover {
    color: var(--color-fg);
    background: rgba(255, 255, 255, 0.04);
  }
  button:focus-visible {
    outline: 2px solid var(--color-accent);
    outline-offset: 1px;
  }
  button.active {
    background: rgba(82, 131, 184, 0.22);
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
</style>
