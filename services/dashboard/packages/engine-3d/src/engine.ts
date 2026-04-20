import {
  Clock,
  Color,
  LineBasicMaterial,
  LineSegments,
  Mesh,
  PerspectiveCamera,
  Scene,
  ShaderMaterial,
  SphereGeometry,
  Vector3,
  WebGLRenderer
} from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';

import {
  bordersToGeometry,
  landmassToGeometry,
  loadBorders,
  loadLandmass,
  type LandmassMesh
} from './landmass';
import { sunDirection } from './sun';
import atmosphereFrag from './shaders/atmosphere.glsl?raw';
import atmosphereVert from './shaders/atmosphere.vert.glsl?raw';
import terminatorFrag from './shaders/terminator.glsl?raw';
import terminatorVert from './shaders/terminator.vert.glsl?raw';
import type {
  AtmosphereEngine,
  EngineConfig,
  EngineEvents,
  FlyToTarget,
  PillarMode,
  ProbeActivity,
  ProbeMarker,
  PropagationEvent
} from './types';

const OCEAN_COLOR = new Color('#050a14');
const LAND_COLOR = new Color('#1a2a44');
const HALO_COLOR = new Color('#5687c8');
const BORDER_COLOR = new Color('#2c3f60');

const SPHERE_RADIUS = 1.0;
const ATMOSPHERE_RADIUS = 1.04;
const SPHERE_SEGMENTS = 64;

const MIN_DISTANCE = SPHERE_RADIUS * 1.2;
const MAX_DISTANCE = SPHERE_RADIUS * 8;
const INITIAL_DISTANCE = SPHERE_RADIUS * 3;

const AUTO_ROTATE_SPEED_RAD_S = 0.05;
const IDLE_BEFORE_AUTOROTATE_MS = 10_000;

const TWILIGHT_HALF_DEG = 2.5;

const DEG = Math.PI / 180;

class Engine implements AtmosphereEngine {
  private readonly config: Required<EngineConfig>;

  private renderer: WebGLRenderer | null = null;
  private scene: Scene | null = null;
  private camera: PerspectiveCamera | null = null;
  private controls: OrbitControls | null = null;
  private oceanMesh: Mesh | null = null;
  private landMesh: Mesh | null = null;
  private bordersMesh: LineSegments | null = null;
  private haloMesh: Mesh | null = null;

  private oceanMaterial: ShaderMaterial | null = null;
  private landMaterial: ShaderMaterial | null = null;
  private haloMaterial: ShaderMaterial | null = null;

  private readonly clock = new Clock();
  private readonly tmpSun = new Vector3(1, 0, 0);
  private sunOverrideMs: number | null = null;

  private rafId: number | null = null;
  private resizeObserver: ResizeObserver | null = null;
  private mounted = false;
  private disposed = false;
  private lastInteractionMs = 0;
  private readonly emitter = new Emitter();

  private reducedMotion = false;
  private mediaQuery: MediaQueryList | null = null;
  private readonly mediaListener = (): void => this.refreshReducedMotion();

  private bordersLoadPromise: Promise<void> | null = null;
  private bordersWanted = false;
  private bordersAbort: AbortController | null = null;

  private flyTween: FlyTween | null = null;

  // Phase 99b will consume these. Stored here so a 99b PR is data-flow only.
  private probes: readonly ProbeMarker[] = [];
  private activity: readonly ProbeActivity[] = [];
  private propagation: readonly PropagationEvent[] = [];
  private pillarMode: PillarMode = 'aleph';

  constructor(config: EngineConfig) {
    this.config = {
      landmassUrl: config.landmassUrl,
      bordersUrl: config.bordersUrl,
      pixelRatioCap: config.pixelRatioCap ?? Math.min(globalThis.devicePixelRatio ?? 1, 2),
      disableAutoRotate: config.disableAutoRotate ?? false
    };
  }

