// Pure derivation + reconciliation helpers for PanelControls — extracted from
// PanelControls.svelte (Phase 141) so the window math, scope-availability gate,
// metric/field list building, and the intricate view-switch reconciliation are
// unit-testable; the component keeps its reactive shell, queries, and markup.
//
// Every function takes its reactive inputs as explicit params (no closure over
// component state) so behaviour is identical to the inlined originals while the
// logic becomes verifiable in isolation.

import {
  CROSS_PROBE_DEFAULT_METRIC,
  DEFAULT_METRIC_NAME,
  metricSupportsPresentation
} from '../presentations';
import type { PresentationDefinition } from '../presentations';
import { defaultMetricForScopes } from './panel-queries';
import type { CellChannelBinding, Panel, Presentation, ScopeGroup } from '$lib/state/url-internals';

// ── Window / date math ──────────────────────────────────────────────────────

export interface WindowBoundsInput {
  panelStart: string | undefined;
  panelEnd: string | undefined;
  urlFrom: string | null;
  urlTo: string | null;
  isEpisteme: boolean;
  now: number;
  lookbackMs: number;
}

export interface WindowBounds {
  startMs: number | undefined;
  endMs: number | undefined;
  isPanelOverride: boolean;
}

// Effective panel window: the bound panel's own windowStart/windowEnd when set,
// else the global url.from/url.to. Episteme (diachronic) defaults to a disclosed
// recent window so the date inputs + availability query reflect the same window
// the cells use; Aleph/Rhizome stay unbounded (undefined ⇒ whole dataset).
export function computeWindowBounds(i: WindowBoundsInput): WindowBounds {
  const fromSrc = i.panelStart ?? i.urlFrom;
  const toSrc = i.panelEnd ?? i.urlTo;
  const fromMs = fromSrc ? Date.parse(fromSrc) : NaN;
  const toMs = toSrc ? Date.parse(toSrc) : NaN;
  return {
    startMs: Number.isFinite(fromMs) ? fromMs : i.isEpisteme ? i.now - i.lookbackMs : undefined,
    endMs: Number.isFinite(toMs) ? toMs : i.isEpisteme ? i.now : undefined,
    isPanelOverride: i.panelStart !== undefined || i.panelEnd !== undefined
  };
}

export interface DateWindow {
  startDate: string | undefined;
  endDate: string | undefined;
  isPanelOverride: boolean;
}

// Date-only form (YYYY-MM-DD) for the /metrics/available window + native date
// inputs. Undefined when that side is unbounded.
export function toDateWindow(b: WindowBounds): DateWindow {
  return {
    startDate: b.startMs !== undefined ? new Date(b.startMs).toISOString().slice(0, 10) : undefined,
    endDate: b.endMs !== undefined ? new Date(b.endMs).toISOString().slice(0, 10) : undefined,
    isPanelOverride: b.isPanelOverride
  };
}

// Full RFC 3339 form for /scope/available-metrics (date-time, both optional).
export function toWindowIso(b: WindowBounds): {
  start: string | undefined;
  end: string | undefined;
} {
  return {
    start: b.startMs !== undefined ? new Date(b.startMs).toISOString() : undefined,
    end: b.endMs !== undefined ? new Date(b.endMs).toISOString() : undefined
  };
}

// ── Scope-availability gate (ADR-038) ────────────────────────────────────────

export interface ScopeGate {
  // Metrics available across the WHOLE scope (intersection); null ⇒ no scope
  // constraint yet (fall back to the unconstrained list).
  scopeAvailableSet: Set<string> | null;
  // Metrics present for SOME but not all scoped sources.
  partialMetricSet: Set<string>;
  // "show anyway" — when on, partials are also offerable.
  showWithheld: boolean;
  // Task A: metrics that are CONSTANT across the scope (one distinct value) —
  // present but signal-free. Never offerable (a constant carries no signal even
  // under "show anyway"); disclosed separately. Optional for back-compat.
  degenerateSet?: Set<string>;
}

