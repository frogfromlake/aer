import { describe, expect, it } from 'vitest';

import {
  SOURCE_PALETTE,
  SHARED_COLOR,
  UNKNOWN_PROVENANCE_COLOR,
  communityColor,
  labelColor,
  buildSourceColorMap,
  sourceColor,
  resolvedSourceCount,
  computeMetricExtent,
  computeMetricColorExtent,
  buildNetworkNodes,
  communityHeads,
  buildNetworkEdges,
  nodeRadius,
  nodeFillColor,
  rampBlueAmber,
  nodeStrokeColor,
  nodeStrokeWidth,
  edgeStrokeColor,
  buildHowToReadFacts,
  buildExportRows,
  buildExportPayload,
  type NetworkNode,
  type NodeColorContext,
  type HowToReadInput
} from '../../src/lib/presentations/cooccurrence-network-shared';
import type { CoOccurrenceGraphDto } from '../../src/lib/api/queries';

// Phase 125b / 142 — the pure, renderer-agnostic co-occurrence logic shared by
// both the SVG and WebGL renderers. These tests pin the colour channels, the
// metric extents, the node-radius mapping, and the export payload so a future
// refactor of either renderer cannot silently change what the two compute.

// A node carrying every field the derivations read; tests override per case.
function node(
  over: Partial<CoOccurrenceGraphDto['nodes'][number]> = {}
): CoOccurrenceGraphDto['nodes'][number] {
  return {
    text: 'Angela Merkel',
    label: 'PER',
    degree: 2,
    totalCount: 10,
    presence: ['tagesschau'],
    wikidataQid: 'Q567',
    metricValue: 0.4,
    ...over
  };
}

function edge(
  over: Partial<CoOccurrenceGraphDto['edges'][number]> = {}
): CoOccurrenceGraphDto['edges'][number] {
  return {
    a: 'Angela Merkel',
    b: 'Olaf Scholz',
    weight: 5,
    articleCount: 3,
    presence: ['tagesschau'],
    ...over
  };
}

function graph(over: Partial<CoOccurrenceGraphDto> = {}): CoOccurrenceGraphDto {
  return {
    topN: 50,
    nodes: [node(), node({ text: 'Olaf Scholz', totalCount: 6, degree: 1, metricValue: 0.2 })],
    edges: [edge()],
    ...over
  };
}

describe('communityColor', () => {
  it('returns the unknown-provenance grey for a null/undefined community', () => {
    expect(communityColor(null)).toBe(UNKNOWN_PROVENANCE_COLOR);
    expect(communityColor(undefined)).toBe(UNKNOWN_PROVENANCE_COLOR);
  });

  it('maps an id into the community palette and wraps past its length', () => {
    expect(communityColor(0)).toBe('#5283b8');
    // id 16 wraps back to palette[0] (16-hue palette).
    expect(communityColor(16)).toBe(communityColor(0));
    // negative ids fold via Math.abs.
    expect(communityColor(-1)).toBe(communityColor(1));
  });
});

describe('labelColor', () => {
  it('is deterministic for the same label', () => {
    expect(labelColor('PER')).toBe(labelColor('PER'));
  });

  it('returns a palette colour (hash-bucketed)', () => {
    const palette = ['#5283b8', '#b87a52', '#52b885', '#a058b8', '#b85265', '#888888'];
    expect(palette).toContain(labelColor('ORG'));
  });

  it('handles the empty string (hash 0 → first bucket)', () => {
    expect(labelColor('')).toBe('#5283b8');
  });
});

describe('buildSourceColorMap', () => {
  it('assigns palette colours by index and wraps past the palette length', () => {
    const names = SOURCE_PALETTE.map((_, i) => `s${i}`).concat('overflow');
    const map = buildSourceColorMap(names);
    expect(map['s0']).toBe(SOURCE_PALETTE[0]);
    expect(map['s1']).toBe(SOURCE_PALETTE[1]);
    // The 9th source (index 8) wraps to palette[0].
    expect(map['overflow']).toBe(SOURCE_PALETTE[0]);
  });

  it('is empty for an empty source list', () => {
    expect(buildSourceColorMap([])).toEqual({});
  });
});

