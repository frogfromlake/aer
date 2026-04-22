<script lang="ts">
  // Terminator-positions story: cycles the sun through four canonical UTC times
  // so reviewers can eyeball the day/night band at solstice + equinox extremes.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';

  const positions = [
    { label: 'Equinox · 06:00 UTC', ms: Date.UTC(2026, 2, 20, 6, 0, 0) },
    { label: 'Equinox · 18:00 UTC', ms: Date.UTC(2026, 2, 20, 18, 0, 0) },
    { label: 'Summer solstice · 12:00 UTC', ms: Date.UTC(2026, 5, 21, 12, 0, 0) },
    { label: 'Winter solstice · 00:00 UTC', ms: Date.UTC(2026, 11, 21, 0, 0, 0) }
  ] as const;

  let index = $state(0);
  const current = $derived(positions[index]!);
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Terminator</title>
</svelte:head>

<AtmosphereCanvas sunOverrideMs={current.ms} />

<div class="hud" role="group" aria-label="Sun position selector">
  {#each positions as pos, i (pos.ms)}
    <button
      type="button"
      onclick={() => (index = i)}
      aria-pressed={index === i}
      class:active={index === i}
    >
      {pos.label}
    </button>
  {/each}
</div>

<style>
  .hud {
    position: fixed;
    bottom: var(--space-5);
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    gap: var(--space-2);
    padding: var(--space-2);
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    backdrop-filter: blur(8px);
  }
  button {
    padding: var(--space-1) var(--space-3);
    font-size: var(--font-size-xs);
    background: transparent;
    color: var(--color-fg-muted);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
  }
  button.active {
    background: var(--color-surface);
    color: var(--color-fg);
    border-color: var(--color-accent);
  }
</style>
