<script lang="ts">
  // Paused atmosphere: auto-rotate disabled, sun pinned to a deterministic
  // moment so visual snapshots are byte-stable across runs.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';

  // 2026-04-20T12:00:00Z — terminator over the Atlantic, Europe in daylight.
  const PINNED_SUN_MS = Date.UTC(2026, 3, 20, 12, 0, 0);
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Paused</title>
</svelte:head>

<AtmosphereCanvas sunOverrideMs={PINNED_SUN_MS} disableAutoRotate />
