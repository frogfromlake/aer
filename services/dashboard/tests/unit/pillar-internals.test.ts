import { describe, expect, it } from 'vitest';

import {
  seedPillarFromCurrent,
  type DefaultPresentationResolver
} from '../../src/lib/pillar-internals';
import type { Panel, PillarState, WorkbenchPillarsState } from '../../src/lib/state/url-internals';

// Phase 142 — pure cross-pillar seed transform, extracted from pillar.ts (which
// couples to the rune state layer). These tests pin the focused-panel clone, the
// view rebase, the deep-copy isolation, and every absent-panel null path.

// A stand-in resolver so the test does not depend on the real registry.
const resolve: DefaultPresentationResolver = (id) =>
  id === 'episteme' ? 'time_series' : id === 'rhizome' ? 'cooccurrence_network' : 'distribution';

function panel(over: Partial<Panel> = {}): Panel {
  return {
    scopes: [{ probeIds: ['probe-0'], sourceIds: ['tagesschau'] }],
    composition: 'merged',
    view: 'distribution',
    metric: 'sentiment_score_bert_multilingual',
    layer: 'gold',
    ...over
  } as Panel;
}

function pillarState(over: Partial<PillarState> = {}): PillarState {
  return {
    windows: [{ panels: [panel()], focusedPanelIndex: 0 }],
    activeWindowIndex: 0,
    ...over
  } as PillarState;
}

/** Fill all three pillar slots (WorkbenchPillarsState requires every key);
 *  `aleph` is the source pillar these tests seed FROM. */
function pillars(aleph: PillarState | null): WorkbenchPillarsState {
  return { aleph, episteme: null, rhizome: null };
}

describe('seedPillarFromCurrent', () => {
  it('clones the focused panel and rebases the view to the target pillar default', () => {
    const seeded = seedPillarFromCurrent(pillars(pillarState()), 'aleph', 'episteme', resolve);
    expect(seeded).not.toBeNull();
    const p = seeded!.windows[0]!.panels[0]!;
    expect(p.view).toBe('time_series'); // target = episteme default
    expect(p.metric).toBe('sentiment_score_bert_multilingual'); // carried from source
    expect(p.layer).toBe('gold');
    // Phase 123a — split is the honest default for a freshly-seeded pillar.
    expect(p.composition).toBe('split');
    expect(seeded!.activeWindowIndex).toBe(0);
    expect(seeded!.windows[0]!.focusedPanelIndex).toBe(0);
  });

  it('deep-copies the scope arrays so the seed cannot alias the source panel', () => {
    const src = pillarState();
    const seeded = seedPillarFromCurrent(pillars(src), 'aleph', 'rhizome', resolve)!;
    const seededScopes = seeded.windows[0]!.panels[0]!.scopes;
    seededScopes[0]!.probeIds.push('probe-1');
    expect(src.windows[0]!.panels[0]!.scopes[0]!.probeIds).toEqual(['probe-0']);
  });

  it('honours the active window + focused panel indices when picking the source panel', () => {
    const focused = panel({ metric: 'word_count' });
    const source = {
      windows: [
        { panels: [panel()], focusedPanelIndex: 0 },
        { panels: [panel(), focused], focusedPanelIndex: 1 }
      ],
      activeWindowIndex: 1
    } as PillarState;
    const seeded = seedPillarFromCurrent(pillars(source), 'aleph', 'episteme', resolve)!;
    expect(seeded.windows[0]!.panels[0]!.metric).toBe('word_count');
  });

  it('returns null when the source pillar has no state', () => {
    expect(seedPillarFromCurrent(pillars(null), 'aleph', 'episteme', resolve)).toBeNull();
  });

  it('returns null when the source pillar has no window', () => {
    const source = { windows: [], activeWindowIndex: 0 } as PillarState;
    expect(seedPillarFromCurrent(pillars(source), 'aleph', 'episteme', resolve)).toBeNull();
  });

  it('returns null when the focused window has no panel', () => {
    const source = {
      windows: [{ panels: [], focusedPanelIndex: 0 }],
      activeWindowIndex: 0
    } as PillarState;
    expect(seedPillarFromCurrent(pillars(source), 'aleph', 'episteme', resolve)).toBeNull();
  });

  it('falls back to window[0] / panel[0] when the indices point out of range', () => {
    const source = {
      windows: [{ panels: [panel({ metric: 'entity_count' })], focusedPanelIndex: 9 }],
      activeWindowIndex: 9
    } as PillarState;
    const seeded = seedPillarFromCurrent(pillars(source), 'aleph', 'episteme', resolve)!;
    expect(seeded.windows[0]!.panels[0]!.metric).toBe('entity_count');
  });
});