// A metric is offerable when there is no scope constraint yet, OR it is in the
// all-source intersection, OR it is a partial metric the user opted to show.
// A degenerate (constant) metric is NEVER offerable — its constant value is
// disclosed elsewhere instead of being silently dropped (ADR-039).
export function isScopeAvailable(name: string, gate: ScopeGate): boolean {
  if (gate.degenerateSet?.has(name)) return false;
  if (gate.scopeAvailableSet === null) return true;
  if (gate.scopeAvailableSet.has(name)) return true;
  if (gate.partialMetricSet.has(name) && gate.showWithheld) return true;
  return false;
}

// The scoped sources that LACK a given partial metric (the "cause" of the
// withholding) so the hint can name them.
export function missingSourcesFor(
  have: readonly string[],
  scopedSourceNames: readonly string[]
): string[] {
  const haveSet = new Set(have);
  return scopedSourceNames.filter((s) => !haveSet.has(s));
}

// ── Metadata fields (categorical) ─────────────────────────────────────────────

export interface MetadataFieldsInput {
  availableMetadataFields: readonly string[];
  partialMetadataFields: readonly { field: string }[];
  showWithheld: boolean;
  // Task A: fields that are CONSTANT across the scope (one distinct value) —
  // excluded from the picker (grouping by a constant is meaningless), disclosed
  // separately. Optional for back-compat.
  degenerateFields?: readonly string[];
}

// The offerable categorical fields: the INTERSECTION by default (Tier 1);
// partials only under "show anyway" (Tier 2). Degenerate (constant) fields are
// always excluded. Deduped + sorted.
export function offerableMetadataFields(i: MetadataFieldsInput): string[] {
  const degenerate = new Set(i.degenerateFields ?? []);
  const seen: Record<string, true> = {};
  const out: string[] = [];
  const add = (f: string) => {
    if (f && !seen[f] && !degenerate.has(f)) {
      seen[f] = true;
      out.push(f);
    }
  };
  for (const f of i.availableMetadataFields) add(f);
  if (i.showWithheld) {
    for (const p of i.partialMetadataFields) add(p.field);
  }
  out.sort();
  return out;
}

// The field-picker list: offerable fields + the active field surfaced so the
// picker reflects the current selection. Empty for non-field views.
export function buildMetadataFields(i: {
  viewUsesMetadataField: boolean;
  offerable: string[];
  activeMetric: string;
}): string[] {
  if (!i.viewUsesMetadataField) return [];
  const out = [...i.offerable];
  if (i.activeMetric && !out.includes(i.activeMetric)) {
    out.push(i.activeMetric);
    out.sort();
  }
  return out;
}

// Substantive editorial dimensions preferred as the default grouping field, in
// priority order, so the seed lands on a meaningful field (e.g. the editorial
// desk `ressort`) instead of an alphabetically-first but low-information one
// (operator decision 2026-06-24). Falls back to the sorted-first field.
const PREFERRED_DEFAULT_FIELDS = ['ressort'];

// The field to seed when switching into a field-driven view: a preferred
// editorial field if the scope offers one, else the first field (deterministic,
// sorted), or '' when the scope shares none. Callers pass the OFFERABLE fields
// (no-signal already excluded), so a structural field like `article_type` is
// never seeded.
export function firstMetadataField(availableMetadataFields: readonly string[]): string {
  for (const pref of PREFERRED_DEFAULT_FIELDS) {
    if (availableMetadataFields.includes(pref)) return pref;
  }
  return [...availableMetadataFields].sort()[0] ?? '';
}

// ── Metric lists ──────────────────────────────────────────────────────────────

export interface MetricListInput {
  view: Presentation;
  availableMetricNames: string[];
  gate: ScopeGate;
  activeMetric: string;
}