describe('sourceColor', () => {
  const map = { tagesschau: '#aaa', zeit: '#bbb' };

  it('returns the unknown-provenance colour for an empty presence list', () => {
    expect(sourceColor([], map)).toBe(UNKNOWN_PROVENANCE_COLOR);
  });

  it('returns the single source colour for one source', () => {
    expect(sourceColor(['tagesschau'], map)).toBe('#aaa');
  });

  it('falls back to unknown-provenance when the single source is unmapped', () => {
    expect(sourceColor(['unknown'], map)).toBe(UNKNOWN_PROVENANCE_COLOR);
  });

  it('returns the shared colour when ≥2 sources are present', () => {
    expect(sourceColor(['tagesschau', 'zeit'], map)).toBe(SHARED_COLOR);
  });
});

describe('resolvedSourceCount', () => {
  it('counts distinct source names across both edge and node presence arrays', () => {
    const g = graph({
      nodes: [node({ presence: ['a', 'b'] })],
      edges: [edge({ presence: ['b', 'c'] })]
    });
    expect(resolvedSourceCount(g)).toBe(3); // a, b, c
  });

  it('tolerates missing presence arrays (key absent → none)', () => {
    // The `presence` key is OMITTED (not set to undefined) to exercise the
    // `?? []` fallback under exactOptionalPropertyTypes.
    const g = graph({
      nodes: [{ text: 'X', label: 'PER', degree: 0, totalCount: 1 }],
      edges: [{ a: 'X', b: 'Y', weight: 1, articleCount: 0 }]
    });
    expect(resolvedSourceCount(g)).toBe(0);
  });
});

describe('computeMetricExtent / computeMetricColorExtent', () => {
  it('returns the min/max of finite node metric values', () => {
    const g = graph({
      nodes: [node({ metricValue: 0.1 }), node({ metricValue: 0.9 }), node({ metricValue: 0.5 })]
    });
    expect(computeMetricExtent(g)).toEqual({ min: 0.1, max: 0.9 });
  });

  it('returns null when no node carries a finite size metric', () => {
    // One node has an explicit null metric; the other omits the key entirely.
    const g = graph({
      nodes: [node({ metricValue: null }), { text: 'Y', label: 'PER', degree: 0, totalCount: 1 }]
    });
    expect(computeMetricExtent(g)).toBeNull();
  });

  it('colour extent reads metricValueColor, falling back to metricValue', () => {
    const g = graph({
      nodes: [
        node({ metricValue: 0.2, metricValueColor: 0.8 }),
        node({ metricValue: 0.3, metricValueColor: null }) // falls back to 0.3
      ]
    });
    expect(computeMetricColorExtent(g)).toEqual({ min: 0.3, max: 0.8 });
  });

  it('colour extent is null when no node has a finite colour value', () => {
    const g = graph({ nodes: [node({ metricValue: null, metricValueColor: null })] });
    expect(computeMetricColorExtent(g)).toBeNull();
  });
});

