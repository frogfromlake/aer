// Public types for `@aer/engine-3d`. The engine is consumed via these types
// only — no internal three.js objects leak through the imperative API.

export type PillarMode = 'aleph' | 'episteme' | 'rhizome';

/**
 * A geographic emission origin for a probe — the location from which one
 * of its bound publishers emits. A probe may have multiple emission
 * points (federated broadcasters, multi-city institutions). The engine
 * renders one glow per emission point.
 *
 * Emission points make no claim about *reach* — see ROADMAP Phase 99b
 * scope decision. Reach is not measured and is not rendered.
 */
export interface EmissionPoint {
  readonly latitude: number;
  readonly longitude: number;
  readonly label: string;
}

export interface ProbeMarker {
  readonly id: string;
  readonly language: string;
  readonly emissionPoints: readonly EmissionPoint[];
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
 * Details of an interaction with a rendered emission point. Events
 * always identify the owning probe *and* the specific emission point
 * under the cursor, so a panel can highlight "Hamburg (Tagesschau / NDR)"
 * versus "Berlin (Bundesregierung / BPA)" without a second round-trip.
 */
export interface ProbeSelection {
  readonly probeId: string;
  readonly emissionPointIndex: number;
  readonly emissionPointLabel: string;
}

export interface EngineEvents {
  /** Emitted on click on an emission point. Wired in Phase 99b. */
  'probe-selected': (selection: ProbeSelection) => void;
  /**
   * Emitted on mouse-move. The payload is non-null when the pointer is
   * over an emission point, and null once it leaves. Hover events drive
   * tooltip state and the intensified-glow feedback on the hot probe.
   */
  'probe-hovered': (selection: ProbeSelection | null) => void;
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
  /** Disable the auto-rotate idle behaviour entirely (in addition to prefers-reduced-motion). */
  readonly disableAutoRotate?: boolean;
}

export interface AtmosphereEngine {
  /** Mount the engine on a canvas. Idempotent: repeated calls are no-ops. */
  mount(element: HTMLCanvasElement): void;
  /**
   * Replace the set of rendered probes. One glow is drawn per emission
   * point. Rebuilding the instance buffers is cheap; the caller may
   * freely diff on its end or just re-push the whole set.
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
  flyTo(target: FlyToTarget): void;
  on<K extends keyof EngineEvents>(event: K, handler: EngineEvents[K]): () => void;
  /** Tear down: stop the loop, dispose geometries/materials, release the GL context. */
  dispose(): void;
}
