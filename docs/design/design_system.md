# AĒR Design System

*Status: Phase 98 baseline. Extended by later phases; never rewritten.*

This document describes the primitives that compose every AĒR surface: design
tokens, typography, color scales, Epistemic Weight treatments, and the base
component library. It is the concrete operationalization of the Design Brief
(§5.2, §5.5, §5.7, §5.8) and ADR-020 (Frontend Technology).

The canonical source lives under `services/dashboard/src/lib/design/` and
`services/dashboard/src/lib/components/base/`. This document is the
human-readable summary; the code is authoritative.

---

## 1. Design tokens

All tokens are CSS custom properties defined in
`services/dashboard/src/lib/design/tokens.css`. They cover six families:

| Family | Prefix | Examples |
|---|---|---|
| Color — palette | `--color-viridis-*`, `--color-accent*`, `--color-status-*` | `--color-viridis-50`, `--color-accent`, `--color-status-validated` |
| Color — theme | `--color-bg*`, `--color-surface*`, `--color-fg*`, `--color-border*` | `--color-bg`, `--color-surface`, `--color-fg-muted` |
| Typography | `--font-*`, `--font-size-*`, `--font-weight-*`, `--line-height-*` | `--font-ui`, `--font-size-lg`, `--line-height-tight` |
| Spacing | `--space-0` … `--space-9` | `--space-4` (16 px), `--space-6` (32 px) |
| Radius | `--radius-sm` … `--radius-pill` | `--radius-md` (4 px) |
| Elevation | `--elevation-1`, `--elevation-2`, `--elevation-3` | Three layered drop-shadow stops |
| Motion | `--motion-duration-*`, `--motion-ease-*` | `--motion-duration-fast` (160 ms) |
| Focus | `--focus-ring-width`, `--focus-ring-offset`, `--focus-ring-color` | 2 px accent ring with 2 px offset |

No framework-specific tokens. No Tailwind. A future visualization module that
needs a color stop for a probe glow asks for `--color-viridis-50` or a
`viridis(t)` lookup — never for an ad-hoc hex.

### 1.1 Dark-mode-first

Dark is the primary theme (Design Brief §5.5). The default root resolves to
the dark token set. An explicit `data-theme="light"` on the `<html>` element
flips to the accessible light-mode fallback. Users without an explicit theme
preference get their system preference honored via `prefers-color-scheme`.
Theme switching is CSS-only — no JavaScript is required to apply a theme at
paint time.

### 1.2 Reduced motion

`prefers-reduced-motion: reduce` collapses every motion-duration token to
`0.01ms`. Components that depend on a meaningful animation simply complete
instantly. Imperative animation loops (e.g. the 3D globe's auto-rotation in
Phase 99) must additionally check the media query at runtime — CSS alone
cannot disable a JS animation loop.

---

## 2. Typography

Self-hosted via `@fontsource-variable/inter` and `@fontsource/ibm-plex-mono`.
No Google Fonts — the Zero-Trust posture of the rest of the stack extends to
the browser (no third-party requests on page load).

| Role | Family | Weights | Subsets |
|---|---|---|---|
| UI text | **Inter Variable** | 100 – 900 (variable axis) | latin, latin-ext, greek, greek-ext |
| Numerics / code | **IBM Plex Mono** | 400, 600 | latin, latin-ext |

The Greek subsets on Inter cover the archaic breathings required for ἀήρ and
related lexicon. IBM Plex Mono is used for numeric rendering (§5.1 Ockham
commitment: numbers that look like numbers) and inline code samples.

Modular scale at a 1.25 ratio, base 16 px. Nine stops from `--font-size-xs`
(12 px) to `--font-size-4xl` (49 px).

---

## 3. Color scales

`services/dashboard/src/lib/design/viridis.ts` exports two 256-stop arrays and
two interpolators:

- `VIRIDIS_256` / `viridis(t)` — the default perceptually-uniform scale.
- `CIVIDIS_256` / `cividis(t)` — the CVD-friendly variant; use where the
  distinction between adjacent stops must remain legible under deuteranopia
  or protanopia (Nuñez et al., 2018).

