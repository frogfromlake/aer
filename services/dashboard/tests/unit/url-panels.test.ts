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

// ── Fixture builders ─────────────────────────────────────────────────────────

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

/** Encode a hand-crafted compact payload into the URL-safe base64 the decoder
 *  expects — used to drive the malformed/legacy decode paths. */
function encodeCompact(compact: unknown): string {
  return btoa(JSON.stringify(compact)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

/** A minimal valid compact panel, spread into hand-crafted payloads. */
const COMPACT_PANEL = {
  s: [{ pi: ['probe-0'], si: [] }],
  c: 'm',
  v: 'time_series',
  m: 'sentiment_score_sentiws',
  l: 'g'
};

/** Round-trip a single-panel pillar state through encode → decode. */
function rt(panel: Partial<Panel>): PillarState | null {
  return decodePillarState(encodePillarState(makePillarState([makeWindow([makePanel(panel)])])));
}

function firstPanel(s: PillarState | null): Panel | undefined {
  return s?.windows[0]?.panels[0];
}

// ── encode/decode round-trips that must be lossless (deep-equal) ──────────────

describe('encodePillarState / decodePillarState — lossless round-trips', () => {
  const lossless: Array<[string, PillarState]> = [
    ['minimal single-window single-panel single-scope', makePillarState()],
    [
      'Phase-131 per-cell config (bins, channels, showBand=false)',
      makePillarState([
        makeWindow([
          makePanel({
            view: 'metric_scatter',
            bins: 64,
            showBand: false,
            forceStrength: 75,
            channels: {
              x: 'word_count',
              y: 'sentiment_score_sentiws',
              size: 'entity_count',
              color: 'language_confidence',
              netSize: 'degree',
              netColor: 'presence'
            }
          })
        ])
      ])
    ],
    [
      'Phase-123b displayLanguage=viewer',
      makePillarState([
        makeWindow([makePanel({ view: 'cooccurrence_network', displayLanguage: 'viewer' })])
      ])
    ],
    [
      'sankey fieldChain + correlation metricSet independently',
      makePillarState([
        makeWindow([
          makePanel({ view: 'sankey', fieldChain: ['article_type', 'section'] }),
          makePanel({ view: 'correlation_matrix', metricSet: ['word_count', 'entity_count'] })
        ])
      ])
    ],
    [
      'Phase-126 per-cell overrides (scalar, channels, both enum sides)',
      makePillarState([
        makeWindow([
          makePanel({
            view: 'metric_scatter',
            bins: 30,
            scales: 'free',
            channels: { x: 'word_count', y: 'sentiment_score_sentiws' },
            cellOverrides: {
              's:tagesschau': {
                bins: 80,
                topN: 120,
                forceStrength: 90,
                showBand: false,
                scales: 'shared',
                displayLanguage: 'viewer',
                channels: { x: 'entity_count' }
              },
              'probe-0:bundesregierung': { showBand: true, displayLanguage: 'source' }
            }
          })
        ])
      ])
    ],
    [
      'per-cell dimension peek override (ADR-038)',
      makePillarState([
        makeWindow([
          makePanel({
            view: 'categorical_distribution',
            metric: 'author',
            cellOverrides: { 's:tagesschau': { metric: 'section' } }
          })
        ])
      ])
    ],
    [
      'optional Panel fields (resolution, normalization, topN, locked)',
      makePillarState([
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
      ])
    ],
    [
      'multi-scope split composition across multiple panels',
      makePillarState([
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
      ])
    ]
  ];

  it.each(lossless)('round-trips %s', (_label, original) => {
    expect(decodePillarState(encodePillarState(original))).toEqual(original);
  });

  it('round-trips multi-window state with non-zero activeWindowIndex', () => {
    const original = makePillarState([makeWindow(), makeWindow()]);
    original.activeWindowIndex = 1;
    expect(decodePillarState(encodePillarState(original))).toEqual(original);
  });
});

// ── Default-omission: optional fields at their default drop out of the URL ────

describe('encodePillarState — drops defaults so the URL stays clean', () => {
  const dropped: Array<[string, Partial<Panel>, (p: Panel | undefined) => unknown]> = [
    ['showBand=true (default)', { showBand: true }, (p) => p?.showBand],
    [
      'displayLanguage=source (default)',
      { view: 'cooccurrence_network', displayLanguage: 'source' },
      (p) => p?.displayLanguage
    ],
    ['empty channels object', { channels: {} }, (p) => p?.channels],
    ['no cellOverrides', {}, (p) => p?.cellOverrides],
    [
      'all-empty cellOverride map',
      { cellOverrides: { 's:a': {}, 's:b': {} } },
      (p) => p?.cellOverrides
    ],
    ['splitDirection (default horizontal)', {}, (p) => p?.splitDirection],
    ['cellControlsCollapsed=false', {}, (p) => p?.cellControlsCollapsed],
    ['showWithheld=false', {}, (p) => p?.showWithheld]
  ];

  it.each(dropped)('omits %s', (_label, panel, read) => {
    expect(read(firstPanel(rt(panel)))).toBeUndefined();
  });
});

// ── Enum/boolean lever round-trips that read back a single field ──────────────

describe('encodePillarState — preserves individual levers', () => {
  const preserved: Array<[string, Partial<Panel>, (p: Panel | undefined) => unknown, unknown]> = [
    [
      'splitDirection=vertical',
      { composition: 'split', splitDirection: 'vertical' },
      (p) => p?.splitDirection,
      'vertical'
    ],
    [
      'splitDirection=horizontal',
      { composition: 'split', splitDirection: 'horizontal' },
      (p) => p?.splitDirection,
      'horizontal'
    ],
    [
      'cellControlsCollapsed=true',
      { cellControlsCollapsed: true },
      (p) => p?.cellControlsCollapsed,
      true
    ],
    ['showWithheld=true', { showWithheld: true }, (p) => p?.showWithheld, true],
    // Phase 149 — human panel caption (pn) round-trips and rides in the analysis state.
    ['label', { label: 'FR vs DE sentiment' }, (p) => p?.label, 'FR vs DE sentiment'],
    // Phase 148g — provenance-border mode (pv) round-trips for each non-default value.
    [
      'provenanceBorder=source',
      { view: 'cooccurrence_network', provenanceBorder: 'source' },
      (p) => p?.provenanceBorder,
      'source'
    ],
    [
      'provenanceBorder=probe',
      { view: 'cooccurrence_network', provenanceBorder: 'probe' },
      (p) => p?.provenanceBorder,
      'probe'
    ],
    [
      'provenanceBorder=both',
      { view: 'cooccurrence_network', provenanceBorder: 'both' },
      (p) => p?.provenanceBorder,
      'both'
    ]
  ];

  it.each(preserved)('round-trips %s', (_label, panel, read, want) => {
    expect(read(firstPanel(rt(panel)))).toBe(want);
  });
});

// ── Special-case decode behaviours that don't fit the tables ──────────────────

describe('encodePillarState / decodePillarState — edge behaviours', () => {
  it('maps a pre-125a sankey ms field-chain onto fieldChain (back-compat)', () => {
    // Pre-125a URLs stored the sankey field chain in `ms` (overloaded metricSet).
    const decoded = rt({ view: 'sankey', metricSet: ['article_type', 'section'] });
    expect(firstPanel(decoded)?.fieldChain).toEqual(['article_type', 'section']);
    expect(firstPanel(decoded)?.metricSet).toBeUndefined();
  });

  it('rejects a malformed cellOverride lever on decode', () => {
    // co with a non-numeric bins (bn) → reject the whole payload.
    const encoded = encodeCompact({
      w: [
        {
          p: [
            {
              ...COMPACT_PANEL,
              c: 's',
              v: 'distribution',
              m: 'word_count',
              co: { 's:a': { bn: 'lots' } }
            }
          ],
          fi: 0
        }
      ],
      aw: 0
    });
    expect(decodePillarState(encoded)).toBeNull();
  });

  it('round-trips maximizedPanelIndex on a multi-panel window (C3)', () => {
    const win = makeWindow([makePanel(), makePanel()]);
    win.maximizedPanelIndex = 1;
    const decoded = decodePillarState(encodePillarState(makePillarState([win])));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBe(1);
  });

  it('omits maximizedPanelIndex when null/undefined', () => {
    const win = makeWindow([makePanel(), makePanel()]);
    win.maximizedPanelIndex = null;
    const decoded = decodePillarState(encodePillarState(makePillarState([win])));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBeUndefined();
  });

  it('drops out-of-bounds maximizedPanelIndex on encode (defensive)', () => {
    const win = makeWindow([makePanel()]); // only 1 panel
    win.maximizedPanelIndex = 5;
    const decoded = decodePillarState(encodePillarState(makePillarState([win])));
    expect(decoded?.windows[0]?.maximizedPanelIndex).toBeUndefined();
  });

  it('rejects an out-of-bounds maximizedPanelIndex on decode (malformed URL)', () => {
    const encoded = encodeCompact({ w: [{ p: [COMPACT_PANEL], fi: 0, mp: 5 }], aw: 0 });
    expect(decodePillarState(encoded)).toBeNull();
  });

  it('produces URL-safe base64 (no +, /, or =)', () => {
    expect(encodePillarState(makePillarState())).not.toMatch(/[+/=]/);
  });

  it('keeps the encoded payload reasonably short for a typical state (< 512 bytes)', () => {
    expect(encodePillarState(makePillarState()).length).toBeLessThan(512);
  });
});

// ── Decode rejection table (malformed / out-of-spec payloads → null) ──────────

describe('decodePillarState — rejects malformed payloads', () => {
  const rejected: Array<[string, string]> = [
    ['malformed base64', '!@#$'],
    ['valid base64 but not JSON', btoa('hello').replace(/=+$/, '')],
    [
      'JSON missing required panel fields',
      encodeCompact({ w: [{ p: [{ s: [] }], fi: 0 }], aw: 0 })
    ],
    [
      'activeWindowIndex out of bounds',
      encodeCompact({ w: [{ p: [COMPACT_PANEL], fi: 0 }], aw: 5 })
    ],
    [
      'invalid view enum value',
      encodeCompact({ w: [{ p: [{ ...COMPACT_PANEL, v: 'scatter_plot' }], fi: 0 }], aw: 0 })
    ]
  ];

  it.each(rejected)('returns null on %s', (_label, encoded) => {
    expect(decodePillarState(encoded)).toBeNull();
  });
});

// ── readFromSearch / writeToSearch (pillar form) ─────────────────────────────

describe('readFromSearch — pillar form (Phase 122i)', () => {
  it('parses ?activePillar=aleph&aleph=<encoded> into pillars.aleph', () => {
    const ps = makePillarState();
    const parsed = readFromSearch(`?activePillar=aleph&aleph=${encodePillarState(ps)}`);
    expect(parsed.activePillar).toBe('aleph');
    expect(parsed.pillars?.aleph).toEqual(ps);
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
    expect(parsed.pillars?.aleph).toBeTruthy();
    expect(parsed.selectedProbes).toEqual([]);
  });

  it('ignores invalid pillar payloads (sets the slot to null)', () => {
    const parsed = readFromSearch('?aleph=invalid-base64-!@#');
    expect(parsed.pillars).not.toBeNull();
    expect(parsed.pillars?.aleph).toBeNull();
  });
});

describe('writeToSearch — pillar form (Phase 122i)', () => {
  const alephOnly = () =>
    state({
      activePillar: 'aleph',
      pillars: { aleph: makePillarState(), episteme: null, rhizome: null }
    });

  it('emits multi-panel form when pillars is non-null', () => {
    const qs = writeToSearch(alephOnly());
    expect(qs).toContain('activePillar=aleph');
    expect(qs).toContain('aleph=');
    expect(qs).not.toContain('episteme=');
    expect(qs).not.toContain('rhizome=');
  });

  it('writes only the canonical Phase-122k grammar (no legacy flat params)', () => {
    const qs = writeToSearch(alephOnly());
    for (const legacy of [
      'probeId=',
      'sourceId=',
      'viewingMode=',
      'viewMode=',
      'metric=',
      'layer='
    ]) {
      expect(qs).not.toContain(legacy);
    }
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

  it('preserves normalization alongside pillar state', () => {
    const qs = writeToSearch(state({ ...alephOnly(), normalization: 'zscore' }));
    expect(qs).toContain('normalization=zscore');
  });
});
