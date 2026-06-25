// App-bootstrap readiness store. Two independent signals gate the boot splash:
//
//   sessionReady  the (app) layout resolved the session probe (GET /auth/me) —
//                 either authenticated, or a terminal state that stops holding
//                 the page (unreachable retry surface / bounce to /login).
//   globeReady    the persistent Atmosphere globe is interactive: the lazy
//                 `@aer/engine-3d` chunk mounted its first frame, OR the
//                 no-WebGL2 fallback decided (there is nothing to load there).
//
// `bootReady()` is the conjunction. The splash (BootSplash.svelte) reads it and
// runs the anti-flicker timing from `./boot-timing`. The signals are
// monotonic — once true they stay true for the page's life; a full reload
// re-creates the module with both false.
//
// Pure timing lives in `./boot-timing` (rune-free, vitest-importable); this
// module owns only the reactive flags and re-exports the timing surface.

export * from './boot-timing';

let sessionReady = $state(false);
let globeReady = $state(false);

/** Mark the session probe resolved (called from the (app) layout). */
export function markSessionReady(): void {
  sessionReady = true;
}

/** Mark the globe interactive (engine first frame, or fallback decided). */
export function markGlobeReady(): void {
  globeReady = true;
}

/** Reactive: true once BOTH the session and the globe are ready. */
export function bootReady(): boolean {
  return sessionReady && globeReady;
}
