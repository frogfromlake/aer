<script lang="ts">
  // Phase 99a landing: full-bleed 3D atmosphere with WebGL2 capability gate.
  // The shell stays light by dynamic-importing the engine inside AtmosphereCanvas.
  import { onMount } from 'svelte';
  import { hasWebGL2 } from '@aer/engine-3d/capability';
  import AtmosphereCanvas from '$lib/components/atmosphere/AtmosphereCanvas.svelte';
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';

  let decision: 'pending' | 'engine' | 'fallback' = $state('pending');

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const forceFallback = params.get('fallback') === '1';
    decision = !forceFallback && hasWebGL2() ? 'engine' : 'fallback';
  });
</script>

<svelte:head>
  <title>AĒR — Atmosphere</title>
</svelte:head>

{#if decision === 'engine'}
  <div class="stage" aria-hidden="false">
    <AtmosphereCanvas />
  </div>
{:else if decision === 'fallback'}
  <div class="centered">
    <WebGLFallback />
  </div>
{/if}

<style>
  .stage {
    position: fixed;
    inset: 0;
    background: #000;
  }
  .centered {
    min-height: 100dvh;
    display: grid;
    place-items: center;
  }
</style>
