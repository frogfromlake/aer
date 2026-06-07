// Phase 125b — pure, renderer-agnostic co-occurrence logic shared by BOTH the
// default SVG cell (CoOccurrenceNetworkCell.svelte, d3-force, small/capped) and
// the large-scale WebGL renderer (CoOccurrenceNetworkAtScale.svelte, sigma.js,
// maximized single-cell). Keeping node sizing, colour, relabelling, how-to-read
// facts and the export payload in ONE place is the anti-stale-code guarantee:
// the two renderers compute identical visuals from the same DTO; neither copies
// the other. Anything here is a pure function/constant — no Svelte, no DOM, no
// rendering primitives (those stay in each renderer).

import type { CoOccurrenceGraphDto } from '$lib/api/queries';
import type { ExportPayload, ExportRow } from './cell-export';
import { composeHowToRead } from './how-to-read';

export type NetSizeChannel = 'total_count' | 'degree' | 'metric';
export type NetColorChannel = 'uniform' | 'label' | 'presence' | 'source_overlay' | 'metric';

export interface MetricExtent {
  min: number;
  max: number;
}

/** Renderer-agnostic node model derived from the DTO. `sizeNorm` is the active
 *  size channel normalised to 0..1 (each renderer maps it to its own radius
 *  range via {@link nodeRadius}). */
export interface NetworkNode {
  id: string;
  label: string;
  sourceText: string;
  viewerLabel: string | null;
  displayName: string;
  relabeled: boolean;
  totalCount: number;
  degree: number;
  presenceCount: number;
  presence: string[];
  wikidataQid: string | null;
  /** Size-channel metric mean (BFF `metricValue`). */
  metricValue: number | null;
  /** Phase 125 / ISSUE 7 — colour-channel metric mean. Falls back to
   *  `metricValue` when colour reuses the size metric (the BFF returns
   *  `metricValueColor` only when the colour metric differs from the size one). */
  metricColorValue: number | null;
  sizeNorm: number;
}

export interface NetworkEdge {
  source: string;
  target: string;
  weight: number;
  articleCount: number;
  presence: string[];
  /** Phase 122d.2 — contributing articles with no real publication date
   *  (`fetch_at_fallback`); >0 only when the NS overlay was requested. */
  nsSupport: number;
}

// ── Palettes (Phase 131/131a — preserved verbatim from the SVG cell) ──────────

const LABEL_PALETTE = ['#5283b8', '#b87a52', '#52b885', '#a058b8', '#b85265', '#888888'];
export const SOURCE_PALETTE = [
  '#5283b8',
  '#b87a52',
  '#52b885',
  '#a058b8',
  '#b85265',
  '#7f8a4e',
  '#3c5da0',
  '#a07b3c'
];
export const SHARED_COLOR = '#9aa1ab';
export const UNKNOWN_PROVENANCE_COLOR = '#3b3f47';
const METRIC_LO = [82, 131, 184];
const METRIC_HI = [224, 160, 80];

export function labelColor(label: string): string {
  let h = 0;
  for (let i = 0; i < label.length; i++) h = (h * 31 + label.charCodeAt(i)) | 0;
  return LABEL_PALETTE[Math.abs(h) % LABEL_PALETTE.length] ?? LABEL_PALETTE[0]!;
}

/** Per-source colour map, assigned by index in the resolved source list so the
 *  graph keeps stable colours while the scope is stable. */
export function buildSourceColorMap(sourceNames: readonly string[]): Record<string, string> {
  const map: Record<string, string> = {};
  sourceNames.forEach((name, i) => {
    map[name] = SOURCE_PALETTE[i % SOURCE_PALETTE.length] ?? '#5283b8';
  });
  return map;
}

/** Source-provenance colour: one source → its palette colour; ≥2 → shared grey;
 *  none → the distinct "no provenance data" colour (never silently "shared"). */
export function sourceColor(presence: string[], sourceColorMap: Record<string, string>): string {
  if (presence.length === 0) return UNKNOWN_PROVENANCE_COLOR;
  if (presence.length === 1) return sourceColorMap[presence[0]!] ?? UNKNOWN_PROVENANCE_COLOR;
  return SHARED_COLOR;
}

function rampBlueAmber(t: number): string {
  const c = METRIC_LO.map((l, i) => Math.round(l + ((METRIC_HI[i] ?? l) - l) * t));
  return `rgb(${c[0]}, ${c[1]}, ${c[2]})`;
}

// ── Derivations from the DTO ──────────────────────────────────────────────────

/** Distinct source names across all edge/node presence arrays — the SoT for
 *  "merged scope" (probe-as-aggregator may resolve to >1 underlying source). */