  mount(canvas: HTMLCanvasElement): void {
    if (this.mounted || this.disposed) return;
    this.mounted = true;

    this.renderer = new WebGLRenderer({
      canvas,
      antialias: true,
      alpha: false,
      powerPreference: 'high-performance'
    });
    this.renderer.setPixelRatio(this.config.pixelRatioCap);
    this.renderer.setClearColor(0x000000, 1);

    this.scene = new Scene();

    const { width, height } = canvas.getBoundingClientRect();
    const aspect = width > 0 && height > 0 ? width / height : 1;
    this.camera = new PerspectiveCamera(35, aspect, 0.01, 100);
    this.camera.position.set(0, 0, INITIAL_DISTANCE);

    this.controls = new OrbitControls(this.camera, canvas);
    this.controls.enableDamping = true;
    this.controls.dampingFactor = 0.08;
    this.controls.enablePan = false;
    this.controls.minDistance = MIN_DISTANCE;
    this.controls.maxDistance = MAX_DISTANCE;
    this.controls.rotateSpeed = 0.4;
    this.controls.zoomSpeed = 0.6;
    this.controls.addEventListener('start', this.onUserInteract);
    this.controls.addEventListener('change', this.onUserInteract);

    this.installReducedMotionListener();
    this.lastInteractionMs = performance.now();

    this.buildOcean();
    this.buildAtmosphere();
    this.beginLandmassLoad();

    this.handleResize();
    if (typeof ResizeObserver !== 'undefined') {
      this.resizeObserver = new ResizeObserver(() => this.handleResize());
      this.resizeObserver.observe(canvas);
    }

    this.clock.start();
    this.tick();
  }

  setProbes(probes: readonly ProbeMarker[]): void {
    this.probes = probes;
  }

  setActivity(activity: readonly ProbeActivity[]): void {
    this.activity = activity;
  }

  setPropagationEvents(events: readonly PropagationEvent[]): void {
    this.propagation = events;
  }

  setPillarMode(mode: PillarMode): void {
    this.pillarMode = mode;
  }

  setTimeRange(_from: Date, _to: Date): void {
    // Phase 99b uses this for activity windowing. In 99a it is a no-op so the
    // shell can already wire the time scrubber into the engine.
  }

  setSunPosition(unixMs: number | null): void {
    this.sunOverrideMs = unixMs;
  }

  async setBordersVisible(visible: boolean): Promise<void> {
    this.bordersWanted = visible;
    if (visible && !this.bordersLoadPromise) {
      this.bordersLoadPromise = this.loadBordersOnce();
    }
    await this.bordersLoadPromise?.catch(() => undefined);
    if (this.bordersMesh) this.bordersMesh.visible = this.bordersWanted;
  }

  flyTo(target: FlyToTarget): void {
    if (!this.camera || !this.controls) return;
    const dist = this.camera.position.length();
    const dest = latLonToCartesian(target.latitude, target.longitude, dist);
    this.flyTween = {
      from: this.camera.position.clone(),
      to: dest,
      startedAt: performance.now(),
      durationMs: target.durationMs ?? 1200
    };
    this.lastInteractionMs = performance.now();
  }

  on<K extends keyof EngineEvents>(event: K, handler: EngineEvents[K]): () => void {
    return this.emitter.on(event, handler);
  }

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    if (this.rafId !== null) cancelAnimationFrame(this.rafId);
    this.rafId = null;
    this.resizeObserver?.disconnect();
    this.resizeObserver = null;
    this.bordersAbort?.abort();
    this.bordersAbort = null;
    this.controls?.removeEventListener('start', this.onUserInteract);
    this.controls?.removeEventListener('change', this.onUserInteract);
    this.controls?.dispose();
    this.controls = null;
    this.uninstallReducedMotionListener();

    disposeMesh(this.oceanMesh);
    disposeMesh(this.landMesh);
    disposeMesh(this.haloMesh);
    if (this.bordersMesh) {
      this.bordersMesh.geometry.dispose();
      (this.bordersMesh.material as LineBasicMaterial).dispose();
    }
    this.oceanMaterial?.dispose();
    this.landMaterial?.dispose();
    this.haloMaterial?.dispose();

