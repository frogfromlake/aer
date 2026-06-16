// Phase 141 — AnalysesOverlay pure internals.
//
// The client-side filter / sort / formatting and deep-link helpers of the
// saved-analyses overlay, lifted out of the component so they can be
// unit-tested in isolation. Every function here is pure: it takes plain data
// (and, for the URL helpers, an `href` string instead of reading
// `window.location`) and returns a value — never mutating its input and never
// touching reactive `$state`, which stays owned by the component.

import type { AnalysisListItem } from '$lib/api/analyses';

/** Sortable columns of the saved-analyses table. */
export type SortKey = 'name' | 'ownerEmail' | 'createdAt' | 'updatedAt';

export type SortDir = 'asc' | 'desc';

/** The live, all-client-side filter inputs of the overlay toolbar. */
export interface AnalysisFilters {
  search: string;
  showOwned: boolean;
  showShared: boolean;
  showEditable: boolean;
  showReadable: boolean;
  createdFrom: string;
  createdTo: string;
}

// Params that are never part of a saved Workbench deep-link: the overlay
// toggles plus `savedAnalysis` (the "which saved analysis is loaded" marker).
export const NON_STATE_PARAMS = ['analyses', 'account', 'admin', 'dossier', 'savedAnalysis'];

const DAY_MS = 86_400_000;

/**
 * Filter the list by the toolbar inputs — free-text over name/description/owner,
 * the owned/shared and editable/read-only toggles, and an inclusive created-date
 * range. Returns a fresh array; never mutates the input.
 */
export function filterAnalyses(
  items: readonly AnalysisListItem[],
  filters: AnalysisFilters
): AnalysisListItem[] {
  const q = filters.search.trim().toLowerCase();
  const from = filters.createdFrom ? new Date(filters.createdFrom).getTime() : null;
  // +1 day so the upper bound is inclusive of the whole selected day.
  const to = filters.createdTo ? new Date(filters.createdTo).getTime() + DAY_MS : null;
  return items.filter((a) => {
    if (q && !`${a.name} ${a.description} ${a.ownerEmail}`.toLowerCase().includes(q)) return false;
    if (!filters.showOwned && a.owned) return false;
    if (!filters.showShared && !a.owned) return false;
    if (!filters.showEditable && a.permission === 'editable') return false;
    if (!filters.showReadable && a.permission === 'readable') return false;
    const t = new Date(a.createdAt).getTime();
    if (from !== null && t < from) return false;
    if (to !== null && t >= to) return false;
    return true;
  });
}

/**
 * Sort a (already filtered) list by the active column + direction. Date columns
 * compare by timestamp; text columns by locale. Returns a fresh array.
 */
export function sortAnalyses(
  rows: readonly AnalysisListItem[],
  sortKey: SortKey,
  sortDir: SortDir
): AnalysisListItem[] {
  const dir = sortDir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    const x = a[sortKey] ?? '';
    const y = b[sortKey] ?? '';
    if (sortKey === 'createdAt' || sortKey === 'updatedAt') {
      return (new Date(x).getTime() - new Date(y).getTime()) * dir;
    }
    return String(x).localeCompare(String(y)) * dir;
  });
}

/**
 * Next (key, dir) after clicking a column header: toggle direction when the
 * same column is re-clicked, else select the column with its natural default
 * (dates descend newest-first, text ascends A→Z).
 */
export function nextSort(
  cur: { key: SortKey; dir: SortDir },
  key: SortKey
): { key: SortKey; dir: SortDir } {
  if (cur.key === key) {
    return { key, dir: cur.dir === 'asc' ? 'desc' : 'asc' };
  }
  return { key, dir: key === 'createdAt' || key === 'updatedAt' ? 'desc' : 'asc' };
}

/** The header arrow glyph for `key` given the active sort, or '' when inactive. */
export function sortArrow(sortKey: SortKey, sortDir: SortDir, key: SortKey): string {
  return sortKey === key ? (sortDir === 'asc' ? '▲' : '▼') : '';
}

/** Format an ISO date as a short local date, or an em-dash for an invalid one. */
export function fmtDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? '—' : d.toLocaleDateString();
}

/**
 * The current view as a re-openable Workbench deep-link: the path + search with
 * the non-state overlay params stripped. A saved analysis IS this string.
 */
export function stripDeepLink(href: string, nonStateParams: readonly string[]): string {
  const u = new URL(href);
  for (const k of nonStateParams) u.searchParams.delete(k);
  return u.pathname + (u.searchParams.toString() ? `?${u.searchParams}` : '');
}

/**
 * "Save current view" only makes sense on a configured Workbench: the
 * `/workbench` path with at least one non-empty pillar (`aleph`/`episteme`/
 * `rhizome`) in the URL grammar.
 */
export function isSaveableWorkbenchUrl(href: string): boolean {
  const u = new URL(href);
  if (!u.pathname.startsWith('/workbench')) return false;
  return ['aleph', 'episteme', 'rhizome'].some((k) => (u.searchParams.get(k) ?? '') !== '');
}

/**
 * The still-editable saved analysis the current view was opened from (tagged via
 * `?savedAnalysis=<id>`), or null — so a later "Save" can offer to update it in
 * place rather than only ever creating a copy.
 */
export function findEditableLoaded(
  items: readonly AnalysisListItem[],
  id: string | null | undefined
): AnalysisListItem | null {
  if (!id) return null;
  return items.find((a) => a.id === id && a.permission === 'editable') ?? null;
}
