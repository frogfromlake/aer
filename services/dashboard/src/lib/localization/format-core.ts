// Locale-aware Intl formatting — PURE core (Phase 144 / ADR-042). Frame­work-
// free: every function takes an explicit `loc`, so it is unit-testable under
// node-env Vitest and safe to import from other pure modules (e.g. the
// l5-evidence helpers). The rune-aware ergonomic wrappers live in `format.ts`,
// which defaults `loc` to the `locale` rune.
//
// `en-CA` is deliberate for English: it yields the ISO-like (YYYY-MM-DD) dates
// the dashboard already shipped, so localisation introduces no EN visual churn.
// `de-DE` gives German conventions (DD.MM.YYYY, comma decimals).
import type { Locale } from '$lib/state/url-internals';

const INTL_LOCALE: Record<Locale, string> = { en: 'en-CA', de: 'de-DE' };

/** The BCP-47 tag to hand to `Intl` for a UI locale. */
export function intlLocale(loc: Locale): string {
  return INTL_LOCALE[loc];
}

/** Date only. No `opts` → ISO-like in EN, DD.MM.YYYY in DE. Falls back to the
 *  raw input on an unparseable date (never renders "Invalid Date"). */
export function localizedDate(iso: string, loc: Locale, opts?: Intl.DateTimeFormatOptions): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  try {
    return d.toLocaleDateString(intlLocale(loc), opts);
  } catch {
    return iso;
  }
}

/** Date + time. Defaults to the medium-date / short-time pairing the L5
 *  evidence reader used before localisation. */
export function localizedDateTime(
  iso: string,
  loc: Locale,
  opts: Intl.DateTimeFormatOptions = { dateStyle: 'medium', timeStyle: 'short' }
): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  try {
    return d.toLocaleString(intlLocale(loc), opts);
  } catch {
    return iso;
  }
}

/** Locale-aware number formatting (thousands separators, decimal mark). */
export function localizedNumber(
  value: number,
  loc: Locale,
  opts?: Intl.NumberFormatOptions
): string {
  try {
    return value.toLocaleString(intlLocale(loc), opts);
  } catch {
    return String(value);
  }
}
