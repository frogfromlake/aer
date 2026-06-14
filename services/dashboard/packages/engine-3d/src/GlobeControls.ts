// GlobeControls — a quaternion turntable + zoom-to-cursor controller for the
// Atmosphere globe (Phase 135).
//
// Why not OrbitControls? OrbitControls is a spherical (azimuth/polar, fixed-up)
// controller: it has a mathematical singularity at the poles and re-derives the
// spherical state from the camera each frame, so a custom zoom-to-cursor that
// moves the camera explodes near the poles. This controller instead keeps the
// camera orientation as a QUATERNION and the pivot fixed at the globe centre, so
// nothing ever gimbal-locks.
//
//   • Drag  — turntable: horizontal = yaw around WORLD up, vertical = pitch
//             around the camera-local right axis. Going "over the pole" just
//             keeps rotating (north flips smoothly), never explodes. Inertia on
//             release.
//   • Wheel — true zoom-to-cursor (Google-Earth style): the world point under the
//             cursor is PINNED under the cursor pixel while the radius dollies, so
//             you fall INTO that point (it stays under the cursor and grows). It
//             is never centred or chased. The anchor is captured once per gesture
//             (a continuous scroll) and holds for zoom-in and zoom-out alike.
//
// The pivot is always the globe centre (0,0,0): the camera looks at the centre
// every frame, so it can never drift sideways.

import {
  Matrix4,
  type PerspectiveCamera,
  Quaternion,
  Raycaster,
  Sphere,
  Vector2,
  Vector3
} from 'three';

export interface GlobeControlsOptions {
  minDistance: number;
  maxDistance: number;
  sphereRadius: number;
}

const WORLD_UP = new Vector3(0, 1, 0);
const ORIGIN = new Vector3(0, 0, 0);

// Scratch objects (no per-frame allocation).
const _fwd = new Vector3();
const _up = new Vector3();
const _right = new Vector3();
const _right2 = new Vector3();
const _v = new Vector3();
const _hit = new Vector3();
const _qa = new Quaternion();
const _qb = new Quaternion();
const _frameA = new Quaternion();
const _frameB = new Quaternion();
const _basis = new Matrix4();

export class GlobeControls {
  enabled = true;

  // ── Tunables ───────────────────────────────────────────────────────────────
  /** Drag rotation, radians per pixel (drag sensitivity) at/above the reference
   *  distance. Scaled down closer in so the *on-screen* surface speed stays
   *  stable (perspective magnifies near the surface). */
  rotateSpeed = 0.0025;
  /** Surface-distance (radius − sphereRadius) at/above which the full rotateSpeed
   *  applies. Below it the speed scales with distance so the felt speed is
   *  constant. Larger = the slow-down starts farther out. */
  dragSpeedRefGap = 1.5;
  /** Floor on the close-in drag speed factor (so the very surface isn't glacial). */
  minDragScale = 0.3;
  /** How fast the accumulated drag is "spent" per frame (0..1). Lower = more
   *  smoothing while dragging AND a longer glide after release (inertia). */
  dragDamping = 0.08;
  /** Multiplicative dolly base per wheel notch — smaller = bigger zoom steps. */
  zoomStep = 0.9;
  /** Wheel sensitivity multiplier. */
  zoomSpeed = 1.0;
  /** Per-frame ease of the radius toward its target (higher = snappier, lower =
   *  softer with a longer glide / coast). */
  radiusEase = 0.09;
  /** Per-frame fraction of the roll error corrected — keeps north up smoothly. */
  rollCorrect = 0.15;
  /** A scroll starting more than this many ms after the last notch begins a NEW gesture
   *  (re-captures the anchor under the cursor). */
  gestureGapMs = 220;

  private readonly camera: PerspectiveCamera;
  private readonly dom: HTMLElement;
  private readonly sphere: Sphere;
  private readonly minD: number;
  private readonly maxD: number;
  private readonly raycaster = new Raycaster();

  /** Camera orientation: maps the base frame (+Z toward the camera, +Y up) to world. */
  private readonly orientation = new Quaternion();
  private radius: number;
  private desiredRadius: number;

  private dragging = false;
  private lastX = 0;
  private lastY = 0;
  // Accumulated, not-yet-applied rotation. The update loop spends a fraction of
  // it per frame (dragDamping) → smooth drag while moving + glide after release.
  private pendingYaw = 0;
  private pendingPitch = 0;

  // Zoom-to-cursor anchor: the world point under the cursor (anchorDir) is PINNED
  // under the cursor pixel (anchorNdc) while the radius dollies — so you fall into
  // the cursor point (Google-Earth style), never centring it / chasing it. Both
  // are captured once per gesture (a continuous scroll).
  private anchorDir: Vector3 | null = null;
  private readonly anchorNdc = new Vector2();
  private lastWheelMs = -1e9;
  private lastWheelX = 0;
  private lastWheelY = 0;

