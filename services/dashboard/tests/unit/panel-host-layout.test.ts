import { describe, it, expect } from 'vitest';
import {
  extractDimensionAvail,
  dimensionSourceFilter,
  droppedSources,
  cellDimensionOptions,
  scalarMetricOptionsFromAvailable,
  renderedProbeCount,
  isIntensiveMetric,
  effectiveCellScale,
  computeSharedCellKeys,
  unionExtent,
  computeSharedDomains,
  type DimensionAvail
} from '../../src/lib/workbench/panel-host-layout';
import type { CellRenderUnit } from '../../src/lib/workbench/panel-queries';
import type { Panel } from '../../src/lib/state/url-internals';
import type {
  ScopeAvailableMetricsDto,
  ScopeAvailableMetadataDto
} from '../../src/lib/api/queries';

const metricDto = (over: Partial<ScopeAvailableMetricsDto> = {}) =>
  ({
    available: ['sentiment_score_sentiws', 'word_count'],
    scopedSources: ['s1', 's2', 's3'],
    partial: [{ metricName: 'image_count', sources: ['s1'] }],
    ...over
  }) as unknown as ScopeAvailableMetricsDto;

const metadataDto = (over: Partial<ScopeAvailableMetadataDto> = {}) =>
  ({
    available: ['section'],
    scopedSources: ['s1', 's2'],
    partial: [{ field: 'author', sources: ['s1'] }],
    ...over
  }) as unknown as ScopeAvailableMetadataDto;

const unit = (key: string, probeId?: string, probeIds: string[] = []): CellRenderUnit =>
  ({ key, probeId, probeIds }) as unknown as CellRenderUnit;

describe('extractDimensionAvail', () => {
  it('returns null for null data', () => {
    expect(extractDimensionAvail(null, false, 'word_count')).toBeNull();
  });

  it('reads the metricName partial key for metric views', () => {
    expect(extractDimensionAvail(metricDto(), false, 'image_count')).toEqual({
      available: ['sentiment_score_sentiws', 'word_count'],
      partialSources: ['s1'],
      scopedSources: ['s1', 's2', 's3']
    });
  });

  it('reads the field partial key for field-driven views', () => {
    expect(extractDimensionAvail(metadataDto(), true, 'author')).toMatchObject({
      partialSources: ['s1']
    });
  });

  it('has null partialSources when the metric is not partial', () => {
    expect(extractDimensionAvail(metricDto(), false, 'word_count')!.partialSources).toBeNull();
  });
});

describe('dimensionSourceFilter', () => {
  const avail: DimensionAvail = {
    available: ['word_count'],
    partialSources: ['s1'],
    scopedSources: ['s1', 's2']
  };

  it('returns null when avail is null or the metric is in the intersection', () => {
    expect(dimensionSourceFilter(null, 'x')).toBeNull();
    expect(dimensionSourceFilter(avail, 'word_count')).toBeNull(); // available → no narrowing
  });

  it('narrows to the partial sources for a partial metric', () => {
    const f = dimensionSourceFilter(avail, 'image_count');
    expect(f).toEqual(new Set(['s1']));
  });
});

describe('droppedSources', () => {
  it('is empty without a partial; else the scoped sources lacking it', () => {
    expect(droppedSources(null)).toEqual([]);
    expect(droppedSources({ available: [], partialSources: null, scopedSources: ['s1'] })).toEqual(
      []
    );
    expect(
      droppedSources({ available: [], partialSources: ['s1'], scopedSources: ['s1', 's2', 's3'] })
    ).toEqual(['s2', 's3']);
  });
});

describe('cellDimensionOptions', () => {
  it('is empty without data or open sources', () => {
    expect(cellDimensionOptions(null, ['s1'], false)).toEqual([]);
    expect(cellDimensionOptions(metricDto(), [], false)).toEqual([]);
  });

  it('adds a partial metric only when an open source carries it (sorted, deduped)', () => {
    // s1 carries the partial image_count → included; s2-only cell would not.
    expect(cellDimensionOptions(metricDto(), ['s1'], false)).toEqual([
      'image_count',
      'sentiment_score_sentiws',
      'word_count'
    ]);
    expect(cellDimensionOptions(metricDto(), ['s2'], false)).toEqual([
      'sentiment_score_sentiws',
      'word_count'
    ]);
  });

  it('uses the field key under field-driven views', () => {
    expect(cellDimensionOptions(metadataDto(), ['s1'], true)).toEqual(['author', 'section']);
  });

  it('excludes degenerate (no-signal / constant) dimensions', () => {
    // word_count is degenerate (constant) for this scope → not offerable per-cell.
    expect(
      cellDimensionOptions(
        metricDto({ degenerate: [{ metricName: 'word_count', value: 0 }] }),
        ['s1'],
        false
      )
    ).toEqual(['image_count', 'sentiment_score_sentiws']);
    // section is a degenerate field → dropped under a field-driven view.
    expect(
      cellDimensionOptions(
        metadataDto({ degenerate: [{ field: 'section', value: 'Politics' }] }),
        ['s1'],
        true
      )
    ).toEqual(['author']);
  });
});

describe('scalarMetricOptionsFromAvailable', () => {
  it('prepends the default and dedups', () => {
    expect(scalarMetricOptionsFromAvailable(['word_count', 'sentiment_score_sentiws'])).toEqual([
      'sentiment_score_sentiws', // DEFAULT_METRIC_NAME prepended
      'word_count'
      // 'sentiment_score_sentiws' from the list is deduped
    ]);
  });
});

