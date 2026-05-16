import { describe, expect, it } from 'vitest';

import {
  coOccurrencePostDescriptorForPanel,
  selectCellRender
} from '../../src/lib/workbench/panel-queries';
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

  it('merged + multiple groups → merged-multi with unioned probes/sources', () => {
    const r = selectCellRender(
      panel({
        composition: 'merged',
        scopes: [group(['probe-0'], ['tagesschau']), group(['probe-0'], ['bundesregierung'])]
      })
    );
    expect(r.strategy).toBe('merged-multi');
    expect(r.units).toHaveLength(1);
    expect(r.units[0]!.probeIds).toEqual(['probe-0']);
    expect(r.units[0]!.sourceIds).toEqual(['tagesschau', 'bundesregierung']);
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
