import { describe, expect, it } from 'vitest';

import {
  addPanelPure as _addPanelPure,
  addScopeGroupPure as _addScopeGroupPure,
  addWindowPure as _addWindowPure,
  focusPanelPure as _focusPanelPure,
  removePanelPure as _removePanelPure,
  updatePanelPure as _updatePanelPure,
  type PanelPath
} from '../../src/lib/workbench/panel-mutators-pure';
import type {
  Panel,
  PillarState,
  ScopeGroup,
  WorkbenchPillarsState,
  WorkbenchWindow
} from '../../src/lib/state/url-internals';

// Phase 122i / ADR-034 — Workbench-state mutator tests against the
// pure helpers (`_*Pure`). The runtime wrappers (focusPanel, addPanel,
// …) just thread urlState/setUrl around these — covering the pure
// helpers verifies the full mutation semantics.

function group(probeIds: string[] = ['probe-0'], sourceIds: string[] = []): ScopeGroup {
  return { probeIds, sourceIds };
}

function panel(overrides: Partial<Panel> = {}): Panel {
  return {
    scopes: [group()],
    composition: 'merged',
    view: 'time_series',
    metric: 'sentiment_score_sentiws',
    layer: 'gold',
    ...overrides
  };
}

function win(panels: Panel[] = [panel()], focusedPanelIndex = 0): WorkbenchWindow {
  return { panels, focusedPanelIndex };
}

function pillarState(windows: WorkbenchWindow[] = [win()], activeWindowIndex = 0): PillarState {
  return { windows, activeWindowIndex };
}

function pillars(alephState: PillarState | null = pillarState()): WorkbenchPillarsState {
  return { aleph: alephState, episteme: null, rhizome: null };
}

const ALEPH_PANEL_0: PanelPath = { pillar: 'aleph', windowIndex: 0, panelIndex: 0 };

describe('_updatePanelPure', () => {
  it('replaces the targeted panel via the update callback', () => {
    const next = _updatePanelPure(pillars(), ALEPH_PANEL_0, (p) => ({
      ...p,
      metric: 'word_count'
    }));
    expect(next?.aleph?.windows[0]?.panels[0]?.metric).toBe('word_count');
  });

  it('returns the original state when the path does not resolve', () => {
    const before = pillars();
    const next = _updatePanelPure(
      before,
      { pillar: 'aleph', windowIndex: 5, panelIndex: 0 },
      (p) => p
    );
    expect(next).toBe(before);
  });

  it('allows non-scope mutations on a locked panel (B1: scope-only lock)', () => {
    const lockedState = pillars(
      pillarState([win([panel({ locked: true, lockedFunction: 'epistemic_authority' })])])
    );
    const next = _updatePanelPure(lockedState, ALEPH_PANEL_0, (p) => ({
      ...p,
      metric: 'word_count'
    }));
    expect(next?.aleph?.windows[0]?.panels[0]?.metric).toBe('word_count');
    // Lock flag preserved.
    expect(next?.aleph?.windows[0]?.panels[0]?.locked).toBe(true);
  });

  it('silently discards scope mutations on a locked panel (B1)', () => {
    const originalScopes = [group(['probe-0'], ['tagesschau'])];
    const lockedState = pillars(
      pillarState([
        win([
          panel({ locked: true, lockedFunction: 'epistemic_authority', scopes: originalScopes })
        ])
      ])
    );
    const next = _updatePanelPure(lockedState, ALEPH_PANEL_0, (p) => ({
      ...p,
      // Attempt to swap scope — must be ignored.
      scopes: [group(['probe-0'], ['bundesregierung'])],
      // Non-scope mutation in same call — must apply.
      metric: 'word_count'
    }));
    expect(next?.aleph?.windows[0]?.panels[0]?.scopes).toEqual(originalScopes);
    expect(next?.aleph?.windows[0]?.panels[0]?.metric).toBe('word_count');
  });

  it('leaves untouched panels alone', () => {
    const initial = pillars(pillarState([win([panel({ metric: 'a' }), panel({ metric: 'b' })])]));
    const next = _updatePanelPure(initial, ALEPH_PANEL_0, (p) => ({ ...p, metric: 'changed' }));
    expect(next?.aleph?.windows[0]?.panels[1]?.metric).toBe('b');
  });
});

