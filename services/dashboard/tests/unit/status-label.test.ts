import { describe, it, expect } from 'vitest';
import { statusLabel } from '../../src/lib/account/status-label';

describe('statusLabel', () => {
  it('returns a localized (non-raw) label for each known status', () => {
    for (const s of ['active', 'invited', 'suspended']) {
      const label = statusLabel(s);
      expect(label).toBeTruthy();
      // The localized label differs from the lowercase machine value.
      expect(label).not.toBe(s);
    }
  });

  it('falls back to the raw value for an unknown status', () => {
    expect(statusLabel('archived')).toBe('archived');
  });

  it('falls back to a dash for nullish input', () => {
    expect(statusLabel(undefined)).toBe('—');
    expect(statusLabel(null)).toBe('—');
  });
});