describe('buildNetworkNodes', () => {
  it('maps DTO nodes to models with sizeNorm over the total_count channel', () => {
    const g = graph({
      nodes: [node({ text: 'A', totalCount: 10 }), node({ text: 'B', totalCount: 5 })]
    });
    const nodes = buildNetworkNodes(g, 'total_count', null);
    expect(nodes[0]!.id).toBe('A');
    expect(nodes[0]!.sizeNorm).toBe(1); // 10 / max(10)
    expect(nodes[1]!.sizeNorm).toBe(0.5); // 5 / 10
  });

  it('normalises the degree channel', () => {
    const g = graph({
      nodes: [node({ text: 'A', degree: 4 }), node({ text: 'B', degree: 2 })]
    });
    const nodes = buildNetworkNodes(g, 'degree', null);
    expect(nodes[1]!.sizeNorm).toBe(0.5); // 2 / 4
  });

  it('normalises the metric channel against the supplied extent (and 0 when no value)', () => {
    const g = graph({
      nodes: [node({ text: 'A', metricValue: 0.5 }), node({ text: 'B', metricValue: null })]
    });
    const nodes = buildNetworkNodes(g, 'metric', { min: 0, max: 1 });
    expect(nodes[0]!.sizeNorm).toBeCloseTo(0.5);
    expect(nodes[1]!.sizeNorm).toBe(0); // no metric value → raw 0
  });

  it('marks a node relabeled when a differing viewerLabel is present', () => {
    const g = graph({
      nodes: [node({ text: 'Angela Merkel', viewerLabel: 'Angela Merkel (de)' })]
    });
    const nodes = buildNetworkNodes(g, 'total_count', null);
    expect(nodes[0]!.relabeled).toBe(true);
    expect(nodes[0]!.displayName).toBe('Angela Merkel (de)');
  });

  it('does NOT relabel when the viewerLabel equals the source text', () => {
    const g = graph({
      nodes: [node({ text: 'Merkel', viewerLabel: 'Merkel' })]
    });
    const nodes = buildNetworkNodes(g, 'total_count', null);
    expect(nodes[0]!.relabeled).toBe(false);
    expect(nodes[0]!.displayName).toBe('Merkel');
  });

  it('attaches the community when a mapping is supplied', () => {
    const g = graph({ nodes: [node({ text: 'A' })] });
    const communities = new Map([['A', 3]]);
    const nodes = buildNetworkNodes(g, 'total_count', null, communities);
    expect(nodes[0]!.community).toBe(3);
  });

  it('defaults missing optional fields (degree/presence/qid/metric) safely', () => {
    const g = graph({
      nodes: [
        {
          text: 'X',
          label: 'LOC',
          degree: 0,
          totalCount: 1
          // presence, wikidataQid, metricValue all omitted
        }
      ]
    });
    const nodes = buildNetworkNodes(g, 'total_count', null);
    expect(nodes[0]!.presence).toEqual([]);
    expect(nodes[0]!.presenceCount).toBe(0);
    expect(nodes[0]!.wikidataQid).toBeNull();
    expect(nodes[0]!.metricValue).toBeNull();
    expect(nodes[0]!.community).toBeNull();
  });
});

describe('communityHeads', () => {
  it('picks the largest-by-total-count node as the head of each community', () => {
    const mk = (id: string, community: number | null, totalCount: number): NetworkNode =>
      ({ id, community, totalCount }) as NetworkNode;
    const heads = communityHeads([
      mk('a', 1, 5),
      mk('b', 1, 9), // head of community 1
      mk('c', 2, 3), // head of community 2 (only member)
      mk('d', null, 100) // no community → ignored
    ]);
    expect(heads.has('b')).toBe(true);
    expect(heads.has('a')).toBe(false);
    expect(heads.has('c')).toBe(true);
    expect(heads.has('d')).toBe(false);
  });
});

describe('buildNetworkEdges', () => {
  it('maps DTO edges, defaulting articleCount/presence/nsSupport', () => {
    // articleCount/presence/nsSupport omitted to exercise the runtime `?? 0`/`?? []`
    // defaults (the schema marks articleCount required; the code guards anyway).
    const g = graph({
      edges: [{ a: 'A', b: 'B', weight: 7 } as CoOccurrenceGraphDto['edges'][number]]
    });
    const edges = buildNetworkEdges(g);
    expect(edges[0]).toEqual({
      source: 'A',
      target: 'B',
      weight: 7,
      articleCount: 0,
      presence: [],
      nsSupport: 0
    });
  });

  it('carries nsSupport through when present', () => {
    const g = graph({ edges: [edge({ nsSupport: 2 })] });
    expect(buildNetworkEdges(g)[0]!.nsSupport).toBe(2);
  });
});

describe('nodeRadius', () => {
  it('uses the default (4, 22) range with a sqrt scale', () => {
    expect(nodeRadius(0)).toBe(4); // min radius
    expect(nodeRadius(1)).toBe(26); // 4 + 22*sqrt(1)
    expect(nodeRadius(0.25)).toBe(15); // 4 + 22*0.5
  });

  it('clamps a negative sizeNorm to the min radius', () => {
    expect(nodeRadius(-5)).toBe(4);
  });

  it('honours a custom min/span', () => {
    expect(nodeRadius(1, 2, 10)).toBe(12);
  });
});