describe('_addPanelPure', () => {
  it('appends a clone of the focused panel by default', () => {
    const initial = pillars(pillarState([win([panel({ metric: 'a' })])]));
    const next = _addPanelPure(initial, 'aleph');
    expect(next?.aleph?.windows[0]?.panels).toHaveLength(2);
    expect(next?.aleph?.windows[0]?.panels[1]?.metric).toBe('a');
    expect(next?.aleph?.windows[0]?.focusedPanelIndex).toBe(1);
  });

  it('clones the explicit template when provided', () => {
    const initial = pillars(pillarState([win([panel({ metric: 'a' })])]));
    const next = _addPanelPure(initial, 'aleph', panel({ metric: 'template-metric' }));
    expect(next?.aleph?.windows[0]?.panels[1]?.metric).toBe('template-metric');
  });

  it('respects MAX_PANELS_PER_WINDOW (8) and refuses to add a 9th', () => {
    const eightPanels = Array.from({ length: 8 }, () => panel());
    const initial = pillars(pillarState([win(eightPanels)]));
    const next = _addPanelPure(initial, 'aleph');
    expect(next).toBe(initial);
  });

  it('no-ops when the pillar has no state', () => {
    const initial: WorkbenchPillarsState = { aleph: null, episteme: null, rhizome: null };
    expect(_addPanelPure(initial, 'aleph')).toBe(initial);
  });
});

describe('_removePanelPure', () => {
  it('removes a panel from a multi-panel window', () => {
    const initial = pillars(pillarState([win([panel({ metric: 'a' }), panel({ metric: 'b' })])]));
    const next = _removePanelPure(initial, ALEPH_PANEL_0);
    expect(next?.aleph?.windows[0]?.panels).toHaveLength(1);
    expect(next?.aleph?.windows[0]?.panels[0]?.metric).toBe('b');
  });

  it('drops the entire pillar when removing the only panel of the only window', () => {
    const initial = pillars();
    const next = _removePanelPure(initial, ALEPH_PANEL_0);
    expect(next?.aleph).toBeNull();
  });

  it('drops a window when removing its only panel and other windows remain', () => {
    const initial = pillars(pillarState([win([panel()]), win([panel({ metric: 'b' })])]));
    const next = _removePanelPure(initial, ALEPH_PANEL_0);
    expect(next?.aleph?.windows).toHaveLength(1);
    expect(next?.aleph?.windows[0]?.panels[0]?.metric).toBe('b');
  });

  it('keeps focusedPanelIndex in bounds after removal', () => {
    const initial = pillars(pillarState([win([panel(), panel(), panel()], 2)]));
    const next = _removePanelPure(initial, { ...ALEPH_PANEL_0, panelIndex: 2 });
    expect(next?.aleph?.windows[0]?.focusedPanelIndex).toBe(1);
  });
});

describe('_focusPanelPure', () => {
  it('moves focus to the targeted panel', () => {
    const initial = pillars(pillarState([win([panel(), panel()], 0)]));
    const next = _focusPanelPure(initial, { ...ALEPH_PANEL_0, panelIndex: 1 });
    expect(next?.aleph?.windows[0]?.focusedPanelIndex).toBe(1);
  });

  it('is a no-op when the focus already matches', () => {
    const initial = pillars(pillarState([win([panel()], 0)]));
    expect(_focusPanelPure(initial, ALEPH_PANEL_0)).toBe(initial);
  });

  it('also updates activeWindowIndex when crossing windows', () => {
    const initial = pillars(pillarState([win([panel()]), win([panel(), panel()])]));
    const next = _focusPanelPure(initial, { pillar: 'aleph', windowIndex: 1, panelIndex: 1 });
    expect(next?.aleph?.activeWindowIndex).toBe(1);
    expect(next?.aleph?.windows[1]?.focusedPanelIndex).toBe(1);
  });
});

describe('_addScopeGroupPure', () => {
  it('appends a new ScopeGroup to the focused panel', () => {
    const initial = pillars(pillarState([win([panel({ scopes: [group(['probe-0'], ['s-a'])] })])]));
    const next = _addScopeGroupPure(initial, 'aleph');
    expect(next?.aleph?.windows[0]?.panels[0]?.scopes).toHaveLength(2);
  });

  it('inherits the first ScopeGroup probes by default', () => {
    const initial = pillars(pillarState([win([panel({ scopes: [group(['probe-X'], ['s-a'])] })])]));
    const next = _addScopeGroupPure(initial, 'aleph');
    expect(next?.aleph?.windows[0]?.panels[0]?.scopes[1]?.probeIds).toEqual(['probe-X']);
    expect(next?.aleph?.windows[0]?.panels[0]?.scopes[1]?.sourceIds).toEqual([]);
  });

  it('refuses to touch a locked panel', () => {
    const initial = pillars(pillarState([win([panel({ locked: true })])]));
    expect(_addScopeGroupPure(initial, 'aleph')).toBe(initial);
  });
});

describe('_addWindowPure', () => {
  it('appends a new window and makes it active', () => {
    const initial = pillars();
    const next = _addWindowPure(initial, 'aleph');
    expect(next?.aleph?.windows).toHaveLength(2);
    expect(next?.aleph?.activeWindowIndex).toBe(1);
  });

  it('respects MAX_WINDOWS_PER_PILLAR (4) and refuses a 5th', () => {
    const fourWindows = Array.from({ length: 4 }, () => win());
    const initial = pillars(pillarState(fourWindows, 3));
    const next = _addWindowPure(initial, 'aleph');
    expect(next).toBe(initial);
  });
});
