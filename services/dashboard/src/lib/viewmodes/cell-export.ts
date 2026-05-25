// Publication-ready export helpers — Phase 131.
//
// Every configurable cell offers the same export set: a PNG raster, an SVG
// vector, and the underlying data (CSV + JSON). The pure pieces (CSV
// serialisation, RFC-4180 escaping, filename slugging) live here so vitest can
// pin them without a DOM; the DOM/browser pieces (blob download, SVG → PNG via
// canvas) are thin wrappers used only client-side.
//
// No new runtime dependency: SVG serialisation is native (`XMLSerializer`),
// rasterisation goes through a `<canvas>`, downloads through an object URL.
// This keeps the cell chunks inside the Brief §7 bundle budget.

/** One exportable row — string keys to scalar values. */
export type ExportRow = Record<string, string | number | boolean | null | undefined>;

/** Escape a single CSV field per RFC 4180: quote when it contains a comma,
 *  quote, or newline; double any embedded quotes. */
export function csvField(value: string | number | boolean | null | undefined): string {
  if (value === null || value === undefined) return '';
  const s = String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

/** Serialise rows to CSV text. Columns are taken in the given order; when
 *  omitted they are the union of all row keys in first-seen order. Always
 *  emits a header row. */
export function toCsv(rows: readonly ExportRow[], columns?: readonly string[]): string {
  const cols =
    columns ??
    (() => {
      const seen: string[] = [];
      const mark: Record<string, true> = {};
      for (const r of rows) {
        for (const k of Object.keys(r)) {
          if (!mark[k]) {
            mark[k] = true;
            seen.push(k);
          }
        }
      }
      return seen;
    })();
  const lines: string[] = [cols.map(csvField).join(',')];
  for (const r of rows) {
    lines.push(cols.map((c) => csvField(r[c])).join(','));
  }
  return lines.join('\n');
}

/** Lowercase, hyphenated, filesystem-safe slug for a download filename. */
export function exportSlug(...parts: Array<string | number | null | undefined>): string {
  return parts
    .filter((p) => p !== null && p !== undefined && String(p).length > 0)
    .map((p) =>
      String(p)
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '')
    )
    .filter((p) => p.length > 0)
    .join('_');
}

// ---------------------------------------------------------------------------
// Browser-only helpers (guarded so an accidental SSR import is a no-op).
// ---------------------------------------------------------------------------

/** Trigger a browser download of `text` as `filename` with the given MIME. */
export function downloadText(filename: string, mime: string, text: string): void {
  if (typeof document === 'undefined') return;
  const blob = new Blob([text], { type: `${mime};charset=utf-8` });
  downloadBlob(filename, blob);
}

/** Trigger a browser download of an arbitrary blob. */
export function downloadBlob(filename: string, blob: Blob): void {
  if (typeof document === 'undefined') return;
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  // Revoke on the next tick so the click has a chance to start the download.
  setTimeout(() => URL.revokeObjectURL(url), 0);
}

// AĒR canvas tokens, hard-coded for standalone export (the CSS custom
// properties that colour the in-app chart are gone once the SVG is detached).
const EXPORT_BG = '#10141a';
const EXPORT_FG = '#c9d3df';
const EXPORT_FONT =
  "ui-monospace, 'SF Mono', 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace";

/** Serialise an SVG element to a standalone, namespaced SVG string that
 *  renders identically to the in-app cell (BUG5). Observable Plot draws axis
 *  text + lines with `currentColor`, which resolves to the page's light fg
 *  in-app but to black in a detached file — so we pin `color` (→ currentColor)
 *  and `background` on the clone, plus a font + a fallback `text { fill }`. */
export function serializeSvg(svg: SVGSVGElement): string {
  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  clone.setAttribute('xmlns:xlink', 'http://www.w3.org/1999/xlink');
  clone.style.color = EXPORT_FG;
  clone.style.background = EXPORT_BG;
  clone.style.fontFamily = EXPORT_FONT;
  // Belt-and-suspenders for renderers that don't resolve `currentColor` from
  // the inline style: a <style> rule for any text/tick that lacks its own fill.
  const styleEl = document.createElementNS('http://www.w3.org/2000/svg', 'style');
  styleEl.textContent = `text{fill:${EXPORT_FG};font-family:${EXPORT_FONT};} [aria-label] path[stroke="currentColor"]{stroke:${EXPORT_FG};}`;
  clone.insertBefore(styleEl, clone.firstChild);
  return '<?xml version="1.0" encoding="UTF-8"?>\n' + new XMLSerializer().serializeToString(clone);
}