    this.scene?.clear();
    this.renderer?.dispose();
    this.renderer?.forceContextLoss();
    this.renderer = null;
    this.scene = null;
    this.camera = null;
    this.mounted = false;
  }

  // -- internals --------------------------------------------------------------

  private buildOcean(): void {
    if (!this.scene) return;
    const geom = new SphereGeometry(SPHERE_RADIUS, SPHERE_SEGMENTS, SPHERE_SEGMENTS);
    this.oceanMaterial = new ShaderMaterial({
      vertexShader: terminatorVert,
      fragmentShader: terminatorFrag,
      uniforms: {
        uSunDirection: { value: new Vector3(1, 0, 0) },
        uOceanColor: { value: OCEAN_COLOR },
        uLandColor: { value: LAND_COLOR },
        uIsLand: { value: 0.0 },
        uTwilightHalfDeg: { value: TWILIGHT_HALF_DEG }
      }
    });
    this.oceanMesh = new Mesh(geom, this.oceanMaterial);
    this.scene.add(this.oceanMesh);
  }

  private buildAtmosphere(): void {
    if (!this.scene) return;
    const geom = new SphereGeometry(ATMOSPHERE_RADIUS, SPHERE_SEGMENTS, SPHERE_SEGMENTS);
    this.haloMaterial = new ShaderMaterial({
      vertexShader: atmosphereVert,
      fragmentShader: atmosphereFrag,
      transparent: true,
      depthWrite: false,
      side: 1, // BackSide
      uniforms: {
        uSunDirection: { value: new Vector3(1, 0, 0) },
        uHaloColor: { value: HALO_COLOR },
        uIntensity: { value: 1.0 }
      }
    });
    this.haloMesh = new Mesh(geom, this.haloMaterial);
    this.scene.add(this.haloMesh);
  }

  private beginLandmassLoad(): void {
    void loadLandmass(this.config.landmassUrl)
      .then((mesh) => this.attachLandmass(mesh))
      .catch((err: unknown) => {
        console.warn('[engine-3d] landmass load failed; ocean-only render', err);
      });
  }

  private attachLandmass(mesh: LandmassMesh): void {
    if (this.disposed || !this.scene) return;
    this.landMaterial = new ShaderMaterial({
      vertexShader: terminatorVert,
      fragmentShader: terminatorFrag,
      uniforms: {
        uSunDirection: { value: new Vector3(1, 0, 0) },
        uOceanColor: { value: OCEAN_COLOR },
        uLandColor: { value: LAND_COLOR },
        uIsLand: { value: 1.0 },
        uTwilightHalfDeg: { value: TWILIGHT_HALF_DEG }
      }
    });
    this.landMesh = new Mesh(landmassToGeometry(mesh), this.landMaterial);
    this.scene.add(this.landMesh);
  }

  private async loadBordersOnce(): Promise<void> {
    this.bordersAbort = new AbortController();
    try {
      const mesh = await loadBorders(this.config.bordersUrl, this.bordersAbort.signal);
      if (this.disposed || !this.scene) return;
      const geom = bordersToGeometry(mesh);
      const mat = new LineBasicMaterial({ color: BORDER_COLOR, transparent: true, opacity: 0.7 });
      this.bordersMesh = new LineSegments(geom, mat);
      this.bordersMesh.visible = this.bordersWanted;
      this.scene.add(this.bordersMesh);
    } catch (err) {
      console.warn('[engine-3d] borders load failed', err);
    }
  }

  private readonly onUserInteract = (): void => {
    this.lastInteractionMs = performance.now();
  };

  private installReducedMotionListener(): void {
    if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return;
    this.mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    this.reducedMotion = this.mediaQuery.matches;
    this.mediaQuery.addEventListener('change', this.mediaListener);
  }

  private uninstallReducedMotionListener(): void {
    this.mediaQuery?.removeEventListener('change', this.mediaListener);
    this.mediaQuery = null;
  }

  private refreshReducedMotion(): void {
    if (this.mediaQuery) this.reducedMotion = this.mediaQuery.matches;
  }

  private handleResize(): void {
    if (!this.renderer || !this.camera) return;
    const canvas = this.renderer.domElement;
    const rect = canvas.getBoundingClientRect();
    const w = Math.max(1, Math.floor(rect.width));
    const h = Math.max(1, Math.floor(rect.height));
    this.renderer.setSize(w, h, false);
    this.camera.aspect = w / h;
    this.camera.updateProjectionMatrix();
  }

  private tick = (): void => {
    if (this.disposed) return;
    const dt = this.clock.getDelta();
    this.applyAutoRotation(dt);
    this.applyFlyTween();
    this.controls?.update();
    this.updateSunUniform();
    if (this.renderer && this.scene && this.camera) {
      this.renderer.render(this.scene, this.camera);
    }
    this.rafId = requestAnimationFrame(this.tick);
  };

  private applyAutoRotation(dt: number): void {
    if (!this.controls || !this.camera) return;
    if (this.config.disableAutoRotate || this.reducedMotion) return;
    if (this.flyTween) return;
    const idle = performance.now() - this.lastInteractionMs;
    if (idle < IDLE_BEFORE_AUTOROTATE_MS) return;
    // Rotate the camera around the world Y axis. Mutating the camera position
    // directly avoids OrbitControls re-emitting a 'change' that would reset
    // the idle clock.
    const angle = AUTO_ROTATE_SPEED_RAD_S * dt;
    const sin = Math.sin(angle);
    const cos = Math.cos(angle);
    const { x, z } = this.camera.position;
    this.camera.position.x = x * cos + z * sin;
    this.camera.position.z = -x * sin + z * cos;
    this.camera.lookAt(0, 0, 0);
  }

  private applyFlyTween(): void {
    if (!this.flyTween || !this.camera || !this.controls) return;
    const t = (performance.now() - this.flyTween.startedAt) / this.flyTween.durationMs;
    if (t >= 1) {
      this.camera.position.copy(this.flyTween.to);
      this.flyTween = null;
    } else {
      const eased = easeInOutCubic(t);
      this.camera.position.lerpVectors(this.flyTween.from, this.flyTween.to, eased);
    }
    this.camera.lookAt(0, 0, 0);
  }

  private updateSunUniform(): void {
    const ms = this.sunOverrideMs ?? Date.now();
    sunDirection(ms, this.tmpSun);
    setVec3(this.oceanMaterial?.uniforms.uSunDirection?.value, this.tmpSun);
    setVec3(this.landMaterial?.uniforms.uSunDirection?.value, this.tmpSun);
    setVec3(this.haloMaterial?.uniforms.uSunDirection?.value, this.tmpSun);
  }
}

