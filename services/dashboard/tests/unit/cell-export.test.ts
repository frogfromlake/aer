import { describe, expect, it } from 'vitest';

import { csvField, toCsv, exportSlug } from '../../src/lib/viewmodes/cell-export';

// Phase 131 — publication-ready data export (pure pieces).

describe('csvField', () => {
  it('passes plain values through unquoted', () => {
    expect(csvField('tagesschau')).toBe('tagesschau');
    expect(csvField(42)).toBe('42');
    expect(csvField(true)).toBe('true');
  });

  it('renders null/undefined as empty', () => {
    expect(csvField(null)).toBe('');
    expect(csvField(undefined)).toBe('');
  });

  it('quotes and escapes fields with commas, quotes, or newlines (RFC 4180)', () => {
    expect(csvField('a,b')).toBe('"a,b"');
    expect(csvField('say "hi"')).toBe('"say ""hi"""');
    expect(csvField('line1\nline2')).toBe('"line1\nline2"');
  });
});

describe('toCsv', () => {
  it('emits a header plus one row per record in column order', () => {
    const csv = toCsv(
      [
        { lower: 0, upper: 10, count: 3 },
        { lower: 10, upper: 20, count: 5 }
      ],
      ['lower', 'upper', 'count']
    );
    expect(csv).toBe('lower,upper,count\n0,10,3\n10,20,5');
  });

  it('infers columns from the union of keys when not given', () => {
    const csv = toCsv([{ a: 1 }, { a: 2, b: 3 }]);
    expect(csv.split('\n')[0]).toBe('a,b');
  });

  it('fills missing cells with empty strings', () => {
    const csv = toCsv([{ a: 1, b: 2 }, { a: 3 }], ['a', 'b']);
    expect(csv).toBe('a,b\n1,2\n3,');
  });
});

describe('exportSlug', () => {
  it('lowercases, hyphenates, and joins parts with underscores', () => {
    expect(exportSlug('aer', 'Distribution word_count')).toBe('aer_distribution-word-count');
  });

  it('drops empty / nullish parts', () => {
    expect(exportSlug('aer', '', null, undefined, 'x')).toBe('aer_x');
  });
});