// Metric list: DEFAULT first, then API order, filtered through the
// metric→presentation map AND the scope gate. The active metric is always
// surfaced (even if it has since become partial) so the picker never drops it.
export function buildMetricList(i: MetricListInput): string[] {
  const seen: Record<string, true> = {};
  const merged: string[] = [];
  for (const name of [DEFAULT_METRIC_NAME, ...i.availableMetricNames]) {
    if (!name || seen[name]) continue;
    if (!metricSupportsPresentation(name, i.view)) continue;
    if (!isScopeAvailable(name, i.gate)) continue;
    seen[name] = true;
    merged.push(name);
  }
  if (
    i.activeMetric &&
    !seen[i.activeMetric] &&
    metricSupportsPresentation(i.activeMetric, i.view)
  ) {
    merged.push(i.activeMetric);
  }
  return merged;
}

// ── Degenerate / low-signal disclosure (Task A) ──────────────────────────────

// Format a constant scalar value for disclosure: integers without decimals,
// otherwise trimmed to 2 dp. Deliberately honest — it renders the raw number;
// the frontend has no per-metric boolean knowledge, so paywall_status reads as
// "0", never a fabricated "false".
export function formatConstantValue(v: number): string {
  if (Number.isInteger(v)) return String(v);
  return String(Math.round(v * 100) / 100);
}

// Integer percentage (0..100) for a 0..1 dominant share.
export function dominantSharePct(share: number): number {
  return Math.round(share * 100);
}

// The first metric the target view can render, preferring the canonical default.
export function firstMetricSupporting(view: Presentation, availableMetricNames: string[]): string {
  if (metricSupportsPresentation(DEFAULT_METRIC_NAME, view)) return DEFAULT_METRIC_NAME;
  return (
    availableMetricNames.find((m) => metricSupportsPresentation(m, view)) ?? DEFAULT_METRIC_NAME
  );
}

export interface ResetMetricInput {
  view: Presentation;
  scopeAvailableSet: Set<string> | null;
  scopes: readonly ScopeGroup[];
  availableMetricNames: string[];
}

// A scope-valid metric to snap to when the active metric is not offerable for
// the scope. Preference: scope's canonical default → multilingual backbone →
// any available sentiment → any available metric.
export function resetMetricForScope(i: ResetMetricInput): string {
  const ok = (m: string) =>
    (i.scopeAvailableSet?.has(m) ?? true) && metricSupportsPresentation(m, i.view);
  const canonical = defaultMetricForScopes(i.scopes);
  if (ok(canonical)) return canonical;
  if (ok(CROSS_PROBE_DEFAULT_METRIC)) return CROSS_PROBE_DEFAULT_METRIC;
  const firstSentiment = i.availableMetricNames.find(
    (m) => m.startsWith('sentiment_score') && ok(m)
  );
  if (firstSentiment) return firstSentiment;
  return i.availableMetricNames.find(ok) ?? canonical;
}

// Scalar-metric options for the scatter axis/size/colour pickers: every real
// /metrics/available metric (default-prepended) passing the scope gate, plus any
// currently-bound channel metric so the selects always reflect the binding.
export function buildScalarMetricOptions(i: {
  availableMetricNames: string[];
  gate: ScopeGate;
  activeChannels: CellChannelBinding;
}): string[] {
  const seen: Record<string, true> = {};
  const out: string[] = [];
  for (const name of [DEFAULT_METRIC_NAME, ...i.availableMetricNames]) {
    if (!name || seen[name]) continue;
    if (!isScopeAvailable(name, i.gate)) continue;
    seen[name] = true;
    out.push(name);
  }
  for (const bound of [
    i.activeChannels.x,
    i.activeChannels.y,
    i.activeChannels.size,
    i.activeChannels.color
  ]) {
    if (bound && !seen[bound]) {
      seen[bound] = true;
      out.push(bound);
    }
  }
  return out;
}

// The Top N upper bound is per-view: co-occurrence accepts up to 6000 edges;
// categorical-distribution clamps to 200 server-side; others cap at 500.
export function computeTopNMax(i: {
  isCooccurrenceView: boolean;
  viewUsesMetadataField: boolean;
}): number {
  return i.isCooccurrenceView ? 6000 : i.viewUsesMetadataField ? 200 : 500;
}

// ── View-switch reconciliation (the pickView body) ───────────────────────────