interface FlyTween {
  from: Vector3;
  to: Vector3;
  startedAt: number;
  durationMs: number;
}

class Emitter {
  private readonly handlers = new Map<keyof EngineEvents, Set<unknown>>();
  on<K extends keyof EngineEvents>(event: K, handler: EngineEvents[K]): () => void {
    let set = this.handlers.get(event);
    if (!set) {
      set = new Set();
      this.handlers.set(event, set);
    }
    set.add(handler);
    return () => set?.delete(handler);
  }
}

function disposeMesh(mesh: Mesh | null): void {
  if (!mesh) return;
  mesh.geometry.dispose();
  // Materials are owned by the engine and disposed separately.
}

function setVec3(target: unknown, src: Vector3): void {
  if (target instanceof Vector3) target.copy(src);
}

function latLonToCartesian(latDeg: number, lonDeg: number, radius: number): Vector3 {
  const lat = latDeg * DEG;
  const lon = lonDeg * DEG;
  const cosLat = Math.cos(lat);
  return new Vector3(
    radius * cosLat * Math.sin(lon),
    radius * Math.sin(lat),
    radius * cosLat * Math.cos(lon)
  );
}

function easeInOutCubic(t: number): number {
  return t < 0.5 ? 4 * t * t * t : 1 - Math.pow(-2 * t + 2, 3) / 2;
}

export function createEngine(config: EngineConfig): AtmosphereEngine {
  return new Engine(config);
}
