import { describe, expect, it } from 'vitest';

import {
  cyclicMetricAxis,
  describeCyclicMean,
  formatCyclicMeanReadout
} from '../../src/lib/presentations/metric-axis';

describe('cyclicMetricAxis (Phase 148g)', () => {
  it('returns null for a non-cyclic metric', () => {
    expect(cyclicMetricAxis('word_count')).toBeNull();
    expect(cyclicMetricAxis('sentiment_score_sentiws')).toBeNull();
  });

  it('formats publication_hour as a 24h clock label, one bin per hour', () => {
    const ax = cyclicMetricAxis('publication_hour');
    expect(ax).not.toBeNull();
    expect(ax!.bins).toBe(24); // per-hour bars
    // …but labels only every 2h, so 24 ticks don't crowd the axis.
    expect(ax!.ticks).toEqual([0, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22]);
    expect(ax!.format(0, 'en')).toBe('00:00');
    expect(ax!.format(14, 'en')).toBe('14:00');
    expect(ax!.format(14, 'de')).toBe('14:00');
    // wraps + rounds defensively
    expect(ax!.format(24, 'en')).toBe('00:00');
  });

  it('formats publication_weekday as localized day names (0=Monday … 6=Sunday)', () => {
    const ax = cyclicMetricAxis('publication_weekday');
    expect(ax).not.toBeNull();
    expect(ax!.bins).toBe(7);
    expect(ax!.ticks).toEqual([0, 1, 2, 3, 4, 5, 6]);
    expect(ax!.format(0, 'en')).toBe('Monday');
    expect(ax!.format(6, 'en')).toBe('Sunday');
    expect(ax!.format(0, 'de')).toBe('Montag');
    expect(ax!.format(2, 'de')).toBe('Mittwoch');
    expect(ax!.format(6, 'de')).toBe('Sonntag');
  });
});

describe('describeCyclicMean (Phase 148g — fractional-mean gloss)', () => {
  it('returns null for non-cyclic metrics and non-finite values', () => {
    expect(describeCyclicMean('word_count', 13.7, 'en')).toBeNull();
    expect(describeCyclicMean('publication_hour', NaN, 'en')).toBeNull();
    expect(describeCyclicMean('publication_hour', Infinity, 'en')).toBeNull();
  });

  it('glosses a fractional hour mean as a minutes-resolved clock time', () => {
    // 0.7 h = 42 min — the decimal is preserved, not rounded to a whole hour.
    expect(describeCyclicMean('publication_hour', 13.7, 'en')).toBe('13:42');
    expect(describeCyclicMean('publication_hour', 0, 'en')).toBe('00:00');
    // minute rounding carries into the next hour without overflowing to 60.
    expect(describeCyclicMean('publication_hour', 13.999, 'en')).toBe('14:00');
    // wraps defensively past midnight.
    expect(describeCyclicMean('publication_hour', 23.75, 'en')).toBe('23:45');
  });

  it('glosses a fractional weekday mean as a span between two day names', () => {
    expect(describeCyclicMean('publication_weekday', 3.4, 'en')).toBe('Thursday–Friday');
    expect(describeCyclicMean('publication_weekday', 3.4, 'de')).toBe('Donnerstag–Freitag');
    // an exact integer mean lands on a single day.
    expect(describeCyclicMean('publication_weekday', 3, 'en')).toBe('Thursday');
    // wraps Sunday → Monday for a fractional mean near the week boundary.
    expect(describeCyclicMean('publication_weekday', 6.5, 'en')).toBe('Sunday–Monday');
  });
});

describe('formatCyclicMeanReadout (Phase 148g)', () => {
  it('attaches the gloss for cyclic metrics, keeping the honest number', () => {
    expect(formatCyclicMeanReadout('publication_hour', 13.7, '13.7', 'en')).toBe('13.7 ≈ 13:42');
    expect(formatCyclicMeanReadout('publication_weekday', 3.4, '3.400', 'de')).toBe(
      '3.400 ≈ Donnerstag–Freitag'
    );
  });

  it('returns the bare number for non-cyclic metrics', () => {
    expect(formatCyclicMeanReadout('sentiment_score_sentiws', 0.42, '0.420', 'en')).toBe('0.420');
  });
});