export interface ReconcileViewContext {
  // The pillar's presentation definitions (for the target-view lookup).
  presentations: readonly PresentationDefinition[];
  // The OUTGOING view's `usesMetadataField` flag.
  prevUsesMetadataField: boolean;
  // Scope-available scalar metrics (scatter/cross/lead-lag/metricSet seeds).
  scalarMetricOptions: string[];
  // Offerable categorical fields (Sankey field-chain seed AND the field-driven
  // default seed — already excludes no-signal degenerate/low-signal fields).
  offerableFields: string[];
  // Raw /metrics/available names (firstMetricSupporting fallback).
  availableMetricNames: string[];
  // Scope-intersection categorical fields (retained for callers; the field seed
  // now uses `offerableFields` so a no-signal field is never auto-selected).
  availableMetadataFields: readonly string[];
  // Phase 148g — the scope's all-source metric intersection (null while
  // availability loads). Used so the metric reset is SCOPE-AWARE: a view switch
  // must never default to a metric the whole scope cannot serve (e.g. the
  // German-only `sentiment_score_sentiws` on a cross-probe DE+FR panel — that
  // metric is withheld and the default must be the multilingual backbone).
  scopeAvailableSet: Set<string> | null;
  // The panel's scope groups — feeds the scope-aware default (backbone).
  scopes: readonly ScopeGroup[];
  // "show anyway" — when on, a withheld (partial) metric is a legitimate active
  // selection and must NOT be snapped away on a view switch.
  showWithheld: boolean;
}

