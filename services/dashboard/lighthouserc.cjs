// Lighthouse CI config (Phase 128) — performance / accessibility / best-practices
// budget gate for the static dashboard. Complements, not replaces:
//   • scripts/check-bundle-size.mjs — gzipped initial + lazy-chunk budgets,
//   • tests/e2e/a11y*.spec.ts        — axe WCAG 2.2 AA (zero violations),
//   • the operator hardware fps pass (Operations Playbook).
//
// Targets the PUBLIC, backend-free routes only: the authenticated surfaces
// (Atmosphere / Workbench) need a live BFF + session, which CI does not stand
// up. The login page (SPA-served via the index.html fallback) and a Storybook
// route exercise the shell bundle, hydration, and base a11y/best-practices —
// the regressions a per-commit gate must catch.
//
// Chrome: set CHROME_PATH to a full Chromium (locally + in CI we reuse the one
// Playwright already downloads — see the Makefile `fe-lighthouse` target).
module.exports = {
  ci: {
    collect: {
      startServerCommand: 'pnpm preview',
      startServerReadyPattern: 'Local:',
      startServerReadyTimeout: 60000,
      url: ['http://localhost:4173/login', 'http://localhost:4173/stories/button'],
      numberOfRuns: 1,
      settings: {
        // CI Chromium runs as root in a container; --no-sandbox is required.
        chromeFlags: '--no-sandbox --headless=new'
      }
    },
    assert: {
      assertions: {
        // Accessibility + best-practices are deterministic enough to fail hard.
        // (Axe e2e is the authoritative WCAG gate; this is a coarse score floor.)
        'categories:accessibility': ['error', { minScore: 0.9 }],
        'categories:best-practices': ['error', { minScore: 0.9 }],
        // Performance score is environment-sensitive in CI shared runners, so it
        // warns rather than blocks — the bundle-size gate is the hard perf gate.
        'categories:performance': ['warn', { minScore: 0.8 }],
        'categories:seo': 'off'
      }
    },
    upload: {
      target: 'filesystem',
      outputDir: './lhci-report'
    }
  }
};
