import { describe, expect, it } from 'vitest';

import {
  filterAnalyses,
  sortAnalyses,
  nextSort,
  sortArrow,
  fmtDate,
  stripDeepLink,
  isSaveableWorkbenchUrl,
  findEditableLoaded,
  NON_STATE_PARAMS,
  type AnalysisFilters
} from '../../src/lib/components/account/analyses-overlay-internals';
import type { AnalysisListItem } from '../../src/lib/api/analyses';

// Phase 141 — pure cores of the AnalysesOverlay's client-side filter / sort /
// formatting + deep-link helpers, lifted out of the component during its
// decomposition. The component just owns the `$state` and delegates here; the
// e2e pins the rendered overlay, these pin the data semantics.

function row(over: Partial<AnalysisListItem> = {}): AnalysisListItem {
  return {
    id: 'id-1',
    name: 'Alpha',
    description: 'desc',
    ownerEmail: 'owner@aer.test',
    createdAt: '2026-05-01T00:00:00Z',
    updatedAt: '2026-05-02T00:00:00Z',
    permission: 'editable',
    owned: true,
    ...over
  };
}

const ALL_ON: AnalysisFilters = {
  search: '',
  showOwned: true,
  showShared: true,
  showEditable: true,
  showReadable: true,
  createdFrom: '',
  createdTo: ''
};

describe('filterAnalyses', () => {
  const items = [
    row({ id: 'a', name: 'Climate report', owned: true, permission: 'editable' }),
    row({
      id: 'b',
      name: 'Weather brief',
      description: 'shared one',
      ownerEmail: 'colleague@aer.test',
      owned: false,
      permission: 'readable'
    })
  ];

  it('returns every row when no filter narrows it', () => {
    expect(filterAnalyses(items, ALL_ON).map((a) => a.id)).toEqual(['a', 'b']);
  });

  it('search matches name, description or owner, case-insensitively', () => {
    expect(filterAnalyses(items, { ...ALL_ON, search: 'CLIMATE' }).map((a) => a.id)).toEqual(['a']);
    expect(filterAnalyses(items, { ...ALL_ON, search: 'shared' }).map((a) => a.id)).toEqual(['b']);
    expect(filterAnalyses(items, { ...ALL_ON, search: 'colleague' }).map((a) => a.id)).toEqual([
      'b'
    ]);
    expect(filterAnalyses(items, { ...ALL_ON, search: '   ' }).map((a) => a.id)).toEqual([
      'a',
      'b'
    ]);
  });

  it('owned/shared toggles drop the matching ownership class', () => {
    expect(filterAnalyses(items, { ...ALL_ON, showOwned: false }).map((a) => a.id)).toEqual(['b']);
    expect(filterAnalyses(items, { ...ALL_ON, showShared: false }).map((a) => a.id)).toEqual(['a']);
  });

  it('editable/read-only toggles drop the matching permission', () => {
    expect(filterAnalyses(items, { ...ALL_ON, showEditable: false }).map((a) => a.id)).toEqual([
      'b'
    ]);
    expect(filterAnalyses(items, { ...ALL_ON, showReadable: false }).map((a) => a.id)).toEqual([
      'a'
    ]);
  });

  it('created-from is an inclusive lower bound', () => {
    const dated = [
      row({ id: 'old', createdAt: '2026-04-01T00:00:00Z' }),
      row({ id: 'new', createdAt: '2026-06-01T00:00:00Z' })
    ];
    expect(
      filterAnalyses(dated, { ...ALL_ON, createdFrom: '2026-05-01' }).map((a) => a.id)
    ).toEqual(['new']);
  });

  it('created-to is inclusive of the whole selected day', () => {
    const dated = [row({ id: 'sameday', createdAt: '2026-05-10T18:30:00Z' })];
    // A naive `< createdTo` bound would exclude an afternoon timestamp on the
    // chosen day; the +1-day inclusive upper bound keeps it.
    expect(filterAnalyses(dated, { ...ALL_ON, createdTo: '2026-05-10' }).map((a) => a.id)).toEqual([
      'sameday'
    ]);
    expect(filterAnalyses(dated, { ...ALL_ON, createdTo: '2026-05-09' })).toHaveLength(0);
  });

  it('does not mutate its input array', () => {
    const input = [...items];
    filterAnalyses(input, { ...ALL_ON, search: 'climate' });
    expect(input.map((a) => a.id)).toEqual(['a', 'b']);
  });
});

