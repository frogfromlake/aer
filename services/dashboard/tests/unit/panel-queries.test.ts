import { describe, expect, it } from 'vitest';

import {
  coOccurrencePostDescriptorForPanel,
  defaultMetricForScopes,
  expandProbeScopeFanout,
  resolveCellConfig,
  selectCellRender,
  shouldRefuseMergedCrossProbe
} from '../../src/lib/workbench/panel-queries';
import { CROSS_PROBE_DEFAULT_METRIC } from '../../src/lib/viewmodes';
import type { Panel, ScopeGroup } from '../../src/lib/state/url-internals';

function panel(overrides: Partial<Panel> = {}): Panel {
  return {
    scopes: [{ probeIds: ['probe-0'], sourceIds: [] }],
    composition: 'merged',
    view: 'time_series',
    metric: 'sentiment_score_sentiws',
    layer: 'gold',
    ...overrides
  };
}

function group(probeIds: string[], sourceIds: string[] = []): ScopeGroup {
  return { probeIds, sourceIds };
}

describe('selectCellRender', () => {
  it('merged + single group → merged-single (probe scope)', () => {
    const r = selectCellRender(panel({ composition: 'merged', scopes: [group(['probe-0'])] }));
    expect(r.strategy).toBe('merged-single');
    expect(r.units).toHaveLength(1);
    expect(r.units[0]!.scope).toBe('probe');
    expect(r.units[0]!.scopeId).toBe('probe-0');
  });

  it('merged + single group with one source → source scope', () => {
    const r = selectCellRender(
      panel({
        composition: 'merged',
        scopes: [group(['probe-0'], ['tagesschau'])]
      })
    );
    expect(r.strategy).toBe('merged-single');
    expect(r.units[0]!.scope).toBe('source');
    expect(r.units[0]!.scopeId).toBe('tagesschau');
  });

  it('merged + multiple groups → one Cell PER ScopeGroup (Phase 122k §11)', () => {
    // Phase 122k §11 finding — merged composition with multi-group keeps
    // each group as its own cell; the merge is INSIDE the group (sources
    // unioned). The groups themselves render side-by-side.
    const r = selectCellRender(
      panel({
        composition: 'merged',
        scopes: [group(['probe-0'], ['tagesschau']), group(['probe-0'], ['bundesregierung'])]
      })
    );
    expect(r.strategy).toBe('merged-multi');
    expect(r.units).toHaveLength(2);
    expect(r.units[0]!.groupIndex).toBe(0);
    expect(r.units[0]!.sourceIds).toEqual(['tagesschau']);
    expect(r.units[1]!.groupIndex).toBe(1);
    expect(r.units[1]!.sourceIds).toEqual(['bundesregierung']);
  });

  it('split + single group with one source → split with one cell', () => {
    // The user explicitly chose split composition; with a single source
    // the result is one Cell rendered under the split strategy. Hosts
    // iterate `{#each units}` and produce one Cell either way.
    const r = selectCellRender(
      panel({ composition: 'split', scopes: [group(['probe-0'], ['tagesschau'])] })
    );
    expect(r.strategy).toBe('split');
    expect(r.units).toHaveLength(1);
    expect(r.units[0]!.scope).toBe('source');
    expect(r.units[0]!.scopeId).toBe('tagesschau');
  });

  it('split + single group with N sources → N cells, one per source', () => {
    const r = selectCellRender(
      panel({
        composition: 'split',
        scopes: [group(['probe-0'], ['tagesschau', 'bundesregierung', 'spiegel'])]
      })
    );
    expect(r.strategy).toBe('split');
    expect(r.units).toHaveLength(3);
    expect(r.units.map((u) => u.scopeId)).toEqual(['tagesschau', 'bundesregierung', 'spiegel']);
    for (const u of r.units) {
      expect(u.scope).toBe('source');
    }
  });

  it('split + N groups → N cells, one per group', () => {
    const r = selectCellRender(
      panel({
        composition: 'split',
        scopes: [group(['probe-0'], ['tagesschau']), group(['probe-0'], ['bundesregierung'])]
      })
    );
    expect(r.strategy).toBe('split');
    expect(r.units).toHaveLength(2);
  });

  it('split + single group without source narrowing → merged-single (shell fans out)', () => {
    // The Panel host cannot enumerate the probe's sources from URL state
    // alone; the Pillar Shell, which has dossier data, handles per-source
    // iteration. We surface this case as merged-single so the shell can
    // decide.
    const r = selectCellRender(panel({ composition: 'split', scopes: [group(['probe-0'], [])] }));
    expect(r.strategy).toBe('merged-single');
    expect(r.units).toHaveLength(1);
    expect(r.units[0]!.sourceIds).toEqual([]);
  });
});

