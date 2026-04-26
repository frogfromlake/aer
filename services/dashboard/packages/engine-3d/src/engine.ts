import {
  AdditiveBlending,
  BufferAttribute,
  BufferGeometry,
  ClampToEdgeWrapping,
  Clock,
  Color,
  DataTexture,
  LinearFilter,
  Mesh,
  PerspectiveCamera,
  Points,
  Raycaster,
  RepeatWrapping,
  Scene,
  ShaderMaterial,
  SphereGeometry,
  type Texture,
  TextureLoader,
  Vector2,
  Vector3,
  WebGLRenderer
} from 'three';
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js';

import {
  CORE_BRIGHTNESS_FLOOR,
  type RaycastCandidate,
  computeCoreBrightness,
  computePulseRate,
  pickNearSideHit,
  probeCentroidLatLon
} from './glow';
import { sunDirection } from './sun';
import atmosphereFrag from './shaders/atmosphere.glsl?raw';
import atmosphereVert from './shaders/atmosphere.vert.glsl?raw';
import glowFrag from './shaders/glow.glsl?raw';
import glowVert from './shaders/glow.vert.glsl?raw';
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
  ProbeSelection,
  PropagationEvent,
  SatelliteSelection
} from './types';

// Palette tuned for optimal data contrast in Phase 99b (Abyssal Palette).
// Ocean fades almost into the pure black backdrop to create depth. Land is
// a muted slate — dark enough to maximize the luminance contrast of glowing
// data points (e.g., Viridis), yet distinct enough from the ocean to clearly
// read as continental regions. No political borders are rendered — region
// identity is expressed by probe-bound activity (99b).
const OCEAN_COLOR = new Color('#060d16');
const LAND_COLOR = new Color('#252e3b');
const HALO_COLOR = new Color('#5283b8');

// --- NEW SHADER CONFIGURATION ---
// The color of the atmospheric rim light applied to the edge of the globe
const RIM_COLOR = new Color('#5283b8');
const RIM_INTENSITY = 0.1;

// Visibility factors for the night side (1.0 = full daylight brightness, 0.0 = pitch black)
const NIGHT_OCEAN_FACTOR = 0.5;

// Set to 1.0 so land is exactly as bright at night as it is during the day.
// This completely uncouples the continents from the sun, creating a perfect, uniform canvas.
const NIGHT_LAND_FACTOR = 1.0;

// Artificial glow on land.
const LAND_ILLUMINATION = 4.0;

const SPHERE_RADIUS = 1.0;
const ATMOSPHERE_RADIUS = 1.011;
const SPHERE_SEGMENTS = 96;

// Probe glyphs (one per probe at the spherical centroid of its emission
// points) and source satellites (one per emission point) sit a hair
// above the surface so the disc is not z-fought by the globe shader and
// so hits on the far side of the sphere can be filtered by a
// normal-vs-camera check. Satellites sit fractionally lower so when a
// satellite happens to coincide with the centroid the probe glyph wins
// the depth ordering on the additive layer.
const GLOW_RADIUS = 1.0035;
const SATELLITE_RADIUS = 1.003;
// World-space diameter of a glow at 1 unit from camera. The vertex
// shader divides by depth so the disc's apparent size is stable as the
// camera orbits.
const GLOW_POINT_WORLD_SIZE = 80.0;
// Source satellites are visibly secondary — smaller and dimmer than the
// probe glyph (Phase 110: "smaller, muted, non-selectable as scope
// targets"). The size factor keeps satellites readable as origins while
// the brightness factor pushes them well below the probe glyph so the
// scope-target reads as the unambiguous primary.
const SATELLITE_POINT_WORLD_SIZE = 32.0;
const SATELLITE_BRIGHTNESS_SCALE = 0.6;
// Tune this to scale all glow rings up (> 1.0) or down (< 1.0).
const GLOW_BRIGHTNESS_SCALE = 1.5;
// Halo spread coefficient — baked as 0.8 in the shader originally.
// Raise to widen/brighten the outer halo; lower to tighten it.
const GLOW_HALO_BRIGHTNESS = 0.7;
// Gaussian ring centred at r≈0.85 — only affects the very outer edge.
// Zero at the core and inner halo (r < 0.5). Safe to raise independently.
const GLOW_OUTER_RING_BRIGHTNESS = 0.35;

