import { describe, expect, it } from 'vitest';
import { Vector3 } from 'three';

import {
  FAN_WORLD_MAX,
  FAN_WORLD_MIN,
  applySpiderSpread,
  computeSpiderLayout,
  worldFanForDistance
} from './spiderfy';

const RADIUS = 1.0035;

// Local copy of the engine's (module-private) lat/lon→Cartesian mapping, so the
// pure-layout tests can build realistic globe positions without importing
// engine internals.
function latLonToCartesian(latDeg: number, lonDeg: number, radius: number): Vector3 {
  const lat = (latDeg * Math.PI) / 180;
  const lon = (lonDeg * Math.PI) / 180;
  const c = Math.cos(lat);
  return new Vector3(c * Math.cos(lon), Math.sin(lat), c * Math.sin(lon)).multiplyScalar(radius);
}

describe('computeSpiderLayout', () => {
  it('leaves a well-separated pair unclustered (Probe 0: Hamburg ↔ Berlin)', () => {
    // ~250 km apart → ~2.25°, far above the 0.5° threshold.
    const hamburg = latLonToCartesian(53.5511, 9.9937, RADIUS);
    const berlin = latLonToCartesian(52.517, 13.3888, RADIUS);
    const slots = computeSpiderLayout([hamburg, berlin]);
    expect(slots[0]!.clusterId).toBe(-1);
    expect(slots[1]!.clusterId).toBe(-1);
  });

  it('clusters a co-located pair (Probe 1: franceinfo ↔ Élysée, both Paris)', () => {
    // ~3.5 km apart → ~0.03°, well below the threshold.
    const franceinfo = latLonToCartesian(48.8389, 2.2766, RADIUS);
    const elysee = latLonToCartesian(48.8704, 2.3169, RADIUS);
    const slots = computeSpiderLayout([franceinfo, elysee]);
    expect(slots[0]!.clusterId).toBe(0);
    expect(slots[1]!.clusterId).toBe(0);
    // Fan directions are opposite (2 members → π apart).
    expect(slots[0]!.offsetDir.dot(slots[1]!.offsetDir)).toBeLessThan(-0.9);
  });

  it('offset directions are tangent at the cluster centroid (and unit length)', () => {
    const a = latLonToCartesian(48.8389, 2.2766, RADIUS);
    const b = latLonToCartesian(48.8704, 2.3169, RADIUS);
    const slots = computeSpiderLayout([a, b]);
    const centroid = a.clone().normalize().add(b.clone().normalize()).normalize();
    expect(Math.abs(centroid.dot(slots[0]!.offsetDir))).toBeLessThan(1e-6);
    expect(slots[0]!.offsetDir.length()).toBeCloseTo(1, 6);
    // Members sit ~0.02° from the centroid, so the offset is still very nearly
    // tangent to each member's own radial.
    expect(Math.abs(a.clone().normalize().dot(slots[0]!.offsetDir))).toBeLessThan(1e-3);
  });

  it('returns an empty array for no points', () => {
    expect(computeSpiderLayout([])).toEqual([]);
  });
});

describe('applySpiderSpread', () => {
  it('returns the original position at zero magnitude', () => {
    const a = latLonToCartesian(48.8389, 2.2766, RADIUS);
    const b = latLonToCartesian(48.8704, 2.3169, RADIUS);
    const slot = computeSpiderLayout([a, b])[0]!;
    const out = applySpiderSpread(a, slot, 0, RADIUS);
    expect(out.distanceTo(a)).toBeLessThan(1e-9);
  });

  it('displaces along the tangent and stays on the sphere', () => {
    const a = latLonToCartesian(48.8389, 2.2766, RADIUS);
    const b = latLonToCartesian(48.8704, 2.3169, RADIUS);
    const slot = computeSpiderLayout([a, b])[0]!;
    const out = applySpiderSpread(a, slot, FAN_WORLD_MAX, RADIUS);
    expect(out.distanceTo(a)).toBeGreaterThan(0.001); // actually moved
    expect(out.length()).toBeCloseTo(RADIUS, 6); // re-projected onto the sphere
  });

  it('never moves a singleton', () => {
    const geo = latLonToCartesian(0, 0, RADIUS);
    const slot = { clusterId: -1, offsetDir: new Vector3(0, 0, 0) };
    expect(applySpiderSpread(geo, slot, FAN_WORLD_MAX, RADIUS).distanceTo(geo)).toBe(0);
  });
});

describe('worldFanForDistance', () => {
  it('grows with camera distance (screen-space-constant separation)', () => {
    const near = worldFanForDistance(1.3);
    const far = worldFanForDistance(3.0);
    expect(far).toBeGreaterThan(near);
  });

  it('clamps to the configured world bounds', () => {
    expect(worldFanForDistance(0.1)).toBe(FAN_WORLD_MIN); // tiny distance → min
    expect(worldFanForDistance(100)).toBe(FAN_WORLD_MAX); // huge distance → max
  });
});
