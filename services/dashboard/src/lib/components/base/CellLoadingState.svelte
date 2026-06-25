<script lang="ts">
  // Phase 149 — the ONE Workbench-wide loading state. Every cell (all pillars, all
  // presentations) and the panel-grid/host loading branches render this instead of
  // a bare "loading…" line, so the wait reads the same everywhere.
  //
  // The motif is atmospheric (AĒR = ἀήρ, air): a soft accent-tinted core that
  // BREATHES (scale + opacity) while concentric rings pulse outward and fade — a
  // calm "sensor listening" pulse, not a busy spinner. `prefers-reduced-motion`
  // collapses it to a still set of concentric rings (no transform, no movement).
  //
  // `label` is the cell's own localized "loading …" text; it is rendered muted
  // below the motif AND carried to assistive tech via role="status" (the motif
  // itself is aria-hidden decoration).
  let { label }: { label?: string } = $props();
</script>

<div class="cell-loading" role="status" aria-live="polite" aria-busy="true">
  <span class="breath" aria-hidden="true">
    <span class="ring"></span>
    <span class="ring"></span>
    <span class="ring"></span>
    <span class="core"></span>
  </span>
  {#if label}<span class="cell-loading-label">{label}</span>{/if}
</div>

<style>
  .cell-loading {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--space-3);
    width: 100%;
    height: 100%;
    min-height: 8rem;
    flex: 1;
    color: var(--color-fg-muted);
  }

  /* Stack the rings + core on one centred grid cell so they share an origin. */
  .breath {
    display: grid;
    place-items: center;
    width: 56px;
    height: 56px;
  }
  .breath > * {
    grid-area: 1 / 1;
    border-radius: 50%;
  }

  .core {
    width: 14px;
    height: 14px;
    background: var(--color-accent);
    box-shadow: 0 0 10px 2px color-mix(in srgb, var(--color-accent) 45%, transparent);
    animation: aer-breathe 3s ease-in-out infinite;
  }

  .ring {
    width: 52px;
    height: 52px;
    border: 2px solid var(--color-accent);
    opacity: 0;
    animation: aer-pulse 3s ease-out infinite;
  }
  /* Stagger the three rings across the breath so a new ring leaves as the last
     one fades — a continuous, calm exhale. */
  .ring:nth-child(1) {
    animation-delay: 0s;
  }
  .ring:nth-child(2) {
    animation-delay: 1s;
  }
  .ring:nth-child(3) {
    animation-delay: 2s;
  }

  @keyframes aer-breathe {
    0%,
    100% {
      transform: scale(0.8);
      opacity: 0.55;
    }
    50% {
      transform: scale(1.12);
      opacity: 1;
    }
  }
  @keyframes aer-pulse {
    0% {
      transform: scale(0.34);
      opacity: 0;
    }
    18% {
      opacity: 0.55;
    }
    100% {
      transform: scale(1.7);
      opacity: 0;
    }
  }

  .cell-loading-label {
    font-size: var(--font-size-xs);
    color: var(--color-fg-muted);
    letter-spacing: 0.02em;
  }

  /* Calm fallback — no movement, just a still set of concentric rings + core. */
  @media (prefers-reduced-motion: reduce) {
    .core,
    .ring {
      animation: none;
      transform: scale(1);
    }
    .core {
      opacity: 0.85;
    }
    .ring:nth-child(1) {
      opacity: 0.1;
    }
    .ring:nth-child(2) {
      opacity: 0.22;
      width: 36px;
      height: 36px;
    }
    .ring:nth-child(3) {
      opacity: 0.3;
      width: 22px;
      height: 22px;
    }
  }
</style>
