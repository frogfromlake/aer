import { describe, it, expect } from 'vitest';
import {
  computeWindowBounds,
  toDateWindow,
  toWindowIso,
  isScopeAvailable,
  missingSourcesFor,
  offerableMetadataFields,
  buildMetadataFields,
  firstMetadataField,
  buildScalarMetricOptions,
  computeTopNMax,
  reconcilePanelForView,
  type ScopeGate
} from '../../src/lib/workbench/panel-controls-derive';
import type { Panel, Presentation } from '../../src/lib/state/url-internals';
import type { PresentationDefinition } from '../../src/lib/presentations';

const DAY = 24 * 60 * 60 * 1000;
const NOW = new Date('2026-06-15T12:00:00.000Z').getTime();

describe('computeWindowBounds', () => {
  const base = { isEpisteme: false, now: NOW, lookbackMs: 7 * DAY };

  it('prefers the panel override over the global url window', () => {
    const b = computeWindowBounds({
      ...base,
      panelStart: '2026-01-01T00:00:00.000Z',
      panelEnd: '2026-01-02T00:00:00.000Z',
      urlFrom: '2020-01-01T00:00:00.000Z',
      urlTo: '2020-02-01T00:00:00.000Z'
    });
    expect(b.startMs).toBe(Date.parse('2026-01-01T00:00:00.000Z'));
    expect(b.endMs).toBe(Date.parse('2026-01-02T00:00:00.000Z'));
    expect(b.isPanelOverride).toBe(true);
  });

  it('falls back to the global url window when the panel has no override', () => {
    const b = computeWindowBounds({
      ...base,
      panelStart: undefined,
      panelEnd: undefined,
      urlFrom: '2026-03-01T00:00:00.000Z',
      urlTo: null
    });
    expect(b.startMs).toBe(Date.parse('2026-03-01T00:00:00.000Z'));
    expect(b.endMs).toBeUndefined(); // unbounded end on a non-episteme pillar
    expect(b.isPanelOverride).toBe(false);
  });

  it('Episteme defaults an unbounded window to [now-lookback, now]; others stay undefined', () => {
    const epi = computeWindowBounds({
      ...base,
      isEpisteme: true,
      panelStart: undefined,
      panelEnd: undefined,
      urlFrom: null,
      urlTo: null
    });
    expect(epi.startMs).toBe(NOW - 7 * DAY);
    expect(epi.endMs).toBe(NOW);

    const aleph = computeWindowBounds({
      ...base,
      panelStart: undefined,
      panelEnd: undefined,
      urlFrom: null,
      urlTo: null
    });
    expect(aleph.startMs).toBeUndefined();
    expect(aleph.endMs).toBeUndefined();
  });

  it('treats an unparseable bound as unbounded', () => {
    const b = computeWindowBounds({
      ...base,
      panelStart: 'not-a-date',
      panelEnd: undefined,
      urlFrom: null,
      urlTo: null
    });
    expect(b.startMs).toBeUndefined();
    // panelStart was set (even if invalid) → still counts as an override.
    expect(b.isPanelOverride).toBe(true);
  });
});

describe('toDateWindow / toWindowIso', () => {
  const bounds = {
    startMs: Date.parse('2026-01-01T08:30:00.000Z'),
    endMs: Date.parse('2026-01-05T20:00:00.000Z'),
    isPanelOverride: true
  };

  it('toDateWindow renders YYYY-MM-DD and carries the override flag', () => {
    expect(toDateWindow(bounds)).toEqual({
      startDate: '2026-01-01',
      endDate: '2026-01-05',
      isPanelOverride: true
    });
  });

  it('toWindowIso renders full RFC 3339; undefined bounds stay undefined', () => {
    expect(toWindowIso(bounds)).toEqual({
      start: '2026-01-01T08:30:00.000Z',
      end: '2026-01-05T20:00:00.000Z'
    });
    expect(toWindowIso({ startMs: undefined, endMs: undefined, isPanelOverride: false })).toEqual({
      start: undefined,
      end: undefined
    });
  });
});

describe('isScopeAvailable', () => {
  const gate = (over: Partial<ScopeGate> = {}): ScopeGate => ({
    scopeAvailableSet: new Set(['a', 'b']),
    partialMetricSet: new Set(['p']),
    showWithheld: false,
    ...over
  });

  it('allows everything when there is no scope constraint (null set)', () => {
    expect(isScopeAvailable('anything', gate({ scopeAvailableSet: null }))).toBe(true);
  });

  it('allows intersection metrics, blocks others', () => {
    expect(isScopeAvailable('a', gate())).toBe(true);
    expect(isScopeAvailable('z', gate())).toBe(false);
  });

  it('allows a partial metric only under "show anyway"', () => {
    expect(isScopeAvailable('p', gate({ showWithheld: false }))).toBe(false);
    expect(isScopeAvailable('p', gate({ showWithheld: true }))).toBe(true);
  });
});

describe('missingSourcesFor', () => {
  it('names the scoped sources that lack the metric', () => {
    expect(missingSourcesFor(['s1', 's3'], ['s1', 's2', 's3', 's4'])).toEqual(['s2', 's4']);
  });
});

