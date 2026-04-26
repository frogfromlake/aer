<script lang="ts">
  // Probe 0 story: renders `probe-0-de-institutional-rss` with its two
  // emission points (Hamburg — Tagesschau/NDR; Berlin — Bundesregierung/BPA).
  // Activity is fixed to mimic a busy news cycle so the pulse is legible
  // in a static screenshot while still honouring the §1.1 pulse clamp.
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import type { AtmosphereEngine, ProbeMarker, ProbeSelection } from '@aer/engine-3d';

  const PROBE_0: ProbeMarker = {
    id: 'probe-0-de-institutional-rss',
    language: 'de',
    label: 'Probe 0 — DE institutional RSS',
    emissionPoints: [
      {
        latitude: 53.5511,
        longitude: 9.9937,
        label: 'Hamburg (Tagesschau / NDR)',
        sourceName: 'tagesschau'
      },
      {
        latitude: 52.517,
        longitude: 13.3888,
        label: 'Berlin (Bundesregierung / BPA)',
        sourceName: 'bundesregierung'
      }
    ]
  };

  let hovered: ProbeSelection | null = $state(null);
  let selected: ProbeSelection | null = $state(null);

  function onready(engine: AtmosphereEngine) {
    engine.setProbes([PROBE_0]);
    // Saturating activity so the pulse is obvious at story load.
    engine.setActivity([{ probeId: PROBE_0.id, documentsPerHour: 12 }]);
    engine.on('probe-hovered', (sel) => {
      hovered = sel;
    });
    engine.on('probe-selected', (sel) => {
      selected = sel;
    });
    // Park the camera over central Europe so the probe glyph + satellites are in-frame.
    engine.flyTo({ latitude: 53, longitude: 11, durationMs: 800 });
  }
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Probe 0</title>
</svelte:head>

<AtmosphereCanvas {onready} />

<div class="hud" role="status" aria-live="polite">
  <strong>Probe 0 — DE institutional RSS</strong>
  <dl>
    <dt>Hovered</dt>
    <dd>{hovered?.probeId ?? '—'}</dd>
    <dt>Selected</dt>
    <dd>{selected?.probeId ?? '—'}</dd>
  </dl>
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
  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--space-1) var(--space-3);
    margin: 0;
  }
  dt {
    color: var(--color-fg-muted);
  }
  dd {
    margin: 0;
  }
</style>
