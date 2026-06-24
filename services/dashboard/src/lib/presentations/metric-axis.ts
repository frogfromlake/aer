// Cyclic-metric axis rendering — Phase 148g.
//
// `publication_hour` (0–23, UTC) and `publication_weekday` (0=Monday … 6=Sunday,
// the worker's Python `datetime.weekday()` — temporal.py) are bounded, cyclic
// integer domains. Rendered with the generic integer axis they read as bare
// ordinals ("0, 2, 4 …" or, worse, rounded fractional histogram edges "0,1,1,2"),
// which a reader cannot map to a clock hour or a weekday. This module gives those
// metrics a real, LOCALISED axis: one bin per value, integer tick positions, and
// a tick formatter that prints "14:00" / "Montag" (weekday names via Intl, so
// they follow the active UI locale).
//
// Pure except for the format closure, which takes the locale string explicitly
// (callers pass the active UI locale) so this module stays unit-testable.

/** Axis LABELS every 2 hours — 24 labels ("00:00 … 23:00") crowd the axis into an
 *  unreadable smear, so we anchor it at the twelve two-hourly marks. The bars keep
 *  per-hour granularity (see `bins: 24` below); only the tick labels are thinned,
 *  and the hover readout still resolves the exact hour. */
const HOUR_TICKS = [0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22];
/** All seven weekday positions (0=Monday … 6=Sunday). */
const WEEKDAY_TICKS = [0, 1, 2, 3, 4, 5, 6];

// 2024-01-01 is a Monday, so `2024-01-01 + w days` maps w=0→Mon … 6→Sun, exactly
// the worker's `datetime.weekday()` convention.
function weekdayName(w: number, locale: string): string {
  const idx = (((Math.round(w) % 7) + 7) % 7) as number;
  const d = new Date(2024, 0, 1 + idx);
  try {
    return new Intl.DateTimeFormat(locale === 'de' ? 'de-DE' : 'en-US', {
      weekday: 'long'
    }).format(d);
  } catch {
    return String(idx);
  }
}

/** A 24-hour clock label, "HH:00" (UTC — the hour the metric is computed in). */
function hourLabel(h: number): string {
  const n = ((Math.round(h) % 24) + 24) % 24;
  return `${String(n).padStart(2, '0')}:00`;
}

/** A minutes-resolved clock time for a *fractional hour mean*: 13.7 → "13:42".
 *  Unlike `hourLabel` (which rounds to a whole hour for an axis tick), this turns
 *  the decimal into real minutes, so the mean is glossed without being rounded
 *  away. A clock time is continuous, so this is a faithful rendering of 13.7 —
 *  not a discrete-hour claim. */
function hourMeanLabel(h: number): string {
  const wrapped = ((h % 24) + 24) % 24;
  let hh = Math.floor(wrapped);
  let mm = Math.round((wrapped - hh) * 60);
  if (mm === 60) {
    mm = 0;
    hh = (hh + 1) % 24;
  }
  return `${String(hh).padStart(2, '0')}:${String(mm).padStart(2, '0')}`;
}

/** A *fractional weekday mean* falls BETWEEN two day names unless it lands exactly
 *  on one: 3.4 → "Thursday–Friday", 3.0 → "Thursday". We never collapse a fraction
 *  to a single nearest day — that would fake a discrete weekday and drop the
 *  decimal (a week has no intuitive sub-day unit the way a clock has minutes). */
function weekdayMeanLabel(w: number, locale: string): string {
  const wrapped = ((w % 7) + 7) % 7;
  const lo = Math.floor(wrapped);
  if (lo === wrapped) return weekdayName(lo, locale);
  const hi = (lo + 1) % 7;
  return `${weekdayName(lo, locale)}–${weekdayName(hi, locale)}`;
}

/** A short, localised gloss that explains a *fractional mean* of a cyclic metric.
 *  A mean publication hour of 13.7 is not "14:00" (rounding would lose the decimal
 *  and fake a discrete hour); it is the clock time 13:42. A mean weekday of 3.4 is
 *  not "Thursday"; it falls between Thursday and Friday. Callers keep the honest
 *  decimal in the readout and ATTACH this gloss so a reader can map the number
 *  onto the clock / week without us collapsing it to one bucket — AĒR's disclose
 *  maxim. Returns null for non-cyclic metrics and non-finite values, so callers
 *  fall through to the bare number. */
export function describeCyclicMean(
  metricName: string,
  value: number,
  locale: string
): string | null {
  if (!Number.isFinite(value)) return null;
  if (metricName === 'publication_hour') return hourMeanLabel(value);
  if (metricName === 'publication_weekday') return weekdayMeanLabel(value, locale);
  return null;
}

/** Compose a readout value for a possibly-cyclic *mean*: the honest formatted
 *  number, plus an "≈ clock/weekday" gloss when the metric is cyclic. Non-cyclic
 *  metrics get the number alone. The number is passed pre-formatted so each cell
 *  keeps its own precision (fmtValue, toFixed(3), …). */
export function formatCyclicMeanReadout(
  metricName: string,
  value: number,
  formattedNumber: string,
  locale: string
): string {
  const gloss = describeCyclicMean(metricName, value, locale);
  return gloss ? `${formattedNumber} ≈ ${gloss}` : formattedNumber;
}

export interface CyclicMetricAxis {
  /** Ideal histogram bin count — one bin per value (so each hour/weekday gets
   *  its own bar instead of fractional-edge bins). */
  bins: number;
  /** Explicit integer tick positions (no rounded-fractional duplicates). */
  ticks: number[];
  /** Tick label for a numeric value, in the given UI locale. */
  format: (value: number, locale: string) => string;
}

const CYCLIC: Readonly<Record<string, CyclicMetricAxis>> = {
  publication_hour: { bins: 24, ticks: HOUR_TICKS, format: (v) => hourLabel(v) },
  publication_weekday: { bins: 7, ticks: WEEKDAY_TICKS, format: (v, loc) => weekdayName(v, loc) }
};

/** The cyclic-axis spec for a metric, or null for a non-cyclic metric (which
 *  keeps the generic numeric/integer axis). */
export function cyclicMetricAxis(metricName: string): CyclicMetricAxis | null {
  return CYCLIC[metricName] ?? null;
}
