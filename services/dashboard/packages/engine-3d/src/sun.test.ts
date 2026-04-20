import { describe, expect, it } from 'vitest';
import { Vector3 } from 'three';
import { sunDirection } from './sun';

// Sub-solar latitude is the declination of the sun. Declination should be ≈ 0
// at equinox, ≈ +23.44° at June solstice, ≈ −23.44° at December solstice.
// The implementation is accurate to ~0.5° (NOAA approximation).
const TOLERANCE_DEG = 0.7;

function declinationDeg(v: Vector3): number {
  return Math.asin(v.y) * (180 / Math.PI);
}

function subSolarLonDeg(v: Vector3): number {
  return Math.atan2(v.x, v.z) * (180 / Math.PI);
}

describe('sunDirection', () => {
  it('returns a unit vector', () => {
    const v = sunDirection(Date.UTC(2026, 0, 1, 12, 0, 0));
    expect(v.length()).toBeCloseTo(1, 5);
  });

  it('places declination near 0 at March equinox', () => {
    // 2026-03-20 ~ vernal equinox
    const v = sunDirection(Date.UTC(2026, 2, 20, 12, 0, 0));
    expect(Math.abs(declinationDeg(v))).toBeLessThan(TOLERANCE_DEG);
  });

  it('places declination near +23.44° at June solstice', () => {
    const v = sunDirection(Date.UTC(2026, 5, 21, 12, 0, 0));
    expect(declinationDeg(v)).toBeGreaterThan(23.44 - TOLERANCE_DEG);
    expect(declinationDeg(v)).toBeLessThan(23.44 + TOLERANCE_DEG);
  });

  it('places declination near −23.44° at December solstice', () => {
    const v = sunDirection(Date.UTC(2026, 11, 21, 12, 0, 0));
    expect(declinationDeg(v)).toBeLessThan(-23.44 + TOLERANCE_DEG);
    expect(declinationDeg(v)).toBeGreaterThan(-23.44 - TOLERANCE_DEG);
  });

  it('rotates the sub-solar longitude westward by ~15°/hour', () => {
    const t0 = Date.UTC(2026, 5, 21, 12, 0, 0);
    const lon0 = subSolarLonDeg(sunDirection(t0));
    const lon1 = subSolarLonDeg(sunDirection(t0 + 3_600_000));
    let delta = lon1 - lon0;
    if (delta > 180) delta -= 360;
    if (delta < -180) delta += 360;
    expect(delta).toBeLessThan(-14);
    expect(delta).toBeGreaterThan(-16);
  });

  it('writes into the provided out vector', () => {
    const out = new Vector3();
    const result = sunDirection(Date.UTC(2026, 0, 1, 0, 0, 0), out);
    expect(result).toBe(out);
  });
});
