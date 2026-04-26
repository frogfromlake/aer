<script lang="ts">
  // View-Mode Switcher (Phase 107).
  // Slots into the Surface II top scope bar. Selection is URL-backed
  // via `?viewMode=…`; deep-link restores the active cell. Switching is
  // a single replaceState write — no page navigation, so the lane shell
  // re-renders the active cell in place.
  import { listPresentations, getPresentation } from '$lib/viewmodes';
  import type { ViewMode } from '$lib/state/url-internals';
  import { setUrl, urlState } from '$lib/state/url.svelte';

  const presentations = listPresentations();
  const url = $derived(urlState());
  let active = $derived<ViewMode>(getPresentation(url.viewMode).id);

  function pick(id: ViewMode) {
    setUrl({ viewMode: id });
  }
</script>

<div class="switcher" role="group" aria-label="View mode">
  <span class="label" aria-hidden="true">View</span>
  {#each presentations as p (p.id)}
    <button
      type="button"
      class="opt"
      class:active={active === p.id}
      aria-pressed={active === p.id}
      title={p.description}
      onclick={() => pick(p.id)}>{p.label}</button
    >
  {/each}
</div>

<style>
  .switcher {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-shrink: 0;
    margin-left: auto;
  }

  .label {
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
    margin-right: var(--space-2);
  }

  .opt {
    appearance: none;
    background: transparent;
    border: 1px solid transparent;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xs);
    padding: 3px var(--space-2);
    border-radius: var(--radius-sm);
    cursor: pointer;
    transition:
      background-color var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard),
      border-color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .opt:hover,
  .opt:focus-visible {
    color: var(--color-fg);
    border-color: var(--color-border);
    background: var(--color-surface);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .opt.active {
    color: var(--color-fg);
    background: var(--color-surface);
    border-color: var(--color-accent-muted);
  }

  @media (prefers-reduced-motion: reduce) {
    .opt {
      transition: none;
    }
  }
</style>
