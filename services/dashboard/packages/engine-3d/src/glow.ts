// Pure helpers backing the Phase 99b emission-point glow layer. Kept in
// their own module so the display-logic maths (probe activity → shader
// uniform) can be unit-tested without instantiating the full engine.

import type { Vector3 } from 'three';

export interface RaycastCandidate {
  index: number;
  position: Vector3;
}

// Given raycaster hits against the glow Points mesh (in ray-distance
// order) and the camera position, pick the first candidate on the near
// hemisphere. Raycaster.intersectObject on a Points material happily
// returns points behind the globe because the material has no depth test
// against the opaque sphere, so we project each candidate's surface
// normal onto the camera direction and reject any hit whose dot product
// is ≤ 0 (far side or exactly on the horizon).
export function pickNearSideHit(
  hits: readonly RaycastCandidate[],
  cameraPosition: Vector3
): number {
  if (hits.length === 0) return -1;
  const camX = cameraPosition.x;
  const camY = cameraPosition.y;
  const camZ = cameraPosition.z;
  const camLen = Math.hypot(camX, camY, camZ);
  if (camLen === 0) return -1;
  const cx = camX / camLen;
  const cy = camY / camLen;
  const cz = camZ / camLen;
  for (const hit of hits) {
    const p = hit.position;
    const len = Math.hypot(p.x, p.y, p.z);
    if (len === 0) continue;
    const dot = (p.x * cx + p.y * cy + p.z * cz) / len;
    if (dot > 0) return hit.index;
  }
  return -1;
}

// 2π / 4 s — the fastest pulse we ever render. §1.1 of the Design Brief
// ("stillness with motion beneath") rules out anything faster; a busy
// probe must still read as calm atmosphere, not as a dashboard alarm.
export const MAX_PULSE_RAD_PER_S = (2.0 * Math.PI) / 4.0;

// Publication-rate at which the pulse saturates at MAX_PULSE_RAD_PER_S.
// Tuned for institutional RSS volumes (Probe 0: ARD ≈ 2/h, BPA ≈ 0.2/h)
// so a busy news cycle reads as more alive without a quiet probe pulsing
// visibly in the still baseline.
export const PULSE_SATURATION_DOCS_PER_HOUR = 10.0;

// A dormant probe must remain visible, so the core never dips below this
// floor. Activity raises it up to 1.0.
export const CORE_BRIGHTNESS_FLOOR = 0.45;
export const CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR = 20.0;

export function computePulseRate(docsPerHour: number): number {
  if (!Number.isFinite(docsPerHour) || docsPerHour <= 0) return 0;
  const t = Math.min(1, docsPerHour / PULSE_SATURATION_DOCS_PER_HOUR);
  return t * MAX_PULSE_RAD_PER_S;
}

export function computeCoreBrightness(docsPerHour: number): number {
  if (!Number.isFinite(docsPerHour) || docsPerHour <= 0) return CORE_BRIGHTNESS_FLOOR;
  const t = Math.min(1, docsPerHour / CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR);
  return CORE_BRIGHTNESS_FLOOR + (1 - CORE_BRIGHTNESS_FLOOR) * t;
}

/**
 * Spherical centroid of a probe's emission points, returned as
 * (latitude, longitude) in degrees. Sums Cartesian unit vectors and
 * normalises — robust across the ±180° dateline (a naive arithmetic
 * mean of longitudes would land in the middle of the wrong hemisphere
 * for a probe spanning the dateline). Falls back to (0, 0) when the
 * input is empty or all points cancel out (antipodal).
 */
export function probeCentroidLatLon(points: readonly { latitude: number; longitude: number }[]): {
  latitude: number;
  longitude: number;
} {
  if (points.length === 0) return { latitude: 0, longitude: 0 };
  if (points.length === 1) {
    const p = points[0]!;
    return { latitude: p.latitude, longitude: p.longitude };
  }
  const DEG = Math.PI / 180;
  let x = 0;
  let y = 0;
  let z = 0;
  for (const p of points) {
    const lat = p.latitude * DEG;
    const lon = p.longitude * DEG;
    const c = Math.cos(lat);
    x += c * Math.cos(lon);
    y += c * Math.sin(lon);
    z += Math.sin(lat);
  }
  const len = Math.hypot(x, y, z);
  if (len < 1e-9) return { latitude: 0, longitude: 0 };
  x /= len;
  y /= len;
  z /= len;
  const lat = Math.asin(Math.max(-1, Math.min(1, z))) / DEG;
  const lon = Math.atan2(y, x) / DEG;
  return { latitude: lat, longitude: lon };
}
