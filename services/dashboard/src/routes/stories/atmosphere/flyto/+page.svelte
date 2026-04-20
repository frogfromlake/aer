<script lang="ts">
  // Fly-to story: drives the imperative `flyTo` API across a few major cities.
  // Validates camera lerp + the engine's onready hook for story-level control.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import type { AtmosphereEngine } from '@aer/engine-3d';

  const targets = [
    { label: 'Berlin', latitude: 52.52, longitude: 13.405 },
    { label: 'Tokyo', latitude: 35.6762, longitude: 139.6503 },
    { label: 'São Paulo', latitude: -23.5505, longitude: -46.6333 },
    { label: 'Sydney', latitude: -33.8688, longitude: 151.2093 }
  ] as const;

  let engine: AtmosphereEngine | null = $state(null);

  function onready(e: AtmosphereEngine) {
    engine = e;
  }

  function go(target: (typeof targets)[number]) {
    engine?.flyTo({ latitude: target.latitude, longitude: target.longitude, durationMs: 1500 });
  }
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Fly-to</title>
</svelte:head>

<AtmosphereCanvas {onready} disableAutoRotate />

<div class="hud" role="group" aria-label="Fly to city">
  {#each targets as target (target.label)}
    <button type="button" onclick={() => go(target)} disabled={engine === null}>
      {target.label}
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
  button:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  button:not(:disabled):hover {
    background: var(--color-surface-hover);
    color: var(--color-fg);
  }
</style>
