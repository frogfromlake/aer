// Pure aggregation/selection helpers for TopicEvolutionCell — extracted from
// TopicEvolutionCell.svelte (Phase 141) so the bucket-slicing, top-K selection,
// stream-row backfill, and legend math are unit-testable; the component keeps
// only its reactive shell + the Observable-Plot DOM effect.

import type { TopicDistributionResponseDto } from '$lib/api/queries';

export const OUTLIER_TOPIC_ID = -1;
export const UNCATEGORISED_LABEL = 'uncategorised';
export const OUTLIER_COLOUR = '#888888';
export const OTHER_COLOUR = '#5b6677';
// Top-K topics surfaced as named series; everything else folds into an "other"
// stream so the stack does not explode visually.
export const TOP_K = 8;
export const TARGET_BUCKETS = 10;
export const MIN_BUCKETS = 3;
export const MAX_BUCKETS = 14;

// Viridis-ish hand-rolled palette so we don't pull a colour-scale dependency.
// 8 bins + grey for outlier + light slate for "other". Per Phase 121, the
// palette is arbitrary — topic colour carries no valence.
export const VIRIDIS = [
  '#440154',
  '#482677',
  '#3F4A8A',
  '#31678E',
  '#25828E',
  '#1FA088',
  '#3FBC73',
  '#7AD151'
];

// Bucket boundaries — split [windowStart, windowEnd) into N equal sub-windows.
// The total span is in milliseconds; we floor to a reasonable bucket count so
// each bucket holds at least a few hours (a sub-window narrower than the
// BERTopic sweep cadence returns identical aggregates and wastes requests).
export interface Bucket {
  start: string;
  end: string;
  midpoint: string;
}

// `now` is the fallback upper bound when the panel is unbounded — the caller
// passes `Date.now()` so this stays pure/testable.
export function computeBuckets(
  windowStart: string | undefined,
  windowEnd: string | undefined,
  now: number
): Bucket[] {
  // Topic evolution needs an explicit span to slice into buckets. When the
  // panel is unbounded (whole dataset), fall back to the last 30 days — the
  // BERTopic sweep window — so the diachronic view still renders.
  const THIRTY_DAYS = 30 * 24 * 60 * 60 * 1000;
  const t1 = windowEnd ? new Date(windowEnd).getTime() : now;
  const t0 = windowStart ? new Date(windowStart).getTime() : t1 - THIRTY_DAYS;
  if (!Number.isFinite(t0) || !Number.isFinite(t1) || t1 <= t0) return [];
  const spanMs = t1 - t0;
  // Target ~1 bucket per 12h, clamped.
  const TWELVE_H = 12 * 60 * 60 * 1000;
  const desired = Math.round(spanMs / TWELVE_H);
  const n = Math.max(MIN_BUCKETS, Math.min(MAX_BUCKETS, desired || TARGET_BUCKETS));
  const step = spanMs / n;
  const out: Bucket[] = [];
  for (let i = 0; i < n; i++) {
    const s = new Date(t0 + i * step);
    const e = new Date(t0 + (i + 1) * step);
    const mid = new Date(t0 + (i + 0.5) * step);
    out.push({
      start: s.toISOString(),
      end: e.toISOString(),
      midpoint: mid.toISOString()
    });
  }
  return out;
}

export interface SeriesKey {
  topicId: number;
  language: string;
  label: string;
  isOutlier: boolean;
}

export function seriesId(k: SeriesKey): string {
  return `${k.language}::${k.topicId}`;
}

export type SeriesMetaEntry = SeriesKey & { totalArticles: number };

// Aggregate per (language, topic) total volume across every successful
// sub-window payload, so the stream stack + legend stay stable across buckets.
export function buildSeriesMeta(
  payloads: TopicDistributionResponseDto[]
): Record<string, SeriesMetaEntry> {
  const map: Record<string, SeriesMetaEntry> = Object.create(null);
  for (const payload of payloads) {
    for (const t of payload.topics) {
      const isOutlier = t.topicId === OUTLIER_TOPIC_ID;
      const key: SeriesKey = {
        topicId: t.topicId,
        language: t.language || 'und',
        label: isOutlier ? UNCATEGORISED_LABEL : t.label,
        isOutlier
      };
      const id = seriesId(key);
      const prior = map[id];
      if (prior) {
        prior.totalArticles += t.articleCount;
      } else {
        map[id] = { ...key, totalArticles: t.articleCount };
      }
    }
  }
  return map;
}

export function seriesMetaValues(meta: Record<string, SeriesMetaEntry>): SeriesMetaEntry[] {
  return Object.values(meta);
}

// Outlier always kept; other topics ordered by total volume desc, top-K named.
export function selectTopSeriesIds(meta: Record<string, SeriesMetaEntry>): Record<string, true> {
  const entries = seriesMetaValues(meta);
  const outliers = entries.filter((e) => e.isOutlier);
  const named = entries
    .filter((e) => !e.isOutlier)
    .sort((a, b) => b.totalArticles - a.totalArticles)
    .slice(0, TOP_K);
  const out: Record<string, true> = Object.create(null);
  for (const e of [...outliers, ...named]) out[seriesId(e)] = true;
  return out;
}

