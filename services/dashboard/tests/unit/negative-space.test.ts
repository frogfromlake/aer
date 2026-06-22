import { describe, expect, it } from 'vitest';

import {
  NS_CLASSES,
  NS_CLASS_DEFINITIONS,
  NS_CLASS_DEFINITIONS_ORDERED,
  getNSClassDef,
  classifyNegativeSpace,
  THIN_CONTENT_WORD_FLOOR,
  LIVE_TICKER_REVISION_FLOOR,
  type NSClass
} from '../../src/lib/negative-space';

// Phase 122d.2 — the NS vocabulary + classifier are a single source of truth;
// pin both so a drift is a deliberate, reviewed change.

describe('NS class vocabulary', () => {
  it('defines exactly the eight shipped classes', () => {
    expect([...NS_CLASSES]).toEqual([
      'structural_metadata_absence',
      'temporal_provenance_absence',
      'silent_edit',
      'analytical_capability_absence',
      'k_anonymity_suppression',
      'equivalence_refusal',
      'thin_content',
      'live_ticker'
    ]);
  });

  it('every class has a complete, methodological-register definition', () => {
    for (const key of NS_CLASSES) {
      const def = NS_CLASS_DEFINITIONS[key];
      expect(def.key).toBe(key);
      expect(def.abbr.length).toBeGreaterThan(0);
      expect(def.label.length).toBeGreaterThan(0);
      expect(def.color).toMatch(/^#[0-9a-f]{6}$/i);
      expect(def.wpAnchor).toMatch(/^\/reflection\/wp\/wp-/);
      // Register discipline: never quality-framing language.
      expect(def.description.toLowerCase()).not.toMatch(/broken|defect|missing data|bad/);
    }
  });

  it('ordered list mirrors NS_CLASSES order', () => {
    expect(NS_CLASS_DEFINITIONS_ORDERED.map((d) => d.key)).toEqual([...NS_CLASSES]);
  });

  it('getNSClassDef resolves known keys and returns null for unknown', () => {
    expect(getNSClassDef('silent_edit')?.label).toBe('Silent Edit');
    const tc = getNSClassDef('thin_content');
    expect(tc?.label).toBe('Thin Content');
    expect(tc?.description).toMatch(/WP-007/);
    const lt = getNSClassDef('live_ticker');
    expect(lt?.label).toBe('Live Ticker');
    expect(lt?.description).toMatch(/WP-007/);
    expect(getNSClassDef('nope')).toBeNull();
    expect(getNSClassDef(null)).toBeNull();
  });
});

describe('classifyNegativeSpace (per-article)', () => {
  it('flags Temporal-Provenance-Absence only for fetch_at_fallback', () => {
    expect(classifyNegativeSpace({ timestampSource: 'fetch_at_fallback' })).toEqual([
      'temporal_provenance_absence'
    ]);
    expect(classifyNegativeSpace({ timestampSource: 'json_ld_published' })).toEqual([]);
    expect(classifyNegativeSpace({ timestampSource: '' })).toEqual([]);
    expect(classifyNegativeSpace({ timestampSource: null })).toEqual([]);
  });

  it('flags Silent-Edit when headline changed', () => {
    expect(classifyNegativeSpace({ hasHeadlineChange: true })).toEqual(['silent_edit']);
    expect(classifyNegativeSpace({ hasHeadlineChange: false })).toEqual([]);
  });

  it('flags Thin-Content below the word floor only, and never for an unknown count', () => {
    expect(classifyNegativeSpace({ wordCount: 12 })).toEqual(['thin_content']);
    expect(classifyNegativeSpace({ wordCount: THIN_CONTENT_WORD_FLOOR - 1 })).toEqual([
      'thin_content'
    ]);
    expect(classifyNegativeSpace({ wordCount: THIN_CONTENT_WORD_FLOOR })).toEqual([]);
    expect(classifyNegativeSpace({ wordCount: 800 })).toEqual([]);
    // Disclose only what we measure — an unknown count must not flag.
    expect(classifyNegativeSpace({ wordCount: null })).toEqual([]);
    expect(classifyNegativeSpace({})).toEqual([]);
  });

  it('flags Live-Ticker at or above the revision floor, never below or unknown', () => {
    expect(classifyNegativeSpace({ chainLength: LIVE_TICKER_REVISION_FLOOR })).toEqual([
      'live_ticker'
    ]);
    expect(classifyNegativeSpace({ chainLength: LIVE_TICKER_REVISION_FLOOR + 5 })).toEqual([
      'live_ticker'
    ]);
    expect(classifyNegativeSpace({ chainLength: LIVE_TICKER_REVISION_FLOOR - 1 })).toEqual([]);
    expect(classifyNegativeSpace({ chainLength: 3 })).toEqual([]);
    expect(classifyNegativeSpace({ chainLength: null })).toEqual([]);
  });

  it('a row can belong to multiple classes', () => {
    const classes = classifyNegativeSpace({
      timestampSource: 'fetch_at_fallback',
      hasHeadlineChange: true
    });
    expect(classes).toContain('temporal_provenance_absence');
    expect(classes).toContain('silent_edit');
    expect(classes).toHaveLength(2);
  });

  it('a clean row yields no classes', () => {
    const none: NSClass[] = classifyNegativeSpace({
      timestampSource: 'open_graph_published',
      hasHeadlineChange: false
    });
    expect(none).toEqual([]);
  });
});
