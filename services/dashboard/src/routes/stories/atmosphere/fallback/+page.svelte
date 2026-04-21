<script lang="ts">
  // Fallback story: renders the WebGLFallback unconditionally with synthetic
  // probe + activity data, so the a11y gate can audit it without needing to
  // disable WebGL2 in the browser. The fixture mirrors Probe 0's shape
  // (Hamburg + Berlin emission points) plus two synthetic probes.
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';
  import type { ProbeActivity } from '@aer/engine-3d';
  import type { ProbeDto } from '$lib/api/queries';

  const probes: ProbeDto[] = [
    {
      probeId: 'probe-0-de-institutional-rss',
      language: 'de',
      sources: ['tagesschau', 'zeit'],
      emissionPoints: [
        { latitude: 53.5511, longitude: 9.9937, label: 'Hamburg' },
        { latitude: 52.52, longitude: 13.405, label: 'Berlin' }
      ]
    },
    {
      probeId: 'probe-fr-synthetic',
      language: 'fr',
      sources: ['lemonde'],
      emissionPoints: [{ latitude: 48.8566, longitude: 2.3522, label: 'Paris' }]
    },
    {
      probeId: 'probe-en-synthetic',
      language: 'en',
      sources: ['bbc'],
      emissionPoints: [{ latitude: 51.5074, longitude: -0.1278, label: 'London' }]
    }
  ];

  const activity: ProbeActivity[] = [
    { probeId: 'probe-0-de-institutional-rss', documentsPerHour: 12.3 },
    { probeId: 'probe-fr-synthetic', documentsPerHour: 4.5 },
    { probeId: 'probe-en-synthetic', documentsPerHour: 0 }
  ];
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Fallback</title>
</svelte:head>

<div class="centered">
  <WebGLFallback {probes} {activity} />
</div>

<style>
  .centered {
    min-height: 100dvh;
    display: grid;
    place-items: center;
  }
</style>