// Emission glow colour. Warm enough to read against the slate land
// palette; cool enough not to scream — the palette target for 99b is a
// calm, atmospheric surface, not a dashboard of alerts.
const GLOW_COLOR = new Color('#d8c28a');

const MIN_DISTANCE = SPHERE_RADIUS * 1.2;
const MAX_DISTANCE = SPHERE_RADIUS * 8;
const INITIAL_DISTANCE = SPHERE_RADIUS * 3;

const TWILIGHT_HALF_DEG = 4.0;

const DEG = Math.PI / 180;

class Engine implements AtmosphereEngine {
  private readonly config: Required<EngineConfig>;

  private renderer: WebGLRenderer | null = null;
  private scene: Scene | null = null;
  private camera: PerspectiveCamera | null = null;
  private controls: OrbitControls | null = null;
  private oceanMesh: Mesh | null = null;
  private haloMesh: Mesh | null = null;

  private oceanMaterial: ShaderMaterial | null = null;
  private haloMaterial: ShaderMaterial | null = null;

  // The SDF starts as a 1×1 ocean-coloured placeholder so the ocean shader
  // renders correctly during the ~100 ms texture fetch, then gets swapped
  // for the real baked texture in place.
  private sdfTexture: Texture | null = null;
  private placeholderSdf: DataTexture | null = null;

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

  private flyTween: FlyTween | null = null;

  private probes: readonly ProbeMarker[] = [];
  // Per-probe activity is folded into per-emission-point buffers on
  // setActivity. We keep the last-seen value so a probe that drops out
  // of a partial update does not silently reset to zero.
  private readonly activityByProbeId = new Map<string, number>();
  private propagation: readonly PropagationEvent[] = [];
  private pillarMode: PillarMode = 'aleph';

  // Glow layer -----------------------------------------------------------
  //
  // Two Points meshes share the glow shader: `probeGlyphMesh` renders one
  // glyph per probe at the spherical centroid of its emission points and
  // is the *only* scope-selectable target on Surface I. `satelliteMesh`
  // renders one smaller, muted disc per emission point — read-only
  // secondary geometry whose click routes to the Probe Dossier with the
  // source pre-filtered (Phase 110).
  //
  // `probeSlots` and `satelliteSlots` are CPU-side metadata parallel to
  // each geometry's vertex attributes; index i in a mesh corresponds to
  // entry i in its slot array. The raycaster maps a hit index back to
  // (probeId) or (probeId, sourceName, label) accordingly.
  private probeGlyphMesh: Points | null = null;
  private probeGlyphGeometry: BufferGeometry | null = null;
  private probeGlyphMaterial: ShaderMaterial | null = null;
  private satelliteMesh: Points | null = null;
  private satelliteGeometry: BufferGeometry | null = null;
  private satelliteMaterial: ShaderMaterial | null = null;
  private probeSlots: Array<{
    readonly probeId: string;
    readonly label: string;
    readonly position: Vector3;
  }> = [];
  private satelliteSlots: Array<{
    readonly probeId: string;
    readonly sourceName: string;
    readonly label: string;
    readonly position: Vector3;
  }> = [];
  private pointerHoveredProbe = -1;
  private externalHoveredProbe = -1;
  private pointerHoveredSatellite = -1;
  private selectedProbeIndex = -1;
  private currentSelection: ProbeSelection | null = null;
  private readonly raycaster = new Raycaster();
  private readonly pointerNdc = new Vector2();
  private pointerInsideCanvas = false;

