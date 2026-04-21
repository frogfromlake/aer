<script lang="ts">
  // Propagation-slot story: pushes synthetic cross-probe propagation
  // events through `setPropagationEvents()` to exercise the shader slot
  // reserved for Phase 99b. No arcs are rendered — the placeholder
  // shader `discard`s every fragment (see propagation.glsl) — so this
  // story only verifies that the API path is wired and does not crash
  // the engine. A later phase will replace the placeholder with a real
  // arc program.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import type { AtmosphereEngine, ProbeMarker, PropagationEvent } from '@aer/engine-3d';

  const PROBES: readonly ProbeMarker[] = [
    {
      id: 'synth-de',
      language: 'de',
      emissionPoints: [{ latitude: 52.52, longitude: 13.405, label: 'Synthetic DE' }]
    },
    {
      id: 'synth-us',
      language: 'en',
      emissionPoints: [{ latitude: 40.7128, longitude: -74.006, label: 'Synthetic US' }]
    }
  ];

  const now = Date.now();
  const EVENTS: readonly PropagationEvent[] = [
    { fromProbeId: 'synth-de', toProbeId: 'synth-us', atUnixMs: now - 60_000 },
    { fromProbeId: 'synth-us', toProbeId: 'synth-de', atUnixMs: now - 30_000 }
  ];

  function onready(engine: AtmosphereEngine) {
    engine.setProbes(PROBES);
    engine.setActivity([
      { probeId: 'synth-de', documentsPerHour: 8 },
      { probeId: 'synth-us', documentsPerHour: 8 }
    ]);
    engine.setPropagationEvents(EVENTS);
  }
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Propagation slot</title>
</svelte:head>

<AtmosphereCanvas {onready} disableAutoRotate />

<div class="hud" role="note">
  <strong>Propagation slot</strong>
  <p>
    Two synthetic probes, two synthetic events fed through
    <code>setPropagationEvents()</code>. The shader slot is reserved but the program currently
    discards every fragment — no arcs are rendered.
  </p>
</div>

<style>
  .hud {
    position: fixed;
    bottom: var(--space-5);
    left: var(--space-5);
    max-width: 44ch;
    padding: var(--space-3) var(--space-4);
    background: rgba(0, 0, 0, 0.6);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    backdrop-filter: blur(8px);
    color: var(--color-fg);
    font-size: var(--font-size-sm);
  }
  strong {
    display: block;
    margin-bottom: var(--space-2);
  }
  p {
    margin: 0;
    color: var(--color-fg-muted);
  }
  code {
    font-family: var(--font-family-mono);
    font-size: 0.9em;
    color: var(--color-fg);
  }
</style>
