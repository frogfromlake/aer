// Pure layout/comparison helpers for PanelHost — extracted from PanelHost.svelte
// (Phase 141) so the shared-axis comparison discipline (Phase 124/126) and the
// dimension-availability filtering (ADR-038) are unit-testable; the component
// keeps its reactive shell, queries, $state extents, and markup.
//
// Every function takes its reactive inputs as explicit params (no closure over
// component state) so behaviour is identical to the inlined originals.

import { DEFAULT_METRIC_NAME } from '../presentations';
import { isPureCountMetric } from '../presentations/metric-presentation';
import type { CellRenderUnit } from './panel-queries';
import type { CellChannelBinding, Panel } from '$lib/state/url-internals';
import type { ScopeAvailableMetricsDto, ScopeAvailableMetadataDto } from '$lib/api/queries';

// ── Dimension availability (ADR-038) ─────────────────────────────────────────

export interface DimensionAvail {
  available: string[];
  partialSources: string[] | null;
  scopedSources: string[];
}

// The active dimension's availability split from whichever endpoint matches the
// view (`field` vs `metricName` partial key). `data` is the success payload or
// null (query pending / refused / errored).
export function extractDimensionAvail(
  data: ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null,
  fieldDriven: boolean,
  metric: string
): DimensionAvail | null {
  if (!data) return null;
  const partial = fieldDriven
    ? (data as ScopeAvailableMetadataDto).partial.find((p) => p.field === metric)
    : (data as ScopeAvailableMetricsDto).partial.find((p) => p.metricName === metric);
  return {
    available: data.available,
    partialSources: partial ? partial.sources : null,
    scopedSources: data.scopedSources
  };
}

// `null` = no narrowing (dimension on every source, or data not loaded). A Set =
// render only these sources for the active dimension.
export function dimensionSourceFilter(
  avail: DimensionAvail | null,
  metric: string
): Set<string> | null {
  if (!avail) return null;
  if (avail.available.includes(metric)) return null;
  return avail.partialSources ? new Set(avail.partialSources) : null;
}

// Scoped sources dropped because they lack the active dimension (named in the
// panel note so absence is disclosed, never silent).
export function droppedSources(avail: DimensionAvail | null): string[] {
  if (!avail || !avail.partialSources) return [];
  const have = new Set(avail.partialSources);
  return avail.scopedSources.filter((s) => !have.has(s));
}

// Per-cell dimension peek list: every intersection dimension plus any partial at
// least one of the open cell's own sources carries (same KIND as the view).
// Phase 148f — degenerate (constant, no-signal) dimensions are excluded, mirroring
// the panel-level metric/field picker (`panel-controls-derive`): a constant field
// like `image_count` carries no signal, so it must not be offerable per-cell.
export function cellDimensionOptions(
  data: ScopeAvailableMetricsDto | ScopeAvailableMetadataDto | null,
  openCellSources: string[],
  fieldDriven: boolean
): string[] {
  if (!data || openCellSources.length === 0) return [];
  const srcSet = new Set(openCellSources);
  const names = [...data.available];
  const exclude = new Set<string>();
  if (fieldDriven) {
    for (const p of (data as ScopeAvailableMetadataDto).partial)
      if (p.sources.some((s) => srcSet.has(s))) names.push(p.field);
    for (const d of (data as ScopeAvailableMetadataDto).degenerate ?? []) exclude.add(d.field);
  } else {
    for (const p of (data as ScopeAvailableMetricsDto).partial)
      if (p.sources.some((s) => srcSet.has(s))) names.push(p.metricName);
    for (const d of (data as ScopeAvailableMetricsDto).degenerate ?? []) exclude.add(d.metricName);
  }
  return [...new Set(names)].filter((n) => !exclude.has(n)).sort();
}

// Scalar metric options for the per-cell scatter-axis pickers: the all-source
// intersection (`available`), default-prepended so the picker is never empty.
export function scalarMetricOptionsFromAvailable(available: readonly string[]): string[] {
  const seen: Record<string, true> = {};
  const out: string[] = [];
  const push = (n: string | undefined) => {
    if (n && !seen[n]) {
      seen[n] = true;
      out.push(n);
    }
  };
  push(DEFAULT_METRIC_NAME);
  for (const m of available) push(m);
  return out;
}

// ── Shared-axis comparison discipline (Phase 124 / 126) ──────────────────────

// Distinct in-scope probe ids across the rendered units (drives the cross-
// context guard + the per-probe accent).
export function renderedProbeCount(units: readonly CellRenderUnit[]): number {
  const seen: Record<string, true> = {};
  for (const u of units) {
    if (u.probeId) seen[u.probeId] = true;
    for (const pid of u.probeIds) seen[pid] = true;
  }
  return Object.keys(seen).length;
}

