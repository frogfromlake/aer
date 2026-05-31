<script lang="ts">
  // Mounts `@aer/engine-3d` lazily onto a full-bleed canvas. The engine module
  // is dynamic-imported so the three.js bundle never enters the shell chunk.
  // Capability detection is the shell's responsibility — see WebGLFallback.svelte
  // for the WebGL2-unavailable path.
  //
  // Phase 110: the engine renders one *probe glyph* per probe (selectable
  // scope target) and one muted *source satellite* per emission point
  // (read-only — clicking routes to the Probe Dossier with the source
  // pre-filtered). Probe and satellite events are surfaced separately.
  import { onDestroy, onMount } from 'svelte';
  import type {
    AtmosphereEngine,
    EngineConfig,
    ProbeActivity,
    ProbeMarker,
    ProbeSelection,
    SatelliteSelection
  } from '@aer/engine-3d';

  interface Props {
    /** Override the live sun for stories: a fixed Unix timestamp pins the terminator. */
    sunOverrideMs?: number | null;
    /** Notified once the engine has mounted; receives the imperative API for story-level control. */
    onready?: (engine: AtmosphereEngine) => void;
    /** Probes to render. Reactive: re-assigning pushes through to `setProbes`. */
    probes?: readonly ProbeMarker[];
    /** Per-probe activity (documents per hour). Reactive: drives `setActivity`. */
    activity?: readonly ProbeActivity[];
    /** Fired when the user clicks a probe glyph (scope-target selection). */
    onProbeSelected?: (selection: ProbeSelection) => void;
    /** Fired on pointer-hover over a probe glyph (null when leaving). */
    onProbeHovered?: (selection: ProbeSelection | null) => void;
    /** Fired when the user clicks a source satellite (navigation, not scope-change). */
    onSatelliteSelected?: (selection: SatelliteSelection) => void;
    /** Fired on pointer-hover over a source satellite (null when leaving). */
    onSatelliteHovered?: (selection: SatelliteSelection | null) => void;
    /** The active (last-clicked) probe. Reactive: drives the camera `flyTo`
     *  re-center. The glyph highlight is driven by `selectedProbeIds` (the
     *  full set), not this. */
    selection?: ProbeSelection | null;
    /** Phase 123c (D2) — the full selection set to highlight on the globe.
     *  SHIFT-click grows it; every member glyph stays lit. When omitted, the
     *  highlight falls back to the single `selection` (legacy callers). */
    selectedProbeIds?: readonly string[];
    /**
     * Forced hover highlight for a probe glyph (keyboard focus driver).
     * Pointer moves override this on the engine side. Null clears the
     * highlight.
     */
    hover?: ProbeSelection | null;
    /** Optional per-probe centroid override for `flyTo` after selection. */
    flyToOnSelection?: { latitude: number; longitude: number } | null;
  }

  let {
    sunOverrideMs = null,
    onready,
    probes,
    activity,
    onProbeSelected,
    onProbeHovered,
    onSatelliteSelected,
    onSatelliteHovered,
    selection = null,
    selectedProbeIds,
    hover = null,
    flyToOnSelection = null
  }: Props = $props();

  let canvas: HTMLCanvasElement | undefined = $state();
  let engine: AtmosphereEngine | null = $state(null);
  let unsubscribers: Array<() => void> = [];

  onMount(async () => {
    if (!canvas) return;
    const mod = await import('@aer/engine-3d');
    if (!canvas) return; // unmounted while the chunk was downloading
    const config: EngineConfig = {
      landSdfUrl: '/data/landmass.sdf.png'
    };
    const e = mod.createEngine(config);
    e.mount(canvas);
    if (sunOverrideMs !== null && sunOverrideMs !== undefined) {
      e.setSunPosition(sunOverrideMs);
    }
    if (probes) e.setProbes(probes);
    if (activity) e.setActivity(activity);
    unsubscribers.push(e.on('probe-selected', (sel) => onProbeSelected?.(sel)));
    unsubscribers.push(e.on('probe-hovered', (sel) => onProbeHovered?.(sel)));
    unsubscribers.push(e.on('satellite-selected', (sel) => onSatelliteSelected?.(sel)));
    unsubscribers.push(e.on('satellite-hovered', (sel) => onSatelliteHovered?.(sel)));
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

  $effect(() => {
    engine?.setHover(hover ?? null);
  });

  // Phase 123c (D2) — highlight the FULL selection set on the globe. Falls
  // back to the single active `selection` when the host doesn't pass a set.
  $effect(() => {
    if (!engine) return;
    const ids = selectedProbeIds ?? (selection ? [selection.probeId] : []);
    engine.setSelectedProbes(ids);
  });

  // Animate the camera to the active probe. Kept separate from the highlight
  // effect so a pure highlight-set change (e.g. `selectedProbeIds` synced
  // from the URL, or a probe removed from the set) never moves the camera —
  // only a fresh active `selection` + `flyToOnSelection` does. The page's
  // SHIFT-add path deliberately sets a new active selection, so adding a
  // probe still centres on it (an active-selection change, not a mere set
  // change).
  $effect(() => {
    if (!engine) return;
    if (selection && flyToOnSelection) {
      engine.flyTo({
        latitude: flyToOnSelection.latitude,
        longitude: flyToOnSelection.longitude,
        durationMs: 900
      });
    }
  });

  onDestroy(() => {
    for (const u of unsubscribers) u();
    unsubscribers = [];
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