No visualization module anywhere in the codebase defines a palette. The
discipline from `visualization_guidelines.md` §1 ("Viridis, no discrete
valence") is enforced by convention *and* by this being the only palette
module in the source tree.

---

## 4. Epistemic Weight

Design Brief §5.8 requires that visual prominence scale with methodological
backing. The treatments live in
`services/dashboard/src/lib/design/epistemic-weight.css` as five CSS classes:

| Class | Treatment |
|---|---|
| `.weight-tier1-unvalidated` | 1.25 px stroke, 0.85 opacity, warm-neutral badge. Default today. |
| `.weight-tier1-validated` | 1.75 px stroke, full opacity, validated badge. |
| `.weight-tier2-validated` | 2 px stroke, full opacity, validated badge. |
| `.weight-tier3` | 1.25 px stroke, 0.7 opacity, dashed pattern, subtle badge. |
| `.weight-expired` | 1.25 px stroke, 0.65 opacity; regions additionally receive `.weight-expired-region` hatched overlay. |

Visualization modules consume these classes rather than hardcoding visual
treatments. Tier classification is read live from the BFF
(`/api/v1/metrics/{metricName}/provenance`) — never from a frontend lookup
table. See Design Brief §5.8 "Tier treatment is not fixed".

---

## 5. Base components

`services/dashboard/src/lib/components/base/` hosts the primitive component
set. Base primitives carry no business logic; they are composed by feature
code into higher-level surfaces.

Landed in Phase 98b:

- `Button.svelte` — primary / secondary / ghost variants, loading state.
- `Dialog.svelte` — WCAG 2.2 AA modal with focus-trap and restore.
- `Tooltip.svelte` — ARIA-described tooltip with keyboard-focus support.
- `Badge.svelte` — status chips consuming Epistemic Weight classes.
- `SkipLink.svelte` — the first focusable element on every page.

Each primitive has an accompanying route-based story under
`services/dashboard/src/routes/stories/<component>/+page.svelte`. The stories
sidebar at `/stories` lists every component and exposes a dark/light theme
toggle that sets `data-theme` on `document.documentElement`. Stories are the
ground truth for the accessible, visual behavior of each component — any new
component must land with a story demonstrating all variants before a later
phase can depend on it.

The route-based harness is first-class: Histoire is not used. At the time of
Phase 98b its Svelte 5 support still lives on the `add-svelte5-support`
feature branch of `histoire-dev/histoire`, and the published
`@histoire/plugin-svelte@1.0.0-beta.1` ships precompiled artifacts that
import from `svelte/internal` (forbidden on Svelte 5). Route stories build
with the same Vite pipeline as the app, which also gives the Phase 98c
Playwright + axe gate a single surface to drive.

---

## 6. Accessibility

`services/dashboard/src/lib/design/a11y.md` documents the ARIA conventions,
the focus-ring contract, the reduced-motion contract, and the axe-core
testing gate that every base component and every surface must pass. See that
file for the exhaustive list.

---

## 7. Visual + a11y gate (Phase 98c)

The story routes double as the harness for two CI gates that run on every
push:

- **Playwright visual regression.** `services/dashboard/tests/e2e/visual.spec.ts`
  navigates each story route at both `data-theme="dark"` and `"light"` and
  compares against committed PNG baselines under
  `services/dashboard/tests/e2e/__snapshots__/`. Dialog is also captured in
  its open state. A diff above `maxDiffPixelRatio: 0.01` fails the build.
- **axe-core a11y gate.** `services/dashboard/tests/e2e/a11y.spec.ts` runs
  `@axe-core/playwright` against every story route plus `/`, tagged with
  `wcag2a`, `wcag2aa`, `wcag21aa`, `wcag22aa`. Any violation fails the build.

Both tests execute inside the pinned image declared as the
`playwright-runner` service in `compose.yaml` (SSoT per Hard Rule 1). Running
inside Docker is mandatory: browser font rendering is OS-sensitive, so
host-local snapshots cannot be trusted against CI. Use `make fe-test-e2e` to
run the gate and `make fe-test-e2e-update` to regenerate committed
baselines after an intentional visual change.

---

## 8. References

- Design Brief §5.2 (No valence, ever)
- Design Brief §5.5 (Long time, long sessions)
- Design Brief §5.7 (Dual-Register Communication)
- Design Brief §5.8 (Epistemic Weight)
- Design Brief §5.9 (Visualization Stack Separation)
- `visualization_guidelines.md` §1 (Viridis discipline)
- ADR-020 (Frontend Technology)
- WCAG 2.2 AA — https://www.w3.org/TR/WCAG22/
