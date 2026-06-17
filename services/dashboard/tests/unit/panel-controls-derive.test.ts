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
  buildMetricList,
  firstMetricSupporting,
  resetMetricForScope,
  buildScalarMetricOptions,
  computeTopNMax,
  reconcilePanelForView,
  type ScopeGate
} from '../../src/lib/workbench/panel-controls-derive';
import { CROSS_PROBE_DEFAULT_METRIC, DEFAULT_METRIC_NAME } from '../../src/lib/presentations';
import type { Panel, Presentation, ScopeGroup } from '../../src/lib/state/url-internals';
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

describe('buildMetricList', () => {
  const openGate: ScopeGate = {
    scopeAvailableSet: null,
    partialMetricSet: new Set(),
    showWithheld: false
  };

  it('prepends the canonical default and filters through metric→presentation + scope gate', () => {
    const out = buildMetricList({
      view: 'distribution',
      availableMetricNames: ['word_count', 'publication_hour'],
      gate: openGate,
      activeMetric: ''
    });
    // distribution supports scalars + publication_hour; default leads.
    expect(out[0]).toBe(DEFAULT_METRIC_NAME);
    expect(out).toContain('word_count');
    expect(out).toContain('publication_hour');
  });

  it('drops a metric the active presentation cannot render', () => {
    const out = buildMetricList({
      view: 'time_series',
      availableMetricNames: ['publication_hour'], // distribution-only
      gate: openGate,
      activeMetric: ''
    });
    expect(out).not.toContain('publication_hour');
  });

  it('respects the scope gate (intersection only)', () => {
    const out = buildMetricList({
      view: 'distribution',
      availableMetricNames: ['word_count', 'entity_count'],
      gate: {
        scopeAvailableSet: new Set(['word_count']),
        partialMetricSet: new Set(),
        showWithheld: false
      },
      activeMetric: ''
    });
    expect(out).toContain('word_count');
    expect(out).not.toContain('entity_count');
    // The default is gated out too (not in the intersection).
    expect(out).not.toContain(DEFAULT_METRIC_NAME);
  });

  it('always surfaces a supported active metric even when the gate would drop it', () => {
    const out = buildMetricList({
      view: 'distribution',
      availableMetricNames: ['word_count'],
      gate: {
        scopeAvailableSet: new Set(['word_count']),
        partialMetricSet: new Set(),
        showWithheld: false
      },
      activeMetric: 'entity_count' // gated out, but active → surfaced
    });
    expect(out).toContain('entity_count');
  });

  it('does NOT surface an active metric the presentation cannot render', () => {
    const out = buildMetricList({
      view: 'time_series',
      availableMetricNames: [],
      gate: openGate,
      activeMetric: 'publication_hour' // distribution-only → not surfaced for time_series
    });
    expect(out).not.toContain('publication_hour');
  });
});

describe('firstMetricSupporting', () => {
  it('prefers the canonical default when it renders the view', () => {
    expect(firstMetricSupporting('distribution', ['word_count'])).toBe(DEFAULT_METRIC_NAME);
  });

  it('falls back to the first available metric the view supports', () => {
    // cooccurrence_network: the default (a scalar) does not support it, so it
    // scans the available list for entity_cooccurrence.
    expect(
      firstMetricSupporting('cooccurrence_network', ['word_count', 'entity_cooccurrence'])
    ).toBe('entity_cooccurrence');
  });

  it('falls back to the default when nothing in the list supports the view', () => {
    expect(firstMetricSupporting('cooccurrence_network', ['word_count'])).toBe(DEFAULT_METRIC_NAME);
  });
});

describe('resetMetricForScope', () => {
  const scopes: ScopeGroup[] = [{ probeIds: ['probe-0'], sourceIds: ['tagesschau'] }];

  it('returns the scope canonical default when it is scope-valid for the view', () => {
    expect(
      resetMetricForScope({
        view: 'distribution',
        scopeAvailableSet: new Set([CROSS_PROBE_DEFAULT_METRIC]),
        scopes,
        availableMetricNames: []
      })
    ).toBe(CROSS_PROBE_DEFAULT_METRIC);
  });

  it('falls back to the cross-probe backbone, then to an available sentiment metric', () => {
    // canonical (CROSS_PROBE_DEFAULT) is gated out; an available sentiment_score* wins.
    const out = resetMetricForScope({
      view: 'distribution',
      scopeAvailableSet: new Set(['sentiment_score_germansentiment']),
      scopes,
      availableMetricNames: ['sentiment_score_germansentiment', 'word_count']
    });
    expect(out).toBe('sentiment_score_germansentiment');
  });

  it('falls back to any available metric when no sentiment metric is offerable', () => {
    const out = resetMetricForScope({
      view: 'distribution',
      scopeAvailableSet: new Set(['word_count']),
      scopes,
      availableMetricNames: ['word_count']
    });
    expect(out).toBe('word_count');
  });

  it('returns the canonical as the last-resort when nothing is offerable', () => {
    const out = resetMetricForScope({
      view: 'distribution',
      scopeAvailableSet: new Set(['nope']),
      scopes,
      availableMetricNames: []
    });
    expect(out).toBe(CROSS_PROBE_DEFAULT_METRIC);
  });

  it('treats a null scope-available set as "everything offerable"', () => {
    const out = resetMetricForScope({
      view: 'distribution',
      scopeAvailableSet: null,
      scopes,
      availableMetricNames: []
    });
    expect(out).toBe(CROSS_PROBE_DEFAULT_METRIC); // canonical passes the null-set ok()
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

  it('seeds the cross-tab numeric metric (channels.x), preferring sentiment', () => {
    const next = reconcilePanelForView(
      panel(),
      'cross_tab' as Presentation,
      ctx([pres({ id: 'cross_tab', configurableParams: ['crossMetric'] })])
    );
    expect(next.channels?.x).toBe('sentiment_score_bert_multilingual');
  });

  it('seeds the lead-lag x/y to two distinct metrics', () => {
    const next = reconcilePanelForView(
      panel(),
      'metric_lead_lag' as Presentation,
      ctx([pres({ id: 'metric_lead_lag', configurableParams: ['leadLagAxes'] })])
    );
    expect(next.channels?.x).toBeDefined();
    expect(next.channels?.y).toBeDefined();
    expect(next.channels?.x).not.toBe(next.channels?.y);
  });

  it('reconciles metric → field when switching from a metric view into a field view', () => {
    // prevUsesMetadataField false → switching into a field view seeds the first field.
    const next = reconcilePanelForView(
      panel({ metric: 'word_count' }),
      'categorical_distribution' as Presentation,
      ctx([pres({ id: 'categorical_distribution', usesMetadataField: true })], {
        prevUsesMetadataField: false
      })
    );
    expect(next.metric).toBe('author');
  });

  it('reconciles field → metric: a metric view re-seeds when coming from a field view', () => {
    const next = reconcilePanelForView(
      panel({ metric: 'section' }), // a field name, not a metric
      'distribution' as Presentation,
      ctx([pres({ id: 'distribution', usesMetric: true })], { prevUsesMetadataField: true })
    );
    // firstMetricSupporting('distribution', ...) → canonical default.
    expect(next.metric).toBe('sentiment_score_sentiws');
  });

  it('keeps a still-valid metric when staying within metric views', () => {
    const next = reconcilePanelForView(
      panel({ metric: 'word_count' }),
      'distribution' as Presentation,
      ctx([pres({ id: 'distribution', usesMetric: true })], { prevUsesMetadataField: false })
    );
    expect(next.metric).toBe('word_count'); // word_count supports distribution → kept
  });
});
