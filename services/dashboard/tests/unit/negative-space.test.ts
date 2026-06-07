import { describe, expect, it } from 'vitest';

import {
  NS_CLASSES,
  NS_CLASS_DEFINITIONS,
  NS_CLASS_DEFINITIONS_ORDERED,
  getNSClassDef,
  classifyNegativeSpace,
  type NSClass
} from '../../src/lib/negative-space';

// Phase 122d.2 — the NS vocabulary + classifier are a single source of truth;
// pin both so a drift is a deliberate, reviewed change.

describe('NS class vocabulary', () => {
  it('defines exactly the six shipped classes', () => {
    expect([...NS_CLASSES]).toEqual([
      'structural_metadata_absence',
      'temporal_provenance_absence',
      'silent_edit',
      'analytical_capability_absence',
      'k_anonymity_suppression',
      'equivalence_refusal'
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