describe('renderedProbeCount', () => {
  it('dedups probeId + probeIds across units', () => {
    expect(
      renderedProbeCount([
        unit('a', 'p1'),
        unit('b', 'p1', ['p2']),
        unit('c', undefined, ['p2', 'p3'])
      ])
    ).toBe(3);
  });
});

describe('isIntensiveMetric', () => {
  it('scatter is intensive iff either axis metric is intensive', () => {
    const scatter = (channels: Record<string, string> | undefined) =>
      isIntensiveMetric({
        view: 'metric_scatter',
        channels,
        metric: 'x',
        presentationUsesMetric: false
      });
    expect(scatter({ x: 'word_count', y: 'entity_count' })).toBe(false); // both pure-count
    expect(scatter({ x: 'word_count', y: 'sentiment_score_sentiws' })).toBe(true);
    expect(scatter(undefined)).toBe(true); // default y = sentiment (intensive)
  });

  it('non-scatter: false when usesMetric is false; else intensive iff the metric is not pure-count', () => {
    expect(
      isIntensiveMetric({
        view: 'distribution',
        channels: undefined,
        metric: 'sentiment_score_sentiws',
        presentationUsesMetric: false
      })
    ).toBe(false);
    expect(
      isIntensiveMetric({
        view: 'distribution',
        channels: undefined,
        metric: 'word_count',
        presentationUsesMetric: true
      })
    ).toBe(false);
    expect(
      isIntensiveMetric({
        view: 'distribution',
        channels: undefined,
        metric: 'sentiment_score_sentiws',
        presentationUsesMetric: true
      })
    ).toBe(true);
  });
});

describe('effectiveCellScale', () => {
  const co = (o: object) => o as Panel['cellOverrides'];

  it('the cross-context guard forces free', () => {
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: true,
        cellOverrides: undefined,
        panelScales: 'shared'
      })
    ).toBe('free');
  });

  it('a per-cell x/y channel override frees the cell', () => {
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: co({ c0: { channels: { x: 'word_count' } } }),
        panelScales: 'shared'
      })
    ).toBe('free');
  });

  it('a per-cell metric override defaults to free (Phase 148f), but an explicit scale wins', () => {
    // metric override, no explicit scale → free (incomparable to the panel).
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: co({ c0: { metric: 'word_count' } }),
        panelScales: 'shared'
      })
    ).toBe('free');
    // explicit scale override on top of a metric override → the user's choice wins.
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: co({ c0: { metric: 'word_count', scales: 'shared' } }),
        panelScales: 'shared'
      })
    ).toBe('shared');
  });

  it('otherwise the cell override wins, then the panel default, then shared', () => {
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: co({ c0: { scales: 'free' } }),
        panelScales: 'shared'
      })
    ).toBe('free');
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: undefined,
        panelScales: 'free'
      })
    ).toBe('free');
    expect(
      effectiveCellScale({
        cellKey: 'c0',
        shareForbidden: false,
        cellOverrides: undefined,
        panelScales: undefined
      })
    ).toBe('shared');
  });
});

describe('computeSharedCellKeys', () => {
  it('is empty when the axis does not apply or sharing is forbidden', () => {
    const units = [unit('a'), unit('b')];
    expect(
      computeSharedCellKeys({
        sharedAxisApplies: false,
        shareForbidden: false,
        units,
        cellOverrides: undefined,
        panelScales: 'shared'
      })
    ).toEqual({});
    expect(
      computeSharedCellKeys({
        sharedAxisApplies: true,
        shareForbidden: true,
        units,
        cellOverrides: undefined,
        panelScales: 'shared'
      })
    ).toEqual({});
  });

  it('includes only the cells whose effective scale is shared', () => {
    const units = [unit('a'), unit('b')];
    const out = computeSharedCellKeys({
      sharedAxisApplies: true,
      shareForbidden: false,
      units,
      cellOverrides: { b: { scales: 'free' } } as Panel['cellOverrides'],
      panelScales: 'shared'
    });
    expect(out).toEqual({ a: true }); // b freed by its override
  });
});

describe('unionExtent / computeSharedDomains', () => {
  const extents = {
    'a|value': [0, 10] as readonly [number, number],
    'b|value': [5, 20] as readonly [number, number],
    'c|value': [-5, 100] as readonly [number, number], // freed cell, excluded
    'a|x': [1, 2] as readonly [number, number]
  };
  const sharedKeys = { a: true as const, b: true as const };

  it('unions only the shared cells on the requested axis', () => {
    expect(unionExtent(extents, sharedKeys, 'value')).toEqual([0, 20]); // c excluded
    expect(unionExtent(extents, sharedKeys, 'x')).toEqual([1, 2]);
    expect(unionExtent({}, sharedKeys, 'value')).toBeUndefined();
  });

  it('computeSharedDomains is undefined unless computeShared, then unions per axis', () => {
    expect(
      computeSharedDomains({
        computeShared: false,
        reportedExtents: extents,
        sharedCellKeys: sharedKeys
      })
    ).toBeUndefined();
    expect(
      computeSharedDomains({
        computeShared: true,
        reportedExtents: extents,
        sharedCellKeys: sharedKeys
      })
    ).toEqual({ value: [0, 20], x: [1, 2] });
  });
});
