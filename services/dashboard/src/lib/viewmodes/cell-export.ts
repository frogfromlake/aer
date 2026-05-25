// Publication-ready export helpers — Phase 131 (whole-cell capture, BUG5/round-3).
//
// Two export families per cell:
//   • Image — the WHOLE cell rasterised/vectorised exactly as shown, via
//     `html-to-image` (lazy-loaded only on an export click, so it never lands
//     in the initial bundle). Two variants:
//       - "full"   the dashboard view: chart + summary + legend + provenance +
//                  the "how to read" note (everything a human reads);
//       - "figure" a clean publication figure: chart + summary + legend only
//                  (no provenance/methodology, no how-to-read).
//     The interactive export toolbar itself is excluded from both.
//   • Data — CSV + JSON with a self-describing metadata header + summary +
//     the how-to-read lines + every row. Pure (vitest-pinnable).
//
// Elements opt out of the image via `data-export-exclude`:
//   "always"     → never in either image variant (the export toolbar)
//   "provenance" → dropped from the "figure" variant only (provenance footer
//                  AND the how-to-read note — methodology a publication figure
//                  shouldn't carry, but the full dashboard export keeps)

/** One exportable row — string keys to scalar values. */
export type ExportRow = Record<string, string | number | boolean | null | undefined>;

/** Escape a single CSV field per RFC 4180. */
export function csvField(value: string | number | boolean | null | undefined): string {
  if (value === null || value === undefined) return '';
  const s = String(value);
  if (/[",\n\r]/.test(s)) {
    return `"${s.replace(/"/g, '""')}"`;
  }
  return s;
}

/** Serialise rows to CSV text (header + one line per row). */
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

/** Build a scientific, sortable filename stem: parts + a UTC timestamp. */
export function exportFilename(parts: Array<string | number | null | undefined>): string {
  const stamp = new Date()
    .toISOString()
    .replace(/[-:]/g, '')
    .replace(/\.\d+Z$/, 'Z')
    .replace('T', '-');
  return exportSlug('aer', ...parts, stamp);
}

// ---------------------------------------------------------------------------
// Data payload (CSV / JSON)
// ---------------------------------------------------------------------------

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
    { meta: p.meta, summary: p.summary, howToRead: p.howToRead, rows: p.rows },
    null,
    2
  );
}

// ---------------------------------------------------------------------------
// Browser-only download + whole-cell image capture
// ---------------------------------------------------------------------------

/** Trigger a browser download of `text` as `filename` with the given MIME. */
export function downloadText(filename: string, mime: string, text: string): void {
  if (typeof document === 'undefined') return;
  downloadBlob(filename, new Blob([text], { type: `${mime};charset=utf-8` }));
}

/** Trigger a browser download of an arbitrary blob. */
export function downloadBlob(filename: string, blob: Blob): void {
  if (typeof document === 'undefined') return;
  triggerDownload(filename, URL.createObjectURL(blob), true);
}

/** Trigger a browser download from a data: URL (html-to-image output). */
export function downloadDataUrl(filename: string, dataUrl: string): void {
  if (typeof document === 'undefined') return;
  triggerDownload(filename, dataUrl, false);
}

function triggerDownload(filename: string, href: string, revoke: boolean): void {
  const a = document.createElement('a');
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  if (revoke) setTimeout(() => URL.revokeObjectURL(href), 0);
}

/** Which image variant to export. */
export type ImageVariant = 'full' | 'figure';

/** html-to-image node filter implementing the `data-export-exclude` opt-outs. */
function exportFilter(variant: ImageVariant): (node: HTMLElement) => boolean {
  return (node: HTMLElement) => {
    const ex = node instanceof HTMLElement ? node.dataset?.exportExclude : undefined;
    if (ex === 'always') return false;
    if (variant === 'figure' && ex === 'provenance') return false;
    return true;
  };
}

// Dark backdrop so a transparent cell exports on the AĒR canvas, not white.
const EXPORT_BG = '#10141a';

/** Capture a cell node to PNG (lazy html-to-image). `variant` controls whether
 *  the provenance footer is included. Computed styles are inlined by
 *  html-to-image, so the export matches the on-screen cell exactly. */
export async function downloadCellPng(
  filename: string,
  node: HTMLElement,
  variant: ImageVariant
): Promise<void> {
  const { toPng } = await import('html-to-image');
  const dataUrl = await toPng(node, {
    filter: exportFilter(variant),
    backgroundColor: EXPORT_BG,
    pixelRatio: 2,
    cacheBust: true
  });
  downloadDataUrl(filename, dataUrl);
}

/** Capture a cell node to SVG (lazy html-to-image). */
export async function downloadCellSvg(
  filename: string,
  node: HTMLElement,
  variant: ImageVariant
): Promise<void> {
  const { toSvg } = await import('html-to-image');
  const dataUrl = await toSvg(node, {
    filter: exportFilter(variant),
    backgroundColor: EXPORT_BG,
    cacheBust: true
  });
  downloadDataUrl(filename, dataUrl);
}
