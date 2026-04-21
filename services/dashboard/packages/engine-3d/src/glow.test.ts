import { describe, expect, it } from 'vitest';
import { Vector3 } from 'three';

import {
  CORE_BRIGHTNESS_FLOOR,
  CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR,
  MAX_PULSE_RAD_PER_S,
  PULSE_SATURATION_DOCS_PER_HOUR,
  computeCoreBrightness,
  computePulseRate,
  pickNearSideHit
} from './glow';

describe('computePulseRate', () => {
  it('returns 0 for a dormant probe', () => {
    expect(computePulseRate(0)).toBe(0);
  });

  it('returns 0 for negative or non-finite input', () => {
    expect(computePulseRate(-5)).toBe(0);
    expect(computePulseRate(Number.NaN)).toBe(0);
    expect(computePulseRate(Number.POSITIVE_INFINITY)).toBe(0);
  });

  it('saturates at MAX_PULSE_RAD_PER_S', () => {
    expect(computePulseRate(PULSE_SATURATION_DOCS_PER_HOUR)).toBeCloseTo(MAX_PULSE_RAD_PER_S, 10);
    expect(computePulseRate(PULSE_SATURATION_DOCS_PER_HOUR * 100)).toBeCloseTo(
      MAX_PULSE_RAD_PER_S,
      10
    );
  });

  it('scales linearly below saturation', () => {
    const half = computePulseRate(PULSE_SATURATION_DOCS_PER_HOUR / 2);
    expect(half).toBeCloseTo(MAX_PULSE_RAD_PER_S / 2, 10);
  });

  it('never exceeds the design-brief ceiling (≥ 4 s per cycle)', () => {
    for (const rate of [0.1, 1, 5, 10, 50, 1_000]) {
      const w = computePulseRate(rate);
      expect(w).toBeLessThanOrEqual(MAX_PULSE_RAD_PER_S + 1e-9);
    }
  });
});

describe('computeCoreBrightness', () => {
  it('returns the floor for a dormant probe so it stays visible', () => {
    expect(computeCoreBrightness(0)).toBe(CORE_BRIGHTNESS_FLOOR);
    expect(computeCoreBrightness(-3)).toBe(CORE_BRIGHTNESS_FLOOR);
    expect(computeCoreBrightness(Number.NaN)).toBe(CORE_BRIGHTNESS_FLOOR);
  });

  it('saturates at 1.0', () => {
    expect(computeCoreBrightness(CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR)).toBeCloseTo(1.0, 10);
    expect(computeCoreBrightness(CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR * 100)).toBeCloseTo(
      1.0,
      10
    );
  });

  it('interpolates linearly between floor and 1.0', () => {
    const mid = computeCoreBrightness(CORE_BRIGHTNESS_SATURATION_DOCS_PER_HOUR / 2);
    expect(mid).toBeCloseTo(CORE_BRIGHTNESS_FLOOR + (1 - CORE_BRIGHTNESS_FLOOR) / 2, 10);
  });

  it('is monotonic in docsPerHour', () => {
    let prev = computeCoreBrightness(0);
    for (const rate of [0.5, 1, 2, 5, 10, 20, 40]) {
      const b = computeCoreBrightness(rate);
      expect(b).toBeGreaterThanOrEqual(prev);
      prev = b;
    }
  });
});

describe('pickNearSideHit', () => {
  // Camera is on +Z looking at the origin — classic "globe from the front".
  const camera = new Vector3(0, 0, 3);

  it('returns -1 for no hits', () => {
    expect(pickNearSideHit([], camera)).toBe(-1);
  });

  it('accepts a point on the near hemisphere', () => {
    const p = new Vector3(0, 0, 1);
    expect(pickNearSideHit([{ index: 0, position: p }], camera)).toBe(0);
  });

  it('rejects a point on the far hemisphere', () => {
    const p = new Vector3(0, 0, -1);
    expect(pickNearSideHit([{ index: 0, position: p }], camera)).toBe(-1);
  });

  it('rejects a point exactly on the horizon (dot == 0)', () => {
    const p = new Vector3(1, 0, 0);
    expect(pickNearSideHit([{ index: 0, position: p }], camera)).toBe(-1);
  });

  it('picks the first near-side hit in ray order, skipping far-side hits', () => {
    // The raycaster returns hits in ascending distance along the ray, so
    // a far-side point can appear before a near-side point on the list.
    const far = { index: 5, position: new Vector3(0, 0, -1) };
    const near = { index: 2, position: new Vector3(0, 0, 1) };
    expect(pickNearSideHit([far, near], camera)).toBe(2);
  });

  it('handles an obliquely placed camera', () => {
    const oblique = new Vector3(3, 0, 3);
    const nearOnCam = { index: 0, position: new Vector3(1, 0, 1).normalize() };
    const farFromCam = { index: 1, position: new Vector3(-1, 0, -1).normalize() };
    expect(pickNearSideHit([farFromCam, nearOnCam], oblique)).toBe(0);
  });

  it('returns -1 when the camera is at the origin (degenerate)', () => {
    const p = new Vector3(0, 0, 1);
    expect(pickNearSideHit([{ index: 0, position: p }], new Vector3(0, 0, 0))).toBe(-1);
  });

  it('skips points at the origin (degenerate position)', () => {
    const degenerate = { index: 0, position: new Vector3(0, 0, 0) };
    const good = { index: 1, position: new Vector3(0, 0, 1) };
    expect(pickNearSideHit([degenerate, good], camera)).toBe(1);
  });
});