describe('nodeFillColor', () => {
  const ctx = (over: Partial<NodeColorContext> = {}): NodeColorContext => ({
    netColor: 'label',
    metricExtent: null,
    maxPresence: 3,
    sourceColorMap: { tagesschau: '#aaa' },
    ...over
  });
  const colorable = (over: Record<string, unknown> = {}) =>
    ({
      label: 'PER',
      metricValue: 0.5,
      metricColorValue: null,
      community: 1,
      presenceCount: 2,
      presence: ['tagesschau'],
      ...over
    }) as Parameters<typeof nodeFillColor>[0];

  it('uniform → the fixed blue', () => {
    expect(nodeFillColor(colorable(), ctx({ netColor: 'uniform' }))).toBe('#5283b8');
  });

  it('community → the community palette colour', () => {
    expect(nodeFillColor(colorable({ community: 0 }), ctx({ netColor: 'community' }))).toBe(
      '#5283b8'
    );
  });

  it('metric → ramp endpoints map min→ramp(0), max→ramp(1) (direction not inverted)', () => {
    const lo = nodeFillColor(
      colorable({ metricColorValue: 0 }),
      ctx({ netColor: 'metric', metricExtent: { min: 0, max: 1 } })
    );
    const hi = nodeFillColor(
      colorable({ metricColorValue: 1 }),
      ctx({ netColor: 'metric', metricExtent: { min: 0, max: 1 } })
    );
    // The min end is the ramp's 0 anchor and the max end its 1 anchor — an
    // inverted ramp (t = (max-cv)/span) would swap these and fail.
    expect(lo).toBe(rampBlueAmber(0));
    expect(hi).toBe(rampBlueAmber(1));
    expect(lo).not.toBe(hi);
  });

  it('metric → colour channel is metricColorValue, falling back to metricValue', () => {
    // metricColorValue present → it wins over metricValue (reading the wrong
    // field would yield ramp(1) here instead of ramp(0)).
    expect(
      nodeFillColor(
        colorable({ metricColorValue: 0, metricValue: 1 }),
        ctx({ netColor: 'metric', metricExtent: { min: 0, max: 1 } })
      )
    ).toBe(rampBlueAmber(0));
    // metricColorValue null → falls back to metricValue.
    expect(
      nodeFillColor(
        colorable({ metricColorValue: null, metricValue: 1 }),
        ctx({ netColor: 'metric', metricExtent: { min: 0, max: 1 } })
      )
    ).toBe(rampBlueAmber(1));
  });

  it('metric → grey when no value or no extent', () => {
    expect(
      nodeFillColor(
        colorable({ metricColorValue: null, metricValue: null }),
        ctx({ netColor: 'metric', metricExtent: { min: 0, max: 1 } })
      )
    ).toBe('#4a4f57');
    expect(
      nodeFillColor(
        colorable({ metricColorValue: 0.5 }),
        ctx({ netColor: 'metric', metricExtent: null })
      )
    ).toBe('#4a4f57');
  });

  it('metric → midpoint colour (ramp(0.5)) when the extent has zero span', () => {
    expect(
      nodeFillColor(
        colorable({ metricColorValue: 0.5 }),
        ctx({ netColor: 'metric', metricExtent: { min: 0.5, max: 0.5 } })
      )
    ).toBe(rampBlueAmber(0.5));
  });

  it('presence → ramp over [1, maxPresence]; collapses to ramp(0) when maxPresence ≤ 1', () => {
    // presenceCount 3 of max 5 → t = (3-1)/(5-1) = 0.5.
    expect(
      nodeFillColor(colorable({ presenceCount: 3 }), ctx({ netColor: 'presence', maxPresence: 5 }))
    ).toBe(rampBlueAmber(0.5));
    // The full-presence end maps to ramp(1).
    expect(
      nodeFillColor(colorable({ presenceCount: 5 }), ctx({ netColor: 'presence', maxPresence: 5 }))
    ).toBe(rampBlueAmber(1));
    // maxPresence ≤ 1 → no spread, anchored at ramp(0).
    expect(
      nodeFillColor(colorable({ presenceCount: 1 }), ctx({ netColor: 'presence', maxPresence: 1 }))
    ).toBe(rampBlueAmber(0));
  });

  it('source_overlay → the source provenance colour', () => {
    expect(
      nodeFillColor(colorable({ presence: ['tagesschau'] }), ctx({ netColor: 'source_overlay' }))
    ).toBe('#aaa');
  });

  it('label (default) → the label hash colour', () => {
    expect(nodeFillColor(colorable({ label: 'PER' }), ctx({ netColor: 'label' }))).toBe(
      labelColor('PER')
    );
  });
});

