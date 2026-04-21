<script lang="ts">
  // Synthetic-probes story: three probes across the globe at differing
  // activity levels so the reviewer can eyeball:
  //   - dormant probe sits at the brightness floor (no visible pulse)
  //   - moderate probe pulses gently
  //   - saturating probe pulses at the §1.1 clamp (never faster)
  // No live BFF call is made; this is a pure engine visual harness.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import type { AtmosphereEngine, ProbeMarker } from '@aer/engine-3d';

  const PROBES: readonly ProbeMarker[] = [
    {
      id: 'synth-de',
      language: 'de',
      emissionPoints: [{ latitude: 52.52, longitude: 13.405, label: 'Synthetic DE' }]
    },
    {
      id: 'synth-jp',
      language: 'ja',
      emissionPoints: [{ latitude: 35.6762, longitude: 139.6503, label: 'Synthetic JP' }]
    },
    {
      id: 'synth-br',
      language: 'pt',
      emissionPoints: [{ latitude: -23.5505, longitude: -46.6333, label: 'Synthetic BR' }]
    }
  ];

  const ACTIVITY = [
    { probeId: 'synth-de', documentsPerHour: 50 }, // saturating
    { probeId: 'synth-jp', documentsPerHour: 3 }, // moderate
    { probeId: 'synth-br', documentsPerHour: 0 } // dormant
  ];

  function onready(engine: AtmosphereEngine) {
    engine.setProbes(PROBES);
    engine.setActivity(ACTIVITY);
  }
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Synthetic probes</title>
</svelte:head>

<AtmosphereCanvas {onready} />

<div class="hud">
  <strong>Synthetic probes</strong>
  <ul>
    <li>DE — 50 docs/h (saturating pulse)</li>
    <li>JP — 3 docs/h (moderate pulse)</li>
    <li>BR — 0 docs/h (dormant, floor brightness)</li>
  </ul>
</div>

<style>
  .hud {
    position: fixed;
    bottom: var(--space-5);
    left: var(--space-5);
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
  ul {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: var(--space-1);
    color: var(--color-fg-muted);
  }
</style>