describe('sortAnalyses', () => {
  const items = [
    row({ id: 'b', name: 'Beta', createdAt: '2026-05-03T00:00:00Z' }),
    row({ id: 'a', name: 'Alpha', createdAt: '2026-05-01T00:00:00Z' }),
    row({ id: 'c', name: 'Gamma', createdAt: '2026-05-02T00:00:00Z' })
  ];

  it('sorts text columns by locale, both directions', () => {
    expect(sortAnalyses(items, 'name', 'asc').map((a) => a.id)).toEqual(['a', 'b', 'c']);
    expect(sortAnalyses(items, 'name', 'desc').map((a) => a.id)).toEqual(['c', 'b', 'a']);
  });

  it('sorts date columns by timestamp, both directions', () => {
    expect(sortAnalyses(items, 'createdAt', 'asc').map((a) => a.id)).toEqual(['a', 'c', 'b']);
    expect(sortAnalyses(items, 'createdAt', 'desc').map((a) => a.id)).toEqual(['b', 'c', 'a']);
  });

  it('does not mutate its input array', () => {
    const input = [...items];
    sortAnalyses(input, 'name', 'asc');
    expect(input.map((a) => a.id)).toEqual(['b', 'a', 'c']);
  });
});

describe('nextSort', () => {
  it('toggles direction when the same column is re-clicked', () => {
    expect(nextSort({ key: 'name', dir: 'asc' }, 'name')).toEqual({ key: 'name', dir: 'desc' });
    expect(nextSort({ key: 'name', dir: 'desc' }, 'name')).toEqual({ key: 'name', dir: 'asc' });
  });

  it('defaults a new date column to descending (newest first)', () => {
    expect(nextSort({ key: 'name', dir: 'asc' }, 'createdAt')).toEqual({
      key: 'createdAt',
      dir: 'desc'
    });
    expect(nextSort({ key: 'name', dir: 'asc' }, 'updatedAt')).toEqual({
      key: 'updatedAt',
      dir: 'desc'
    });
  });

  it('defaults a new text column to ascending (A→Z)', () => {
    expect(nextSort({ key: 'updatedAt', dir: 'desc' }, 'name')).toEqual({
      key: 'name',
      dir: 'asc'
    });
    expect(nextSort({ key: 'name', dir: 'asc' }, 'ownerEmail')).toEqual({
      key: 'ownerEmail',
      dir: 'asc'
    });
  });
});

describe('sortArrow', () => {
  it('shows the direction glyph for the active column only', () => {
    expect(sortArrow('name', 'asc', 'name')).toBe('▲');
    expect(sortArrow('name', 'desc', 'name')).toBe('▼');
    expect(sortArrow('name', 'asc', 'createdAt')).toBe('');
  });
});

describe('fmtDate', () => {
  it('formats a valid ISO date and falls back to an em-dash', () => {
    expect(fmtDate('2026-05-01T00:00:00Z', 'en')).not.toBe('—');
    expect(typeof fmtDate('2026-05-01T00:00:00Z', 'en')).toBe('string');
    expect(fmtDate('not-a-date', 'en')).toBe('—');
  });
});

describe('stripDeepLink', () => {
  it('removes the non-state overlay params and keeps the Workbench grammar', () => {
    const href =
      'https://aer.test/workbench?activePillar=aleph&aleph=ABC&analyses=open&savedAnalysis=x';
    expect(stripDeepLink(href, NON_STATE_PARAMS)).toBe('/workbench?activePillar=aleph&aleph=ABC');
  });

  it('omits the question mark when nothing remains', () => {
    expect(stripDeepLink('https://aer.test/workbench?analyses=open', NON_STATE_PARAMS)).toBe(
      '/workbench'
    );
  });
});

describe('isSaveableWorkbenchUrl', () => {
  it('requires the /workbench path with at least one non-empty pillar', () => {
    expect(isSaveableWorkbenchUrl('https://aer.test/workbench?aleph=ABC')).toBe(true);
    expect(isSaveableWorkbenchUrl('https://aer.test/workbench?episteme=XYZ')).toBe(true);
    expect(isSaveableWorkbenchUrl('https://aer.test/workbench?activePillar=aleph&aleph=')).toBe(
      false
    );
    expect(isSaveableWorkbenchUrl('https://aer.test/workbench')).toBe(false);
    expect(isSaveableWorkbenchUrl('https://aer.test/?aleph=ABC')).toBe(false);
  });
});

describe('findEditableLoaded', () => {
  const items = [
    row({ id: 'edit', permission: 'editable' }),
    row({ id: 'read', permission: 'readable' })
  ];

  it('returns the matching editable analysis, else null', () => {
    expect(findEditableLoaded(items, 'edit')?.id).toBe('edit');
    expect(findEditableLoaded(items, 'read')).toBeNull();
    expect(findEditableLoaded(items, 'missing')).toBeNull();
    expect(findEditableLoaded(items, null)).toBeNull();
    expect(findEditableLoaded(items, undefined)).toBeNull();
  });
});
