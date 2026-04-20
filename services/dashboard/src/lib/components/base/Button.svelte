<script lang="ts">
  import type { HTMLButtonAttributes } from 'svelte/elements';
  import type { Snippet } from 'svelte';

  type Variant = 'primary' | 'secondary' | 'ghost';
  type Size = 'sm' | 'md';

  interface Props extends HTMLButtonAttributes {
    variant?: Variant;
    size?: Size;
    loading?: boolean;
    children?: Snippet;
  }

  let {
    variant = 'primary',
    size = 'md',
    loading = false,
    disabled,
    type = 'button',
    children,
    ...rest
  }: Props = $props();
</script>

<button
  {type}
  class="btn btn-{variant} btn-{size}"
  class:loading
  disabled={disabled || loading}
  aria-busy={loading || undefined}
  {...rest}
>
  {#if loading}
    <span class="spinner" aria-hidden="true"></span>
  {/if}
  <span class="label" class:label-hidden={loading}>
    {#if children}{@render children()}{/if}
  </span>
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    position: relative;
    font-family: var(--font-ui);
    font-weight: var(--font-weight-medium);
    line-height: 1;
    border-radius: var(--radius-md);
    border: 1px solid transparent;
    cursor: pointer;
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .btn:disabled {
    cursor: not-allowed;
    opacity: 0.55;
  }

  .btn-sm {
    font-size: var(--font-size-sm);
    padding: var(--space-2) var(--space-3);
    min-height: 28px;
  }

  .btn-md {
    font-size: var(--font-size-base);
    padding: var(--space-3) var(--space-4);
    min-height: 36px;
  }

  .btn-primary {
    background: var(--color-accent);
    color: var(--color-bg);
    border-color: var(--color-accent);
  }
  .btn-primary:hover:not(:disabled) {
    background: color-mix(in oklab, var(--color-accent) 88%, white);
  }

  .btn-secondary {
    background: transparent;
    color: var(--color-fg);
    border-color: var(--color-border-strong);
  }
  .btn-secondary:hover:not(:disabled) {
    background: var(--color-surface-hover);
    border-color: var(--color-accent);
  }

  .btn-ghost {
    background: transparent;
    color: var(--color-fg-muted);
    border-color: transparent;
  }
  .btn-ghost:hover:not(:disabled) {
    background: var(--color-surface-hover);
    color: var(--color-fg);
  }

  .label-hidden {
    visibility: hidden;
  }

  .spinner {
    position: absolute;
    width: 14px;
    height: 14px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: spin var(--motion-duration-stately) linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner {
      animation: none;
    }
  }
</style>
