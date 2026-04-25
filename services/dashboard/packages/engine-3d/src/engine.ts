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
  pickNearSideHit
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
  PropagationEvent
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

// Emission-point glows sit a hair above the surface so the disc is not
// z-fought by the globe shader along the rim and so hits on the far side
// of the sphere can be filtered by a normal-vs-camera check.
const GLOW_RADIUS = 1.003;
// World-space diameter of a glow at 1 unit from camera. The vertex
// shader divides by depth so the disc's apparent size is stable as the
// camera orbits.
const GLOW_POINT_WORLD_SIZE = 80.0;
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
  // One Points mesh renders every emission point across all probes.
  // `emissionSlots` is the CPU-side metadata parallel to the geometry's
  // vertex attributes; index i in the mesh corresponds to entry i here.
  // The raycaster maps a hit index back to (probeId, emissionPointIndex,
  // label) through this array.
  private glowMesh: Points | null = null;
  private glowGeometry: BufferGeometry | null = null;
  private glowMaterial: ShaderMaterial | null = null;
  private emissionSlots: Array<{
    readonly probeId: string;
    readonly emissionPointIndex: number;
    readonly label: string;
    readonly position: Vector3;
  }> = [];
  private pointerHoveredSlotIndex = -1;
  private externalHoveredSlotIndex = -1;
  private selectedSlotIndex = -1;
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
    this.rebuildGlowLayer();
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
      this.setExternalHoverSlot(-1);
      return;
    }
    const idx = this.emissionSlots.findIndex(
      (s) =>
        s.probeId === selection.probeId && s.emissionPointIndex === selection.emissionPointIndex
    );
    this.setExternalHoverSlot(idx);
  }

  setSelection(selection: ProbeSelection | null): void {
    this.currentSelection = selection;

    if (!this.glowGeometry) return;
    const attr = this.glowGeometry.getAttribute('aSelected') as BufferAttribute | undefined;
    if (!attr) return;

    // Clear previous selection
    if (this.selectedSlotIndex !== -1 && this.selectedSlotIndex < this.emissionSlots.length) {
      attr.setX(this.selectedSlotIndex, 0);
    }

    this.selectedSlotIndex = -1;

    // Apply new selection if provided
    if (selection) {
      const idx = this.emissionSlots.findIndex(
        (s) =>
          s.probeId === selection.probeId && s.emissionPointIndex === selection.emissionPointIndex
      );
      if (idx !== -1) {
        this.selectedSlotIndex = idx;
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
    this.glowGeometry?.dispose();
    this.glowMaterial?.dispose();
    this.glowGeometry = null;
    this.glowMaterial = null;
    this.glowMesh = null;
    this.emissionSlots = [];
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

    if (this.glowMaterial?.uniforms.uTime) {
      this.glowMaterial.uniforms.uTime.value = this.clock.getElapsedTime();
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
    this.glowGeometry = new BufferGeometry();
    this.glowMaterial = new ShaderMaterial({
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
    this.glowMesh = new Points(this.glowGeometry, this.glowMaterial);
    // Start with nothing to render until setProbes() supplies data.
    this.glowMesh.frustumCulled = false;
    this.glowMesh.visible = false;
    this.scene.add(this.glowMesh);
  }

  private rebuildGlowLayer(): void {
    if (!this.glowGeometry || !this.glowMesh) return;

    const slots: typeof this.emissionSlots = [];
    for (const probe of this.probes) {
      probe.emissionPoints.forEach((ep, idx) => {
        slots.push({
          probeId: probe.id,
          emissionPointIndex: idx,
          label: ep.label,
          position: latLonToCartesian(ep.latitude, ep.longitude, GLOW_RADIUS)
        });
      });
    }
    this.emissionSlots = slots;
    this.pointerHoveredSlotIndex = -1;
    this.externalHoveredSlotIndex = -1;

    const n = slots.length;
    if (n === 0) {
      this.glowMesh.visible = false;
      return;
    }

    const positions = new Float32Array(n * 3);
    const pulseRates = new Float32Array(n);
    const brightness = new Float32Array(n);
    const hover = new Float32Array(n);
    const selected = new Float32Array(n);
    for (let i = 0; i < n; i++) {
      const slot = slots[i]!;
      const p = slot.position;
      positions[i * 3 + 0] = p.x;
      positions[i * 3 + 1] = p.y;
      positions[i * 3 + 2] = p.z;
      brightness[i] = CORE_BRIGHTNESS_FLOOR;
      pulseRates[i] = 0;
      hover[i] = 0;
      selected[i] = 0;
    }

    // Dispose the previous attribute buffers before swapping so the GL
    // resources do not leak across successive setProbes() calls.
    this.glowGeometry.dispose();
    this.glowGeometry = new BufferGeometry();
    this.glowGeometry.setAttribute('position', new BufferAttribute(positions, 3));
    this.glowGeometry.setAttribute('aPulseRate', new BufferAttribute(pulseRates, 1));
    this.glowGeometry.setAttribute('aCoreBrightness', new BufferAttribute(brightness, 1));
    this.glowGeometry.setAttribute('aHover', new BufferAttribute(hover, 1));
    this.glowGeometry.setAttribute('aSelected', new BufferAttribute(selected, 1));
    this.glowMesh.geometry = this.glowGeometry;
    this.glowMesh.visible = true;

    // Re-apply any activity we already knew about so a probe re-push
    // does not wipe its pulse.
    this.applyActivityToBuffers();
    this.setSelection(this.currentSelection);
  }

  private applyActivityToBuffers(): void {
    if (!this.glowGeometry) return;
    const pulseAttr = this.glowGeometry.getAttribute('aPulseRate') as BufferAttribute | undefined;
    const brightAttr = this.glowGeometry.getAttribute('aCoreBrightness') as
      | BufferAttribute
      | undefined;
    if (!pulseAttr || !brightAttr) return;

    for (let i = 0; i < this.emissionSlots.length; i++) {
      const probeId = this.emissionSlots[i]!.probeId;
      const docsPerHour = this.activityByProbeId.get(probeId) ?? 0;
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
    if (this.pointerHoveredSlotIndex !== -1) {
      this.setPointerHoverSlot(-1);
      this.emitter.emit('probe-hovered', null);
    }
  };

  private readonly onCanvasClick = (e: MouseEvent): void => {
    this.updatePointerNdc(e);
    const hit = this.pickEmissionSlot();
    if (hit !== -1) {
      const slot = this.emissionSlots[hit]!;
      this.emitter.emit('probe-selected', toSelection(slot));
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
    const hit = this.pickEmissionSlot();
    if (hit === this.pointerHoveredSlotIndex) return;
    this.setPointerHoverSlot(hit);
    if (hit === -1) {
      this.emitter.emit('probe-hovered', null);
    } else {
      this.emitter.emit('probe-hovered', toSelection(this.emissionSlots[hit]!));
    }
  }

  private pickEmissionSlot(): number {
    if (!this.camera || !this.glowMesh || this.emissionSlots.length === 0) return -1;
    this.raycaster.setFromCamera(this.pointerNdc, this.camera);

    // Dynamic raycast threshold based on camera distance.
    // At the initial distance of 3.0, camDist * 0.01 exactly matches the previous
    // static threshold of 0.03. As the user zooms in (e.g., to distance 1.2),
    // the world-space threshold shrinks proportionally to 0.012, keeping the
    // interactive hit area visually stable on the screen.
    const camDist = this.camera.position.length();
    this.raycaster.params.Points = { threshold: camDist * 0.01 };

    const intersects = this.raycaster.intersectObject(this.glowMesh, false);
    if (intersects.length === 0) return -1;

    const candidates: RaycastCandidate[] = [];
    for (const hit of intersects) {
      const idx = hit.index;
      if (idx === undefined || idx < 0 || idx >= this.emissionSlots.length) continue;
      candidates.push({ index: idx, position: this.emissionSlots[idx]!.position });
    }
    return pickNearSideHit(candidates, this.camera.position);
  }

  private setPointerHoverSlot(idx: number): void {
    if (idx === this.pointerHoveredSlotIndex) return;
    const prev = this.pointerHoveredSlotIndex;
    this.pointerHoveredSlotIndex = idx;
    this.refreshHoverAttr(prev);
  }

  private setExternalHoverSlot(idx: number): void {
    if (idx === this.externalHoveredSlotIndex) return;
    const prev = this.externalHoveredSlotIndex;
    this.externalHoveredSlotIndex = idx;
    this.refreshHoverAttr(prev);
  }

  private refreshHoverAttr(previouslyLit: number): void {
    if (!this.glowGeometry) return;
    const attr = this.glowGeometry.getAttribute('aHover') as BufferAttribute | undefined;
    if (!attr) return;
    const p = this.pointerHoveredSlotIndex;
    const x = this.externalHoveredSlotIndex;
    if (
      previouslyLit !== -1 &&
      previouslyLit < this.emissionSlots.length &&
      previouslyLit !== p &&
      previouslyLit !== x
    ) {
      attr.setX(previouslyLit, 0);
    }
    if (p !== -1 && p < this.emissionSlots.length) attr.setX(p, 1);
    if (x !== -1 && x < this.emissionSlots.length) attr.setX(x, 1);
    attr.needsUpdate = true;
  }
}

function toSelection(slot: {
  probeId: string;
  emissionPointIndex: number;
  label: string;
}): ProbeSelection {
  return {
    probeId: slot.probeId,
    emissionPointIndex: slot.emissionPointIndex,
    emissionPointLabel: slot.label
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
