// Shared UI state for the Negative Space overlay.
//
// Originally this module also carried the right-edge methodology tray's
// open/closed flag. The tray was retired in favour of an inline
// methodology accordion on Surface II L3 (see FunctionLaneShell), so
// only the URL-backed Negative Space toggle lives here now. The module
// name is preserved to avoid churning import paths across the codebase.
//
// `negativeSpaceActive` is URL-backed (`?negSpace=1`) so the overlay
// persists across surface transitions and can be deep-linked.

import { setUrl, urlState } from './url.svelte';

const browser = typeof window !== 'undefined';

export function negativeSpaceActive(): boolean {
  return urlState().negSpace === true;
}

export function setNegativeSpaceActive(next: boolean): void {
  if (!browser) return;
  setUrl({ negSpace: next ? true : null });
}
