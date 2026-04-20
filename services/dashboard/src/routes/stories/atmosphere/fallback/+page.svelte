<script lang="ts">
  // Fallback story: renders the static WebGLFallback unconditionally so the
  // a11y gate can audit it without needing to disable WebGL2 in the browser.
  import WebGLFallback from '$lib/components/atmosphere/WebGLFallback.svelte';
</script>

<svelte:head>
  <title>AĒR Stories — Atmosphere · Fallback</title>
</svelte:head>

<div class="centered">
  <WebGLFallback />
</div>

<style>
  .centered {
    min-height: 100dvh;
    display: grid;
    place-items: center;
  }
</style>