  constructor(camera: PerspectiveCamera, dom: HTMLElement, opts: GlobeControlsOptions) {
    this.camera = camera;
    this.dom = dom;
    this.minD = opts.minDistance;
    this.maxD = opts.maxDistance;
    this.sphere = new Sphere(new Vector3(0, 0, 0), opts.sphereRadius);
    this.radius = camera.position.length() || (this.minD + this.maxD) / 2;
    this.desiredRadius = this.radius;
    this.syncFromCamera();

    dom.addEventListener('pointerdown', this.onPointerDown);
    dom.addEventListener('pointermove', this.onPointerMove);
    window.addEventListener('pointerup', this.onPointerUp);
    dom.addEventListener('wheel', this.onWheel, { passive: false });
  }

  dispose(): void {
    this.dom.removeEventListener('pointerdown', this.onPointerDown);
    this.dom.removeEventListener('pointermove', this.onPointerMove);
    window.removeEventListener('pointerup', this.onPointerUp);
    this.dom.removeEventListener('wheel', this.onWheel);
  }

  /** Re-derive orientation + radius from the current camera. Call this after an
   *  external write to `camera.position` (e.g. flyTo) so the controller resumes
   *  from there without a jump. */
  syncFromCamera(): void {
    const pos = this.camera.position;
    const len = pos.length();
    if (len > 1e-6) this.radius = len;
    this.desiredRadius = this.clampRadius(this.radius);
    this.anchorDir = null;
    _fwd.copy(pos).normalize(); // +Z of our frame = direction toward the camera
    _up.copy(this.camera.up).normalize();
    _right.crossVectors(_up, _fwd);
    if (_right.lengthSq() < 1e-8) _right.set(1, 0, 0);
    _right.normalize();
    _up.crossVectors(_fwd, _right).normalize();
    // Build orientation from the orthonormal basis (right, up, fwd).
    this.orientation.setFromRotationMatrix(_basis.makeBasis(_right, _up, _fwd));
  }

  update(): void {
    // Drag: spend a fraction of the accumulated rotation per frame. While the
    // pointer moves, input keeps refilling `pending`, so the view follows with a
    // smooth lag; after release `pending` drains on its own → glide / inertia.
    const applyYaw = this.pendingYaw * this.dragDamping;
    const applyPitch = this.pendingPitch * this.dragDamping;
    if (Math.abs(applyYaw) > 1e-6 || Math.abs(applyPitch) > 1e-6) {
      this.rotate(applyYaw, applyPitch);
      this.pendingYaw -= applyYaw;
      this.pendingPitch -= applyPitch;
    }

    // Dolly toward the desired radius. The eased tail is the zoom glide.
    const prevRadius = this.radius;
    this.radius += (this.desiredRadius - this.radius) * this.radiusEase;
    const zooming = Math.abs(this.radius - prevRadius) > 1e-6;

    // Keep north up EVERY frame (also while zooming). The anchor-pin below
    // re-pins the cursor point right after, so the net per-frame motion during a
    // zoom is yaw/pitch only — roll never accumulates, so there is no visible
    // correction at the end of the zoom. (Running this only when settled — as
    // before — let a whole zoom's worth of roll build up, then snap.)
    this.levelRoll();

    if (this.anchorDir && zooming && !this.dragging) {
      // ZOOM-TO-CURSOR: pin the captured world point under the cursor pixel while
      // the radius changes, so you fall INTO it (it stays under the cursor and
      // grows) instead of it being centred / chased. We rotate the globe by
      // exactly the drift the dolly (and the level step) introduced this frame.
      this.writeCamera(); // reflect the new radius before raycasting
      this.camera.updateMatrixWorld();
      this.raycaster.setFromCamera(this.anchorNdc, this.camera);
      const hit = this.raycaster.ray.intersectSphere(this.sphere, _hit);
      if (hit) {
        _hit.normalize();
        // Rotate so the point now under the cursor (_hit) maps to the anchor.
        // Frame-to-frame (north-up) rotation injects the least roll; levelRoll
        // (above, every frame) mops up the small remainder so it never builds.
        if (this.frameQuat(_hit, _frameA) && this.frameQuat(this.anchorDir, _frameB)) {
          _qa.copy(_frameB).multiply(_frameA.invert());
        } else {
          _qa.setFromUnitVectors(_hit, this.anchorDir); // pole fallback (rare)
        }
        this.orientation.premultiply(_qa).normalize();
      }
    }

    this.writeCamera();
  }

  /** Rotate the camera around its own forward axis to bring the horizon level
   *  (camera-up toward world-up projected into view), eased by `rollCorrect`. */
  private levelRoll(): void {
    _fwd.set(0, 0, 1).applyQuaternion(this.orientation); // view-centre dir
    // desired up = world-up with the forward component removed.
    const d = WORLD_UP.dot(_fwd);
    _up.copy(WORLD_UP).addScaledVector(_fwd, -d);
    if (_up.lengthSq() < 1e-3) return; // near a pole: leave roll free
    _up.normalize();
    _right.set(0, 1, 0).applyQuaternion(this.orientation); // current up
    // Signed roll angle from current-up to desired-up about the forward axis.
    const cos = Math.min(1, Math.max(-1, _right.dot(_up)));
    _v.crossVectors(_right, _up);
    const sin = _v.dot(_fwd);
    const angle = Math.atan2(sin, cos);
    if (Math.abs(angle) < 1e-4) return;
    _qa.setFromAxisAngle(_fwd, angle * this.rollCorrect);
    this.orientation.premultiply(_qa).normalize();
  }

