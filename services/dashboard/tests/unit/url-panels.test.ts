import { describe, expect, it } from 'vitest';

import {
  EMPTY_URL_STATE,
  encodePillarState,
  decodePillarState,
  readFromSearch,
  writeToSearch,
  type Panel,
  type PillarState,
  type ScopeGroup,
  type UrlState,
  type WorkbenchWindow
} from '../../src/lib/state/url-internals';

// Phase 122i / ADR-034 — Multi-Panel Workbench URL state tests.

function state(overrides: Partial<UrlState> = {}): UrlState {
  return { ...EMPTY_URL_STATE, ...overrides };
}

function makeScopeGroup(p: string[] = ['probe-0'], s: string[] = []): ScopeGroup {
  return { probeIds: p, sourceIds: s };
}

function makePanel(overrides: Partial<Panel> = {}): Panel {
  return {
    scopes: [makeScopeGroup()],
    composition: 'merged',
    view: 'time_series',
    metric: 'sentiment_score_sentiws',
    layer: 'gold',
    ...overrides
  };
}

function makeWindow(panels: Panel[] = [makePanel()]): WorkbenchWindow {
  return { panels, focusedPanelIndex: 0 };
}

function makePillarState(windows: WorkbenchWindow[] = [makeWindow()]): PillarState {
  return { windows, activeWindowIndex: 0 };
}

