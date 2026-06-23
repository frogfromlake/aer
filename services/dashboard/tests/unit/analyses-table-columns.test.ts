import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  ANALYSIS_COLUMN_IDS,
  DEFAULT_COLUMN_WIDTHS,
  MIN_COLUMN_WIDTH,
  MAX_COLUMN_WIDTH,
  clampColumnWidth,
  loadColumnWidths,
  saveColumnWidths,
  resetColumnWidths,
  totalColumnsWidth,
  loadColumnOrder,
  saveColumnOrder,
  resetColumnOrder,
  moveColumn,
  type AnalysisColumnId,
  type ColumnWidths
} from '../../src/lib/components/account/analyses-table-columns';

function mockStorage() {
  const map = new Map<string, string>();
  return {
    getItem: (k: string) => map.get(k) ?? null,
    setItem: (k: string, v: string) => void map.set(k, v),
    removeItem: (k: string) => void map.delete(k)
  };
}

describe('clampColumnWidth', () => {
  it('clamps below the minimum and above the maximum, rounding', () => {
    expect(clampColumnWidth(10)).toBe(MIN_COLUMN_WIDTH);
    expect(clampColumnWidth(99999)).toBe(MAX_COLUMN_WIDTH);
    expect(clampColumnWidth(180.6)).toBe(181);
  });
  it('falls back to the minimum for non-finite input', () => {
    expect(clampColumnWidth(NaN)).toBe(MIN_COLUMN_WIDTH);
    expect(clampColumnWidth(Infinity)).toBe(MIN_COLUMN_WIDTH);
  });
});

describe('totalColumnsWidth', () => {
  it('sums all column widths', () => {
    const sum = ANALYSIS_COLUMN_IDS.reduce((s, id) => s + DEFAULT_COLUMN_WIDTHS[id], 0);
    expect(totalColumnsWidth(DEFAULT_COLUMN_WIDTHS)).toBe(sum);
  });
});

describe('persistence', () => {
  beforeEach(() => vi.stubGlobal('localStorage', mockStorage()));
  afterEach(() => vi.unstubAllGlobals());

  it('returns the defaults when nothing is stored', () => {
    expect(loadColumnWidths()).toEqual(DEFAULT_COLUMN_WIDTHS);
  });

  it('round-trips saved widths (merged over defaults + clamped)', () => {
    const widths: ColumnWidths = { ...DEFAULT_COLUMN_WIDTHS, name: 400, owner: 5 };
    saveColumnWidths(widths);
    const loaded = loadColumnWidths();
    expect(loaded.name).toBe(400);
    expect(loaded.owner).toBe(MIN_COLUMN_WIDTH); // clamped up from 5
    expect(loaded.created).toBe(DEFAULT_COLUMN_WIDTHS.created); // untouched → default
  });

  it('ignores corrupt stored JSON and keeps defaults', () => {
    localStorage.setItem('aer.analyses.col-widths.v1', '{not json');
    expect(loadColumnWidths()).toEqual(DEFAULT_COLUMN_WIDTHS);
  });

  it('reset clears the store and returns defaults', () => {
    saveColumnWidths({ ...DEFAULT_COLUMN_WIDTHS, name: 400 });
    expect(resetColumnWidths()).toEqual(DEFAULT_COLUMN_WIDTHS);
    expect(loadColumnWidths()).toEqual(DEFAULT_COLUMN_WIDTHS);
  });

  it('order round-trips and reset restores the default order', () => {
    const custom: AnalysisColumnId[] = [
      'access',
      ...ANALYSIS_COLUMN_IDS.filter((id) => id !== 'access')
    ];
    saveColumnOrder(custom);
    expect(loadColumnOrder()).toEqual(custom);
    expect(resetColumnOrder()).toEqual([...ANALYSIS_COLUMN_IDS]);
    expect(loadColumnOrder()).toEqual([...ANALYSIS_COLUMN_IDS]);
  });

  it('reconciles a partial/dirty stored order: drops unknown + dupes, appends missing', () => {
    localStorage.setItem(
      'aer.analyses.col-order.v1',
      JSON.stringify(['owner', 'owner', 'bogus', 'name'])
    );
    const loaded = loadColumnOrder();
    // known (deduped) first, in stored order, then the rest in default order.
    expect(loaded.slice(0, 2)).toEqual(['owner', 'name']);
    expect([...loaded].sort()).toEqual([...ANALYSIS_COLUMN_IDS].sort());
    expect(loaded).toHaveLength(ANALYSIS_COLUMN_IDS.length);
  });

  it('returns the default order for corrupt JSON', () => {
    localStorage.setItem('aer.analyses.col-order.v1', '{not json');
    expect(loadColumnOrder()).toEqual([...ANALYSIS_COLUMN_IDS]);
  });
});

describe('moveColumn', () => {
  it('moves a column to another position, shifting the rest', () => {
    const order = ['name', 'description', 'owner', 'created'] as const;
    // move 'created' before 'description'
    expect(moveColumn([...order], 'created', 'description')).toEqual([
      'name',
      'created',
      'description',
      'owner'
    ]);
  });
  it('is a no-op for the same id or an unknown id', () => {
    const order = [...ANALYSIS_COLUMN_IDS];
    expect(moveColumn(order, 'name', 'name')).toEqual(order);
  });
});
