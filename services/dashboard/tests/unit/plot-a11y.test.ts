import { describe, it, expect } from 'vitest';
import { hasProhibitedAria, sanitizePlotA11y } from '../../src/lib/presentations/plot-a11y';

// Node-env Vitest has no DOM, so we duck-type the tiny Element surface the
// helper touches (tagName / hasAttribute / removeAttribute / querySelectorAll).
// The real SVG behaviour is covered end-to-end by tests/e2e/a11y-app.spec.ts.
function fakeEl(tag: string, attrs: string[], children: Element[] = []): Element {
  const set = new Set(attrs);
  return {
    tagName: tag,
    hasAttribute: (n: string) => set.has(n),
    removeAttribute: (n: string) => void set.delete(n),
    querySelectorAll: () => children as unknown as NodeListOf<Element>,
    // test-only peek
    _attrs: set
  } as unknown as Element;
}

describe('hasProhibitedAria', () => {
  it('is true for a <g> with aria-label and no role', () => {
    expect(hasProhibitedAria(fakeEl('g', ['aria-label']))).toBe(true);
    expect(hasProhibitedAria(fakeEl('G', ['aria-label']))).toBe(true); // case-insensitive
  });

  it('is false when a role is present', () => {
    expect(hasProhibitedAria(fakeEl('g', ['aria-label', 'role']))).toBe(false);
  });

  it('is false for a non-<g> element or a <g> without aria-label', () => {
    expect(hasProhibitedAria(fakeEl('rect', ['aria-label']))).toBe(false);
    expect(hasProhibitedAria(fakeEl('g', []))).toBe(false);
  });
});

describe('sanitizePlotA11y', () => {
  it('is null-safe and returns its input', () => {
    expect(sanitizePlotA11y(null)).toBeNull();
    expect(sanitizePlotA11y(undefined)).toBeUndefined();
  });

  it('strips the prohibited aria-label from the root and from descendants', () => {
    const childA = fakeEl('g', ['aria-label']);
    const childB = fakeEl('g', ['aria-label']);
    const root = fakeEl('g', ['aria-label'], [childA, childB]);

    const out = sanitizePlotA11y(root);

    expect(out).toBe(root);
    expect(root.hasAttribute('aria-label')).toBe(false);
    expect(childA.hasAttribute('aria-label')).toBe(false);
    expect(childB.hasAttribute('aria-label')).toBe(false);
  });

  it('leaves a root that is not a prohibited <g> untouched at the root level', () => {
    const child = fakeEl('g', ['aria-label']);
    const root = fakeEl('svg', [], [child]);

    sanitizePlotA11y(root);

    // svg root keeps whatever it had; only matched descendants are cleaned.
    expect(child.hasAttribute('aria-label')).toBe(false);
  });
});
