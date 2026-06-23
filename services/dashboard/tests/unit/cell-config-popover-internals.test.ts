import { describe, expect, it } from 'vitest';

import {
  clampBins,
  clampTopN,
  clampForce,
  axisOverrideForcesFree,
  buildAxisMetricOptions,
  buildDimensionOptions
} from '../../src/lib/workbench/cell-config-popover-internals';
import type { CellChannelBinding } from '../../src/lib/state/url-internals';

describe('clampBins', () => {
  it('rounds and clamps into the 1–200 range', () => {
    expect(clampBins(42.4)).toBe(42);
    expect(clampBins(42.6)).toBe(43);
    expect(clampBins(0)).toBe(1);
    expect(clampBins(-10)).toBe(1);
    expect(clampBins(9999)).toBe(200);
  });

  it('returns null for a non-finite input (override left untouched)', () => {
    expect(clampBins(Number.NaN)).toBeNull();
    expect(clampBins(Number.POSITIVE_INFINITY)).toBeNull();
  });
});

describe('clampTopN', () => {
  it('rounds and clamps into the 1–6000 range (matches the edge cap MaxCoOccurrenceTopN)', () => {
    expect(clampTopN(250.5)).toBe(251);
    expect(clampTopN(0)).toBe(1);
    expect(clampTopN(10_000)).toBe(6000);
  });

  it('clamps to the view-dependent ceiling when a max is passed', () => {
    // Metadata-field views cap at 200, all-others at 500 (computeTopNMax).
    expect(clampTopN(10_000, 200)).toBe(200);
    expect(clampTopN(10_000, 500)).toBe(500);
    expect(clampTopN(150, 200)).toBe(150);
  });

  it('returns null for a non-finite input', () => {
    expect(clampTopN(Number.NaN)).toBeNull();
  });
});

describe('clampForce', () => {
  it('rounds and clamps into the 0–100 range (0 allowed)', () => {
    expect(clampForce(0)).toBe(0);
    expect(clampForce(-5)).toBe(0);
    expect(clampForce(55.5)).toBe(56);
    expect(clampForce(200)).toBe(100);
  });

  it('returns null for a non-finite input', () => {
    expect(clampForce(Number.NaN)).toBeNull();
  });
});

describe('axisOverrideForcesFree', () => {
  it('is true when an X or Y channel is overridden', () => {
    expect(axisOverrideForcesFree({ channels: { x: 'word_count' } })).toBe(true);
    expect(axisOverrideForcesFree({ channels: { y: 'word_count' } })).toBe(true);
  });

  it('is false when no axis channel is overridden', () => {
    expect(axisOverrideForcesFree(undefined)).toBe(false);
    expect(axisOverrideForcesFree({})).toBe(false);
    expect(axisOverrideForcesFree({ channels: {} })).toBe(false);
    // A non-axis channel (size/colour) does not force free.
    expect(axisOverrideForcesFree({ channels: { size: 'word_count' } })).toBe(false);
  });
});

describe('buildAxisMetricOptions', () => {
  it('de-duplicates the available metrics, preserving first-seen order', () => {
    expect(buildAxisMetricOptions(['a', 'b', 'a', 'c'], {})).toEqual(['a', 'b', 'c']);
  });

  it('appends bound channel metrics that have slipped out of the available set', () => {
    const channels: CellChannelBinding = { x: 'x_metric', y: 'b', size: 'size_metric' };
    expect(buildAxisMetricOptions(['a', 'b'], channels)).toEqual([
      'a',
      'b',
      'x_metric',
      'size_metric'
    ]);
  });

  it('does not duplicate a bound metric already in the available set', () => {
    expect(buildAxisMetricOptions(['a', 'b'], { x: 'a', y: 'b' })).toEqual(['a', 'b']);
  });

  it('ignores unbound (absent) channels and appends only the bound one', () => {
    // Only `size` is bound; x / y / color are absent and contribute nothing.
    expect(buildAxisMetricOptions(['a'], { size: 'size_only' })).toEqual(['a', 'size_only']);
  });
});

describe('buildDimensionOptions', () => {
  it('returns a copy of the source-valid dimensions when there is no override', () => {
    const src = ['sentiment', 'word_count'];
    const out = buildDimensionOptions(src, undefined);
    expect(out).toEqual(src);
    expect(out).not.toBe(src);
  });

  it('keeps an active override visible even when it has slipped out of the set', () => {
    expect(buildDimensionOptions(['sentiment'], 'stale_metric')).toEqual([
      'sentiment',
      'stale_metric'
    ]);
  });

  it('does not duplicate an override already present in the set', () => {
    expect(buildDimensionOptions(['sentiment', 'word_count'], 'word_count')).toEqual([
      'sentiment',
      'word_count'
    ]);
  });
});
