import { describe, expect, it } from 'vitest';

import {
  downloadBlob,
  downloadDataUrl,
  downloadText,
  exportFilename,
  payloadToCsv,
  payloadToJson,
  type ExportPayload
} from '../../src/lib/presentations/cell-export';

// The DOM-bound capture paths (exportFilter / triggerDownload / html-to-image
// PNG+SVG) are browser-only — covered by E2E, not these node-env unit tests,
// which exercise the pure serialisation + the no-DOM guards.

const PAYLOAD: ExportPayload = {
  meta: { metric: 'sentiment', scope: 'probe-0', empty: '' },
  summary: { mean: 0.2, n: 120 },
  howToRead: ['Line one.', 'Line two.'],
  rows: [
    { bin: '0.0–0.2', count: 10 },
    { bin: '0.2–0.4', count: 30 }
  ]
};

describe('exportFilename', () => {
  it('prefixes aer, joins parts, and appends a UTC timestamp stem', () => {
    const name = exportFilename(['sentiment', 'probe-0']);
    // exportSlug lowercases, so the trailing Z of the ISO stamp becomes z.
    expect(name).toMatch(/^aer_sentiment_probe-0_\d{8}-\d{6}z$/);
  });
});

describe('payloadToCsv', () => {
  it('emits a commented meta/summary/how-to-read header then the rows', () => {
    const csv = payloadToCsv(PAYLOAD);
    expect(csv).toContain('# metric: sentiment');
    expect(csv).toContain('# summary.mean: 0.2');
    expect(csv).toContain('# how-to-read: Line one.');
    expect(csv).toContain('bin,count');
    expect(csv).toContain('0.0–0.2,10');
    // an empty meta value is skipped
    expect(csv).not.toContain('# empty:');
  });
});

describe('payloadToJson', () => {
  it('serialises meta/summary/howToRead/rows as pretty JSON', () => {
    const parsed = JSON.parse(payloadToJson(PAYLOAD));
    expect(parsed.meta.metric).toBe('sentiment');
    expect(parsed.summary.n).toBe(120);
    expect(parsed.rows).toHaveLength(2);
    expect(parsed.howToRead).toEqual(['Line one.', 'Line two.']);
  });
});

describe('download helpers without a DOM', () => {
  it('no-op safely when document is undefined (node env)', () => {
    expect(() => downloadText('f.csv', 'text/csv', 'a,b')).not.toThrow();
    expect(() => downloadBlob('f.bin', new Blob(['x']))).not.toThrow();
    expect(() => downloadDataUrl('f.png', 'data:image/png;base64,AAAA')).not.toThrow();
  });
});
