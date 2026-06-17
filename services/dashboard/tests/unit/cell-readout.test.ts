import { afterEach, describe, expect, it, vi } from 'vitest';

import {
  fmtValue,
  fmtTimestamp,
  clampReadoutPosition,
  markIndexFromEvent,
  HIDDEN_READOUT
} from '../../src/lib/presentations/cell-readout';

// `markIndexFromEvent` is DOM-bound (closest / ownerSVGElement /
// querySelectorAll); the node env has no DOM lib, so the test below stubs the
// minimal `Element`/`SVGElement` surface it touches (the in-browser verify pass
// covers the real Observable-Plot wiring end-to-end).

// Phase 132 — exact-value hover readout (pure pieces).

describe('fmtValue', () => {
  it('renders integers exact (counts stay clean)', () => {
    expect(fmtValue(0)).toBe('0');
    expect(fmtValue(5)).toBe('5');
    expect(fmtValue(1234)).toBe('1234');
    expect(fmtValue(-7)).toBe('-7');
  });

  it('drops the fraction for large magnitudes (|n| >= 100)', () => {
    expect(fmtValue(123.456)).toBe('123');
    expect(fmtValue(-999.9)).toBe('-1000');
  });

  it('keeps 3 dp for small fractional values', () => {
    expect(fmtValue(0.123456)).toBe('0.123');
    expect(fmtValue(-0.5)).toBe('-0.500');
    expect(fmtValue(12.3)).toBe('12.300');
  });

  it('renders an em-dash for non-finite / nullish', () => {
    expect(fmtValue(null)).toBe('—');
    expect(fmtValue(undefined)).toBe('—');
    expect(fmtValue(NaN)).toBe('—');
    expect(fmtValue(Infinity)).toBe('—');
  });
});

describe('fmtTimestamp', () => {
  it('formats a seconds-epoch as YYYY-MM-DD HH:mm (UTC)', () => {
    // 2026-05-28T13:45:00Z
    const secs = Date.parse('2026-05-28T13:45:00Z') / 1000;
    expect(fmtTimestamp(secs)).toBe('2026-05-28 13:45');
  });

  it('renders an em-dash for non-finite input', () => {
    expect(fmtTimestamp(NaN)).toBe('—');
  });
});

describe('clampReadoutPosition', () => {
  const W = 1000;
  const H = 800;

  it('places the box below-right of the pointer with room to spare', () => {
    expect(clampReadoutPosition(100, 100, 200, 80, W, H)).toEqual({ left: 114, top: 114 });
  });

  it('flips left when the box would overflow the right edge', () => {
    const { left } = clampReadoutPosition(950, 100, 200, 80, W, H);
    // 950 - 14 - 200 = 736
    expect(left).toBe(736);
  });

  it('flips up when the box would overflow the bottom edge', () => {
    const { top } = clampReadoutPosition(100, 780, 200, 80, W, H);
    // 780 - 14 - 80 = 686
    expect(top).toBe(686);
  });

  it('never pushes the box past the top/left margin', () => {
    const { left, top } = clampReadoutPosition(5, 5, 400, 400, W, H);
    expect(left).toBeGreaterThanOrEqual(8);
    expect(top).toBeGreaterThanOrEqual(8);
  });
});

describe('HIDDEN_READOUT', () => {
  it('is an invisible, empty, frozen state', () => {
    expect(HIDDEN_READOUT.visible).toBe(false);
    expect(HIDDEN_READOUT.rows).toEqual([]);
    expect(Object.isFrozen(HIDDEN_READOUT)).toBe(true);
  });
});

describe('markIndexFromEvent', () => {
  // Minimal Element/SVGElement stand-ins covering the surface the resolver
  // touches: `instanceof Element`, `.closest()`, `.ownerSVGElement`,
  // `.querySelectorAll()`.
  class StubElement {
    closestResult: StubElement | null = null;
    ownerSVGElement: StubSvg | null = null;
    closest() {
      return this.closestResult;
    }
  }
  class StubSvg extends StubElement {
    marks: StubElement[] = [];
    querySelectorAll() {
      return this.marks;
    }
  }

  afterEach(() => vi.unstubAllGlobals());

  function stubDom() {
    vi.stubGlobal('Element', StubElement);
    vi.stubGlobal('SVGElement', StubElement);
  }

  it('returns null for a null target (no DOM element)', () => {
    stubDom();
    expect(markIndexFromEvent(null, 'rect')).toBeNull();
  });

  it('returns null when the target is not inside a matching mark', () => {
    stubDom();
    const el = new StubElement(); // closest() → null
    expect(markIndexFromEvent(el as unknown as EventTarget, 'rect')).toBeNull();
  });

  it('returns null when the mark has no owner SVG', () => {
    stubDom();
    const mark = new StubElement();
    const el = new StubElement();
    el.closestResult = mark; // closest finds a mark, but ownerSVGElement is null
    expect(markIndexFromEvent(el as unknown as EventTarget, 'rect')).toBeNull();
  });

  it('returns the DOM-order index of the hovered mark', () => {
    stubDom();
    const svg = new StubSvg();
    const m0 = new StubElement();
    const m1 = new StubElement();
    const m2 = new StubElement();
    svg.marks = [m0, m1, m2];
    m1.ownerSVGElement = svg;
    const el = new StubElement();
    el.closestResult = m1;
    expect(markIndexFromEvent(el as unknown as EventTarget, 'rect')).toBe(1);
  });

  it('returns null when the resolved mark is not in the query set', () => {
    stubDom();
    const svg = new StubSvg();
    svg.marks = [new StubElement()];
    const orphan = new StubElement();
    orphan.ownerSVGElement = svg; // svg exists, but orphan ∉ svg.marks → indexOf -1
    const el = new StubElement();
    el.closestResult = orphan;
    expect(markIndexFromEvent(el as unknown as EventTarget, 'rect')).toBeNull();
  });
});
