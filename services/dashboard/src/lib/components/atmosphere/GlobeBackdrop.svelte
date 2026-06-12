<script lang="ts">
  // A non-interactive, fixed full-screen render of the Atmosphere globe, used
  // as a backdrop behind the settings surfaces (/account, /admin) so they read
  // as glassy overlays over the globe — the same feel as ?dossier=open.
  //
  // The engine is lazy-imported inside AtmosphereCanvas (shell-chunk rule), so
  // importing this component does not pull three/engine into the shell.
  import { createQuery } from '@tanstack/svelte-query';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import type { ProbeMarker } from '@aer/engine-3d';
  import AtmosphereCanvas from './AtmosphereCanvas.svelte';
  import {
    probesQuery,
    type FetchContext,
    type ProbeDto,
    type QueryOutcome
  } from '$lib/api/queries';

  const ctx: FetchContext = { baseUrl: '/api/v1' };

  const webgl = $derived(hasWebGL2());

  const probesQ = createQuery<QueryOutcome<ProbeDto[]>, Error, QueryOutcome<ProbeDto[]>>(() => {
    const o = probesQuery(ctx);
    return { queryKey: [...o.queryKey], queryFn: o.queryFn, staleTime: o.staleTime };
  });

  const probeDtos = $derived.by<ProbeDto[]>(() => {
    const d = probesQ.data;
    return d?.kind === 'success' ? d.data : [];
  });

  const probeMarkers = $derived.by<ProbeMarker[]>(() =>
    probeDtos.map((p) => ({
      id: p.probeId,
      language: p.language,
      label: p.shortName,
      emissionPoints: p.emissionPoints.map((ep, i) => {
        const source = p.sources[i];
        return source !== undefined
          ? { latitude: ep.latitude, longitude: ep.longitude, label: ep.label, sourceName: source }
          : { latitude: ep.latitude, longitude: ep.longitude, label: ep.label };
      })
    }))
  );
</script>

{#if webgl}
  <div class="globe-backdrop" aria-hidden="true">
    <AtmosphereCanvas probes={probeMarkers} />
  </div>
{/if}

<style>
  .globe-backdrop {
    position: fixed;
    inset: 0;
    z-index: 0;
    pointer-events: none;
  }
</style>
