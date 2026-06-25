import { describe, expect, it } from 'vitest';
import {
  FADE_MS,
  HARD_CAP_MS,
  MIN_VISIBLE_MS,
  SHOW_DELAY_MS,
  splashPhase,
  splashVisible
} from '../../src/lib/state/boot-timing';

// Phase 149 — the anti-flicker contract of the boot splash. The whole reason
// this logic is pure is so these timing guarantees are pinned by tests rather
// than eyeballed in the browser: a fast load must never flash the splash, and a
// shown splash must never blink out.

const MOUNT = 1000; // arbitrary clock origin; only deltas matter

describe('splashPhase', () => {
  it('stays hidden (pending) before the show-delay while still booting', () => {
    expect(splashPhase(MOUNT, null, MOUNT)).toBe('pending');
    expect(splashPhase(MOUNT, null, MOUNT + SHOW_DELAY_MS - 1)).toBe('pending');
  });

  it('appears once the show-delay elapses and boot has not finished', () => {
    expect(splashPhase(MOUNT, null, MOUNT + SHOW_DELAY_MS)).toBe('visible');
    expect(splashPhase(MOUNT, null, MOUNT + 1000)).toBe('visible');
  });

  it('NEVER shows when boot completes before the show-delay (fast load)', () => {
    const readyMs = MOUNT + SHOW_DELAY_MS - 20; // ready just before it would appear
    expect(splashPhase(MOUNT, readyMs, MOUNT)).toBe('done');
    expect(splashPhase(MOUNT, readyMs, MOUNT + SHOW_DELAY_MS - 20)).toBe('done');
    expect(splashVisible(splashPhase(MOUNT, readyMs, MOUNT + 5))).toBe(false);
  });

  it('holds for the minimum visible window once shown, even if boot finishes immediately after', () => {
    // Becomes ready right after it appears → must still linger MIN_VISIBLE_MS.
    const readyMs = MOUNT + SHOW_DELAY_MS + 10;
    const fadeStart = MOUNT + SHOW_DELAY_MS + MIN_VISIBLE_MS;
    expect(splashPhase(MOUNT, readyMs, fadeStart - 1)).toBe('visible');
    expect(splashPhase(MOUNT, readyMs, fadeStart)).toBe('fading');
    expect(splashPhase(MOUNT, readyMs, fadeStart + FADE_MS - 1)).toBe('fading');
    expect(splashPhase(MOUNT, readyMs, fadeStart + FADE_MS)).toBe('done');
  });

  it('fades starting at readiness when readiness outlasts the minimum window', () => {
    // Ready well after the min-visible floor → fade is gated on readiness.
    const readyMs = MOUNT + SHOW_DELAY_MS + MIN_VISIBLE_MS + 2000;
    expect(splashPhase(MOUNT, readyMs, readyMs - 1)).toBe('visible');
    expect(splashPhase(MOUNT, readyMs, readyMs)).toBe('fading');
    expect(splashPhase(MOUNT, readyMs, readyMs + FADE_MS)).toBe('done');
  });

  it('failsafe: clears even if readiness never arrives (hard cap)', () => {
    expect(splashPhase(MOUNT, null, MOUNT + HARD_CAP_MS - 1)).toBe('visible');
    // At the cap it behaves as if ready fired at the cap: fade, then done.
    const fadeStart = Math.max(MOUNT + SHOW_DELAY_MS + MIN_VISIBLE_MS, MOUNT + HARD_CAP_MS);
    expect(splashPhase(MOUNT, null, fadeStart)).toBe('fading');
    expect(splashPhase(MOUNT, null, fadeStart + FADE_MS)).toBe('done');
  });
});

describe('splashVisible', () => {
  it('paints only during visible and fading', () => {
    expect(splashVisible('pending')).toBe(false);
    expect(splashVisible('visible')).toBe(true);
    expect(splashVisible('fading')).toBe(true);
    expect(splashVisible('done')).toBe(false);
  });
});
