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
      exclude: [
        '**/*.svelte',
        // Svelte-5 rune state (`*.svelte.ts`) is the reactive layer, not pure
        // logic — it cannot run under node-env Vitest (same reason as `.svelte`)
        // and is covered by E2E (ADR-041).
        '**/*.svelte.ts',
        // Browser-only bootstrap: the frontend OpenTelemetry wiring is lazy-
        // loaded post-paint and needs a browser + the OTel web SDK — E2E/manual
        // territory, like the engine-3d WebGL core (ADR-041).
        'src/lib/observability/otel.ts',
        '**/*.d.ts',
        'src/lib/api/types.ts',
        '**/*.{test,spec}.ts'
      ],
      reporter: ['text-summary'],
      // Phase-142 Step 3: ratcheted up from the Step-1 baseline to lock the API-
      // query + auth/analyses + cell-export test gains. Final climb to the
      // ADR-041 80%-line floor lands with the pillar.ts pure-logic extraction
      // (rune-coupled today) + branch coverage.
      thresholds: { lines: 75, statements: 72, functions: 72, branches: 65 }
    }
  }
});
