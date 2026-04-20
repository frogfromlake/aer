// Public types for `@aer/engine-3d`. The engine is consumed via these types
// only — no internal three.js objects leak through the imperative API.

export type PillarMode = 'aleph' | 'episteme' | 'rhizome';

export interface ProbeMarker {
  readonly id: string;
  readonly latitude: number;
  readonly longitude: number;
}

export interface ProbeActivity {
  readonly probeId: string;
  /** Documents per hour in the current rolling window. Phase 99b consumes this. */
  readonly documentsPerHour: number;
}

export interface PropagationEvent {
  readonly fromProbeId: string;
  readonly toProbeId: string;
  readonly atUnixMs: number;
}

export interface Probe {
  readonly id: string;
  readonly latitude: number;
  readonly longitude: number;
}

export interface FlyToTarget {
  readonly latitude: number;
  readonly longitude: number;
  readonly durationMs?: number;
}

export interface EngineEvents {
  /** Wired in Phase 99b once raycasting lands. Declared in 99a per ROADMAP. */
  'probe-selected': (probe: Probe) => void;
}

export interface EngineConfig {
  /** Path (relative to the static origin) of the baked landmass mesh. */
  readonly landmassUrl: string;
  /** Path of the borders mesh. Loaded only on the first `setBordersVisible(true)`. */
  readonly bordersUrl: string;
  /** Override the device pixel ratio cap. Defaults to `min(devicePixelRatio, 2)`. */
  readonly pixelRatioCap?: number;
  /** Disable the auto-rotate idle behaviour entirely (in addition to prefers-reduced-motion). */
  readonly disableAutoRotate?: boolean;
}

export interface AtmosphereEngine {
  /** Mount the engine on a canvas. Idempotent: repeated calls are no-ops. */
  mount(element: HTMLCanvasElement): void;
  /** Phase 99b will render these. In 99a they accept input and store it for later. */
  setProbes(probes: readonly ProbeMarker[]): void;
  setActivity(activity: readonly ProbeActivity[]): void;
  setPropagationEvents(events: readonly PropagationEvent[]): void;
  setPillarMode(mode: PillarMode): void;
  setTimeRange(from: Date, to: Date): void;
  /** Static-position override for the sun direction (for terminator stories). Pass `null` to resume live tracking. */
  setSunPosition(unixMs: number | null): void;
  /** Reveal or hide the country-borders layer. Lazy-loads the asset on first `true`. */
  setBordersVisible(visible: boolean): Promise<void>;
  flyTo(target: FlyToTarget): void;
  on<K extends keyof EngineEvents>(event: K, handler: EngineEvents[K]): () => void;
  /** Tear down: stop the loop, dispose geometries/materials, release the GL context. */
  dispose(): void;
}