// Intensiveness for the cross-context guard. metric_scatter (usesMetric:false)
// binds its axes to channel metrics (defaulting exactly as ScatterCell does), so
// the guard inspects those; everything else carries its axis metric in
// `panel.metric`.
export function isIntensiveMetric(i: {
  view: string;
  channels: CellChannelBinding | undefined;
  metric: string;
  presentationUsesMetric: boolean | undefined;
}): boolean {
  if (i.view === 'metric_scatter') {
    const x = i.channels?.x ?? 'word_count';
    const y = i.channels?.y ?? DEFAULT_METRIC_NAME;
    return !isPureCountMetric(x) || !isPureCountMetric(y);
  }
  return i.presentationUsesMetric !== false && !isPureCountMetric(i.metric);
}

// A cell's EFFECTIVE axis-scale, honouring per-cell overrides: the cross-context
// guard forces 'free'; a per-cell x/y channel override changes WHAT the axis
// measures → 'free'; otherwise the per-cell `scales` override wins over the
// panel default ('shared').
export function effectiveCellScale(i: {
  cellKey: string;
  shareForbidden: boolean;
  cellOverrides: Panel['cellOverrides'];
  panelScales: Panel['scales'];
}): 'shared' | 'free' {
  if (i.shareForbidden) return 'free';
  const ov = i.cellOverrides?.[i.cellKey];
  if (ov?.channels?.x !== undefined || ov?.channels?.y !== undefined) return 'free';
  // An explicit per-cell scale override wins (the user can turn sharing back on).
  if (ov?.scales !== undefined) return ov.scales;
  // Phase 148f — a per-cell METRIC override measures a different metric than the
  // panel, so its values are not comparable on the shared axis: default to free
  // (overridable above). `ov.metric` is only present when it differs (the mutator
  // clears it when equal to the panel metric).
  if (ov?.metric !== undefined) return 'free';
  return i.panelScales ?? 'shared';
}

// The cells that actually share the axis — only these feed AND read the union, so
// a freed cell neither stretches its siblings' axis nor is distorted by theirs.
export function computeSharedCellKeys(i: {
  sharedAxisApplies: boolean;
  shareForbidden: boolean;
  units: readonly CellRenderUnit[];
  cellOverrides: Panel['cellOverrides'];
  panelScales: Panel['scales'];
}): Record<string, true> {
  const out: Record<string, true> = {};
  if (!i.sharedAxisApplies || i.shareForbidden) return out;
  for (const u of i.units) {
    if (
      effectiveCellScale({
        cellKey: u.key,
        shareForbidden: i.shareForbidden,
        cellOverrides: i.cellOverrides,
        panelScales: i.panelScales
      }) === 'shared'
    )
      out[u.key] = true;
  }
  return out;
}

// Union of the reported extents over the shared cells only, for one axis. Keys
// are `${cellKey}|${axis}`; a freed cell still reports its extent (so it stays
// fresh if re-shared) but is excluded from the union.
export function unionExtent(
  reportedExtents: Record<string, readonly [number, number]>,
  sharedCellKeys: Record<string, true>,
  axis: 'value' | 'x'
): readonly [number, number] | undefined {
  let lo = Infinity;
  let hi = -Infinity;
  for (const [k, v] of Object.entries(reportedExtents)) {
    if (!k.endsWith(`|${axis}`)) continue;
    if (!sharedCellKeys[k.slice(0, k.lastIndexOf('|'))]) continue;
    if (v[0] < lo) lo = v[0];
    if (v[1] > hi) hi = v[1];
  }
  return lo <= hi ? [lo, hi] : undefined;
}

// The shared value/x domains handed back to the cells. Undefined when nothing is
// shared (so the cell auto-scales).
export function computeSharedDomains(i: {
  computeShared: boolean;
  reportedExtents: Record<string, readonly [number, number]>;
  sharedCellKeys: Record<string, true>;
}): { value?: readonly [number, number]; x?: readonly [number, number] } | undefined {
  if (!i.computeShared) return undefined;
  const value = unionExtent(i.reportedExtents, i.sharedCellKeys, 'value');
  const x = unionExtent(i.reportedExtents, i.sharedCellKeys, 'x');
  const out: { value?: readonly [number, number]; x?: readonly [number, number] } = {};
  if (value) out.value = value;
  if (x) out.x = x;
  return value || x ? out : undefined;
}