describe('coOccurrencePostDescriptorForPanel', () => {
  it('returns null for non-cooccurrence views', () => {
    expect(
      coOccurrencePostDescriptorForPanel(
        panel({
          view: 'time_series',
          composition: 'merged',
          scopes: [group(['probe-0']), group(['probe-1'])]
        })
      )
    ).toBeNull();
  });

  it('returns null for split composition (each group queries singly)', () => {
    expect(
      coOccurrencePostDescriptorForPanel(
        panel({
          view: 'cooccurrence_network',
          composition: 'split',
          scopes: [group(['probe-0']), group(['probe-1'])]
        })
      )
    ).toBeNull();
  });

  it('returns null for merged + single group (legacy GET endpoint handles it)', () => {
    expect(
      coOccurrencePostDescriptorForPanel(
        panel({ view: 'cooccurrence_network', composition: 'merged', scopes: [group(['probe-0'])] })
      )
    ).toBeNull();
  });

  it('returns a POST descriptor for cooccurrence + merged + multiple groups', () => {
    const d = coOccurrencePostDescriptorForPanel(
      panel({
        view: 'cooccurrence_network',
        composition: 'merged',
        scopes: [group(['probe-0'], ['tagesschau']), group(['probe-0'], ['bundesregierung'])],
        topN: 25
      })
    );
    expect(d).not.toBeNull();
    expect(d!.scopes).toHaveLength(2);
    expect(d!.scopes[0]).toEqual({ probeIds: ['probe-0'], sourceIds: ['tagesschau'] });
    expect(d!.scopes[1]).toEqual({ probeIds: ['probe-0'], sourceIds: ['bundesregierung'] });
    expect(d!.topN).toBe(25);
  });

  it('omits topN when the panel does not specify one', () => {
    const d = coOccurrencePostDescriptorForPanel(
      panel({
        view: 'cooccurrence_network',
        composition: 'merged',
        scopes: [group(['probe-0']), group(['probe-1'])]
      })
    );
    expect(d).not.toBeNull();
    expect(d!.topN).toBeUndefined();
  });
});

describe('shouldRefuseMergedCrossProbe (Phase 130 / ADR-035 — Brief §1.3)', () => {
  it('refuses merged scaled metric pooling >1 probe in one Cell', () => {
    // single ScopeGroup with two probes, merged → one Cell pools both
    expect(
      shouldRefuseMergedCrossProbe(
        panel({
          composition: 'merged',
          view: 'distribution',
          metric: 'sentiment_score_sentiws',
          scopes: [group(['probe-0', 'probe-1'])]
        })
      )
    ).toBe(true);
  });

  it('permits merged when the metric is a pure count', () => {
    expect(
      shouldRefuseMergedCrossProbe(
        panel({
          composition: 'merged',
          view: 'distribution',
          metric: 'word_count',
          scopes: [group(['probe-0', 'probe-1'])]
        })
      )
    ).toBe(false);
  });

  it('permits split / overlay cross-probe (each probe keeps its own axis)', () => {
    const scaledTwoProbes = {
      view: 'distribution' as const,
      metric: 'sentiment_score_sentiws',
      scopes: [group(['probe-0']), group(['probe-1'])]
    };
    expect(shouldRefuseMergedCrossProbe(panel({ composition: 'split', ...scaledTwoProbes }))).toBe(
      false
    );
    expect(
      shouldRefuseMergedCrossProbe(panel({ composition: 'overlay', ...scaledTwoProbes }))
    ).toBe(false);
  });

  it('does not refuse merged single-probe scope', () => {
    expect(
      shouldRefuseMergedCrossProbe(
        panel({
          composition: 'merged',
          view: 'distribution',
          metric: 'sentiment_score_sentiws',
          scopes: [group(['probe-0'])]
        })
      )
    ).toBe(false);
  });

  it('does not apply to metric-less presentations (cooccurrence handled by cross-language gate)', () => {
    expect(
      shouldRefuseMergedCrossProbe(
        panel({
          composition: 'merged',
          view: 'cooccurrence_network',
          metric: 'sentiment_score_sentiws',
          scopes: [group(['probe-0', 'probe-1'])]
        })
      )
    ).toBe(false);
  });

  it('treats multi-group merged (one Cell per group) as not cross-probe-in-one-axis', () => {
    // merged + multiple single-probe groups → merged-multi: each group is its
    // own Cell, so no single Cell pools >1 probe.
    expect(
      shouldRefuseMergedCrossProbe(
        panel({
          composition: 'merged',
          view: 'distribution',
          metric: 'sentiment_score_sentiws',
          scopes: [group(['probe-0']), group(['probe-1'])]
        })
      )
    ).toBe(false);
  });
});

