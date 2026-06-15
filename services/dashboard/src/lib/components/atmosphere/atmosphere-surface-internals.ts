// Pure data transforms for AtmosphereSurface — extracted from
// AtmosphereSurface.svelte (Phase 141) so the probe→marker mapping, URL→time-
// window math, metric→activity aggregation, and fly-to resolution are
// unit-testable; the component keeps its reactive shell + interaction handlers.

import type { ProbeActivity, ProbeMarker } from '@aer/engine-3d';
import type { MetricsResponseDto, ProbeDto } from '$lib/api/queries';

// Probe → engine model. Each emission point carries the canonical source name
// aligned positionally with `probe.sources[i]`; when sources and emissionPoints
// have unequal lengths the trailing entries get no sourceName (no satellite).
// The marker `label` is the human-friendly short name, never the machine id;
// `id` stays the probeId (the selection key).
export function buildProbeMarkers(probeDtos: ProbeDto[]): ProbeMarker[] {
  return probeDtos.map((p) => ({
    id: p.probeId,
    language: p.language,
    label: p.shortName,
    emissionPoints: p.emissionPoints.map((ep, i) => {
      const source = p.sources[i];
      return source !== undefined
        ? {
            latitude: ep.latitude,
            longitude: ep.longitude,
            label: ep.label,
            sourceName: source
          }
        : { latitude: ep.latitude, longitude: ep.longitude, label: ep.label };
    })
  }));
}

export interface TimeWindow {
  start: string;
  end: string;
  hours: number;
}

// URL-backed [from, to) window. Missing/invalid bounds fall back to the default
// lookback (from) and `now` (to); the span is floored to at least one hour so
// the per-hour activity rate never divides by zero. `now` + `lookbackMs` are
// passed in (the caller reads `Date.now()` + the DEFAULT_LOOKBACK_MS SoT) so
// this stays pure/testable with no module-level coupling.
export function computeWindow(
  from: string | null,
  to: string | null,
  now: number,
  lookbackMs: number
): TimeWindow {
  const fromMs = from ? Date.parse(from) : now - lookbackMs;
  const toMs = to ? Date.parse(to) : now;
  const safeFrom = Number.isFinite(fromMs) ? fromMs : now - lookbackMs;
  const safeTo = Number.isFinite(toMs) ? toMs : now;
  return {
    start: new Date(safeFrom).toISOString(),
    end: new Date(safeTo).toISOString(),
    hours: Math.max(1, (safeTo - safeFrom) / (60 * 60 * 1000))
  };
}

// Metric rows → per-probe documents/hour. Sums each source's counts, then for
// each probe sums its sources and divides by the window's hour span.
export function computeActivity(
  rows: MetricsResponseDto['data'],
  probeDtos: ProbeDto[],
  hours: number
): ProbeActivity[] {
  const perSource: Record<string, number> = {};
  for (const row of rows) {
    perSource[row.source] = (perSource[row.source] ?? 0) + (row.count ?? 0);
  }
  return probeDtos.map((p) => {
    const total = p.sources.reduce((sum, s) => sum + (perSource[s] ?? 0), 0);
    return { probeId: p.probeId, documentsPerHour: total / hours };
  });
}

// Fly-to target for the active probe = its first emission point's coordinates.
export function resolveFlyTo(
  probeDtos: ProbeDto[],
  activeProbeId: string | null
): { latitude: number; longitude: number } | null {
  if (!activeProbeId) return null;
  const p = probeDtos.find((d) => d.probeId === activeProbeId);
  const ep = p?.emissionPoints[0];
  return ep ? { latitude: ep.latitude, longitude: ep.longitude } : null;
}

export interface FlatProbe {
  probeId: string;
  displayName: string;
  language: string;
}

// Flat probe list for the keyboard descent grammar.
export function buildFlatProbes(probeDtos: ProbeDto[]): FlatProbe[] {
  return probeDtos.map((p) => ({
    probeId: p.probeId,
    displayName: p.displayName,
    language: p.language
  }));
}