describe('offerableMetadataFields / buildMetadataFields / firstMetadataField', () => {
  const partial = [{ field: 'paywall' }, { field: 'author' }];

  it('offers the intersection (sorted, deduped); partials only under show-anyway', () => {
    expect(
      offerableMetadataFields({
        availableMetadataFields: ['section', 'author', 'section'],
        partialMetadataFields: partial,
        showWithheld: false
      })
    ).toEqual(['author', 'section']);

    expect(
      offerableMetadataFields({
        availableMetadataFields: ['section'],
        partialMetadataFields: partial,
        showWithheld: true
      })
    ).toEqual(['author', 'paywall', 'section']);
  });

  it('buildMetadataFields is empty for non-field views and surfaces the active field', () => {
    expect(
      buildMetadataFields({ viewUsesMetadataField: false, offerable: ['a'], activeMetric: 'a' })
    ).toEqual([]);
    expect(
      buildMetadataFields({
        viewUsesMetadataField: true,
        offerable: ['section'],
        activeMetric: 'author'
      })
    ).toEqual(['author', 'section']);
  });

  it('firstMetadataField is the sorted-first field, else empty string', () => {
    expect(firstMetadataField(['section', 'author'])).toBe('author');
    expect(firstMetadataField([])).toBe('');
  });
});

describe('buildScalarMetricOptions', () => {
  it('applies the scope gate and surfaces currently-bound channel metrics', () => {
    // Gate excludes the default + every name except a/b, but a bound channel
    // metric (z) must still appear so the select reflects the binding.
    const out = buildScalarMetricOptions({
      availableMetricNames: ['a', 'b', 'c'],
      gate: {
        scopeAvailableSet: new Set(['a', 'b']),
        partialMetricSet: new Set(),
        showWithheld: false
      },
      activeChannels: { x: 'a', y: 'z' }
    });
    expect(out).toContain('a');
    expect(out).toContain('b');
    expect(out).not.toContain('c'); // gated out
    expect(out).toContain('z'); // bound channel surfaced despite the gate
  });
});

describe('computeTopNMax', () => {
  it('co-occurrence 6000, categorical 200, else 500', () => {
    expect(computeTopNMax({ isCooccurrenceView: true, viewUsesMetadataField: false })).toBe(6000);
    expect(computeTopNMax({ isCooccurrenceView: false, viewUsesMetadataField: true })).toBe(200);
    expect(computeTopNMax({ isCooccurrenceView: false, viewUsesMetadataField: false })).toBe(500);
  });
});

describe('reconcilePanelForView', () => {
  // Minimal panel + presentation-definition stubs so the seeding/reconcile logic
  // is exercised deterministically (no registry dependency: every stub sets
  // usesMetric:false so the metric→metric branch — which calls the real
  // metricSupportsPresentation — is skipped except where a test opts in).
  const panel = (over: Partial<Panel> = {}): Panel =>
    ({
      scopes: [],
      composition: 'merged',
      view: 'distribution',
      metric: 'sentiment_score',
      layer: 'gold',
      ...over
    }) as unknown as Panel;

  const pres = (over: Record<string, unknown>): PresentationDefinition =>
    ({ usesMetric: false, ...over }) as unknown as PresentationDefinition;

  const ctx = (presDefs: PresentationDefinition[], over = {}) => ({
    presentations: presDefs,
    prevUsesMetadataField: false,
    scalarMetricOptions: ['sentiment_score_bert_multilingual', 'word_count', 'entity_count'],
    offerableFields: ['author', 'paywall', 'section', 'kicker'],
    availableMetricNames: ['sentiment_score', 'word_count'],
    availableMetadataFields: ['section', 'author'],
    ...over
  });

  it('sets the new view and discards presentation-specific overrides + stale lists', () => {
    const p = panel({
      cellOverrides: { c0: {} },
      metricSet: ['a', 'b'],
      fieldChain: ['x'],
      facetField: 'section'
    } as Partial<Panel>);
    const next = reconcilePanelForView(
      p,
      'distribution' as Presentation,
      ctx([pres({ id: 'distribution' })])
    );
    expect(next.view).toBe('distribution');
    expect(next.cellOverrides).toBeUndefined();
    expect(next.metricSet).toBeUndefined(); // not in configurableParams → dropped
    expect(next.fieldChain).toBeUndefined();
    expect(next.facetField).toBeUndefined(); // supportsFaceting falsy → dropped
  });

  it('seeds metric_scatter axes: sentiment on X, word_count on Y', () => {
    const next = reconcilePanelForView(
      panel(),
      'metric_scatter' as Presentation,
      ctx([pres({ id: 'metric_scatter' })])
    );
    expect(next.channels).toEqual({ x: 'sentiment_score_bert_multilingual', y: 'word_count' });
  });

  it('seeds the N-metric set (first up-to-4 scalars) for a metricSet view', () => {
    const next = reconcilePanelForView(
      panel(),
      'correlation_matrix' as Presentation,
      ctx([pres({ id: 'correlation_matrix', configurableParams: ['metricSet'] })])
    );
    expect(next.metricSet).toEqual([
      'sentiment_score_bert_multilingual',
      'word_count',
      'entity_count'
    ]);
  });

  it('seeds the Sankey field chain (first up-to-3 offerable fields)', () => {
    const next = reconcilePanelForView(
      panel(),
      'sankey' as Presentation,
      ctx([pres({ id: 'sankey', configurableParams: ['sankeyFields'] })])
    );
    expect(next.fieldChain).toEqual(['author', 'paywall', 'section']);
  });

  it('falls a no-op overlay composition back to split', () => {
    const next = reconcilePanelForView(
      panel({ composition: 'overlay' }),
      'distribution' as Presentation,
      ctx([pres({ id: 'distribution', supportsOverlay: false })])
    );
    expect(next.composition).toBe('split');
  });

  it('seeds the first intersection field when switching into a field-driven view', () => {
    const next = reconcilePanelForView(
      panel(),
      'categorical_distribution' as Presentation,
      ctx([pres({ id: 'categorical_distribution', usesMetadataField: true })])
    );
    expect(next.metric).toBe('author'); // firstMetadataField(['section','author']) sorted
  });
});