export function resolvedSourceCount(data: CoOccurrenceGraphDto): number {
  const names: Record<string, true> = {};
  for (const e of data.edges) for (const s of e.presence ?? []) names[s] = true;
  for (const n of data.nodes) for (const s of n.presence ?? []) names[s] = true;
  return Object.keys(names).length;
}

/** min/max of the per-node SIZE metric across the graph, for normalising the
 *  metric size channel. Null when no node carries a finite metric value. */
export function computeMetricExtent(data: CoOccurrenceGraphDto): MetricExtent | null {
  const vals = data.nodes
    .map((n) => n.metricValue)
    .filter((v): v is number => v != null && Number.isFinite(v));
  if (vals.length === 0) return null;
  return { min: Math.min(...vals), max: Math.max(...vals) };
}

/** Phase 125 / ISSUE 7 — min/max of the per-node COLOUR metric. Reads
 *  `metricValueColor` (the separate colour metric) and falls back to
 *  `metricValue` when colour reuses the size metric. Null when no node carries a
 *  finite colour value. */
export function computeMetricColorExtent(data: CoOccurrenceGraphDto): MetricExtent | null {
  const vals = data.nodes
    .map((n) => n.metricValueColor ?? n.metricValue)
    .filter((v): v is number => v != null && Number.isFinite(v));
  if (vals.length === 0) return null;
  return { min: Math.min(...vals), max: Math.max(...vals) };
}

/** Raw size for a node under the active channel (pre-normalisation). */
function rawSize(
  n: CoOccurrenceGraphDto['nodes'][number],
  netSize: NetSizeChannel,
  metricExtent: MetricExtent | null
): number {
  if (netSize === 'metric') {
    if (n.metricValue == null || !metricExtent) return 0;
    const span = metricExtent.max - metricExtent.min;
    return span > 0 ? (n.metricValue - metricExtent.min) / span : 0.5;
  }
  return netSize === 'degree' ? (n.degree ?? 0) : n.totalCount;
}

/** Build the renderer-agnostic node models, with `sizeNorm` ∈ [0,1]. The relabel
 *  rule (Phase 123b): show the viewer-language label only when one was returned
 *  for the node's QID and it differs from the source form. */
export function buildNetworkNodes(
  data: CoOccurrenceGraphDto,
  netSize: NetSizeChannel,
  metricExtent: MetricExtent | null
): NetworkNode[] {
  const maxSize =
    netSize === 'metric'
      ? 1
      : data.nodes.reduce((m, n) => Math.max(m, rawSize(n, netSize, metricExtent)), 1);
  return data.nodes.map((n) => {
    const viewerLabel = n.viewerLabel ?? null;
    const relabeled = !!viewerLabel && viewerLabel !== n.text;
    return {
      id: n.text,
      label: n.label,
      sourceText: n.text,
      viewerLabel,
      displayName: relabeled ? (viewerLabel as string) : n.text,
      relabeled,
      totalCount: n.totalCount,
      degree: n.degree ?? 0,
      presenceCount: n.presence?.length ?? 0,
      presence: n.presence ?? [],
      wikidataQid: n.wikidataQid ?? null,
      metricValue: n.metricValue ?? null,
      metricColorValue: n.metricValueColor ?? n.metricValue ?? null,
      sizeNorm: maxSize > 0 ? rawSize(n, netSize, metricExtent) / maxSize : 0
    };
  });
}

export function buildNetworkEdges(data: CoOccurrenceGraphDto): NetworkEdge[] {
  return data.edges.map((e) => ({
    source: e.a,
    target: e.b,
    weight: e.weight,
    articleCount: e.articleCount ?? 0,
    presence: e.presence ?? [],
    nsSupport: e.nsSupport ?? 0
  }));
}

/** Map sizeNorm (0..1) to a radius. The SVG cell uses (4, 22); the at-scale
 *  renderer can pass a tighter range. sqrt keeps small nodes visible. */
export function nodeRadius(sizeNorm: number, minRadius = 4, span = 22): number {
  return minRadius + span * Math.sqrt(Math.max(0, sizeNorm));
}

// ── Visual channels (structural node type so SimNode/NetworkNode both fit) ─────

interface ColorableNode {
  label: string;
  metricValue?: number | null;
  /** Phase 125 / ISSUE 7 — colour-channel metric value (falls back to
   *  metricValue when colour reuses the size metric). */
  metricColorValue?: number | null;
  presenceCount: number;
  presence: string[];
}

export interface NodeColorContext {
  netColor: NetColorChannel;
  metricExtent: MetricExtent | null;
  maxPresence: number;
  sourceColorMap: Record<string, string>;
}

