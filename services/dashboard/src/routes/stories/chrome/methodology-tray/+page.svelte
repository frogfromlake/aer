<script lang="ts">
  import MethodologyTray from '$lib/components/chrome/MethodologyTray.svelte';
</script>

<div class="story">
  <h2>MethodologyTray</h2>
  <p class="desc">
    Right-edge methodology tray shell (Design Brief §3.3). Content binding ships in Phase 108.
    Closed state: narrow tab strip with rotated "Methodology" label. Open state: full-height panel
    in push-mode (narrows scope bar + surface content). Overlay-mode fallback below 900 px.
  </p>

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
