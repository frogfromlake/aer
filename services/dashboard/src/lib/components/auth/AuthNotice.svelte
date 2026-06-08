<script lang="ts">
  // Inline notice for the auth forms — error / success / info. Uses the
  // methodological status colours (neutral, never alarming red).
  import type { Snippet } from 'svelte';

  interface Props {
    variant?: 'error' | 'success' | 'info';
    children: Snippet;
  }

  let { variant = 'error', children }: Props = $props();
</script>

<div class="notice {variant}" role={variant === 'error' ? 'alert' : 'status'} aria-live="polite">
  {@render children()}
</div>

<style>
  .notice {
    font-size: var(--font-size-sm);
    line-height: var(--line-height-base);
    padding: var(--space-3) var(--space-3);
    border-radius: var(--radius-md);
    border: 1px solid;
  }
  .error {
    color: var(--color-status-expired);
    border-color: color-mix(in oklab, var(--color-status-expired) 45%, transparent);
    background: color-mix(in oklab, var(--color-status-expired) 10%, transparent);
  }
  .success {
    color: var(--color-status-validated);
    border-color: color-mix(in oklab, var(--color-status-validated) 45%, transparent);
    background: color-mix(in oklab, var(--color-status-validated) 10%, transparent);
  }
  .info {
    color: var(--color-fg-muted);
    border-color: var(--color-border);
    background: var(--color-bg);
  }
</style>
