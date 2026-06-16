// Pure helpers for CellConfigPopover (Phase 141 decomposition). The popover and
// its per-type lever children (CellConfigValueLevers / CellConfigChannelLevers)
// own only the Svelte markup + the rune-bound override writes; the value-clamping
// and option-list building live here so they are unit-testable in isolation
// (vitest is node-only, so component logic worth pinning is lifted to a `.ts`).
import type { CellChannelBinding } from '$lib/state/url-internals';

// Slider clamps — return the rounded, range-clamped value, or null for a
// non-finite input (the caller then leaves the override untouched). The ranges
// match CellConfigPopover's historical clamps (wider than the slider min/max so
// a programmatic value is still accepted): bins 1–200, topN 1–500, force 0–100.
export function clampBins(raw: number): number | null {
  if (!Number.isFinite(raw)) return null;
  return Math.min(200, Math.max(1, Math.round(raw)));
}

export function clampTopN(raw: number): number | null {
  if (!Number.isFinite(raw)) return null;
  return Math.min(500, Math.max(1, Math.round(raw)));
}

export function clampForce(raw: number): number | null {
  if (!Number.isFinite(raw)) return null;
  return Math.min(100, Math.max(0, Math.round(raw)));
}

// A per-cell X/Y axis override measures different metrics than the sibling cells,
// so the cell can never share their axis — PanelHost forces it to 'free'. The
// Scale lever reflects that (shows 'free', locked) instead of claiming 'shared'.
export function axisOverrideForcesFree(
  override: { channels?: CellChannelBinding } | undefined
): boolean {
  return override?.channels?.x !== undefined || override?.channels?.y !== undefined;
}

// Scatter axis picker options: the scope-available scalar metrics, de-duplicated
// and order-preserving, plus any currently-bound channel metric that has slipped
// out of the available set (kept so the select still reflects the live binding).
export function buildAxisMetricOptions(
  scalarMetricOptions: readonly string[],
  channels: CellChannelBinding
): string[] {
  const seen: Record<string, true> = {};
  const out: string[] = [];
  for (const m of scalarMetricOptions) {
    if (!seen[m]) {
      seen[m] = true;
      out.push(m);
    }
  }
  for (const bound of [channels.x, channels.y, channels.size, channels.color]) {
    if (bound && !seen[bound]) {
      seen[bound] = true;
      out.push(bound);
    }
  }
  return out;
}

// Dimension-peek options (ADR-038): the dimensions valid for this cell's own
// source, plus the active override even if it has slipped out of that set — so
// the select always reflects the dimension the cell actually renders.
export function buildDimensionOptions(
  cellDimensionOptions: readonly string[],
  overrideMetric: string | undefined
): string[] {
  const out = [...cellDimensionOptions];
  if (overrideMetric && !out.includes(overrideMetric)) out.push(overrideMetric);
  return out;
}
