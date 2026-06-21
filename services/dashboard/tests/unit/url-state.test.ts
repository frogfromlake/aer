import { describe, expect, it } from 'vitest';

import {
  EMPTY_URL_STATE,
  readFromSearch,
  writeToSearch,
  type UrlState
} from '../../src/lib/state/url-internals';

// Phase 122k — URL grammar reduced to a single canonical form. The
// Phase-122h flat reader (?probeId=&sourceId=&viewingMode=&view=&metric=
// &viewMode=&layer=) was retired entirely; the only fields the URL
// surface carries are `?from`, `?to`, `?resolution`,
// `?normalization`, `?activePillar`, `?aleph`/`?episteme`/`?rhizome`
// (pillar-state base64url), and `?selectedProbes`.

function state(overrides: Partial<UrlState> = {}): UrlState {
  return { ...EMPTY_URL_STATE, ...overrides };
}

describe('readFromSearch', () => {
  it('returns the empty default for an empty search string', () => {
    expect(readFromSearch('')).toEqual(state());
  });

  it('parses ISO dates and normalises them to UTC ISO form', () => {
    const s = '?from=2026-04-01T00:00Z&to=2026-04-22T00:00:00Z';
    const parsed = readFromSearch(s);
    expect(parsed.from).toBe('2026-04-01T00:00:00.000Z');
    expect(parsed.to).toBe('2026-04-22T00:00:00.000Z');
  });

  it('drops invalid dates rather than surfacing NaN', () => {
    expect(readFromSearch('?from=not-a-date').from).toBeNull();
  });

  it('rejects ambiguous partial dates that would coerce to an unintended window (SEC-093)', () => {
    // Bare year / year-month / date-only must NOT be silently widened to a
    // midnight-UTC instant by new Date(); only a full date+time+offset parses.
    expect(readFromSearch('?from=2026').from).toBeNull();
    expect(readFromSearch('?from=2026-04').from).toBeNull();
    expect(readFromSearch('?from=2026-04-01').from).toBeNull();
    // Calendar-impossible values matching the shape are still rejected.
    expect(readFromSearch('?from=2026-13-01T00:00:00Z').from).toBeNull();
    // An offset instant remains accepted and normalises to UTC.
    expect(readFromSearch('?from=2026-04-01T02:00:00%2B02:00').from).toBe(
      '2026-04-01T00:00:00.000Z'
    );
  });

  it('validates resolution against its enum', () => {
    expect(readFromSearch('?resolution=hourly').resolution).toBe('hourly');
    expect(readFromSearch('?resolution=yearly').resolution).toBeNull();
  });

  it('validates activePillar against its enum', () => {
    expect(readFromSearch('?activePillar=episteme').activePillar).toBe('episteme');
    expect(readFromSearch('?activePillar=rhizome').activePillar).toBe('rhizome');
    expect(readFromSearch('?activePillar=ghosts').activePillar).toBeNull();
  });

  it('parses comma-separated selectedProbes into an array', () => {
    expect(readFromSearch('?selectedProbes=probe-0-de-institutional-web').selectedProbes).toEqual([
      'probe-0-de-institutional-web'
    ]);
    expect(
      readFromSearch('?selectedProbes=probe-0-de-institutional-web,probe-1-de-public-rss')
        .selectedProbes
    ).toEqual(['probe-0-de-institutional-web', 'probe-1-de-public-rss']);
    expect(readFromSearch('').selectedProbes).toEqual([]);
  });

  it('ignores legacy Phase-122h flat params (probeId, sourceId, metric, …)', () => {
    // Phase 122k — these used to populate fields on UrlState. After the
    // single-grammar cleanup they are simply ignored.
    const parsed = readFromSearch(
      '?probeId=probe-0&sourceId=tagesschau&viewingMode=episteme&view=actors-topics&metric=sentiment_score&viewMode=distribution&layer=silver'
    );
    // activePillar is the surviving enum and is read from its canonical
    // key only, not from `viewingMode`.
    expect(parsed.activePillar).toBeNull();
    expect(parsed.selectedProbes).toEqual([]);
    // No `viewingMode` / `sourceIds` / `metric` / `viewMode` / `layer`
    // fields exist on UrlState anymore — this is a type-level guarantee
    // checked by tsc; here we only verify the surviving shape.
    expect(parsed).toEqual(state());
  });
});

describe('writeToSearch', () => {
  it('omits all fields when state is empty', () => {
    expect(writeToSearch(state())).toBe('');
  });

  it('emits only populated fields', () => {
    const qs = writeToSearch(
      state({
        from: '2026-04-01T00:00:00.000Z',
        to: '2026-04-22T00:00:00.000Z',
        resolution: 'hourly'
      })
    );
    expect(qs).toContain('from=2026-04-01');
    expect(qs).toContain('to=2026-04-22');
    expect(qs).toContain('resolution=hourly');
    expect(qs).not.toContain('viewingMode');
    expect(qs).not.toContain('layer=');
  });

  it('emits activePillar when set', () => {
    expect(writeToSearch(state({ activePillar: 'episteme' }))).toContain('activePillar=episteme');
  });

  it('emits selectedProbes as comma-separated', () => {
    expect(
      writeToSearch(
        state({ selectedProbes: ['probe-0-de-institutional-web', 'probe-1-de-public-rss'] })
      )
    ).toContain('selectedProbes=probe-0-de-institutional-web%2Cprobe-1-de-public-rss');
    expect(writeToSearch(state())).not.toContain('selectedProbes=');
  });

  it('normalization roundtrips zscore and percentile, omits raw', () => {
    expect(readFromSearch(writeToSearch(state({ normalization: 'zscore' }))).normalization).toBe(
      'zscore'
    );
    expect(
      readFromSearch(writeToSearch(state({ normalization: 'percentile' }))).normalization
    ).toBe('percentile');
    expect(writeToSearch(state({ normalization: 'raw' }))).not.toContain('normalization');
  });

  it('round-trips selectedProbes through readFromSearch', () => {
    const original = state({
      selectedProbes: ['probe-0-de-institutional-web', 'probe-1-de-public-rss']
    });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });

  it('round-trips activePillar + selectedProbes + normalization through readFromSearch', () => {
    const original = state({
      activePillar: 'rhizome',
      selectedProbes: ['probe-0-de-institutional-web'],
      normalization: 'zscore'
    });
    expect(readFromSearch(writeToSearch(original))).toEqual(original);
  });
});