export function collectLanguages(meta: Record<string, SeriesMetaEntry>): string[] {
  const seen: string[] = [];
  for (const e of seriesMetaValues(meta)) {
    if (!seen.includes(e.language)) seen.push(e.language);
  }
  return seen;
}

export interface PlotRow {
  bucket: Date;
  series: string;
  label: string;
  language: string;
  topicId: number;
  isOutlier: boolean;
  articleCount: number;
}

// Flatten the per-bucket payloads into per-bucket per-series rows: top-K series
// emit their real volume, non-top-K topics fold into a per-language "other"
// stream, and absent top-K series are backfilled with explicit zeros so the
// area mark interpolates cleanly across gaps. `aligned` carries each successful
// payload paired with its bucket (non-success buckets are dropped upstream).
export function buildPlotRows(
  aligned: { bucket: Bucket; payload: TopicDistributionResponseDto }[],
  topSeriesIds: Record<string, true>,
  seriesMeta: Record<string, SeriesMetaEntry>
): PlotRow[] {
  const rows: PlotRow[] = [];
  for (const { bucket: b, payload } of aligned) {
    // Bucket totals — used to fold non-top-K topics into "other".
    const otherByLang: Record<string, number> = Object.create(null);
    const present: Record<string, true> = Object.create(null);
    for (const t of payload.topics) {
      const isOutlier = t.topicId === OUTLIER_TOPIC_ID;
      const lang = t.language || 'und';
      const key: SeriesKey = {
        topicId: t.topicId,
        language: lang,
        label: isOutlier ? UNCATEGORISED_LABEL : t.label,
        isOutlier
      };
      const id = seriesId(key);
      if (topSeriesIds[id]) {
        rows.push({
          bucket: new Date(b.midpoint),
          series: id,
          label: key.label,
          language: lang,
          topicId: t.topicId,
          isOutlier,
          articleCount: t.articleCount
        });
        present[id] = true;
      } else {
        otherByLang[lang] = (otherByLang[lang] ?? 0) + t.articleCount;
      }
    }
    // Backfill explicit zeros for top-K series absent in this bucket
    // so the area mark interpolates cleanly across gaps.
    for (const id of Object.keys(topSeriesIds)) {
      if (present[id]) continue;
      const meta = seriesMeta[id];
      if (!meta) continue;
      rows.push({
        bucket: new Date(b.midpoint),
        series: id,
        label: meta.label,
        language: meta.language,
        topicId: meta.topicId,
        isOutlier: meta.isOutlier,
        articleCount: 0
      });
    }
    // Emit the "other" bucket per language if any non-top-K topics
    // contributed in this bucket.
    for (const [lang, count] of Object.entries(otherByLang)) {
      rows.push({
        bucket: new Date(b.midpoint),
        series: `${lang}::other`,
        label: 'other topics',
        language: lang,
        topicId: -2,
        isOutlier: false,
        articleCount: count
      });
    }
  }
  return rows;
}

export interface LegendEntry {
  id: string;
  label: string;
  language: string;
  isOutlier: boolean;
  isOther: boolean;
  colour: string;
}

// Custom legend — Plot's auto-legend cannot label series with our composed
// `series` ids directly without a colour scale; the entries here match the
// on-chart colour exactly (same orderEntries sort + Viridis index as the
// stacked mark's colourFor in the render effect).
export function buildLegendEntries(
  meta: Record<string, SeriesMetaEntry>,
  topSeriesIds: Record<string, true>,
  languages: string[]
): LegendEntry[] {
  const orderEntries = seriesMetaValues(meta).filter((e) => topSeriesIds[seriesId(e)]);
  orderEntries.sort((a, b) => {
    if (a.isOutlier !== b.isOutlier) return a.isOutlier ? 1 : -1;
    return b.totalArticles - a.totalArticles;
  });
  const out: LegendEntry[] = [];
  const named = orderEntries.filter((e) => !e.isOutlier);
  named.forEach((e, idx) =>
    out.push({
      id: seriesId(e),
      label: e.label,
      language: e.language,
      isOutlier: false,
      isOther: false,
      colour: VIRIDIS[idx % VIRIDIS.length]!
    })
  );
  for (const lang of languages) {
    out.push({
      id: `${lang}::other`,
      label: 'other topics',
      language: lang,
      isOutlier: false,
      isOther: true,
      colour: OTHER_COLOUR
    });
  }
  for (const e of orderEntries.filter((x) => x.isOutlier)) {
    out.push({
      id: seriesId(e),
      label: UNCATEGORISED_LABEL,
      language: e.language,
      isOutlier: true,
      isOther: false,
      colour: OUTLIER_COLOUR
    });
  }
  return out;
}
