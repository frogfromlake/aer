import { afterAll, describe, expect, it } from 'vitest';
import { OPEN_QUESTIONS, getOpenQuestion } from '../../src/lib/reflection/open-questions';
import { OPEN_QUESTIONS_DE_PROSE } from '../../src/lib/reflection/open-questions.de';
import { overwriteGetLocale } from '../../src/lib/paraglide/runtime';

// Phase 144c — EN⇔DE parity gate for the Open Research Questions prose catalog.
// The German renderings live in a per-locale static sibling (open-questions.de.ts)
// keyed by the shared question `id`; this test is the structural guarantee the
// messages/ parity gate (check-i18n-parity.mjs) cannot give for a TS data table:
// every English question has a populated German counterpart, no orphans, and the
// accessors actually overlay the German prose when the locale resolves to `de`.

describe('open-questions EN⇔DE prose parity', () => {
  const enIds = OPEN_QUESTIONS.map((q) => q.id);

  // The other unit suites assert English content under the base locale; make sure
  // a `de` override from this file never leaks out of it.
  afterAll(() => overwriteGetLocale(() => 'en'));

  it('every EN question has a DE prose entry with the same id', () => {
    const missing = enIds.filter((id) => !(id in OPEN_QUESTIONS_DE_PROSE));
    expect(missing).toEqual([]);
  });

  it('has no orphan DE entries without a matching EN question', () => {
    const enSet = new Set(enIds);
    const orphans = Object.keys(OPEN_QUESTIONS_DE_PROSE).filter((id) => !enSet.has(id));
    expect(orphans).toEqual([]);
  });

  it('DE entry count equals EN question count', () => {
    expect(Object.keys(OPEN_QUESTIONS_DE_PROSE)).toHaveLength(OPEN_QUESTIONS.length);
  });

  it('required DE fields are populated; optional fields mirror EN presence', () => {
    for (const q of OPEN_QUESTIONS) {
      const de = OPEN_QUESTIONS_DE_PROSE[q.id];
      expect(de, q.id).toBeDefined();
      if (!de) continue; // narrowed; the assertion above already failed the test
      expect(de.disciplinaryScope.trim().length, `${q.id} disciplinaryScope`).toBeGreaterThan(0);
      expect(de.shortLabel.trim().length, `${q.id} shortLabel`).toBeGreaterThan(0);
      expect(de.question.trim().length, `${q.id} question`).toBeGreaterThan(0);
      // deliverable / pipelineHook: present in DE exactly when present in EN.
      expect(Boolean(de.deliverable), `${q.id} deliverable presence`).toBe(Boolean(q.deliverable));
      expect(Boolean(de.pipelineHook), `${q.id} pipelineHook presence`).toBe(
        Boolean(q.pipelineHook)
      );
      if (de.deliverable)
        expect(de.deliverable.trim().length, `${q.id} deliverable`).toBeGreaterThan(0);
      if (de.pipelineHook)
        expect(de.pipelineHook.trim().length, `${q.id} pipelineHook`).toBeGreaterThan(0);
    }
  });

  it('the DE prose is actually German (differs from the EN source text)', () => {
    // Guards against an accidental copy of the English string into a DE slot.
    for (const q of OPEN_QUESTIONS) {
      const de = OPEN_QUESTIONS_DE_PROSE[q.id];
      if (!de) continue; // parity is asserted above; nothing to compare if absent
      expect(de.shortLabel, `${q.id} shortLabel not translated`).not.toBe(q.shortLabel);
      expect(de.question, `${q.id} question not translated`).not.toBe(q.question);
    }
  });

  it('accessors overlay the German prose when the locale is de', () => {
    overwriteGetLocale(() => 'de');
    const de1 = OPEN_QUESTIONS_DE_PROSE['wp-001-q1'];
    expect(de1).toBeDefined();
    const q = getOpenQuestion('wp-001-q1');
    expect(q?.shortLabel).toBe(de1?.shortLabel);
    expect(q?.question).toBe(de1?.question);
    // Locale-independent structure is preserved through the overlay.
    expect(q?.sourceWp).toBe('wp-001');
    expect(q?.sourceSection).toBe('8');
  });

  it('accessors return the English prose under the base locale', () => {
    overwriteGetLocale(() => 'en');
    const en = OPEN_QUESTIONS.find((x) => x.id === 'wp-001-q1');
    const q = getOpenQuestion('wp-001-q1');
    expect(q?.shortLabel).toBe(en?.shortLabel);
  });
});
