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
//                        known-limitations-first mode (Phase 112 also
//                        adds per-surface visual behaviour). URL-backed
//                        since Phase 112 (`?negSpace=1`).
//
// `trayOpen` is not URL-backed: it is a transient disclosure that does
// not survive navigation or tab reload. `negativeSpaceActive` IS URL-
// backed so the overlay persists across surface transitions and can be
// deep-linked.

import { setUrl, urlState } from './url.svelte';

const browser = typeof window !== 'undefined';

let _trayOpen = $state(false);

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
  return urlState().negSpace === true;
}

export function setNegativeSpaceActive(next: boolean): void {
  if (!browser) return;
  setUrl({ negSpace: next ? true : null });
}
