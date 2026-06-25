// Boot-splash timing — the pure, rune-free core of the app-bootstrap loading
// screen. Kept in a plain `.ts` (no Svelte runes) so vitest can import it
// without a compiler pass; the rune-backed readiness store lives in
// `boot.svelte.ts` and re-exports these.
//
// The splash bridges the two blank moments of startup — the session probe in
// the (app) layout and the lazy-loaded 3D engine — and clears once the globe is
// interactive. The whole point of this module is ANTI-FLICKER: on a fast load
// the splash must never blink into view, and once shown it must never blink
// out. Three thresholds enforce that:
//
//   SHOW_DELAY_MS   The splash is withheld for this long. If boot completes
//                   first, the splash is never rendered at all (no flash on a
//                   fast machine / warm cache).
//   MIN_VISIBLE_MS  Once shown, the splash stays at least this long before it
//                   may fade — so a splash that DID appear never blinks out.
//   FADE_MS         Cross-fade duration onto the globe, so even a brief splash
//                   reads as intentional rather than a glitch.
//
//   HARD_CAP_MS     Failsafe: if readiness never arrives (engine error, dead
//                   query), the splash fades anyway after this cap rather than
//                   trapping the user behind it forever.

export const SHOW_DELAY_MS = 160;
export const MIN_VISIBLE_MS = 480;
export const FADE_MS = 320;
export const HARD_CAP_MS = 8000;

/**
 * Splash lifecycle phase. `pending` (not yet shown) and `done` (finished) both
 * render NOTHING; only `visible` and `fading` paint the overlay. A consumer
 * advances `nowMs` (e.g. via requestAnimationFrame) until the phase reaches
 * `done`, then stops.
 */
export type SplashPhase = 'pending' | 'visible' | 'fading' | 'done';

/** True when the phase paints the overlay (drives the `{#if}` in BootSplash). */
export function splashVisible(phase: SplashPhase): boolean {
  return phase === 'visible' || phase === 'fading';
}

/**
 * Pure splash state machine. Given the boot start (`mountMs`), the instant
 * readiness was reached (`readyMs`, or `null` if still booting) and the current
 * time (`nowMs`), return which phase the splash is in. All inputs share one
 * clock (caller's choice of `performance.now()` / `Date.now()`); the function
 * only ever subtracts them, so the origin is irrelevant.
 */
export function splashPhase(mountMs: number, readyMs: number | null, nowMs: number): SplashPhase {
  const elapsed = nowMs - mountMs;

  // Failsafe — treat a never-arriving readiness as ready at the hard cap so the
  // splash always clears.
  let effectiveReadyMs = readyMs;
  if (effectiveReadyMs === null && elapsed >= HARD_CAP_MS) {
    effectiveReadyMs = mountMs + HARD_CAP_MS;
  }

  // Still booting: invisible until the show-delay elapses, then a live splash.
  if (effectiveReadyMs === null) {
    return elapsed < SHOW_DELAY_MS ? 'pending' : 'visible';
  }

  // Boot finished before the splash would ever have appeared → never show it.
  if (effectiveReadyMs - mountMs < SHOW_DELAY_MS) {
    return 'done';
  }

  // The splash became visible at the show-delay; it must linger for the minimum
  // hold AND until readiness, whichever is later, before fading out.
  const shownAtMs = mountMs + SHOW_DELAY_MS;
  const fadeStartMs = Math.max(shownAtMs + MIN_VISIBLE_MS, effectiveReadyMs);
  if (nowMs < fadeStartMs) return 'visible';
  if (nowMs < fadeStartMs + FADE_MS) return 'fading';
  return 'done';
}