  constructor(config: EngineConfig) {
    this.config = {
      landSdfUrl: config.landSdfUrl,
      pixelRatioCap: config.pixelRatioCap ?? Math.min(globalThis.devicePixelRatio ?? 1, 2)
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

    this.installReducedMotionListener();

    this.buildGlobe();
    this.buildAtmosphere();
    this.buildGlowLayer();
    this.beginSdfLoad();

    canvas.addEventListener('pointermove', this.onPointerMove);
    canvas.addEventListener('pointerleave', this.onPointerLeave);
    canvas.addEventListener('click', this.onCanvasClick);

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
    this.rebuildGlowLayers();
  }

  setActivity(activity: readonly ProbeActivity[]): void {
    for (const a of activity) {
      this.activityByProbeId.set(a.probeId, a.documentsPerHour);
    }
    this.applyActivityToBuffers();
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

  setHover(selection: ProbeSelection | null): void {
    if (selection === null) {
      this.setExternalHoverProbe(-1);
      return;
    }
    const idx = this.probeSlots.findIndex((s) => s.probeId === selection.probeId);
    this.setExternalHoverProbe(idx);
  }

  setSelection(selection: ProbeSelection | null): void {
    this.currentSelection = selection;

    if (!this.probeGlyphGeometry) return;
    const attr = this.probeGlyphGeometry.getAttribute('aSelected') as BufferAttribute | undefined;
    if (!attr) return;

    // Clear previous selection
    if (this.selectedProbeIndex !== -1 && this.selectedProbeIndex < this.probeSlots.length) {
      attr.setX(this.selectedProbeIndex, 0);
    }

    this.selectedProbeIndex = -1;

    // Apply new selection if provided
    if (selection) {
      const idx = this.probeSlots.findIndex((s) => s.probeId === selection.probeId);
      if (idx !== -1) {
        this.selectedProbeIndex = idx;
        attr.setX(idx, 1);
      }
    }

    attr.needsUpdate = true;
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
    this.controls?.dispose();
    this.controls = null;
    this.uninstallReducedMotionListener();

    const canvas = this.renderer?.domElement;
    if (canvas) {
      canvas.removeEventListener('pointermove', this.onPointerMove);
      canvas.removeEventListener('pointerleave', this.onPointerLeave);
      canvas.removeEventListener('click', this.onCanvasClick);
    }

    disposeMesh(this.oceanMesh);
    disposeMesh(this.haloMesh);
    this.oceanMaterial?.dispose();
    this.haloMaterial?.dispose();
    this.probeGlyphGeometry?.dispose();
    this.probeGlyphMaterial?.dispose();
    this.satelliteGeometry?.dispose();
    this.satelliteMaterial?.dispose();
    this.probeGlyphGeometry = null;
    this.probeGlyphMaterial = null;
    this.probeGlyphMesh = null;
    this.satelliteGeometry = null;
    this.satelliteMaterial = null;
    this.satelliteMesh = null;
    this.probeSlots = [];
    this.satelliteSlots = [];
    this.sdfTexture?.dispose();
    this.placeholderSdf?.dispose();
    this.sdfTexture = null;
    this.placeholderSdf = null;

    this.scene?.clear();
    this.renderer?.dispose();
    this.renderer?.forceContextLoss();
    this.renderer = null;
    this.scene = null;
    this.camera = null;
    this.mounted = false;
  }

  // -- internals --------------------------------------------------------------

  private buildGlobe(): void {
    if (!this.scene) return;
    const geom = new SphereGeometry(SPHERE_RADIUS, SPHERE_SEGMENTS, SPHERE_SEGMENTS);
    this.placeholderSdf = makeOceanPlaceholderTexture();
    this.oceanMaterial = new ShaderMaterial({
      vertexShader: terminatorVert,
      fragmentShader: terminatorFrag,
      // `fwidth` is a derivatives function. WebGL2 (three's current default)
      // has it in core; the WebGL1 fallback uses the
      // `#extension GL_OES_standard_derivatives` pragma inside the shader
      // source, so no extensions flag is needed on the material.
      uniforms: {
        uSunDirection: { value: new Vector3(1, 0, 0) },
        uOceanColor: { value: OCEAN_COLOR },
        uLandColor: { value: LAND_COLOR },
        uRimColor: { value: RIM_COLOR },
        uRimIntensity: { value: RIM_INTENSITY },
        uNightOceanFactor: { value: NIGHT_OCEAN_FACTOR },
        uNightLandFactor: { value: NIGHT_LAND_FACTOR },
        uLandIllumination: { value: LAND_ILLUMINATION },
        uLandSdf: { value: this.placeholderSdf },
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
      side: 0, // Frontside
      blending: 2,
      uniforms: {
        uSunDirection: { value: new Vector3(1, 0, 0) },
        uHaloColor: { value: HALO_COLOR },
        uIntensity: { value: 1.0 },
        uCameraDistance: { value: INITIAL_DISTANCE }
      }
    });
    this.haloMesh = new Mesh(geom, this.haloMaterial);
    this.scene.add(this.haloMesh);
  }

  private beginSdfLoad(): void {
    const loader = new TextureLoader();
    loader.load(
      this.config.landSdfUrl,
      (tex) => this.attachSdf(tex),
      undefined,
      (err) => console.warn('[engine-3d] landmass SDF load failed; ocean-only render', err)
    );
  }

  private attachSdf(tex: Texture): void {
    if (this.disposed || !this.oceanMaterial) {
      tex.dispose();
      return;
    }
    // RepeatWrapping on S so bilinear sampling across the ±180° longitude
    // seam stays continuous (the bake script padded the EDT for exactly
    // this reason). ClampToEdge on T so pole samples never wrap to the
    // opposite pole. Mipmaps are disabled because lower mip levels would
    // bleed ocean and land into each other along the coast and undo the
    // SDF's subpixel precision at high zoom.
    tex.wrapS = RepeatWrapping;
    tex.wrapT = ClampToEdgeWrapping;
    tex.minFilter = LinearFilter;
    tex.magFilter = LinearFilter;
    tex.generateMipmaps = false;
    tex.anisotropy = 1;
    tex.needsUpdate = true;
    this.sdfTexture = tex;
    this.oceanMaterial!.uniforms!.uLandSdf!.value = tex;
    this.placeholderSdf?.dispose();
    this.placeholderSdf = null;
  }

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
    this.applyFlyTween();
    this.controls?.update();
    this.updateSunUniform();

    if (this.haloMaterial?.uniforms.uCameraDistance && this.camera) {
      this.haloMaterial.uniforms.uCameraDistance.value = this.camera.position.length();
    }

    const t = this.clock.getElapsedTime();
    if (this.probeGlyphMaterial?.uniforms.uTime) {
      this.probeGlyphMaterial.uniforms.uTime.value = t;
    }
    if (this.satelliteMaterial?.uniforms.uTime) {
      this.satelliteMaterial.uniforms.uTime.value = t;
    }

    if (this.pointerInsideCanvas) {
      this.updateHover();
    }

    if (this.renderer && this.scene && this.camera) {
      this.renderer.render(this.scene, this.camera);
    }
    this.rafId = requestAnimationFrame(this.tick);
  };

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
    setVec3(this.haloMaterial?.uniforms.uSunDirection?.value, this.tmpSun);
  }

  // -- glow layer -------------------------------------------------------

  private buildGlowLayer(): void {
    if (!this.scene) return;

    this.probeGlyphGeometry = new BufferGeometry();
    this.probeGlyphMaterial = new ShaderMaterial({
      vertexShader: glowVert,
      fragmentShader: glowFrag,
      transparent: true,
      depthWrite: false,
      blending: AdditiveBlending,
      uniforms: {
        uTime: { value: 0 },
        uPixelRatio: { value: this.config.pixelRatioCap },
        uPointWorldSize: { value: GLOW_POINT_WORLD_SIZE },
        uGlowColor: { value: GLOW_COLOR },
        uBrightnessScale: { value: GLOW_BRIGHTNESS_SCALE },
        uHaloBrightness: { value: GLOW_HALO_BRIGHTNESS },
        uOuterRingBrightness: { value: GLOW_OUTER_RING_BRIGHTNESS }
      }
    });
    this.probeGlyphMesh = new Points(this.probeGlyphGeometry, this.probeGlyphMaterial);
    this.probeGlyphMesh.frustumCulled = false;
    this.probeGlyphMesh.visible = false;
    // Higher renderOrder so probe glyphs draw over satellites in the
    // additive layer (they share radius almost exactly).
    this.probeGlyphMesh.renderOrder = 2;
    this.scene.add(this.probeGlyphMesh);

    this.satelliteGeometry = new BufferGeometry();
    this.satelliteMaterial = new ShaderMaterial({
      vertexShader: glowVert,
      fragmentShader: glowFrag,
      transparent: true,
      depthWrite: false,
      blending: AdditiveBlending,
      uniforms: {
        uTime: { value: 0 },
        uPixelRatio: { value: this.config.pixelRatioCap },
        uPointWorldSize: { value: SATELLITE_POINT_WORLD_SIZE },
        uGlowColor: { value: GLOW_COLOR },
        // Muted: probe glyph carries scope identity, satellites are read-only origins.
        uBrightnessScale: { value: GLOW_BRIGHTNESS_SCALE * SATELLITE_BRIGHTNESS_SCALE },
        uHaloBrightness: { value: GLOW_HALO_BRIGHTNESS * SATELLITE_BRIGHTNESS_SCALE },
        uOuterRingBrightness: { value: GLOW_OUTER_RING_BRIGHTNESS * SATELLITE_BRIGHTNESS_SCALE }
      }
    });
    this.satelliteMesh = new Points(this.satelliteGeometry, this.satelliteMaterial);
    this.satelliteMesh.frustumCulled = false;
    this.satelliteMesh.visible = false;
    this.satelliteMesh.renderOrder = 1;
    this.scene.add(this.satelliteMesh);
  }

  private rebuildGlowLayers(): void {
    if (
      !this.probeGlyphGeometry ||
      !this.probeGlyphMesh ||
      !this.satelliteGeometry ||
      !this.satelliteMesh
    )
      return;

    const probeSlots: typeof this.probeSlots = [];
    const satelliteSlots: typeof this.satelliteSlots = [];
    for (const probe of this.probes) {
      if (probe.emissionPoints.length === 0) continue;
      const centroid = probeCentroidLatLon(probe.emissionPoints);
      probeSlots.push({
        probeId: probe.id,
        label: probe.label ?? probe.id,
        position: latLonToCartesian(centroid.latitude, centroid.longitude, GLOW_RADIUS)
      });
      for (const ep of probe.emissionPoints) {
        if (!ep.sourceName) continue;
        satelliteSlots.push({
          probeId: probe.id,
          sourceName: ep.sourceName,
          label: ep.label,
          position: latLonToCartesian(ep.latitude, ep.longitude, SATELLITE_RADIUS)
        });
      }
    }
    this.probeSlots = probeSlots;
    this.satelliteSlots = satelliteSlots;
    this.pointerHoveredProbe = -1;
    this.externalHoveredProbe = -1;
    this.pointerHoveredSatellite = -1;
    this.selectedProbeIndex = -1;

    this.probeGlyphGeometry = this.replaceGlowGeometry(
      this.probeGlyphGeometry,
      this.probeGlyphMesh,
      probeSlots.map((s) => s.position),
      true
    );
    this.satelliteGeometry = this.replaceGlowGeometry(
      this.satelliteGeometry,
      this.satelliteMesh,
      satelliteSlots.map((s) => s.position),
      false
    );

    // Re-apply any activity we already knew about so a probe re-push
    // does not wipe its pulse.
    this.applyActivityToBuffers();
    this.setSelection(this.currentSelection);
  }

  private replaceGlowGeometry(
    previous: BufferGeometry,
    mesh: Points,
    positions: readonly Vector3[],
    withSelection: boolean
  ): BufferGeometry {
    previous.dispose();
    const next = new BufferGeometry();
    const n = positions.length;
    if (n === 0) {
      mesh.geometry = next;
      mesh.visible = false;
      return next;
    }
    const pos = new Float32Array(n * 3);
    const pulseRates = new Float32Array(n);
    const brightness = new Float32Array(n);
    const hover = new Float32Array(n);
    const selected = new Float32Array(n);
    for (let i = 0; i < n; i++) {
      const p = positions[i]!;
      pos[i * 3 + 0] = p.x;
      pos[i * 3 + 1] = p.y;
      pos[i * 3 + 2] = p.z;
      brightness[i] = CORE_BRIGHTNESS_FLOOR;
    }
    next.setAttribute('position', new BufferAttribute(pos, 3));
    next.setAttribute('aPulseRate', new BufferAttribute(pulseRates, 1));
    next.setAttribute('aCoreBrightness', new BufferAttribute(brightness, 1));
    next.setAttribute('aHover', new BufferAttribute(hover, 1));
    // The shader expects aSelected on every vertex (varying flow). For
    // satellites we keep it pinned to 0 — selection only highlights probe
    // glyphs (Phase 110 contract).
    next.setAttribute('aSelected', new BufferAttribute(selected, 1));
    void withSelection;
    mesh.geometry = next;
    mesh.visible = true;
    return next;
  }

  private applyActivityToBuffers(): void {
    if (this.probeGlyphGeometry) {
      this.applyActivityToGeometry(this.probeGlyphGeometry, this.probeSlots);
    }
    if (this.satelliteGeometry) {
      this.applyActivityToGeometry(this.satelliteGeometry, this.satelliteSlots);
    }
  }

  private applyActivityToGeometry(
    geom: BufferGeometry,
    slots: ReadonlyArray<{ probeId: string }>
  ): void {
    const pulseAttr = geom.getAttribute('aPulseRate') as BufferAttribute | undefined;
    const brightAttr = geom.getAttribute('aCoreBrightness') as BufferAttribute | undefined;
    if (!pulseAttr || !brightAttr) return;
    for (let i = 0; i < slots.length; i++) {
      const docsPerHour = this.activityByProbeId.get(slots[i]!.probeId) ?? 0;
      pulseAttr.setX(i, computePulseRate(docsPerHour));
      brightAttr.setX(i, computeCoreBrightness(docsPerHour));
    }
    pulseAttr.needsUpdate = true;
    brightAttr.needsUpdate = true;
  }

  // -- interaction ------------------------------------------------------

  private readonly onPointerMove = (e: PointerEvent): void => {
    this.pointerInsideCanvas = true;
    this.updatePointerNdc(e);
  };

  private readonly onPointerLeave = (): void => {
    this.pointerInsideCanvas = false;
    if (this.pointerHoveredProbe !== -1) {
      this.setPointerHoverProbe(-1);
      this.emitter.emit('probe-hovered', null);
    }
    if (this.pointerHoveredSatellite !== -1) {
      this.pointerHoveredSatellite = -1;
      this.emitter.emit('satellite-hovered', null);
    }
  };

  private readonly onCanvasClick = (e: MouseEvent): void => {
    this.updatePointerNdc(e);
    const probeHit = this.pickProbeSlot();
    if (probeHit !== -1) {
      const slot = this.probeSlots[probeHit]!;
      this.emitter.emit('probe-selected', { probeId: slot.probeId });
      return;
    }
    const satHit = this.pickSatelliteSlot();
    if (satHit !== -1) {
      const s = this.satelliteSlots[satHit]!;
      this.emitter.emit('satellite-selected', toSatelliteSelection(s));
    }
  };

  private updatePointerNdc(e: MouseEvent): void {
    const canvas = this.renderer?.domElement;
    if (!canvas) return;
    const rect = canvas.getBoundingClientRect();
    // NDC: (-1,-1) bottom-left → (1,1) top-right.
    this.pointerNdc.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
    this.pointerNdc.y = -(((e.clientY - rect.top) / rect.height) * 2 - 1);
  }

  private updateHover(): void {
    // Probe glyphs take precedence — they are the scope-target. A hover
    // over a probe suppresses any concurrent satellite-hover so the
    // tooltip never flickers between two registers.
    const probeHit = this.pickProbeSlot();
    if (probeHit !== this.pointerHoveredProbe) {
      this.setPointerHoverProbe(probeHit);
      if (probeHit === -1) {
        this.emitter.emit('probe-hovered', null);
      } else {
        this.emitter.emit('probe-hovered', { probeId: this.probeSlots[probeHit]!.probeId });
      }
    }

    if (probeHit !== -1) {
      if (this.pointerHoveredSatellite !== -1) {
        this.pointerHoveredSatellite = -1;
        this.emitter.emit('satellite-hovered', null);
      }
      return;
    }

    const satHit = this.pickSatelliteSlot();
    if (satHit === this.pointerHoveredSatellite) return;
    this.pointerHoveredSatellite = satHit;
    if (satHit === -1) {
      this.emitter.emit('satellite-hovered', null);
    } else {
      this.emitter.emit('satellite-hovered', toSatelliteSelection(this.satelliteSlots[satHit]!));
    }
  }

  private pickProbeSlot(): number {
    return this.pickGlowSlot(this.probeGlyphMesh, this.probeSlots);
  }

  private pickSatelliteSlot(): number {
    return this.pickGlowSlot(this.satelliteMesh, this.satelliteSlots);
  }

  private pickGlowSlot(mesh: Points | null, slots: ReadonlyArray<{ position: Vector3 }>): number {
    if (!this.camera || !mesh || slots.length === 0 || !mesh.visible) return -1;
    this.raycaster.setFromCamera(this.pointerNdc, this.camera);

    // Dynamic raycast threshold based on camera distance.
    // At the initial distance of 3.0, camDist * 0.01 exactly matches the previous
    // static threshold of 0.03. As the user zooms in (e.g., to distance 1.2),
    // the world-space threshold shrinks proportionally to 0.012, keeping the
    // interactive hit area visually stable on the screen.
    const camDist = this.camera.position.length();
    this.raycaster.params.Points = { threshold: camDist * 0.01 };

    const intersects = this.raycaster.intersectObject(mesh, false);
    if (intersects.length === 0) return -1;

    const candidates: RaycastCandidate[] = [];
    for (const hit of intersects) {
      const idx = hit.index;
      if (idx === undefined || idx < 0 || idx >= slots.length) continue;
      candidates.push({ index: idx, position: slots[idx]!.position });
    }
    return pickNearSideHit(candidates, this.camera.position);
  }

  private setPointerHoverProbe(idx: number): void {
    if (idx === this.pointerHoveredProbe) return;
    const prev = this.pointerHoveredProbe;
    this.pointerHoveredProbe = idx;
    this.refreshProbeHoverAttr(prev);
  }

  private setExternalHoverProbe(idx: number): void {
    if (idx === this.externalHoveredProbe) return;
    const prev = this.externalHoveredProbe;
    this.externalHoveredProbe = idx;
    this.refreshProbeHoverAttr(prev);
  }

  private refreshProbeHoverAttr(previouslyLit: number): void {
    if (!this.probeGlyphGeometry) return;
    const attr = this.probeGlyphGeometry.getAttribute('aHover') as BufferAttribute | undefined;
    if (!attr) return;
    const p = this.pointerHoveredProbe;
    const x = this.externalHoveredProbe;
    if (
      previouslyLit !== -1 &&
      previouslyLit < this.probeSlots.length &&
      previouslyLit !== p &&
      previouslyLit !== x
    ) {
      attr.setX(previouslyLit, 0);
    }
    if (p !== -1 && p < this.probeSlots.length) attr.setX(p, 1);
    if (x !== -1 && x < this.probeSlots.length) attr.setX(x, 1);
    attr.needsUpdate = true;
  }
}

function toSatelliteSelection(slot: {
  probeId: string;
  sourceName: string;
  label: string;
}): SatelliteSelection {
  return {
    probeId: slot.probeId,
    sourceName: slot.sourceName,
    label: slot.label
  };
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
  emit<K extends keyof EngineEvents>(event: K, ...args: Parameters<EngineEvents[K]>): void {
    const set = this.handlers.get(event);
    if (!set) return;
    for (const handler of set) {
      (handler as (...a: Parameters<EngineEvents[K]>) => void)(...args);
    }
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

// 1×1 fully-ocean SDF used before the real texture finishes loading, so the
// ocean shader samples a defined value instead of WebGL's default white
// texture (which would flash the entire globe "land"-coloured for ~100 ms).
function makeOceanPlaceholderTexture(): DataTexture {
  const tex = new DataTexture(new Uint8Array([0, 0, 0, 255]), 1, 1);
  tex.needsUpdate = true;
  return tex;
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
