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
        // Imports `$lib/state/url.svelte` (the Svelte-5 rune state layer), so it
        // cannot load under node-env Vitest — same rationale as `**/*.svelte.ts`.
        // Its pure logic is extracted to `panel-mutators-pure.ts` (floored here).
        'src/lib/workbench/panel-mutators.ts',
        // Depends on the browser-only WebAuthentication API
        // (`navigator.credentials`); no node equivalent — E2E/manual territory,
        // same rationale as the otel.ts browser bootstrap (ADR-041).
        'src/lib/api/webauthn-browser.ts',
        // Rune-coupled shell: after the pure logic moved to `pillar-internals.ts`
        // (floored + tested here) only `pickPillar`/`activePillarDefinition`
        // remain, and they read/write the `$lib/state/url.svelte` rune store —
        // unloadable under node-env Vitest. Same rationale as panel-mutators.ts.
        'src/lib/pillar.ts',
        '**/*.d.ts',
        'src/lib/api/types.ts',
        '**/*.{test,spec}.ts'
      ],
      reporter: ['text-summary'],
      // Phase-142 COMPLETE: at the ADR-041 80% floor on all four metrics
      // (measured stmts 89.8 / branch 83.7 / funcs 95.5 / lines 92.3 after the
      // pillar-internals extraction + cooccurrence-network-shared + branch tests).
      thresholds: { lines: 80, statements: 80, functions: 80, branches: 80 }
    }
  }
});
