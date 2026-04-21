import { describe, expect, it } from 'vitest';

import { readFromSearch, writeToSearch } from '../../src/lib/state/url-internals';

describe('readFromSearch', () => {
  it('returns all-null for an empty search string', () => {
    expect(readFromSearch('')).toEqual({
      from: null,
      to: null,
      probe: null,
      resolution: null,
      viewingMode: null
    });
  });

  it('parses ISO dates and normalises them to UTC ISO form', () => {
    const s = '?from=2026-04-01T00:00Z&to=2026-04-22T00:00:00Z';
    const state = readFromSearch(s);
    expect(state.from).toBe('2026-04-01T00:00:00.000Z');
    expect(state.to).toBe('2026-04-22T00:00:00.000Z');
  });

  it('drops invalid dates rather than surfacing NaN', () => {
    const state = readFromSearch('?from=not-a-date');
    expect(state.from).toBeNull();
  });

  it('validates resolution and viewingMode against their enums', () => {
    const good = readFromSearch('?resolution=hourly&viewingMode=episteme');
    expect(good.resolution).toBe('hourly');
    expect(good.viewingMode).toBe('episteme');

    const bad = readFromSearch('?resolution=yearly&viewingMode=ghosts');
    expect(bad.resolution).toBeNull();
    expect(bad.viewingMode).toBeNull();
  });

  it('passes probe through unmodified', () => {
    const state = readFromSearch('?probe=probe-0-de-institutional-rss');
    expect(state.probe).toBe('probe-0-de-institutional-rss');
  });
});

describe('writeToSearch', () => {
  it('omits null fields entirely', () => {
    expect(
      writeToSearch({ from: null, to: null, probe: null, resolution: null, viewingMode: null })
    ).toBe('');
  });

  it('emits only populated fields', () => {
    const qs = writeToSearch({
      from: '2026-04-01T00:00:00.000Z',
      to: '2026-04-22T00:00:00.000Z',
      probe: 'probe-0',
      resolution: 'hourly',
      viewingMode: null
    });
    expect(qs).toContain('from=2026-04-01');
    expect(qs).toContain('to=2026-04-22');
    expect(qs).toContain('probe=probe-0');
    expect(qs).toContain('resolution=hourly');
    expect(qs).not.toContain('viewingMode');
  });

  it('round-trips through readFromSearch', () => {
    const original = {
      from: '2026-04-01T00:00:00.000Z',
      to: '2026-04-22T00:00:00.000Z',
      probe: 'probe-0-de-institutional-rss',
      resolution: 'daily' as const,
      viewingMode: 'aleph' as const
    };
    const qs = writeToSearch(original);
    expect(readFromSearch(qs)).toEqual(original);
  });
});
