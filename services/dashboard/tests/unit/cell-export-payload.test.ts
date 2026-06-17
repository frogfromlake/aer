import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  downloadBlob,
  downloadCellPng,
  downloadCellSvg,
  downloadDataUrl,
  downloadText,
  exportFilename,
  payloadToCsv,
  payloadToJson,
  type ExportPayload
} from '../../src/lib/presentations/cell-export';

// The serialisation pieces run pure; the DOM-bound capture paths
// (exportFilter / triggerDownload / html-to-image PNG+SVG) are exercised here
// behind a minimal jsdom-free DOM + html-to-image stub so the node-env unit
// runner still covers the anchor-click + the lazy-import wiring.

// Mock the lazy-loaded html-to-image so downloadCell{Png,Svg} resolve in node.
// The mocks capture the options object so the test can exercise the node-filter
// (`exportFilter`) that downloadCell{Png,Svg} builds from the variant.
type CaptureOpts = { filter?: (n: HTMLElement) => boolean };
const captured: { png?: CaptureOpts; svg?: CaptureOpts } = {};
vi.mock('html-to-image', () => ({
  toPng: vi.fn(async (_n: HTMLElement, opts: CaptureOpts) => {
    captured.png = opts;
    return 'data:image/png;base64,PNG';
  }),
  toSvg: vi.fn(async (_n: HTMLElement, opts: CaptureOpts) => {
    captured.svg = opts;
    return 'data:image/svg+xml;base64,SVG';
  })
}));

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

describe('payloadToCsv without summary / howToRead', () => {
  it('emits only the meta block + rows when summary/howToRead are absent', () => {
    const csv = payloadToCsv({ meta: { metric: 'sentiment' }, rows: [{ a: 1 }] });
    expect(csv).toContain('# metric: sentiment');
    expect(csv).not.toContain('# summary.');
    expect(csv).not.toContain('# how-to-read:');
    expect(csv).toContain('a\n1');
  });
});

describe('download helpers without a DOM', () => {
  it('no-op safely when document is undefined (node env)', () => {
    expect(() => downloadText('f.csv', 'text/csv', 'a,b')).not.toThrow();
    expect(() => downloadBlob('f.bin', new Blob(['x']))).not.toThrow();
    expect(() => downloadDataUrl('f.png', 'data:image/png;base64,AAAA')).not.toThrow();
  });
});

describe('download + capture helpers WITH a stubbed DOM', () => {
  // A minimal document stub: createElement('a') returns a fake anchor whose
  // click/remove are recorded; body.appendChild is a no-op. URL.createObjectURL
  // + revokeObjectURL are stubbed so downloadBlob can run.
  let clicked: Array<{ href: string; download: string }>;
  // A class so `node instanceof HTMLElement` in exportFilter passes for stubs.
  class FakeEl {
    dataset: { exportExclude?: string };
    constructor(exportExclude?: string) {
      this.dataset = exportExclude !== undefined ? { exportExclude } : {};
    }
  }
  beforeEach(() => {
    clicked = [];
    const anchor = {
      href: '',
      download: '',
      click() {
        clicked.push({ href: this.href, download: this.download });
      },
      remove() {}
    };
    vi.stubGlobal('document', {
      createElement: () => anchor,
      body: { appendChild() {} }
    });
    vi.stubGlobal('URL', {
      createObjectURL: () => 'blob:fake',
      revokeObjectURL: vi.fn()
    });
    vi.stubGlobal('HTMLElement', FakeEl);
  });
  afterEach(() => vi.unstubAllGlobals());

  it('downloadText writes a blob URL and clicks the anchor', () => {
    downloadText('out.csv', 'text/csv', 'a,b\n1,2');
    expect(clicked).toHaveLength(1);
    expect(clicked[0]!.download).toBe('out.csv');
    expect(clicked[0]!.href).toBe('blob:fake');
  });

  it('downloadDataUrl clicks the anchor with the data URL directly', () => {
    downloadDataUrl('fig.png', 'data:image/png;base64,AAAA');
    expect(clicked[0]!.href).toBe('data:image/png;base64,AAAA');
  });

  it('downloadCellPng captures, downloads, and builds a full-variant node filter', async () => {
    const node = new FakeEl() as unknown as HTMLElement;
    await downloadCellPng('cell.png', node, 'full');
    expect(clicked[0]!.href).toBe('data:image/png;base64,PNG');
    const filter = captured.png!.filter!;
    // 'always' is dropped in both variants; 'provenance' is KEPT in full.
    expect(filter(new FakeEl('always') as unknown as HTMLElement)).toBe(false);
    expect(filter(new FakeEl('provenance') as unknown as HTMLElement)).toBe(true);
    expect(filter(new FakeEl() as unknown as HTMLElement)).toBe(true);
  });

  it('downloadCellSvg builds a figure-variant filter that also drops provenance', async () => {
    const node = new FakeEl() as unknown as HTMLElement;
    await downloadCellSvg('cell.svg', node, 'figure');
    expect(clicked[0]!.href).toBe('data:image/svg+xml;base64,SVG');
    const filter = captured.svg!.filter!;
    expect(filter(new FakeEl('always') as unknown as HTMLElement)).toBe(false);
    expect(filter(new FakeEl('provenance') as unknown as HTMLElement)).toBe(false);
    expect(filter(new FakeEl() as unknown as HTMLElement)).toBe(true);
  });
});
