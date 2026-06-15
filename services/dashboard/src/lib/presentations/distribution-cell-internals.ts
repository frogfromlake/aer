// Pure histogram-row + axis-domain helpers for DistributionCell — extracted
// from DistributionCell.svelte (Phase 141) so the bin/overflow/degenerate math
// is unit-testable and the component carries only its reactive shell.

import { fmtValue } from './cell-readout';

export interface DistributionBin {
  lower: number;
  upper: number;
  count: number;
}

export interface DistributionData {
  bins: DistributionBin[];
  clampedUpper: number | null;
  overflowCount: number;
  summary: { max: number };
}

export interface PlotRow {
  center: number;
  width: number;
  rw: number;
  lower: number;
  upper: number;
  count: number;
  overflow: boolean;
}

// Histogram rows = in-range bins + one explicit overflow bar (Phase 133 B) when
// the domain was clamped, values fell beyond it, AND the axis is per-cell (free).
// `rw` is the half-width used for RENDERING: a degenerate bin (lower == upper, the
// BFF's single-bin response for a constant metric) gets a nominal render width
// (1 for an integer metric, 0.5 otherwise) so the bar is finite and centred;
// `lower`/`upper` stay exact for the readout.
export function buildPlotRows(
  d: DistributionData | null,
  integerValued: boolean,
  sharedXActive: boolean
): PlotRow[] {
  if (!d) return [];
  const nominal = integerValued ? 1 : 0.5;
  const rows: PlotRow[] = d.bins.map((b) => {
    const width = b.upper - b.lower;
    return {
      center: (b.lower + b.upper) / 2,
      width,
      rw: (width || nominal) / 2,
      lower: b.lower,
      upper: b.upper,
      count: b.count,
      overflow: false
    };
  });
  const cu = d.clampedUpper;
  const last = rows[rows.length - 1];
  if (d.overflowCount > 0 && cu != null && last && !sharedXActive) {
    const w = last.width || 1;
    rows.push({
      center: cu + w / 2,
      width: w,
      rw: w / 2,
      lower: cu,
      upper: d.summary.max,
      count: d.overflowCount,
      overflow: true
    });
  }
  return rows;
}

// A degenerate distribution: every bin is zero-width, i.e. every in-scope article
// shares one value (a constant metric like a paywall flag that is always 0).
export function isDegenerate(rows: PlotRow[]): boolean {
  return rows.length > 0 && rows.every((r) => r.width === 0);
}

// Explicit x-domain for the degenerate case so the axis reads the real value
// rather than an auto-domain "0"; null lets the plot auto-domain otherwise.
export function computePlotDomain(rows: PlotRow[], degenerate: boolean): [number, number] | null {
  if (!degenerate) return null;
  const los = rows.map((r) => r.lower);
  const his = rows.map((r) => r.upper);
  return [Math.min(...los) - 1, Math.max(...his) + 1];
}

// Bin-range readout label. For integer metrics the fractional binning boundary
// is rounded to the integer it brackets; a bin that collapses to a single integer
// (sub-unit bin width) shows that one value.
export function fmtBinRange(lower: number, upper: number, integerValued: boolean): string {
  if (integerValued) {
    const lo = Math.round(lower);
    const hi = Math.round(upper);
    return lo === hi ? String(lo) : `${lo} – ${hi}`;
  }
  return `${fmtValue(lower)} – ${fmtValue(upper)}`;
}
