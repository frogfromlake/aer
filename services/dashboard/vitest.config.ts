import { defineConfig } from 'vitest/config';

// Phase 142 / ADR-041 — the dashboard coverage floor governs the measurable
// `$lib` TypeScript logic layer ONLY. Svelte components are not unit-rendered
// here (node env, Svelte-5 runes unsupported under jsdom) — they are covered
// by Playwright E2E + svelte-check, NOT by a Vitest line-coverage number, so
// `.svelte` is excluded from the denominator. Component logic should be lifted
// into companion `.ts` (which IS floored here). Threshold ratchets to 80 in
// Phase-142 Step 5.
export default defineConfig({
  test: {
    include: ['tests/unit/**/*.{test,spec}.{js,ts}'],
    environment: 'node',
    globals: false,
    coverage: {
      enabled: true,
      provider: 'v8',
      include: ['src/lib/**/*.ts', 'src/routes/**/*.ts'],
      exclude: ['**/*.svelte', '**/*.d.ts', 'src/lib/api/types.ts', '**/*.{test,spec}.ts'],
      reporter: ['text-summary'],
      thresholds: { lines: 58, statements: 58, functions: 50, branches: 55 }
    }
  }
});
