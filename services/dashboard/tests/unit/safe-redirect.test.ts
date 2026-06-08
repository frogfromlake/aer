import { describe, expect, it } from 'vitest';
import { safeRedirect } from '../../src/lib/auth/safe-redirect';

describe('safeRedirect (open-redirect guard)', () => {
  it('passes same-app absolute paths', () => {
    expect(safeRedirect('/workbench')).toBe('/workbench');
    expect(safeRedirect('/reflection/wp/WP-001?x=1')).toBe('/reflection/wp/WP-001?x=1');
  });

  it('falls back to / for empty / nullish input', () => {
    expect(safeRedirect(null)).toBe('/');
    expect(safeRedirect(undefined)).toBe('/');
    expect(safeRedirect('')).toBe('/');
  });

  it('rejects protocol-relative and absolute URLs (no open redirect)', () => {
    expect(safeRedirect('//evil.com')).toBe('/');
    expect(safeRedirect('/\\evil.com')).toBe('/');
    expect(safeRedirect('https://evil.com')).toBe('/');
    expect(safeRedirect('http://evil.com')).toBe('/');
    expect(safeRedirect('javascript:alert(1)')).toBe('/');
  });

  it('rejects non-path values', () => {
    expect(safeRedirect('workbench')).toBe('/');
    expect(safeRedirect('..')).toBe('/');
  });
});