describe('expandProbeScopeFanout (Phase 123c B — cross-probe split fan-out)', () => {
  const srcByProbe = (pid: string): readonly string[] =>
    ({
      'probe-0': ['tagesschau', 'bundesregierung'],
      'probe-1': ['franceinfo', 'elysee']
    })[pid] ?? [];

  it('fans out one unit per (probe, source) across ALL in-scope probes', () => {
    const p = panel({ composition: 'split', scopes: [group(['probe-0', 'probe-1'], [])] });
    const r = expandProbeScopeFanout(p, selectCellRender(p), srcByProbe, 'probe-0');
    expect(r).not.toBeNull();
    expect(r!.probeOrder).toEqual(['probe-0', 'probe-1']);
    expect(r!.units).toHaveLength(4);
    expect(r!.units.map((u) => u.scopeId)).toEqual([
      'tagesschau',
      'bundesregierung',
      'franceinfo',
      'elysee'
    ]);
    expect(r!.units.every((u) => u.scope === 'source')).toBe(true);
    // Multi-probe units are tagged with their originating probe (per-probe tint).
    expect(r!.units[0]!.probeId).toBe('probe-0');
    expect(r!.units[2]!.probeId).toBe('probe-1');
    // Each unit key is stable + distinct.
    expect(new Set(r!.units.map((u) => u.key)).size).toBe(4);
  });

  it('single-probe fan-out keeps the legacy untagged shape (no per-probe accent)', () => {
    const p = panel({ composition: 'split', scopes: [group(['probe-0'], [])] });
    const r = expandProbeScopeFanout(p, selectCellRender(p), srcByProbe, 'probe-0');
    expect(r!.probeOrder).toEqual(['probe-0']);
    expect(r!.units).toHaveLength(2);
    expect(r!.units[0]!.probeId).toBeUndefined();
  });

  it('falls back to the host probe id when the group has no probes', () => {
    const p = panel({ composition: 'split', scopes: [group([], [])] });
    const r = expandProbeScopeFanout(p, selectCellRender(p), srcByProbe, 'probe-0');
    expect(r!.probeOrder).toEqual(['probe-0']);
    expect(r!.units.map((u) => u.scopeId)).toEqual(['tagesschau', 'bundesregierung']);
  });

  it('returns null for shapes that are not a probe-scope split fan-out', () => {
    // merged composition
    expect(
      expandProbeScopeFanout(
        panel({ composition: 'merged', scopes: [group(['probe-0', 'probe-1'], [])] }),
        selectCellRender(panel({ composition: 'merged', scopes: [group(['probe-0', 'probe-1'])] })),
        srcByProbe,
        'probe-0'
      )
    ).toBeNull();
    // split but with explicit source narrowing (strategy = 'split', not 'merged-single')
    const sp = panel({ composition: 'split', scopes: [group(['probe-0'], ['tagesschau'])] });
    expect(expandProbeScopeFanout(sp, selectCellRender(sp), srcByProbe, 'probe-0')).toBeNull();
    // split but multiple scope groups
    const mg = panel({ composition: 'split', scopes: [group(['probe-0']), group(['probe-1'])] });
    expect(expandProbeScopeFanout(mg, selectCellRender(mg), srcByProbe, 'probe-0')).toBeNull();
  });
});

