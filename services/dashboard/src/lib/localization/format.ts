// Locale-aware Intl formatting — rune-aware ergonomic wrappers (Phase 144 /
// ADR-042). Components call the no-arg form (`formatNumber(x)`); it defaults
// `loc` to the `locale` rune, so output is reactive to a language switch. The
// pure implementation lives in `format-core.ts` (unit-tested); this thin shell
// is rune-coupled and therefore excluded from the Vitest coverage denominator
// (same pattern as panel-mutators.ts → panel-mutators-pure.ts).
import { locale, type Locale } from '$lib/state/locale.svelte';
import {
  intlLocale as coreIntlLocale,
  localizedDate,
  localizedDateTime,
  localizedNumber
} from './format-core';

/** The BCP-47 tag for the active (or given) UI locale. */
export function intlLocale(loc: Locale = locale()): string {
  return coreIntlLocale(loc);
}

/** Date only, in the active (or given) locale. */
export function formatDate(
  iso: string,
  opts?: Intl.DateTimeFormatOptions,
  loc: Locale = locale()
): string {
  return localizedDate(iso, loc, opts);
}

/** Date + time, in the active (or given) locale. */
export function formatDateTime(
  iso: string,
  opts?: Intl.DateTimeFormatOptions,
  loc: Locale = locale()
): string {
  return localizedDateTime(iso, loc, opts);
}

/** Number, in the active (or given) locale. */
export function formatNumber(
  value: number,
  opts?: Intl.NumberFormatOptions,
  loc: Locale = locale()
): string {
  return localizedNumber(value, loc, opts);
}
