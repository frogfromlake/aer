// Spiderfy layout for co-located globe markers — Phase 123b precursor.
//
// Problem: when two emission points sit in the same city (e.g. Probe 1's
// franceinfo + Élysée, both in Paris, ~3.5 km / ~0.03° apart) their glow
// points project onto the same pixels and visually merge into one — the user
// cannot see or click the individual source satellites. At globe scale this is
// the common case (national media + seats of government concentrate in
// capitals), so it must be solved generically.
//
// Approach: points whose great-circle separation is below a small angular
// threshold are grouped into a *cluster* and fanned out evenly around the
// cluster centroid, in the tangent plane of the sphere at that centroid.
//
// Crucially the fan magnitude is SCREEN-SPACE-CONSTANT, not fixed in world
// units: 3.5 km never resolves on its own at any globe zoom, so a fixed-world
// fan is wrong both ways — too small to see when zoomed out, and flung far from
// the true city when large. Under perspective the screen separation of a world
// offset scales ~1/cameraDistance, so to hold the screen gap constant the world
// offset must scale WITH camera distance (`worldFanForDistance`). That makes the
// markers stay separated at every zoom AND visibly react to zoom (they "unfold"
// as you pull back), while staying close to the true location when zoomed in.
//
// The rule is purely geometric, never probe-specific: a pair that is genuinely
// far apart (Probe 0's Hamburg↔Berlin, ~2.25°) never enters a cluster and is
// never fanned. Same rule for every probe — co-located sources fan, separated
// ones don't.
//
// All functions are pure (no three.js scene state) so the layout maths is
// unit-testable without a WebGL context, matching the `glow.ts` pattern.

import { Vector3 } from 'three';

/**
 * Per-point spiderfy layout. `offsetDir` is a unit tangent vector at the
 * cluster centroid; the engine displaces a clustered point along it by a
 * camera-derived world magnitude (`worldFanForDistance`). Singletons carry a
 * zero direction and are never moved.
 */
export interface SpiderSlot {
  /** -1 for singletons; otherwise the index of the cluster this point joined. */
  readonly clusterId: number;
  /** Unit tangent direction to fan along. Zero vector for singletons. */
  readonly offsetDir: Vector3;
}

/** Default: ~0.5° great-circle. Pairs closer than this are treated as co-located. */
export const DEFAULT_CLUSTER_THRESHOLD_RAD = (0.5 * Math.PI) / 180;

/**
 * Screen-space-constant fan tuning (unit sphere; camera distance in the same
 * units, ranging ~1.2 near to ~8 far, ~3 default — see engine MIN/MAX/INITIAL).
 *
 * `worldFan = clamp(COEFF * cameraDistance, MIN, MAX)`. COEFF is chosen so the
 * fan is a comfortable ~1.3° at the default distance; the clamp keeps it
 * visible when zoomed in and stops it flinging markers off-city when zoomed all
 * the way out.
 */
export const FAN_SCREEN_COEFF = 0.008;
export const FAN_WORLD_MIN = 0.01; // ~0.6° — still separates two discs when zoomed in
export const FAN_WORLD_MAX = 0.03; // ~1.7° — stays visually attached to the city when zoomed out

/**
 * World-space fan magnitude for a given camera distance, holding the on-screen
 * gap roughly constant across the zoom range (see module header).
 */
export function worldFanForDistance(cameraDistance: number): number {
  const raw = FAN_SCREEN_COEFF * cameraDistance;
  return Math.min(FAN_WORLD_MAX, Math.max(FAN_WORLD_MIN, raw));
}

/**
 * Build the spiderfy layout for a set of geo positions (all on the same sphere
 * radius). Single-linkage clustering by angular proximity; O(n²), fine for the
 * marker counts the globe renders.
 *
 * Cluster members get evenly-spaced fan directions around the cluster
 * centroid's tangent plane. A point with no near neighbour gets a singleton
 * slot (`clusterId = -1`, zero direction) and is never moved.
 */
export function computeSpiderLayout(
  positions: readonly Vector3[],
  thresholdRad: number = DEFAULT_CLUSTER_THRESHOLD_RAD
): SpiderSlot[] {
  const n = positions.length;
  const slots: SpiderSlot[] = new Array(n);
  if (n === 0) return slots;

  const cosThreshold = Math.cos(thresholdRad);
  const dirs = positions.map((p) => p.clone().normalize());

  // Single-linkage clustering: union points whose normalised dot exceeds the
  // angular threshold's cosine.
  const parent = Array.from({ length: n }, (_, i) => i);
  const find = (a: number): number => {
    let r = a;
    while (parent[r] !== r) r = parent[r]!;
    while (parent[a] !== r) {
      const next = parent[a]!;
      parent[a] = r;
      a = next;
    }
    return r;
  };
  for (let i = 0; i < n; i++) {
    for (let j = i + 1; j < n; j++) {
      if (dirs[i]!.dot(dirs[j]!) >= cosThreshold) {
        parent[find(i)] = find(j);
      }
    }
  }

  // Group member indices by cluster root.
  const groups = new Map<number, number[]>();
  for (let i = 0; i < n; i++) {
    const root = find(i);
    const g = groups.get(root);
    if (g) g.push(i);
    else groups.set(root, [i]);
  }

  let clusterId = 0;
  for (const members of groups.values()) {
    if (members.length < 2) {
      const i = members[0]!;
      slots[i] = { clusterId: -1, offsetDir: new Vector3(0, 0, 0) };
      continue;
    }
    // Cluster centroid direction + a tangent basis (u, v) perpendicular to it.
    const centroid = new Vector3();
    for (const i of members) centroid.add(dirs[i]!);
    centroid.normalize();
    const { u, v } = tangentBasis(centroid);

    const k = members.length;
    members.forEach((i, idx) => {
      const angle = (2 * Math.PI * idx) / k;
      const offsetDir = u
        .clone()
        .multiplyScalar(Math.cos(angle))
        .addScaledVector(v, Math.sin(angle));
      slots[i] = { clusterId, offsetDir };
    });
    clusterId++;
  }

  return slots;
}

/**
 * Displace a clustered point by `worldMag` along its fan direction and
 * re-project onto the sphere of `radius`. Singletons (zero direction) and a
 * non-positive magnitude return the original position unchanged.
 */
export function applySpiderSpread(
  geo: Vector3,
  slot: SpiderSlot,
  worldMag: number,
  radius: number
): Vector3 {
  if (slot.clusterId < 0 || worldMag <= 0) return geo.clone();
  return geo.clone().addScaledVector(slot.offsetDir, worldMag).normalize().multiplyScalar(radius);
}

/** Orthonormal tangent basis (u, v) at a unit sphere direction `n`. */
function tangentBasis(n: Vector3): { u: Vector3; v: Vector3 } {
  // Pick a reference axis not parallel to n. North pole (Y) unless n ~ ±Y.
  const ref = Math.abs(n.y) > 0.99 ? new Vector3(1, 0, 0) : new Vector3(0, 1, 0);
  const u = new Vector3().crossVectors(ref, n).normalize();
  const v = new Vector3().crossVectors(n, u).normalize();
  return { u, v };
}
