import { describe, expect, it } from 'vitest';

import {
  DEFAULT_BINS,
  DEFAULT_FORCE_STRENGTH,
  DEFAULT_TOPN,
  NET_COLOR_CHANNELS,
  NET_SIZE_CHANNELS
} from '../../src/lib/workbench/cell-levers';

// cell-levers is the single source of truth for the cell-shape lever defaults +
// channel option tables shared by PanelControls and CellConfigPopover. The test
// pins the contract so the two surfaces can never silently drift.

describe('cell-shape lever defaults', () => {
  it('match the values the cells render when a lever is unset', () => {
    expect(DEFAULT_BINS).toBe(30);
    expect(DEFAULT_TOPN).toBe(60);
    expect(DEFAULT_FORCE_STRENGTH).toBe(50);
  });
});

describe('network channel option tables', () => {
  it('expose the size channels with stable ids', () => {
    expect(NET_SIZE_CHANNELS.map((c) => c.id)).toEqual(['total_count', 'degree', 'metric']);
  });

  it('expose the colour channels including the Louvain theme-cluster default', () => {
    const ids = NET_COLOR_CHANNELS.map((c) => c.id);
    expect(ids[0]).toBe('community');
    expect(ids).toContain('metric');
    expect(ids).toContain('uniform');
  });

  it('give every channel a human label', () => {
    for (const c of [...NET_SIZE_CHANNELS, ...NET_COLOR_CHANNELS]) {
      expect(c.label.length).toBeGreaterThan(0);
    }
  });
});
