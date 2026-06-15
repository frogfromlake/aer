import { describe, it, expect } from 'vitest';
import {
  buildPlotRows,
  isDegenerate,
  computePlotDomain,
  fmtBinRange,
  type DistributionData
} from '../../src/lib/presentations/distribution-cell-internals';

const dist = (over: Partial<DistributionData> = {}): DistributionData => ({
  bins: [
    { lower: 0, upper: 10, count: 3 },
    { lower: 10, upper: 20, count: 5 }
  ],
  clampedUpper: null,
  overflowCount: 0,
  summary: { max: 20 },
  ...over
});

describe('buildPlotRows', () => {
  it('returns [] for null distribution', () => {
    expect(buildPlotRows(null, false, false)).toEqual([]);
  });

  it('maps bins to centred rows with real widths', () => {
    const rows = buildPlotRows(dist(), false, false);
    expect(rows).toHaveLength(2);
    expect(rows[0]).toMatchObject({
      center: 5,
      width: 10,
      rw: 5,
      lower: 0,
      upper: 10,
      count: 3,
      overflow: false
    });
  });

  it('gives a degenerate (zero-width) bin a nominal render width (0.5 / 1 for integers)', () => {
    const d = dist({ bins: [{ lower: 7, upper: 7, count: 9 }], summary: { max: 7 } });
    expect(buildPlotRows(d, false, false)[0]!.rw).toBe(0.25); // (0 || 0.5) / 2
    expect(buildPlotRows(d, true, false)[0]!.rw).toBe(0.5); // (0 || 1) / 2
  });

  it('appends an overflow bar when clamped + overflow on a free axis', () => {
    const d = dist({ clampedUpper: 20, overflowCount: 4, summary: { max: 99 } });
    const rows = buildPlotRows(d, false, false);
    expect(rows).toHaveLength(3);
    expect(rows[2]).toMatchObject({ lower: 20, upper: 99, count: 4, overflow: true });
  });

  it('suppresses the overflow bar under a shared x-axis (disclosed in caption instead)', () => {
    const d = dist({ clampedUpper: 20, overflowCount: 4, summary: { max: 99 } });
    expect(buildPlotRows(d, false, true)).toHaveLength(2);
  });
});

describe('isDegenerate / computePlotDomain', () => {
  it('isDegenerate is true only when every bin is zero-width', () => {
    expect(
      isDegenerate(buildPlotRows(dist({ bins: [{ lower: 3, upper: 3, count: 1 }] }), false, false))
    ).toBe(true);
    expect(isDegenerate(buildPlotRows(dist(), false, false))).toBe(false);
    expect(isDegenerate([])).toBe(false);
  });

  it('computePlotDomain brackets the constant value when degenerate, else null', () => {
    const rows = buildPlotRows(dist({ bins: [{ lower: 3, upper: 3, count: 1 }] }), false, false);
    expect(computePlotDomain(rows, true)).toEqual([2, 4]);
    expect(computePlotDomain(rows, false)).toBeNull();
  });
});

describe('fmtBinRange', () => {
  it('rounds to bracketing integers for integer metrics, collapsing single-value bins', () => {
    expect(fmtBinRange(2.1, 5.9, true)).toBe('2 – 6');
    expect(fmtBinRange(7.2, 7.4, true)).toBe('7');
  });
});
