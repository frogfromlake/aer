<script lang="ts">
  import MethodologyTray from '$lib/components/chrome/MethodologyTray.svelte';
  import { setFocusedMetric } from '$lib/state/metric.svelte';
  import {
    negativeSpaceActive,
    setNegativeSpaceActive,
    setTrayOpen,
    trayOpen
  } from '$lib/state/tray.svelte';

  const open = $derived(trayOpen());
  const negSpace = $derived(negativeSpaceActive());
</script>

<div class="story">
  <h2>MethodologyTray</h2>
  <p class="desc">
    Right-edge methodology tray (Design Brief §3.3, §5.7). Phase 108 binds the tray to live
    /content/metric and /metrics/{name}/provenance feeds. Closed state shows the live tier badge and
    a known-limitations indicator dot when applicable. Open state renders the Dual-Register
    methodological content with a "Read the full Working Paper" deep link. Overlay-mode fallback
    below 900 px.
  </p>

  <div class="controls" role="group" aria-label="Story controls">
    <button type="button" onclick={() => setTrayOpen(!open)}>
      {open ? 'Close' : 'Open'} tray
    </button>
    <button type="button" onclick={() => setFocusedMetric({ metricName: 'sentiment_score' })}>
      Focus: sentiment_score
    </button>
    <button type="button" onclick={() => setFocusedMetric({ metricName: 'word_count' })}>
      Focus: word_count
    </button>
    <button
      type="button"
      onclick={() =>
        setFocusedMetric({
          metricName: 'sentiment_score',
          chartContext: 'probe-0 · 2026-04-22T14:00Z'
        })}
    >
      Focus + chart context
    </button>
    <button type="button" onclick={() => setFocusedMetric(null)}>Clear focus</button>
    <label class="toggle">
      <input
        type="checkbox"
        checked={negSpace}
        onchange={(e) => setNegativeSpaceActive((e.currentTarget as HTMLInputElement).checked)}
      />
      Negative Space (limitations-first)
    </label>
  </div>

  <p class="note">
    The tray is fixed-position in production. In this story it renders at the right edge of the
    story frame — interact with the tab on the far right to toggle open/closed.
  </p>

  <div class="states">
    <div class="state-card">
      <h3>Closed state (no probe)</h3>
      <p>Tab strip visible at right edge; opacity reduced (no probe selected).</p>
    </div>
    <div class="state-card">
      <h3>Open state</h3>
      <p>
        Click the "‹ Methodology" tab at the right edge to open the panel. Push-mode applies when
        viewport ≥ 900 px.
      </p>
    </div>
  </div>

  <div class="a11y">
    <h3>A11y</h3>
    <ul>
      <li>aside[role="complementary"][aria-label="Methodology"] — not a dialog; no focus trap</li>
      <li>
        Tab button: aria-expanded reflects open state; aria-label changes between "Open" / "Close"
      </li>
      <li>Panel region: role="region" aria-label="Methodology content"</li>
      <li>Reduced-motion: slide-in animation suppressed</li>
    </ul>
  </div>
</div>

<!-- Tray is rendered at story level (fixed to the right edge of the viewport) -->
<MethodologyTray />

<style>
  .story {
    display: flex;
    flex-direction: column;
    gap: var(--space-5);
    max-width: 52ch;
  }
  h2 {
    font-size: var(--font-size-xl);
    margin: 0;
  }
  h3 {
    font-size: var(--font-size-base);
    margin: 0 0 var(--space-2);
  }
  .desc,
  .note {
    color: var(--color-fg-muted);
    font-size: var(--font-size-sm);
    line-height: var(--line-height-loose);
    margin: 0;
  }
  .note {
    border-left: 2px solid var(--color-border-strong);
    padding-left: var(--space-3);
    color: var(--color-fg-subtle);
  }
  .states {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  .state-card {
    padding: var(--space-4);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
  }
  .state-card p {
    margin: 0;
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-base);
  }
  .controls {
    display: flex;
    flex-wrap: wrap;
    gap: var(--space-2);
    align-items: center;
  }
  .controls button {
    padding: 4px 10px;
    background: var(--color-bg-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-fg);
    font-size: var(--font-size-xs);
    cursor: pointer;
  }
  .toggle {
    display: inline-flex;
    align-items: center;
    gap: var(--space-1);
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
  }
  .a11y {
    display: flex;
    flex-direction: column;
    gap: var(--space-3);
  }
  ul {
    margin: 0;
    padding-left: var(--space-4);
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-base);
  }
</style>
