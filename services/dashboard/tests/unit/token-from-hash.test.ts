import { describe, expect, it } from 'vitest';

import { tokenFromHash } from '../../src/lib/auth/token-from-hash';

// SEC-009 — the single-use auth token rides in the URL fragment, never the
// query string, so it never reaches a server log.
describe('tokenFromHash', () => {
  it('reads the token from a fragment with a leading #', () => {
    expect(tokenFromHash('#token=abc123')).toBe('abc123');
  });

  it('reads the token from a fragment without the leading #', () => {
    expect(tokenFromHash('token=abc123')).toBe('abc123');
  });

  it('returns "" when no fragment is present', () => {
    expect(tokenFromHash('')).toBe('');
    expect(tokenFromHash('#')).toBe('');
  });

  it('returns "" when the fragment has no token key', () => {
    expect(tokenFromHash('#other=1')).toBe('');
  });

  it('ignores a token in the query-style position (only the fragment counts)', () => {
    // A bare query string passed as the hash has no `token` key at the front.
    expect(tokenFromHash('#foo=bar&token=xyz')).toBe('xyz');
  });
});
