// Shared UI state for the right-edge methodology tray (Phase 108).
//
// Two flags live here so that the tray, the surfaces that focus it,
// and the Negative Space toggle can share state without prop-drilling
// across the (app) layout boundary:
//
//   trayOpen           — open/closed for the tray panel (Phase 108
//                        replaces the legacy L4ProvenanceFlyout — the
//                        tray is now the single L4 Provenance surface).
//   negativeSpaceActive — Design Brief §3.4 / §4.4 overlay flag. When
//                        on, the tray's open-state body switches into
//                        known-limitations-first mode (Phase 113 visual
//                        reweighting builds on the same flag).
//
// Both are intentionally not URL-backed: the tray is a transient
// disclosure, and the overlay is a session-level reading mode.

const browser = typeof window !== 'undefined';

let _trayOpen = $state(false);
let _negativeSpaceActive = $state(false);

export function trayOpen(): boolean {
  return _trayOpen;
}

export function setTrayOpen(next: boolean): void {
  if (!browser) return;
  _trayOpen = next;
}

export function toggleTray(): void {
  if (!browser) return;
  _trayOpen = !_trayOpen;
}

export function negativeSpaceActive(): boolean {
  return _negativeSpaceActive;
}

export function setNegativeSpaceActive(next: boolean): void {
  if (!browser) return;
  _negativeSpaceActive = next;
}