  /** Build the north-up frame quaternion for a direction: forward = `dir`, up =
   *  world-up projected perpendicular to it. Returns false near the poles where
   *  "up" is undefined (the caller falls back to a minimal rotation). */
  private frameQuat(dir: Vector3, out: Quaternion): boolean {
    const d = WORLD_UP.dot(dir);
    _v.copy(WORLD_UP).addScaledVector(dir, -d); // up ⟂ dir
    if (_v.lengthSq() < 1e-6) return false;
    _v.normalize();
    _right2.crossVectors(_v, dir).normalize(); // right = up × fwd
    out.setFromRotationMatrix(_basis.makeBasis(_right2, _v, dir));
    return true;
  }

  // ── internals ────────────────────────────────────────────────────────────────

  private clampRadius(r: number): number {
    return Math.min(this.maxD, Math.max(this.minD, r));
  }

  private writeCamera(): void {
    _fwd.set(0, 0, 1).applyQuaternion(this.orientation);
    this.camera.position.copy(_fwd).multiplyScalar(this.radius);
    _up.set(0, 1, 0).applyQuaternion(this.orientation);
    this.camera.up.copy(_up);
    this.camera.lookAt(ORIGIN);
  }

  /** Turntable rotation: yaw around WORLD up, pitch around the camera-local right
   *  axis. Both are applied as quaternion premultiplies, so crossing a pole just
   *  keeps rotating (no singularity). */
  private rotate(yaw: number, pitch: number): void {
    _right.set(1, 0, 0).applyQuaternion(this.orientation);
    _qa.setFromAxisAngle(_right, pitch);
    _qb.setFromAxisAngle(WORLD_UP, yaw);
    this.orientation.premultiply(_qa).premultiply(_qb).normalize();
  }

  private onPointerDown = (e: PointerEvent): void => {
    if (!this.enabled || e.button !== 0) return;
    this.dragging = true;
    this.lastX = e.clientX;
    this.lastY = e.clientY;
    this.pendingYaw = 0;
    this.pendingPitch = 0;
    // A drag cancels any in-flight zoom anchor so they never fight.
    this.anchorDir = null;
  };

  private onPointerMove = (e: PointerEvent): void => {
    if (!this.dragging) return;
    const dx = e.clientX - this.lastX;
    const dy = e.clientY - this.lastY;
    this.lastX = e.clientX;
    this.lastY = e.clientY;
    // Scale by distance-from-surface so the felt (on-screen) speed stays stable:
    // perspective magnifies the surface as the camera nears it, so the same angle
    // sweeps more pixels close in. ∝ gap below the reference, capped at 1 above it.
    const gap = this.radius - this.sphere.radius;
    const scale = Math.min(1, Math.max(this.minDragScale, gap / this.dragSpeedRefGap));
    // Accumulate — the update loop spends it gradually (smoothing + inertia).
    this.pendingYaw += -dx * this.rotateSpeed * scale;
    this.pendingPitch += -dy * this.rotateSpeed * scale;
  };

  private onPointerUp = (): void => {
    this.dragging = false;
  };

  private onWheel = (e: WheelEvent): void => {
    if (!this.enabled) return;
    e.preventDefault();
    const unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 400 : 1;
    const steps = (e.deltaY * unit) / 100; // ≈ ±1 per notch
    const factor = Math.pow(this.zoomStep, -steps * this.zoomSpeed);
    this.desiredRadius = this.clampRadius(this.desiredRadius * factor);

    // (Re)capture the cursor anchor only when a NEW gesture begins — a pause or a
    // moved cursor. Keeping the SAME anchor + cursor pixel through a continuous
    // scroll is what pins the point under the cursor instead of chasing it. The
    // anchor holds for zoom-in AND zoom-out (the point stays put either way).
    const now = performance.now();
    const moved = Math.hypot(e.clientX - this.lastWheelX, e.clientY - this.lastWheelY) > 6;
    if (now - this.lastWheelMs > this.gestureGapMs || moved || !this.anchorDir) {
      const rect = this.dom.getBoundingClientRect();
      this.anchorNdc.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
      this.anchorNdc.y = -(((e.clientY - rect.top) / rect.height) * 2 - 1);
      this.raycaster.setFromCamera(this.anchorNdc, this.camera);
      const hit = this.raycaster.ray.intersectSphere(this.sphere, _hit);
      this.anchorDir = hit ? _hit.clone().normalize() : null;
    }
    this.lastWheelMs = now;
    this.lastWheelX = e.clientX;
    this.lastWheelY = e.clientY;
  };
}
