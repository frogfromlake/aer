<script lang="ts">
  // Mounts `@aer/engine-3d` lazily onto a full-bleed canvas. The engine module
  // is dynamic-imported so the three.js bundle never enters the shell chunk.
  // Capability detection is the shell's responsibility — see WebGLFallback.svelte
  // for the WebGL2-unavailable path.
  import { onDestroy, onMount } from 'svelte';
  import type { AtmosphereEngine, EngineConfig } from '@aer/engine-3d';

  interface Props {
    /** Override the live sun for stories: a fixed Unix timestamp pins the terminator. */
    sunOverrideMs?: number | null;
    /** Disable the 10s-idle auto-rotation (for paused stories). */
    disableAutoRotate?: boolean;
    /** Whether to load and reveal the optional borders layer. */
    showBorders?: boolean;
    /** Notified once the engine has mounted; receives the imperative API for story-level control. */
    onready?: (engine: AtmosphereEngine) => void;
  }

  let {
    sunOverrideMs = null,
    disableAutoRotate = false,
    showBorders = false,
    onready
  }: Props = $props();

  let canvas: HTMLCanvasElement | undefined = $state();
  let engine: AtmosphereEngine | null = null;

  onMount(async () => {
    if (!canvas) return;
    const mod = await import('@aer/engine-3d');
    if (!canvas) return; // unmounted while the chunk was downloading
    const config: EngineConfig = {
      landmassUrl: '/data/landmass.json',
      bordersUrl: '/data/borders.json',
      disableAutoRotate
    };
    engine = mod.createEngine(config);
    engine.mount(canvas);
    if (sunOverrideMs !== null && sunOverrideMs !== undefined) {
      engine.setSunPosition(sunOverrideMs);
    }
    if (showBorders) await engine.setBordersVisible(true);
    onready?.(engine);
  });

  $effect(() => {
    engine?.setSunPosition(sunOverrideMs ?? null);
  });

  $effect(() => {
    void engine?.setBordersVisible(showBorders);
  });

  onDestroy(() => {
    engine?.dispose();
    engine = null;
  });
</script>

<figure aria-label="AĒR atmosphere: 3D rotating Earth with live day/night terminator">
  <canvas bind:this={canvas}></canvas>
</figure>

<style>
  figure {
    margin: 0;
    width: 100%;
    height: 100%;
  }
  canvas {
    display: block;
    width: 100%;
    height: 100%;
    background: #000;
    /* The engine paints over this background as soon as it has a frame. */
  }
</style>