describe('encodePillarState / decodePillarState', () => {
  it('round-trips a minimal single-window single-panel single-scope state', () => {
    const original = makePillarState();
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded).toEqual(original);
  });

  it('preserves optional Panel fields (resolution, normalization, topN, locked)', () => {
    const original = makePillarState([
      makeWindow([
        makePanel({
          resolution: 'daily',
          normalization: 'zscore',
          topN: 25,
          locked: true,
          lockedReason: 'df_entry',
          lockedFunction: 'epistemic_authority'
        })
      ])
    ]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded).toEqual(original);
  });

  it('round-trips multi-scope split composition across multiple panels', () => {
    const original = makePillarState([
      makeWindow([
        makePanel({
          composition: 'split',
          scopes: [makeScopeGroup(['probe-0'], ['src-a']), makeScopeGroup(['probe-0'], ['src-b'])]
        }),
        makePanel({
          composition: 'merged',
          view: 'distribution',
          metric: 'word_count',
          layer: 'silver'
        })
      ])
    ]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded).toEqual(original);
  });

  it('round-trips multi-window state with non-zero activeWindowIndex', () => {
    const original = makePillarState([makeWindow(), makeWindow()]);
    original.activeWindowIndex = 1;
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded).toEqual(original);
  });

  // Phase 122i revision (R2): splitDirection, cellControlsCollapsed,
  // maximizedPanelIndex round-trip.

  it('round-trips splitDirection=vertical (D2)', () => {
    const original = makePillarState([
      makeWindow([makePanel({ composition: 'split', splitDirection: 'vertical' })])
    ]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.panels[0]?.splitDirection).toBe('vertical');
  });

  it('round-trips splitDirection=horizontal (D2)', () => {
    const original = makePillarState([
      makeWindow([makePanel({ composition: 'split', splitDirection: 'horizontal' })])
    ]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.panels[0]?.splitDirection).toBe('horizontal');
  });

  it('omits splitDirection from encoded form when undefined (default horizontal)', () => {
    const original = makePillarState([makeWindow([makePanel()])]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.panels[0]?.splitDirection).toBeUndefined();
  });

  it('round-trips cellControlsCollapsed=true (C4)', () => {
    const original = makePillarState([makeWindow([makePanel({ cellControlsCollapsed: true })])]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.panels[0]?.cellControlsCollapsed).toBe(true);
  });

  it('omits cellControlsCollapsed when false/undefined (default)', () => {
    const original = makePillarState([makeWindow([makePanel()])]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.panels[0]?.cellControlsCollapsed).toBeUndefined();
  });

  it('round-trips maximizedPanelIndex on a multi-panel window (C3)', () => {
    const win = makeWindow([makePanel(), makePanel()]);
    win.maximizedPanelIndex = 1;
    const original = makePillarState([win]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBe(1);
  });

  it('omits maximizedPanelIndex when null/undefined', () => {
    const win = makeWindow([makePanel(), makePanel()]);
    // explicit null
    win.maximizedPanelIndex = null;
    const original = makePillarState([win]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBeUndefined();
  });

  it('drops out-of-bounds maximizedPanelIndex on encode (defensive)', () => {
    const win = makeWindow([makePanel()]); // only 1 panel
    win.maximizedPanelIndex = 5;
    const original = makePillarState([win]);
    const decoded = decodePillarState(encodePillarState(original));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBeUndefined();
  });

  it('rejects an out-of-bounds maximizedPanelIndex on decode (malformed URL)', () => {
    // Hand-craft a payload with mp=5 in a 1-panel window.
    const compact = {
      w: [
        {
          p: [
            {
              s: [{ pi: ['probe-0'], si: [] }],
              c: 'm',
              v: 'time_series',
              m: 'sentiment_score_sentiws',
              l: 'g'
            }
          ],
          fi: 0,
          mp: 5
        }
      ],
      aw: 0
    };
    const encoded = btoa(JSON.stringify(compact))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    expect(decodePillarState(encoded)).toBeNull();
  });

  it('produces URL-safe base64 (no +, /, or =)', () => {
    const encoded = encodePillarState(makePillarState());
    expect(encoded).not.toMatch(/[+/=]/);
  });

  it('keeps the encoded payload reasonably short for a typical state (< 512 bytes)', () => {
    const encoded = encodePillarState(makePillarState());
    expect(encoded.length).toBeLessThan(512);
  });

  it('returns null on malformed base64', () => {
    expect(decodePillarState('!@#$')).toBeNull();
  });

  it('returns null on non-JSON payload', () => {
    // Encode plain "hello" — valid base64 but not JSON.
    const garbage = btoa('hello').replace(/=+$/, '');
    expect(decodePillarState(garbage)).toBeNull();
  });

  it('returns null on JSON missing required panel fields', () => {
    const invalid = btoa(JSON.stringify({ w: [{ p: [{ s: [] }], fi: 0 }], aw: 0 }))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    expect(decodePillarState(invalid)).toBeNull();
  });

  it('returns null when activeWindowIndex is out of bounds', () => {
    const invalid = btoa(
      JSON.stringify({
        w: [
          {
            p: [
              {
                s: [{ pi: ['probe-0'], si: [] }],
                c: 'm',
                v: 'time_series',
                m: 'sentiment_score_sentiws',
                l: 'g'
              }
            ],
            fi: 0
          }
        ],
        aw: 5
      })
    )
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    expect(decodePillarState(invalid)).toBeNull();
  });

  it('rejects invalid view enum values', () => {
    const invalid = btoa(
      JSON.stringify({
        w: [
          {
            p: [
              {
                s: [{ pi: ['probe-0'], si: [] }],
                c: 'm',
                v: 'scatter_plot',
                m: 'sentiment_score_sentiws',
                l: 'g'
              }
            ],
            fi: 0
          }
        ],
        aw: 0
      })
    )
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    expect(decodePillarState(invalid)).toBeNull();
  });
});

