import { defineConfig } from 'vitest/config';

// Phase 142 / ADR-041 — the engine-3d coverage floor governs the extractable
// math/state layer (capability/glow/spiderfy/sun). The WebGL render core
// (`engine.ts` + `GlobeControls.ts`) issues three.js/GL calls that need a real
// GL context and is NOT unit-testable — it is covered by visual-regression E2E
// (dashboard `tests/e2e/visual.spec.ts`), so it is excluded from the
// denominator. Shaders/types/barrels carry no testable logic. Threshold
// ratchets to 80 in Phase-142 Step 5.
export default defineConfig({
  test: {
    environment: 'jsdom',
    globals: false,
    include: ['src/**/*.test.ts'],
    coverage: {
      enabled: true,
      provider: 'v8',
      include: ['src/**/*.ts'],
      exclude: [
        'src/engine.ts',
        'src/GlobeControls.ts',
        'src/index.ts',
        'src/types.ts',
        'src/shaders/**',
        '**/*.d.ts',
        '**/*.test.ts'
      ],
      reporter: ['text-summary'],
      thresholds: { lines: 80, statements: 80, functions: 80, branches: 80 }
    }
  }
});