/** Pick the chart's MAIN svg from a host that may contain several (Observable
 *  Plot emits a `<figure>` with a colour-legend swatch svg PLUS the plot svg;
 *  a naive `querySelector('svg')` grabs the legend — that was the scatter PNG
 *  bug). Returns the largest svg by rendered area, or null. */
export function pickChartSvg(host: HTMLElement | null | undefined): SVGSVGElement | null {
  if (!host) return null;
  const svgs = Array.from(host.querySelectorAll('svg')) as SVGSVGElement[];
  if (svgs.length === 0) return null;
  let best = svgs[0]!;
  let bestArea = -1;
  for (const s of svgs) {
    const r = s.getBoundingClientRect();
    const area = r.width * r.height;
    if (area > bestArea) {
      bestArea = area;
      best = s;
    }
  }
  return best;
}

/** Build a scientific, sortable filename stem: parts + a UTC timestamp so an
 *  exported file is identifiable among thousands (BUG5). */
export function exportFilename(parts: Array<string | number | null | undefined>): string {
  const stamp = new Date()
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d+Z$/, 'Z')
    .replace('T', '-');
  return exportSlug('aer', ...parts, stamp);
}

/** Compose the metadata + data payload shared by CSV and JSON exports so an
 *  exported artefact is self-describing (BUG5: include ALL data + the how-to
 *  note). */
export interface ExportPayload {
  meta: Record<string, string | number | undefined>;
  /** Summary key/value pairs (e.g. distribution quantiles). */
  summary?: Record<string, string | number> | undefined;
  /** "How to read this" lines. */
  howToRead?: readonly string[] | undefined;
  rows: readonly ExportRow[];
  columns?: readonly string[] | undefined;
}

/** Render the payload as a CSV with a leading `#`-commented metadata block. */
export function payloadToCsv(p: ExportPayload): string {
  const lines: string[] = [];
  for (const [k, v] of Object.entries(p.meta)) {
    if (v !== undefined && v !== '') lines.push(`# ${k}: ${v}`);
  }
  if (p.summary) {
    for (const [k, v] of Object.entries(p.summary)) lines.push(`# summary.${k}: ${v}`);
  }
  for (const line of p.howToRead ?? []) lines.push(`# how-to-read: ${line}`);
  lines.push('');
  lines.push(toCsv(p.rows, p.columns));
  return lines.join('\n');
}

/** Render the payload as a pretty JSON object. */
export function payloadToJson(p: ExportPayload): string {
  return JSON.stringify(
    {
      meta: p.meta,
      summary: p.summary,
      howToRead: p.howToRead,
      rows: p.rows
    },
    null,
    2
  );
}

/** Download an SVG element as a `.svg` file. */
export function downloadSvg(filename: string, svg: SVGSVGElement): void {
  downloadText(filename, 'image/svg+xml', serializeSvg(svg));
}

/** Rasterise an SVG element to a PNG blob via an offscreen canvas. The
 *  `scale` factor (default 2) yields a retina-quality raster. */
export async function svgToPngBlob(svg: SVGSVGElement, scale = 2): Promise<Blob> {
  const source = serializeSvg(svg);
  const rect = svg.getBoundingClientRect();
  const width = Math.max(1, Math.round((rect.width || svg.clientWidth || 720) * scale));
  const height = Math.max(1, Math.round((rect.height || svg.clientHeight || 500) * scale));
  const svgBlob = new Blob([source], { type: 'image/svg+xml;charset=utf-8' });
  const url = URL.createObjectURL(svgBlob);
  try {
    const img = await loadImage(url);
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const cctx = canvas.getContext('2d');
    if (!cctx) throw new Error('2d canvas context unavailable');
    cctx.fillStyle = '#10141a';
    cctx.fillRect(0, 0, width, height);
    cctx.drawImage(img, 0, 0, width, height);
    return await canvasToBlob(canvas);
  } finally {
    URL.revokeObjectURL(url);
  }
}

/** Download an SVG element as a `.png` file. */
export async function downloadPng(filename: string, svg: SVGSVGElement, scale = 2): Promise<void> {
  const blob = await svgToPngBlob(svg, scale);
  downloadBlob(filename, blob);
}

function loadImage(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.onload = () => resolve(img);
    img.onerror = () => reject(new Error('failed to load SVG for rasterisation'));
    img.src = url;
  });
}

function canvasToBlob(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob) resolve(blob);
      else reject(new Error('canvas.toBlob returned null'));
    }, 'image/png');
  });
}