describe('readFromSearch — pillar form (Phase 122i)', () => {
  it('parses ?activePillar=aleph&aleph=<encoded> into pillars.aleph', () => {
    const pillarState = makePillarState();
    const encoded = encodePillarState(pillarState);
    const search = `?activePillar=aleph&aleph=${encoded}`;
    const parsed = readFromSearch(search);
    expect(parsed.activePillar).toBe('aleph');
    expect(parsed.pillars?.aleph).toEqual(pillarState);
    expect(parsed.pillars?.episteme).toBeNull();
    expect(parsed.pillars?.rhizome).toBeNull();
  });

  it('preserves all three pillar states when present together', () => {
    const aleph = makePillarState();
    const episteme = makePillarState([makeWindow([makePanel({ view: 'topic_distribution' })])]);
    const rhizome = makePillarState([makeWindow([makePanel({ view: 'cooccurrence_network' })])]);
    const search =
      `?activePillar=episteme&aleph=${encodePillarState(aleph)}` +
      `&episteme=${encodePillarState(episteme)}` +
      `&rhizome=${encodePillarState(rhizome)}`;
    const parsed = readFromSearch(search);
    expect(parsed.activePillar).toBe('episteme');
    expect(parsed.pillars?.aleph).toEqual(aleph);
    expect(parsed.pillars?.episteme).toEqual(episteme);
    expect(parsed.pillars?.rhizome).toEqual(rhizome);
  });

  it('ignores legacy Phase-122h flat params (retired in 122k)', () => {
    const encoded = encodePillarState(makePillarState());
    const parsed = readFromSearch(`?aleph=${encoded}&probeId=probe-X&sourceId=src-Y`);
    // Legacy fields no longer exist on UrlState; the pillar form alone
    // survives.
    expect(parsed.pillars?.aleph).toBeTruthy();
    expect(parsed.selectedProbes).toEqual([]);
  });

  it('ignores invalid pillar payloads (sets the slot to null)', () => {
    const parsed = readFromSearch('?aleph=invalid-base64-!@#');
    // When any pillar key is present (even malformed), the pillars
    // wrapper is populated with the failing slot set to null.
    expect(parsed.pillars).not.toBeNull();
    expect(parsed.pillars?.aleph).toBeNull();
  });
});

describe('writeToSearch — pillar form (Phase 122i)', () => {
  it('emits multi-panel form when pillars is non-null', () => {
    const aleph = makePillarState();
    const qs = writeToSearch(
      state({ activePillar: 'aleph', pillars: { aleph, episteme: null, rhizome: null } })
    );
    expect(qs).toContain('activePillar=aleph');
    expect(qs).toContain('aleph=');
    expect(qs).not.toContain('episteme=');
    expect(qs).not.toContain('rhizome=');
  });

  it('writes only the canonical Phase-122k grammar (no legacy flat params)', () => {
    const aleph = makePillarState();
    const qs = writeToSearch(
      state({
        activePillar: 'aleph',
        pillars: { aleph, episteme: null, rhizome: null }
      })
    );
    // Legacy flat params are no longer expressible on UrlState. We only
    // verify here that the writer produces the canonical grammar.
    expect(qs).not.toContain('probeId=');
    expect(qs).not.toContain('sourceId=');
    expect(qs).not.toContain('viewingMode=');
    expect(qs).not.toContain('viewMode=');
    expect(qs).not.toContain('metric=');
    expect(qs).not.toContain('layer=');
    expect(qs).toContain('activePillar=aleph');
    expect(qs).toContain('aleph=');
  });

  it('round-trips a multi-pillar state through write → read', () => {
    const aleph = makePillarState([
      makeWindow([
        makePanel({
          composition: 'split',
          scopes: [makeScopeGroup(['probe-0'], ['src-a']), makeScopeGroup(['probe-0'], ['src-b'])]
        })
      ])
    ]);
    const episteme = makePillarState([
      makeWindow([makePanel({ view: 'topic_distribution', resolution: 'daily', topN: 15 })])
    ]);
    const original = state({
      activePillar: 'aleph',
      pillars: { aleph, episteme, rhizome: null },
      from: '2026-05-01T00:00:00.000Z',
      to: '2026-05-08T00:00:00.000Z',
      resolution: 'hourly'
    });
    const parsed = readFromSearch(writeToSearch(original));
    expect(parsed.activePillar).toBe('aleph');
    expect(parsed.pillars?.aleph).toEqual(aleph);
    expect(parsed.pillars?.episteme).toEqual(episteme);
    expect(parsed.pillars?.rhizome).toBeNull();
    expect(parsed.from).toBe(original.from);
    expect(parsed.to).toBe(original.to);
    expect(parsed.resolution).toBe('hourly');
  });

  it('preserves negSpace and normalization alongside pillar state', () => {
    const aleph = makePillarState();
    const qs = writeToSearch(
      state({
        activePillar: 'aleph',
        pillars: { aleph, episteme: null, rhizome: null },
        negSpace: true,
        normalization: 'zscore'
      })
    );
    expect(qs).toContain('negSpace=1');
    expect(qs).toContain('normalization=zscore');
  });
});
