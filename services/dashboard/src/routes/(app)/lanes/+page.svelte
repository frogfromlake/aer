<script lang="ts">
  // Surface II — Function Lanes stub (Phase 105).
  // Full implementation ships in Phase 106 (Probe Dossier + Function-Lane Shell).
  import { ScopeBar } from '$lib/components/chrome';
</script>

<svelte:head>
  <title>AĒR — Function Lanes</title>
</svelte:head>

<ScopeBar label="Function Lanes surface navigation">
  <span class="surface-label">Function Lanes</span>
  <span class="phase-note">Surface II · Phase 106</span>
</ScopeBar>

<main class="stub" id="main-lanes">
  <div class="stub-card">
    <h1 class="stub-title">Function Lanes</h1>
    <p class="stub-body">
      Surface II lands in Phase 106. It will present the four discourse functions from WP-001 as
      horizontal time-series lanes, seeded from the Probe Dossier.
    </p>
  </div>
</main>

<style>
  .stub {
    position: fixed;
    inset: 0;
    left: var(--rail-width);
    top: var(--scope-bar-height);
    right: var(--tray-right-edge, var(--tray-closed-width));
    display: grid;
    place-items: center;
    background: var(--color-bg);
  }
  .stub-card {
    max-width: 28rem;
    padding: var(--space-6);
    background: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    gap: var(--space-4);
  }
  .stub-title {
    font-size: var(--font-size-xl);
    margin: 0;
    color: var(--color-fg);
  }
  .stub-body {
    font-size: var(--font-size-sm);
    color: var(--color-fg-muted);
    line-height: var(--line-height-loose);
    margin: 0;
  }
  .surface-label {
    font-size: var(--font-size-sm);
    font-weight: var(--font-weight-medium);
    color: var(--color-fg);
  }
  .phase-note {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    font-family: var(--font-mono);
  }
</style>
