// Resizable-column model + persistence for the saved-analyses table (Phase
// 148e). Pure + framework-free so it unit-tests without a DOM: the component
// owns the `$state`, this owns the column ids, defaults, clamping and the
// localStorage round-trip. Reordering is a later step; the id order here is the
// fixed render order for now.

export const ANALYSIS_COLUMN_IDS = [
  'name',
  'description',
  'owner',
  'created',
  'updated',
  'access'
] as const;

export type AnalysisColumnId = (typeof ANALYSIS_COLUMN_IDS)[number];

export type ColumnWidths = Record<AnalysisColumnId, number>;

/** Sensible starting widths (px); the sum is the table's default natural width. */
export const DEFAULT_COLUMN_WIDTHS: ColumnWidths = {
  name: 240,
  description: 280,
  owner: 170,
  created: 130,
  updated: 130,
  access: 120
};

export const MIN_COLUMN_WIDTH = 72;
export const MAX_COLUMN_WIDTH = 720;

const STORE_KEY = 'aer.analyses.col-widths.v1';
const ORDER_KEY = 'aer.analyses.col-order.v1';

/** Clamp + round a width to a sane pixel range. */
export function clampColumnWidth(px: number): number {
  if (!Number.isFinite(px)) return MIN_COLUMN_WIDTH;
  return Math.max(MIN_COLUMN_WIDTH, Math.min(MAX_COLUMN_WIDTH, Math.round(px)));
}

/** Load persisted widths, merged over the defaults and clamped. Never throws —
 *  a missing / corrupt / unavailable store falls back to the defaults. */
export function loadColumnWidths(): ColumnWidths {
  const out: ColumnWidths = { ...DEFAULT_COLUMN_WIDTHS };
  try {
    const raw = globalThis.localStorage?.getItem(STORE_KEY);
    if (!raw) return out;
    const parsed = JSON.parse(raw) as Partial<Record<string, unknown>>;
    for (const id of ANALYSIS_COLUMN_IDS) {
      const v = parsed[id];
      if (typeof v === 'number' && Number.isFinite(v)) out[id] = clampColumnWidth(v);
    }
  } catch {
    /* absent/corrupt/blocked storage — keep the defaults */
  }
  return out;
}

/** Persist the current widths. Non-fatal if storage is unavailable / full. */
export function saveColumnWidths(widths: ColumnWidths): void {
  try {
    globalThis.localStorage?.setItem(STORE_KEY, JSON.stringify(widths));
  } catch {
    /* storage unavailable / quota — the in-memory widths still apply this session */
  }
}

/** Clear the persisted override and return the defaults (the "reset columns"). */
export function resetColumnWidths(): ColumnWidths {
  try {
    globalThis.localStorage?.removeItem(STORE_KEY);
  } catch {
    /* ignore */
  }
  return { ...DEFAULT_COLUMN_WIDTHS };
}

/** Total natural table width (sum of the column widths) for `width: max-content`
 *  predictability with `table-layout: fixed`. */
export function totalColumnsWidth(widths: ColumnWidths): number {
  return ANALYSIS_COLUMN_IDS.reduce((sum, id) => sum + (widths[id] ?? 0), 0);
}

// --- column order (reorderable, persisted) ---------------------------------

const KNOWN = new Set<string>(ANALYSIS_COLUMN_IDS);

/** Coerce any stored value into a valid, complete permutation of the known
 *  column ids: dedupe + drop unknowns, then append any ids the stored order is
 *  missing (so a newly-added column appears at the end rather than vanishing). */
function reconcileOrder(value: unknown): AnalysisColumnId[] {
  const seen = new Set<AnalysisColumnId>();
  const out: AnalysisColumnId[] = [];
  if (Array.isArray(value)) {
    for (const v of value) {
      if (typeof v === 'string' && KNOWN.has(v) && !seen.has(v as AnalysisColumnId)) {
        seen.add(v as AnalysisColumnId);
        out.push(v as AnalysisColumnId);
      }
    }
  }
  for (const id of ANALYSIS_COLUMN_IDS) if (!seen.has(id)) out.push(id);
  return out;
}

/** Load the persisted column order, reconciled to a valid permutation. Never
 *  throws — absent / corrupt storage falls back to the default order. */
export function loadColumnOrder(): AnalysisColumnId[] {
  try {
    const raw = globalThis.localStorage?.getItem(ORDER_KEY);
    if (raw) return reconcileOrder(JSON.parse(raw));
  } catch {
    /* absent/corrupt/blocked storage — default order */
  }
  return [...ANALYSIS_COLUMN_IDS];
}

/** Persist the column order. Non-fatal if storage is unavailable. */
export function saveColumnOrder(order: readonly AnalysisColumnId[]): void {
  try {
    globalThis.localStorage?.setItem(ORDER_KEY, JSON.stringify(order));
  } catch {
    /* storage unavailable / quota — the in-memory order still applies */
  }
}

/** Clear the persisted order and return the default. */
export function resetColumnOrder(): AnalysisColumnId[] {
  try {
    globalThis.localStorage?.removeItem(ORDER_KEY);
  } catch {
    /* ignore */
  }
  return [...ANALYSIS_COLUMN_IDS];
}

/** Move `fromId` to `toId`'s position, shifting the rest. Pure; returns the
 *  same array reference unchanged when the move is a no-op / ids are unknown. */
export function moveColumn(
  order: readonly AnalysisColumnId[],
  fromId: AnalysisColumnId,
  toId: AnalysisColumnId
): AnalysisColumnId[] {
  if (fromId === toId) return [...order];
  const from = order.indexOf(fromId);
  const to = order.indexOf(toId);
  if (from < 0 || to < 0) return [...order];
  const next = [...order];
  next.splice(from, 1);
  next.splice(to, 0, fromId);
  return next;
}