// Compute the next Panel when switching INTO presentation `id`: discard
// presentation-specific overrides, reconcile metric↔field across the
// metric/field boundary, drop a no-op overlay composition, and seed the
// channel/metric-set/field-chain config so the new cell renders immediately.
// Pure transform of (panel, id, ctx) → next panel; mirrors the original
// pickView updatePanel callback exactly.
export function reconcilePanelForView(
  p: Panel,
  id: Presentation,
  ctx: ReconcileViewContext
): Panel {
  const next = { ...p, view: id };
  // A view change discards presentation-specific per-cell overrides.
  delete next.cellOverrides;
  const pres = ctx.presentations.find((x) => x.id === id);
  // Drop list-state the new view does not consume (so it neither lingers in the
  // URL nor blocks the seed below).
  const nextParams = pres?.configurableParams ?? [];
  if (!nextParams.includes('metricSet')) delete next.metricSet;
  if (!nextParams.includes('sankeyFields')) delete next.fieldChain;
  if (!(pres?.supportsFaceting ?? false)) delete next.facetField;
  const usesMetric = pres?.usesMetric ?? true;
  const nextUsesMetadataField = pres?.usesMetadataField ?? false;
  const prevUsesMetadataField = ctx.prevUsesMetadataField;
  // Reconcile `Panel.metric` across the field/metric boundary. Seed from the
  // OFFERABLE fields (intersection minus no-signal — degenerate/low-signal — and
  // plus partials under "show anyway"), never the raw intersection, so a
  // no-signal field (e.g. article_type) is never auto-selected. A field carried
  // over from a prior field-driven view is also snapped away if it has since
  // become no-signal in this scope.
  if (nextUsesMetadataField) {
    const keep =
      prevUsesMetadataField && !!next.metric && ctx.offerableFields.includes(next.metric);
    if (!keep) {
      next.metric = firstMetadataField(ctx.offerableFields);
    }
  } else if (usesMetric) {
    // Phase 148g — `metricSupportsPresentation` defaults an UNKNOWN name to the
    // scalar presentations, so a categorical FIELD stranded in `Panel.metric`
    // (e.g. `author` left over from a field-driven or channel-driven view)
    // would pass the support check and survive into a single-metric view as a
    // bogus scalar dimension. The clean separator is the `/metrics/available`
    // registry: a name absent from it (once loaded) is not a real metric and
    // must be snapped to a genuine one. The default metric is always legitimate
    // even before the registry loads.
    const registryLoaded = ctx.availableMetricNames.length > 0;
    const isRealMetric =
      next.metric === DEFAULT_METRIC_NAME || ctx.availableMetricNames.includes(next.metric);
    // Phase 148g — also snap when the carried-over metric is NOT available across
    // the whole scope (and the user has not opted into "show anyway"): otherwise a
    // metric that is valid on one probe but withheld on a cross-probe scope (the
    // German-only SentiWS on a DE+FR panel) would survive a view switch as the
    // active default — exactly the cross-probe-backbone guarantee this restores.
    const scopeUnavailable =
      ctx.scopeAvailableSet !== null &&
      !ctx.showWithheld &&
      !!next.metric &&
      !ctx.scopeAvailableSet.has(next.metric);
    if (
      prevUsesMetadataField ||
      !metricSupportsPresentation(next.metric, id) ||
      (registryLoaded && !isRealMetric) ||
      scopeUnavailable
    ) {
      // Scope-AWARE reset: the canonical scope default (multilingual backbone for
      // a cross-probe scope), never the scope-blind `firstMetricSupporting`
      // (which preferred the German-only default regardless of scope).
      next.metric = resetMetricForScope({
        view: id,
        scopeAvailableSet: ctx.scopeAvailableSet,
        scopes: ctx.scopes,
        availableMetricNames: ctx.availableMetricNames
      });
    }
  }
  // Reconcile a no-op overlay composition (only time-series renders overlay).
  if (next.composition === 'overlay' && !(pres?.supportsOverlay ?? false)) {
    next.composition = 'split';
  }
  // Seed scatter position channels (sentiment on X, word_count on Y).
  if (id === 'metric_scatter' && (!next.channels?.x || !next.channels?.y)) {
    const opts = ctx.scalarMetricOptions;
    const sentiment =
      opts.find((m) => m === 'sentiment_score_bert_multilingual') ??
      opts.find((m) => m.startsWith('sentiment_score'));
    const x = next.channels?.x ?? sentiment ?? opts[0] ?? DEFAULT_METRIC_NAME;
    let y = next.channels?.y;
    if (!y) {
      y =
        opts.includes('word_count') && x !== 'word_count'
          ? 'word_count'
          : (opts.find((m) => m !== x) ?? x);
    }
    next.channels = { ...(next.channels ?? {}), x, y };
  }
  // Seed the N-metric set for multivariate cells (first up-to-4 scalars).
  if (nextParams.includes('metricSet') && (next.metricSet?.length ?? 0) < 2) {
    next.metricSet = ctx.scalarMetricOptions.slice(0, 4);
  }
  // Seed the Sankey field chain (first up-to-3 offerable categorical fields).
  if (nextParams.includes('sankeyFields') && (next.fieldChain?.length ?? 0) < 2) {
    next.fieldChain = ctx.offerableFields.slice(0, 3);
  }
  // Seed the cross-tab numeric metric (channels.x), preferring sentiment.
  if (nextParams.includes('crossMetric') && !next.channels?.x) {
    const opts = ctx.scalarMetricOptions;
    const seed =
      opts.find((m) => m === 'sentiment_score_bert_multilingual') ??
      opts.find((m) => m.startsWith('sentiment_score')) ??
      opts.find((m) => m === 'word_count') ??
      opts[0];
    if (seed) next.channels = { ...(next.channels ?? {}), x: seed };
  }
  // Seed the lead-lag x/y metrics (two distinct metrics).
  if (nextParams.includes('leadLagAxes') && (!next.channels?.x || !next.channels?.y)) {
    const opts = ctx.scalarMetricOptions;
    const x =
      next.channels?.x ??
      opts.find((m) => m.startsWith('sentiment_score')) ??
      opts[0] ??
      DEFAULT_METRIC_NAME;
    const y =
      next.channels?.y ??
      (opts.includes('word_count') && x !== 'word_count'
        ? 'word_count'
        : (opts.find((m) => m !== x) ?? x));
    next.channels = { ...(next.channels ?? {}), x, y };
  }
  return next;
}
