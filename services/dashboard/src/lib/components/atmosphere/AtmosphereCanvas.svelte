<script lang="ts">
  // Mounts `@aer/engine-3d` lazily onto a full-bleed canvas. The engine module
  // is dynamic-imported so the three.js bundle never enters the shell chunk.
  // Capability detection is the shell's responsibility — see WebGLFallback.svelte
  // for the WebGL2-unavailable path.
  import { onDestroy, onMount } from 'svelte';
  import type {
    AtmosphereEngine,
    EngineConfig,
    ProbeActivity,
    ProbeMarker,
    ProbeSelection
  } from '@aer/engine-3d';

  interface Props {
    /** Override the live sun for stories: a fixed Unix timestamp pins the terminator. */
    sunOverrideMs?: number | null;
    /** Disable the 10s-idle auto-rotation (for paused stories). */
    disableAutoRotate?: boolean;
    /** Notified once the engine has mounted; receives the imperative API for story-level control. */
    onready?: (engine: AtmosphereEngine) => void;
    /** Probes to render as emission-point glows. Reactive: re-assigning pushes through to `setProbes`. */
    probes?: readonly ProbeMarker[];
    /** Per-probe activity (documents per hour). Reactive: drives `setActivity`. */
    activity?: readonly ProbeActivity[];
    /** Fired when the user clicks an emission point. */
    onProbeSelected?: (selection: ProbeSelection) => void;
  }

  let {
    sunOverrideMs = null,
    disableAutoRotate = false,
    onready,
    probes,
    activity,
    onProbeSelected
  }: Props = $props();

  let canvas: HTMLCanvasElement | undefined = $state();
  let engine: AtmosphereEngine | null = $state(null);
  let unsubscribeSelected: (() => void) | null = null;

  onMount(async () => {
    if (!canvas) return;
    const mod = await import('@aer/engine-3d');
    if (!canvas) return; // unmounted while the chunk was downloading
    const config: EngineConfig = {
      landSdfUrl: '/data/landmass.sdf.png',
      disableAutoRotate
    };
    const e = mod.createEngine(config);
    e.mount(canvas);
    if (sunOverrideMs !== null && sunOverrideMs !== undefined) {
      e.setSunPosition(sunOverrideMs);
    }
    if (probes) e.setProbes(probes);
    if (activity) e.setActivity(activity);
    unsubscribeSelected = e.on('probe-selected', (sel) => onProbeSelected?.(sel));
    engine = e;
    onready?.(e);
  });

  $effect(() => {
    engine?.setSunPosition(sunOverrideMs ?? null);
  });

  $effect(() => {
    if (engine && probes) engine.setProbes(probes);
  });

  $effect(() => {
    if (engine && activity) engine.setActivity(activity);
  });

  onDestroy(() => {
    unsubscribeSelected?.();
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