/** Node fill bound to the active colour channel. A node with no metric value is
 *  greyed (honest absence, never a fake 0). For the 'metric' channel, `ctx.metricExtent`
 *  is the COLOUR metric's extent and the value read is `metricColorValue` (Phase
 *  125 / ISSUE 7), falling back to `metricValue` when colour reuses the size metric. */
export function nodeFillColor(n: ColorableNode, ctx: NodeColorContext): string {
  if (ctx.netColor === 'uniform') return '#5283b8';
  if (ctx.netColor === 'metric') {
    const cv = n.metricColorValue ?? n.metricValue ?? null;
    if (cv == null || !ctx.metricExtent) return '#4a4f57';
    const span = ctx.metricExtent.max - ctx.metricExtent.min;
    const t = span > 0 ? (cv - ctx.metricExtent.min) / span : 0.5;
    return rampBlueAmber(t);
  }
  if (ctx.netColor === 'presence') {
    const t = ctx.maxPresence > 1 ? (n.presenceCount - 1) / (ctx.maxPresence - 1) : 0;
    return rampBlueAmber(t);
  }
  if (ctx.netColor === 'source_overlay') return sourceColor(n.presence, ctx.sourceColorMap);
  return labelColor(n.label);
}

/** Node border channel: source provenance when the scope is merged; the
 *  selection ring (--color-fg) takes priority (a UI affordance, not data). */
export function nodeStrokeColor(
  n: Pick<ColorableNode, 'presence'>,
  isMergedScope: boolean,
  sourceColorMap: Record<string, string>,
  isSelected: boolean
): string | 'none' {
  if (isSelected) return 'var(--color-fg)';
  if (!isMergedScope) return 'none';
  return sourceColor(n.presence, sourceColorMap);
}

export function nodeStrokeWidth(
  radius: number,
  isMergedScope: boolean,
  isSelected: boolean
): number {
  if (isSelected) return 1.5;
  if (!isMergedScope) return 0;
  return Math.max(1.5, radius * 0.18);
}

/** Edge stroke: source-provenance when merged, neutral grey otherwise (edges
 *  carry no metric channel). */
export function edgeStrokeColor(
  e: Pick<NetworkEdge, 'presence'>,
  isMergedScope: boolean,
  sourceColorMap: Record<string, string>
): string {
  if (isMergedScope) return sourceColor(e.presence, sourceColorMap);
  return 'rgba(180, 200, 220, 0.5)';
}

// ── How-to-read facts + export payload (shared output contracts) ──────────────

export interface HowToReadInput {
  topN: number;
  netSize: NetSizeChannel;
  netColor: NetColorChannel;
  renderedCount: number;
  displayLanguage: 'source' | 'viewer';
  viewerLanguage: string | undefined;
  linkedNodeCount: number;
  labeledNodeCount: number;
  configOverridden: boolean | undefined;
}

export function buildHowToReadFacts(i: HowToReadInput) {
  return {
    topN: i.topN,
    netSize: i.netSize,
    netColor: i.netColor,
    renderedCount: i.renderedCount,
    displayLanguage: i.displayLanguage,
    viewerLanguage: i.viewerLanguage,
    linkedNodeCount: i.linkedNodeCount,
    labeledNodeCount: i.labeledNodeCount,
    configOverridden: i.configOverridden
  };
}

export function buildExportRows(data: CoOccurrenceGraphDto | null): ExportRow[] {
  return (data?.edges ?? []).map((e) => ({
    entityA: e.a,
    entityB: e.b,
    weight: e.weight,
    articleCount: e.articleCount ?? '',
    sources: (e.presence ?? []).join('|')
  }));
}

export interface ExportPayloadInput {
  scope: string;
  scopeId: string;
  windowStart: string | undefined;
  windowEnd: string | undefined;
  topN: number;
  netSize: NetSizeChannel;
  netColor: NetColorChannel;
  nodeCount: number;
  edgeCount: number;
  howToReadFacts: HowToReadInput;
  data: CoOccurrenceGraphDto | null;
}

export function buildExportPayload(i: ExportPayloadInput): ExportPayload {
  return {
    meta: {
      viewMode: 'cooccurrence_network',
      scope: i.scope,
      scopeId: i.scopeId,
      windowStart: i.windowStart,
      windowEnd: i.windowEnd,
      topN: i.topN,
      sizeChannel: i.netSize,
      colorChannel: i.netColor
    },
    summary: { nodes: i.nodeCount, edges: i.edgeCount },
    howToRead: composeHowToRead('cooccurrence_network', buildHowToReadFacts(i.howToReadFacts)),
    rows: buildExportRows(i.data),
    columns: ['entityA', 'entityB', 'weight', 'articleCount', 'sources']
  };
}
