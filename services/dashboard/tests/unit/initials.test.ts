import { describe, it, expect } from 'vitest';
import { initials, displayName, hasName } from '../../src/lib/identity/initials';

describe('initials', () => {
  it('uses first + last name when present', () => {
    expect(initials({ firstName: 'Anna', lastName: 'Schmidt' })).toBe('AS');
  });

  it('uppercases lowercase names', () => {
    expect(initials({ firstName: 'anna', lastName: 'müller' })).toBe('AM');
  });

  it('is umlaut/code-point safe', () => {
    expect(initials({ firstName: 'Über', lastName: 'Ärzte' })).toBe('ÜÄ');
  });

  it('falls back to one initial when only one name part exists', () => {
    expect(initials({ firstName: 'Anna' })).toBe('A');
    expect(initials({ lastName: 'Schmidt' })).toBe('S');
  });

  it('derives two initials from a dotted email local-part', () => {
    expect(initials({ email: 'anna.schmidt@example.org' })).toBe('AS');
  });

  it('takes the first two code points of a single-token email local-part', () => {
    expect(initials({ email: 'nelixposteo@example.org' })).toBe('NE');
  });

  it('prefers the name over the email when both are present', () => {
    expect(initials({ firstName: 'Anna', lastName: 'Schmidt', email: 'x.y@z.org' })).toBe('AS');
  });

  it('returns empty string when nothing is available', () => {
    expect(initials({})).toBe('');
    expect(initials({ firstName: '  ', email: '   ' })).toBe('');
  });
});

describe('displayName', () => {
  it('joins first and last name', () => {
    expect(displayName({ firstName: 'Anna', lastName: 'Schmidt' })).toBe('Anna Schmidt');
  });

  it('uses a single name part when the other is missing', () => {
    expect(displayName({ firstName: 'Anna' })).toBe('Anna');
  });

  it('falls back to the email when no name is present', () => {
    expect(displayName({ email: 'anna@example.org' })).toBe('anna@example.org');
  });

  it('trims surrounding whitespace', () => {
    expect(displayName({ firstName: ' Anna ', lastName: ' Schmidt ' })).toBe('Anna Schmidt');
  });
});

describe('hasName', () => {
  it('is true when a name part is present', () => {
    expect(hasName({ firstName: 'Anna' })).toBe(true);
    expect(hasName({ lastName: 'Schmidt' })).toBe(true);
  });

  it('is false for email-only or blank identities', () => {
    expect(hasName({ email: 'anna@example.org' })).toBe(false);
    expect(hasName({ firstName: '  ', lastName: '' })).toBe(false);
  });
});
