<script lang="ts">
  // ReflectionBackLink — an always-reachable "back to Reflection" control
  // (Phase 127). Long Reflection pages buried the bottom-of-page back link below
  // the fold; placed as the first child of a page's scroll *column* (sibling of
  // the centred reading container) it sticks to the top-left gutter, so a reader
  // who descended into a metric, probe, paper, or question can step back out from
  // anywhere — without scrolling to the end and without hovering over the text.
  /* eslint-disable svelte/no-navigation-without-resolve -- internal reflection navigation */
  import { m } from '$lib/paraglide/messages.js';

  interface Props {
    href?: string;
    label?: string;
  }
  let { href = '/reflection', label = m.reflection_back_label() }: Props = $props();
</script>

<a class="reflection-back" {href} title={label}>
  <span class="arrow" aria-hidden="true">←</span>
  <span class="text">{label}</span>
</a>

<style>
  .reflection-back {
    position: sticky;
    top: var(--space-3);
    z-index: 5;
    margin-left: var(--space-4);
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-2) var(--space-3);
    background: color-mix(in srgb, var(--color-bg-elevated) 92%, transparent);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    text-decoration: none;
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    transition: all var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .reflection-back:hover,
  .reflection-back:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-accent-muted);
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .arrow {
    font-size: var(--font-size-sm);
    line-height: 1;
  }

  @media (prefers-reduced-motion: reduce) {
    .reflection-back {
      transition: none;
    }
  }
</style>
