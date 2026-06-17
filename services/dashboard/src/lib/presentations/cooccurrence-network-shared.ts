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
export type NetColorChannel =
  | 'uniform'
  | 'label'
  | 'presence'
  | 'source_overlay'
  | 'metric'
  | 'community';

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
  /** Co-occurrence redesign — Louvain community id (theme cluster), or null when
   *  not computed. Drives the 'community' colour channel. */
  community: number | null;
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

// Co-occurrence redesign — a wide categorical palette for theme-cluster
// (Louvain community) colouring. 16 perceptually-distinct hues so adjacent
// topic regions read apart; communities beyond 16 wrap (acceptable — the big
// clusters get distinct colours, the long tail rarely matters).
const COMMUNITY_PALETTE = [
  '#5283b8',
  '#b87a52',
  '#52b885',
  '#a058b8',
  '#b85265',
  '#7f8a4e',
  '#3c9aa0',
  '#a07b3c',
  '#6d6fb8',
  '#b85299',
  '#52b8a0',
  '#b89752',
  '#8a5fb8',
  '#5ba36f',
  '#b8525f',
  '#4e7f8a'
];

/** Colour for a Louvain community id; grey when the node has no community
 *  (unconnected / not computed yet) — honest absence, never a fake cluster. */
export function communityColor(id: number | null | undefined): string {
  if (id == null) return UNKNOWN_PROVENANCE_COLOR;
  return COMMUNITY_PALETTE[Math.abs(id) % COMMUNITY_PALETTE.length] ?? COMMUNITY_PALETTE[0]!;
}

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

// Exported so the node-fill ramp can be pinned in unit tests (the blue→amber
// endpoints and direction are a behavioural contract, not an implementation
// detail the test should be blind to).
export function rampBlueAmber(t: number): string {
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
  metricExtent: MetricExtent | null,
  communities?: Map<string, number> | null
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
      community: communities?.get(n.text) ?? null,
      sizeNorm: maxSize > 0 ? rawSize(n, netSize, metricExtent) / maxSize : 0
    };
  });
}

/** Co-occurrence redesign — detect theme clusters (Louvain communities) from the
 *  edge topology. Lazy-imports graphology + the Louvain module so they ship only
 *  when community colouring is used. Returns node-text → community id. Weighted
 *  (co-occurrence strength) so frequent partners group. Best-effort: any failure
 *  returns an empty map (→ grey nodes, honest absence — never a fake cluster). */
export async function computeCommunities(data: CoOccurrenceGraphDto): Promise<Map<string, number>> {
  const out = new Map<string, number>();
  if (data.nodes.length === 0 || data.edges.length === 0) return out;
  try {
    const [{ default: Graph }, louvainMod] = await Promise.all([
      import('graphology'),
      import('graphology-communities-louvain')
    ]);
    const louvain = louvainMod.default;
    const g = new Graph({ multi: false, type: 'undirected' });
    for (const n of data.nodes) if (!g.hasNode(n.text)) g.addNode(n.text);
    for (const e of data.edges) {
      if (!g.hasNode(e.a) || !g.hasNode(e.b) || g.hasEdge(e.a, e.b)) continue;
      g.addEdge(e.a, e.b, { weight: e.weight });
    }
    const mapping = louvain(g, { getEdgeWeight: 'weight' }) as Record<string, number>;
    for (const [node, comm] of Object.entries(mapping)) out.set(node, comm);
  } catch {
    /* best-effort — leave empty so nodes render grey, never a fabricated cluster */
  }
  return out;
}

/** The representative (largest by total co-occurrence count) node per community —
 *  so each theme cluster can be labelled with one name (the Kriesel map effect). */
export function communityHeads(nodes: NetworkNode[]): Set<string> {
  const best = new Map<number, { id: string; total: number }>();
  for (const n of nodes) {
    if (n.community == null) continue;
    const cur = best.get(n.community);
    if (!cur || n.totalCount > cur.total) best.set(n.community, { id: n.id, total: n.totalCount });
  }
  return new Set([...best.values()].map((b) => b.id));
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
  /** Co-occurrence redesign — Louvain community id for the 'community' channel. */
  community?: number | null;
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
  if (ctx.netColor === 'community') return communityColor(n.community);
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
