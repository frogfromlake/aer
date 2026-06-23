import { describe, expect, it } from 'vitest';
import {
  buildCellNotes,
  composeReadingGuide,
  readingGuideLines,
  type ReadingGuideInput
} from '../../src/lib/presentations/reading-guide';
import { composeHowToRead } from '../../src/lib/presentations/how-to-read';

const base = (over: Partial<ReadingGuideInput> = {}): ReadingGuideInput => ({
  presentation: 'distribution',
  subjectKind: 'metric',
  metricLabel: 'Sentiment Score',
  composition: 'split',
  normalization: 'raw',
  sourceCount: 3,
  probeCount: 1,
  bins: 50,
  scales: 'shared',
  ...over
});

const q = (g: ReturnType<typeof composeReadingGuide>, question: string) =>
  g.notes.filter((n) => n.question === question);

describe('composeReadingGuide — six-question composition', () => {
  it('emits panel WHAT (template + composition) and MEASURE (metric)', () => {
    const g = composeReadingGuide(base(), 'TEMPLATE LINE');
    const what = q(g, 'what');
    expect(what[0]).toMatchObject({ level: 'panel', text: 'TEMPLATE LINE' });
    expect(what.some((n) => n.dimension === 'split')).toBe(true);
    const measure = q(g, 'measure');
    expect(measure[0]).toMatchObject({ level: 'panel', channel: 'value' });
    expect(measure[0]!.text).toContain('Sentiment Score');
  });

  it('emits SAMPLE scope + window and COMPARE raw', () => {
    const g = composeReadingGuide(base());
    const sample = q(g, 'sample');
    expect(sample.some((n) => n.channel === 'scope')).toBe(true);
    expect(sample.some((n) => n.text.length > 0)).toBe(true); // window note
    const compare = q(g, 'compare').filter((n) => n.level === 'panel');
    expect(compare[0]!.dimension).toBe('raw');
  });

  it('emits a COMPARE anchor for an equivalence-gated normalization', () => {
    const g = composeReadingGuide(base({ normalization: 'zscore' }));
    const panelCompare = q(g, 'compare').find((n) => n.level === 'panel');
    expect(panelCompare!.dimension).toBe('zscore');
    expect(panelCompare!.anchor?.label).toBe('WP-004 §5.3');
  });

  it('uses the field description for a field-driven subject', () => {
    const g = composeReadingGuide(
      base({
        presentation: 'categorical_distribution',
        subjectKind: 'field',
        fieldLabel: 'Section',
        fieldDescription: 'The newspaper section.'
      })
    );
    expect(q(g, 'measure')[0]!.text).toBe('The newspaper section.');
  });

  it("falls back to the intrinsic subject for a 'none' view", () => {
    const topics = composeReadingGuide(
      base({ presentation: 'topic_distribution', subjectKind: 'none' })
    );
    expect(q(topics, 'measure')[0]!.text.toLowerCase()).toContain('topic');
    const net = composeReadingGuide(
      base({ presentation: 'cooccurrence_network', subjectKind: 'none' })
    );
    expect(q(net, 'measure')[0]!.text.toLowerCase()).toContain('entit');
  });

  it('tags scatter channels so the UI can mirror the chart colours', () => {
    const g = composeReadingGuide(
      base({
        presentation: 'metric_scatter',
        subjectKind: 'none',
        x: 'word_count',
        y: 'sentiment',
        color: 'entity_count'
      })
    );
    const enc = q(g, 'encoding');
    expect(enc.some((n) => n.channel === 'x')).toBe(true);
    expect(enc.some((n) => n.channel === 'color')).toBe(true);
  });
});

describe('readingGuideLines — level filtering + export compatibility', () => {
  it('splits panel (config, shared) vs cell (runtime data) notes', () => {
    // Scatter carries cell-level runtime notes (this cell's Pearson r + point
    // count); the config notes (cloud/x×y/channels) + scales are panel-level.
    const g = composeReadingGuide(
      base({
        presentation: 'metric_scatter',
        subjectKind: 'none',
        x: 'a',
        y: 'b',
        r: 0.5,
        renderedCount: 100
      })
    );
    const panel = readingGuideLines(g, 'panel');
    const cell = readingGuideLines(g, 'cell');
    expect(panel.length).toBeGreaterThan(0);
    expect(cell.length).toBeGreaterThan(0);
    expect(readingGuideLines(g).length).toBe(panel.length + cell.length);
  });

  it('emits the shared/free axis note at PANEL level (not duplicated per cell)', () => {
    const g = composeReadingGuide(base({ scales: 'shared' }));
    const compare = g.notes.filter((n) => n.question === 'compare');
    expect(compare.every((n) => n.level === 'panel')).toBe(true);
    expect(readingGuideLines(g, 'cell').some((t) => t.toLowerCase().includes('scale'))).toBe(false);
  });

  it('buildCellNotes text reproduces the legacy composeHowToRead export body', () => {
    // The export path = [templateBase, ...buildCellNotes(...).text]; assert the
    // cell-note text matches composeHowToRead minus its leading template line.
    const facts = { bins: 50, scales: 'shared' as const };
    const legacy = composeHowToRead('distribution', facts, 'TPL');
    const cellText = buildCellNotes('distribution', facts).map((n) => n.text);
    expect(legacy).toEqual(['TPL', ...cellText]);
  });
});
