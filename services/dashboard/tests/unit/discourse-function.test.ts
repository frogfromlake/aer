import { describe, expect, it } from 'vitest';

import {
  DISCOURSE_FUNCTIONS,
  FUNCTION_DEFINITIONS,
  FUNCTION_DEFINITIONS_ORDERED,
  FUNCTION_INFO_HREF,
  getFunctionDef
} from '../../src/lib/discourse-function';

// Phase 122h / ADR-033 §4 / 142 — the four WP-001 §3 discourse functions are the
// SoT referenced from every UI location. These tests pin the taxonomy keys, the
// abbreviations, the lookup escape valve, and the canonical info anchor.

describe('DISCOURSE_FUNCTIONS', () => {
  it('lists the four canonical keys in taxonomy order', () => {
    expect(DISCOURSE_FUNCTIONS).toEqual([
      'epistemic_authority',
      'power_legitimation',
      'cohesion_identity',
      'subversion_friction'
    ]);
  });
});

describe('FUNCTION_DEFINITIONS', () => {
  it('carries the two-letter abbreviation for every function', () => {
    expect(FUNCTION_DEFINITIONS.epistemic_authority.abbr).toBe('EA');
    expect(FUNCTION_DEFINITIONS.power_legitimation.abbr).toBe('PL');
    expect(FUNCTION_DEFINITIONS.cohesion_identity.abbr).toBe('CI');
    expect(FUNCTION_DEFINITIONS.subversion_friction.abbr).toBe('SF');
  });

  it('every definition is self-consistent (key matches the map slot) and WP-001-anchored', () => {
    for (const key of DISCOURSE_FUNCTIONS) {
      const def = FUNCTION_DEFINITIONS[key];
      expect(def.key).toBe(key);
      expect(def.label.length).toBeGreaterThan(0);
      expect(def.color).toMatch(/^#[0-9a-f]{6}$/i);
      expect(def.description).toContain('WP-001 §3');
    }
  });
});

describe('getFunctionDef', () => {
  it('returns the definition for a known key', () => {
    expect(getFunctionDef('power_legitimation')?.abbr).toBe('PL');
  });

  it('returns null for an unknown key', () => {
    expect(getFunctionDef('not_a_function')).toBeNull();
  });

  it('returns null for null/undefined/empty input', () => {
    expect(getFunctionDef(null)).toBeNull();
    expect(getFunctionDef(undefined)).toBeNull();
    expect(getFunctionDef('')).toBeNull();
  });
});

describe('FUNCTION_DEFINITIONS_ORDERED', () => {
  it('mirrors DISCOURSE_FUNCTIONS order', () => {
    expect(FUNCTION_DEFINITIONS_ORDERED.map((d) => d.key)).toEqual([...DISCOURSE_FUNCTIONS]);
  });
});

describe('FUNCTION_INFO_HREF', () => {
  it('points at the WP-001 §3 section', () => {
    expect(FUNCTION_INFO_HREF).toBe('/reflection/wp/wp-001?section=3');
  });
});