describe('defaultMetricForScopes', () => {
  // The default is ALWAYS the multilingual backbone — for single- and
  // cross-probe alike — so no probe (e.g. Probe 1, French) ever opens on a
  // German-only metric that renders an empty cell, and a single probe never
  // needs a runtime metric-reconcile to correct its default.
  it('single-probe scope → the multilingual backbone (never German-only SentiWS)', () => {
    expect(defaultMetricForScopes([group(['probe-0'], [])])).toBe(CROSS_PROBE_DEFAULT_METRIC);
    expect(defaultMetricForScopes([group(['probe-1'], [])])).toBe(CROSS_PROBE_DEFAULT_METRIC);
    expect(defaultMetricForScopes([group(['probe-0'], ['tagesschau'])])).toBe(
      CROSS_PROBE_DEFAULT_METRIC
    );
  });

  it('cross-probe scope → the multilingual backbone (so FR cells are not empty)', () => {
    expect(defaultMetricForScopes([group(['probe-0', 'probe-1'], [])])).toBe(
      CROSS_PROBE_DEFAULT_METRIC
    );
    expect(defaultMetricForScopes([group(['probe-0']), group(['probe-1'])])).toBe(
      CROSS_PROBE_DEFAULT_METRIC
    );
  });

  it('empty scope → the multilingual backbone', () => {
    expect(defaultMetricForScopes([])).toBe(CROSS_PROBE_DEFAULT_METRIC);
  });
});

// Phase 126 — stable cell keys + per-cell config resolution.
describe('Phase 126 — stable cell keys (override binding)', () => {
  it('split single-group keys cells by source NAME, not render index', () => {
    const r = selectCellRender(
      panel({ composition: 'split', scopes: [group(['probe-0'], ['tagesschau', 'taz'])] })
    );
    expect(r.units.map((u) => u.key)).toEqual(['s:tagesschau', 's:taz']);
  });

  it('a source key is unchanged when a sibling is inserted before it', () => {
    const before = selectCellRender(
      panel({ composition: 'split', scopes: [group(['probe-0'], ['taz'])] })
    );
    const after = selectCellRender(
      panel({ composition: 'split', scopes: [group(['probe-0'], ['tagesschau', 'taz'])] })
    );
    const tazBefore = before.units.find((u) => u.scopeId === 'taz')!.key;
    const tazAfter = after.units.find((u) => u.scopeId === 'taz')!.key;
    expect(tazAfter).toBe(tazBefore); // override on taz survives the reorder
  });

  it('probe-scope fan-out keys by probeId:name', () => {
    const p = panel({ composition: 'split', scopes: [group(['probe-0', 'probe-1'], [])] });
    const fan = expandProbeScopeFanout(
      p,
      selectCellRender(p),
      (pid) => (pid === 'probe-0' ? ['tagesschau'] : ['franceinfo']),
      'probe-0'
    );
    expect(fan?.units.map((u) => u.key)).toEqual(['probe-0:tagesschau', 'probe-1:franceinfo']);
  });
});

describe('resolveCellConfig (override-with-inheritance)', () => {
  it('inherits every panel default when the cell has no override', () => {
    const p = panel({ view: 'distribution', bins: 40, scales: 'free' });
    const cfg = resolveCellConfig(p, 's:tagesschau');
    expect(cfg.bins).toBe(40);
    expect(cfg.scales).toBe('free');
    expect(cfg.isOverridden).toBe(false);
  });

  it('override wins per-lever; non-overridden levers still inherit', () => {
    const p = panel({
      view: 'distribution',
      bins: 40,
      scales: 'shared',
      cellOverrides: { 's:taz': { bins: 90 } }
    });
    const overridden = resolveCellConfig(p, 's:taz');
    expect(overridden.bins).toBe(90); // override wins
    expect(overridden.scales).toBe('shared'); // inherited
    expect(overridden.isOverridden).toBe(true);

    const sibling = resolveCellConfig(p, 's:tagesschau');
    expect(sibling.bins).toBe(40); // sibling untouched
    expect(sibling.isOverridden).toBe(false);
  });

  it('a false/shared override is honoured (does not fall through to the default)', () => {
    const p = panel({
      view: 'time_series',
      showBand: true,
      scales: 'free',
      cellOverrides: { 's:taz': { showBand: false, scales: 'shared' } }
    });
    const cfg = resolveCellConfig(p, 's:taz');
    expect(cfg.showBand).toBe(false);
    expect(cfg.scales).toBe('shared');
  });

  it('merges channels one level deep (override one axis, inherit the rest)', () => {
    const p = panel({
      view: 'metric_scatter',
      channels: { x: 'word_count', y: 'sentiment_score_sentiws', size: 'entity_count' },
      cellOverrides: { 's:taz': { channels: { x: 'language_confidence' } } }
    });
    const cfg = resolveCellConfig(p, 's:taz');
    expect(cfg.channels).toEqual({
      x: 'language_confidence', // overridden
      y: 'sentiment_score_sentiws', // inherited
      size: 'entity_count' // inherited
    });
  });
});
