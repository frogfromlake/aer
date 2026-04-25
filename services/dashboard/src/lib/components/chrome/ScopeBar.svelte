<script lang="ts">
  // Top scope bar — Design Brief §3.2.
  // Slot-based horizontal strip accepting per-surface navigation content:
  //   Surface I  — time window label + resolution selector + Neg-Space toggle
  //   Surface II — lane switcher (Phase 106)
  //   Surface III — section anchor / reading progress (Phase 109)
  //
  // Positioned left of the side rail and right of the tray's right edge.
  // The `--tray-right-edge` custom property (updated by MethodologyTray when
  // the tray opens in push-mode) governs the right inset so scope bar and
  // tray never overlap.
  import type { Snippet } from 'svelte';

  interface Props {
    children?: Snippet;
    /** Accessible label for the navigation region. */
    label?: string;
  }

  let { children, label = 'Surface navigation' }: Props = $props();
</script>

<div class="scope-bar" role="navigation" aria-label={label}>
  <div class="inner">
    {#if children}{@render children()}{/if}
  </div>
</div>

<style>
  .scope-bar {
    position: fixed;
    top: 0;
    left: var(--rail-width);
    right: var(--tray-right-edge, var(--tray-closed-width));
    z-index: 440;
    background: var(--color-bg-overlay);
    backdrop-filter: blur(8px);
    -webkit-backdrop-filter: blur(8px);
    border-bottom: 1px solid var(--color-border);
    min-height: var(--scope-bar-height);
  }

  .inner {
    display: flex;
    align-items: center;
    gap: var(--space-3);
    padding: 0 var(--space-4);
    min-height: var(--scope-bar-height);
    flex-wrap: wrap;
  }
</style>
