<script lang="ts">
  // Right-edge methodology tray — Design Brief §3.3.
  //
  // Two states:
  //   Closed — narrow tab strip (--tray-closed-width) always visible at the
  //     right edge. Label "Methodology" rotated 90° + tier-badge slot.
  //     Minimized (lower opacity) on Surface I when no probe is selected.
  //   Open   — full-height panel (--tray-open-width) slides in from the right.
  //     Push-mode by default: sets --tray-right-edge on :root so the scope bar
  //     and Surface II content compress to make room. Falls back to overlay-mode
  //     below the PUSH_BREAKPOINT_PX threshold.
  //
  // Content binding ships in Phase 108. The panel body is a stub.
  //
  // A11y: the tray has role="complementary" with an accessible label; it is
  // not a dialog — it does not trap focus, because the surface behind it
  // remains live (Design Brief §4.1 rule 2 "no layer replaces").
  import type { Snippet } from 'svelte';
  import { urlState } from '$lib/state/url.svelte';
  import { focusedMetric } from '$lib/state/metric.svelte';

  interface Props {
    /** Slot for a tier-badge rendered inside the closed-state tab. */
    tierBadge?: Snippet;
  }

  let { tierBadge }: Props = $props();

  const PUSH_BREAKPOINT_PX = 900;

  let open = $state(false);
  const url = $derived(urlState());
  let probeSelected = $derived(url.probe !== null);
  let metric = $derived(focusedMetric());

  function isOverlayMode(): boolean {
    if (typeof window === 'undefined') return false;
    return window.innerWidth < PUSH_BREAKPOINT_PX;
  }

  $effect(() => {
    const inset = open && !isOverlayMode() ? 'var(--tray-open-width)' : 'var(--tray-closed-width)';
    document.documentElement.style.setProperty('--tray-right-edge', inset);
    return () => {
      document.documentElement.style.setProperty('--tray-right-edge', 'var(--tray-closed-width)');
    };
  });

  function toggle() {
    open = !open;
  }
</script>

<aside class="tray" class:tray-open={open} aria-label="Methodology">
  <!-- Tab strip — always visible -->
  <button
    type="button"
    class="tab"
    class:tab-dim={!probeSelected}
    aria-label="{open ? 'Close' : 'Open'} methodology tray"
    aria-expanded={open}
    onclick={toggle}
  >
    {#if tierBadge && probeSelected}
      <span class="badge-slot" aria-hidden="true">{@render tierBadge()}</span>
    {/if}
    <span class="tab-label">Methodology</span>
    <span class="tab-chevron" aria-hidden="true">{open ? '›' : '‹'}</span>
  </button>

  <!-- Open-state panel -->
  {#if open}
    <div class="panel" role="region" aria-label="Methodology content">
      <header class="panel-header">
        <span class="panel-title">Methodology</span>
        <button
          type="button"
          class="close-btn"
          aria-label="Close methodology tray"
          onclick={toggle}
        >
          ×
        </button>
      </header>
      <div class="panel-body">
        {#if metric}
          <p class="metric-focus">
            <span class="metric-label">Focused metric</span>
            <code class="metric-name">{metric.metricName}</code>
          </p>
        {/if}
        <p class="stub-notice">
          Content binding ships in Phase 108. This tray will surface per-metric provenance,
          validation tier, known limitations, and Working Paper links when a metric is in focus.
        </p>
      </div>
    </div>
  {/if}
</aside>

<style>
  .tray {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    z-index: 480;
    display: flex;
    flex-direction: row-reverse;
    pointer-events: none;
  }

  /* Tab strip (always rendered, receives pointer events) */
  .tab {
    pointer-events: auto;
    width: var(--tray-closed-width);
    background: var(--color-bg-elevated);
    border: none;
    border-left: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-2);
    cursor: pointer;
    color: var(--color-fg-muted);
    transition:
      background var(--motion-duration-fast) var(--motion-ease-standard),
      color var(--motion-duration-fast) var(--motion-ease-standard);
  }

  .tab:hover,
  .tab:focus-visible {
    background: var(--color-surface-hover);
    color: var(--color-fg);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  /* Minimized on Surface I when no probe is selected */
  .tab-dim {
    opacity: 0.35;
  }

  .badge-slot {
    writing-mode: horizontal-tb;
  }

  .tab-label {
    writing-mode: vertical-lr;
    text-orientation: mixed;
    transform: rotate(180deg);
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    user-select: none;
  }

  .tab-chevron {
    font-size: var(--font-size-base);
    writing-mode: horizontal-tb;
  }

  /* Open-state panel */
  .panel {
    pointer-events: auto;
    width: calc(var(--tray-open-width) - var(--tray-closed-width));
    background: var(--color-surface);
    border-left: 1px solid var(--color-border);
    box-shadow: var(--elevation-3);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    animation: slide-in var(--motion-duration-base) var(--motion-ease-emphasized);
  }

  @keyframes slide-in {
    from {
      transform: translateX(100%);
      opacity: 0;
    }
    to {
      transform: translateX(0);
      opacity: 1;
    }
  }

  .panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: var(--space-4) var(--space-3) var(--space-4) var(--space-4);
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
  }

  .panel-title {
    font-size: var(--font-size-xs);
    font-weight: var(--font-weight-semibold);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-muted);
  }

  .close-btn {
    background: transparent;
    border: none;
    color: var(--color-fg-muted);
    font-size: var(--font-size-xl);
    line-height: 1;
    cursor: pointer;
    padding: var(--space-1) var(--space-2);
    border-radius: var(--radius-sm);
  }

  .close-btn:hover,
  .close-btn:focus-visible {
    color: var(--color-fg);
    background: var(--color-surface-hover);
    outline: var(--focus-ring-width) solid var(--focus-ring-color);
    outline-offset: var(--focus-ring-offset);
  }

  .panel-body {
    flex: 1;
    overflow-y: auto;
    padding: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }

  .metric-focus {
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    padding: var(--space-3);
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    margin: 0;
  }

  .metric-label {
    font-size: 10px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--color-fg-subtle);
  }

  .metric-name {
    font-family: var(--font-mono);
    font-size: var(--font-size-sm);
    color: var(--color-fg);
  }

  .stub-notice {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    border: 1px dashed var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--space-4);
    margin: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .tab {
      transition: none;
    }

    .panel {
      animation: none;
    }
  }
</style>