describe('nodeStrokeColor / nodeStrokeWidth', () => {
  const map = { tagesschau: '#aaa' };

  it('selection ring wins over everything', () => {
    expect(nodeStrokeColor({ presence: [] }, true, map, true)).toBe('var(--color-fg)');
    expect(nodeStrokeWidth(10, true, true)).toBe(1.5);
  });

  it('no stroke when not merged and not selected', () => {
    expect(nodeStrokeColor({ presence: ['tagesschau'] }, false, map, false)).toBe('none');
    expect(nodeStrokeWidth(10, false, false)).toBe(0);
  });

  it('source-provenance stroke when merged', () => {
    expect(nodeStrokeColor({ presence: ['tagesschau'] }, true, map, false)).toBe('#aaa');
    // width scales with radius, floored at 1.5.
    expect(nodeStrokeWidth(100, true, false)).toBe(18);
    expect(nodeStrokeWidth(1, true, false)).toBe(1.5);
  });
});

describe('edgeStrokeColor', () => {
  const map = { tagesschau: '#aaa', zeit: '#bbb' };

  it('source-provenance colour when merged', () => {
    expect(edgeStrokeColor({ presence: ['tagesschau'] }, true, map)).toBe('#aaa');
    expect(edgeStrokeColor({ presence: ['tagesschau', 'zeit'] }, true, map)).toBe(SHARED_COLOR);
  });

  it('neutral grey when not merged', () => {
    expect(edgeStrokeColor({ presence: ['tagesschau'] }, false, map)).toBe(
      'rgba(180, 200, 220, 0.5)'
    );
  });
});

describe('buildHowToReadFacts', () => {
  it('passes through every fact unchanged', () => {
    const input: HowToReadInput = {
      topN: 80,
      netSize: 'degree',
      netColor: 'presence',
      renderedCount: 40,
      displayLanguage: 'viewer',
      viewerLanguage: 'en',
      linkedNodeCount: 12,
      labeledNodeCount: 8,
      configOverridden: true
    };
    expect(buildHowToReadFacts(input)).toEqual(input);
  });
});

describe('buildExportRows / buildExportPayload', () => {
  it('builds one export row per edge with joined sources', () => {
    const g = graph({
      edges: [edge({ a: 'A', b: 'B', weight: 4, articleCount: 2, presence: ['x', 'y'] })]
    });
    expect(buildExportRows(g)).toEqual([
      { entityA: 'A', entityB: 'B', weight: 4, articleCount: 2, sources: 'x|y' }
    ]);
  });

  it('returns no rows for null data', () => {
    expect(buildExportRows(null)).toEqual([]);
  });

  it('renders an empty articleCount when omitted on an edge', () => {
    const g = graph({
      edges: [{ a: 'A', b: 'B', weight: 1 } as CoOccurrenceGraphDto['edges'][number]]
    });
    expect(buildExportRows(g)[0]!.articleCount).toBe('');
  });

  it('assembles the full export payload (meta + summary + how-to-read + rows + columns)', () => {
    const facts: HowToReadInput = {
      topN: 50,
      netSize: 'total_count',
      netColor: 'label',
      renderedCount: 2,
      displayLanguage: 'source',
      viewerLanguage: undefined,
      linkedNodeCount: 1,
      labeledNodeCount: 0,
      configOverridden: undefined
    };
    const payload = buildExportPayload({
      scope: 'probe',
      scopeId: 'probe-0',
      windowStart: '2026-01-01T00:00:00Z',
      windowEnd: undefined,
      topN: 50,
      netSize: 'total_count',
      netColor: 'label',
      nodeCount: 2,
      edgeCount: 1,
      howToReadFacts: facts,
      data: graph()
    });
    expect(payload.meta).toMatchObject({
      viewMode: 'cooccurrence_network',
      scope: 'probe',
      scopeId: 'probe-0',
      windowStart: '2026-01-01T00:00:00Z',
      sizeChannel: 'total_count',
      colorChannel: 'label'
    });
    expect(payload.summary).toEqual({ nodes: 2, edges: 1 });
    expect(payload.columns).toEqual(['entityA', 'entityB', 'weight', 'articleCount', 'sources']);
    expect(payload.rows).toHaveLength(1);
    expect(Array.isArray(payload.howToRead)).toBe(true);
    expect((payload.howToRead ?? []).length).toBeGreaterThan(0);
  });
});
