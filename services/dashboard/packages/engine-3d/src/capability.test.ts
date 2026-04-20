import { describe, expect, it, vi } from 'vitest';
import { hasWebGL2, prefersReducedMotion } from './capability';

describe('hasWebGL2', () => {
  it('returns true when getContext("webgl2") yields a context', () => {
    const fakeCtx = {} as WebGL2RenderingContext;
    const spy = vi
      .spyOn(HTMLCanvasElement.prototype, 'getContext')
      .mockImplementation(((id: string) => (id === 'webgl2' ? fakeCtx : null)) as never);
    try {
      expect(hasWebGL2()).toBe(true);
    } finally {
      spy.mockRestore();
    }
  });

  it('returns false when getContext returns null', () => {
    const spy = vi
      .spyOn(HTMLCanvasElement.prototype, 'getContext')
      .mockImplementation(() => null as never);
    try {
      expect(hasWebGL2()).toBe(false);
    } finally {
      spy.mockRestore();
    }
  });

  it('swallows exceptions thrown by getContext', () => {
    const spy = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(() => {
      throw new Error('GL boom');
    });
    try {
      expect(hasWebGL2()).toBe(false);
    } finally {
      spy.mockRestore();
    }
  });
});

describe('prefersReducedMotion', () => {
  it('reflects matchMedia.matches', () => {
    const original = window.matchMedia;
    window.matchMedia = ((query: string) =>
      ({
        matches: query.includes('reduce'),
        media: query,
        addEventListener: () => {},
        removeEventListener: () => {}
      }) as unknown as MediaQueryList) as typeof window.matchMedia;
    try {
      expect(prefersReducedMotion()).toBe(true);
    } finally {
      window.matchMedia = original;
    }
  });
});
