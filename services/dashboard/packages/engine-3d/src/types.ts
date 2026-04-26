// Public types for `@aer/engine-3d`. The engine is consumed via these types
// only — no internal three.js objects leak through the imperative API.

export type PillarMode = 'aleph' | 'episteme' | 'rhizome';

/**
 * A geographic emission origin for a probe — the location from which one
 * of its bound publishers emits. A probe may have multiple emission
 * points (federated broadcasters, multi-city institutions). After Phase
 * 110, emission points are rendered as *source satellites* — secondary
 * geometry around the central probe glyph, not selectable as scope
 * targets. Click and hover on a satellite raise dedicated
 * `satellite-*` events the surface routes to the Probe Dossier with the
 * source pre-filtered.
 *
 * Emission points make no claim about *reach* — see ROADMAP Phase 99b
 * scope decision. Reach is not measured and is not rendered.
 */
export interface EmissionPoint {
  readonly latitude: number;
  readonly longitude: number;
  readonly label: string;
  /**
   * Canonical source name the satellite identifies. Required for the
   * satellite's click-to-dossier route (`?sourceId=…`). When absent, the
   * satellite is rendered but its click is treated as a no-op (the
   * source identity cannot be resolved).
   */
  readonly sourceName?: string;
}

export interface ProbeMarker {
  readonly id: string;
  readonly language: string;
  readonly emissionPoints: readonly EmissionPoint[];
  /**
   * Plain-language identity for the probe glyph (Progressive Semantics
   * §4.5 — the semantic register prominent on hover). Defaults to `id`
   * when absent.
   */
  readonly label?: string;
}

export interface ProbeActivity {
  readonly probeId: string;
  /**
   * Documents per hour in the current rolling window. Drives `uPulseRate`
   * in the glow shader. The engine clamps this internally so the fastest
   * pulse completes no more than one cycle per ~4 seconds (§1.1
   * "stillness with motion beneath").
   */
  readonly documentsPerHour: number;
}

export interface PropagationEvent {
  readonly fromProbeId: string;
  readonly toProbeId: string;
  readonly atUnixMs: number;
}

export interface FlyToTarget {
  readonly latitude: number;
  readonly longitude: number;
  readonly durationMs?: number;
}

/**
 * Scope-target selection on the globe. After Phase 110, the only scope
 * target on Surface I is the probe — not its emission points. Clicking
 * an emission-point satellite is a navigation event (see
 * `SatelliteSelection`) and never changes scope to "source-only".
 */
export interface ProbeSelection {
  readonly probeId: string;
}

/**
 * A satellite click/hover. Routed by the surface to
 * `/lanes/:probeId/dossier?sourceId=…` (click) or a tooltip (hover).
 */
export interface SatelliteSelection {
  readonly probeId: string;
  readonly sourceName: string;
  readonly label: string;
}

export interface EngineEvents {
  /** Emitted on click on a probe glyph. The single scope-target event on Surface I. */
  'probe-selected': (selection: ProbeSelection) => void;
  /**
   * Emitted on mouse-move. The payload is non-null when the pointer is
   * over a probe glyph, and null once it leaves. Hover events drive the
   * Progressive Semantics tooltip and the intensified-glow feedback on
   * the hot probe.
   */
  'probe-hovered': (selection: ProbeSelection | null) => void;
  /** Emitted on click on a source satellite. NOT a scope-change event. */
  'satellite-selected': (selection: SatelliteSelection) => void;
  /** Emitted on mouse-move when the pointer is over a satellite, null on leave. */
  'satellite-hovered': (selection: SatelliteSelection | null) => void;
}

export interface EngineConfig {
  /**
   * Path (relative to the static origin) of the baked landmass SDF PNG.
   * See scripts/bake-landmass.mjs for the encoding — equirectangular,
   * red channel, 0.5 at the coastline.
   */
  readonly landSdfUrl: string;
  /** Override the device pixel ratio cap. Defaults to `min(devicePixelRatio, 2)`. */
  readonly pixelRatioCap?: number;
}

export interface AtmosphereEngine {
  /** Mount the engine on a canvas. Idempotent: repeated calls are no-ops. */
  mount(element: HTMLCanvasElement): void;
  /**
   * Replace the set of rendered probes. The engine renders one *probe
   * glyph* per probe (at the spherical centroid of its emission points)
   * plus one muted *source satellite* per emission point. The satellite
   * layer is non-selectable as a scope target.
   */
  setProbes(probes: readonly ProbeMarker[]): void;
  /**
   * Push per-probe activity. Missing probes keep their previous pulse
   * values — the engine never silently zeroes a probe because a single
   * request failed.
   */
  setActivity(activity: readonly ProbeActivity[]): void;
  setPropagationEvents(events: readonly PropagationEvent[]): void;
  setPillarMode(mode: PillarMode): void;
  setTimeRange(from: Date, to: Date): void;
  /** Static-position override for the sun direction (for terminator stories). Pass `null` to resume live tracking. */
  setSunPosition(unixMs: number | null): void;
  setSelection(selection: ProbeSelection | null): void;
  /**
   * Force-set the hover highlight on a probe glyph. Used by keyboard
   * navigation so a focused probe glows on the globe even though the
   * pointer is not over it. Pass `null` to clear. The pointer-driven
   * raycaster overrides this on the next `pointermove` (pointer-over-
   * keyboard while both are active).
   */
  setHover(selection: ProbeSelection | null): void;
  flyTo(target: FlyToTarget): void;
  on<K extends keyof EngineEvents>(event: K, handler: EngineEvents[K]): () => void;
  /** Tear down: stop the loop, dispose geometries/materials, release the GL context. */
  dispose(): void;
}
