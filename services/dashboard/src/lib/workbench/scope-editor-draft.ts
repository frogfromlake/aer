// Phase 122k §11 — ScopeEditor draft persistence (one-shot).
//
// The draft is alive ONLY for the immediate back-nav from a populated
// Workbench following an Apply. Any other navigation invalidates it
// instantly. The rule, stated as the user did:
//
//   "Wenn ich vom ScopeEditor in die Workbench/Panel gehe (über welchen
//    Weg auch immer) und dann mit browser return 1x zurück, dann sollte
//    der state gespeichert werden. Bei ALLEN anderen routings geht er
//    sofort verloren."
//
// Implementation:
//   * The Workbench-page auto-open ScopeEditor is the ONLY mount path
//     that reads + writes sessionStorage (via `enableDraftPersistence`).
//   * Even on that path the draft is consumed once-and-only-once:
//     reading uses the snapshot, then the storage is cleared.
//   * Any other ScopeEditor mount (`+Panel`, per-Panel Edit-Scope, edit
//     mode) clears the draft as a side effect — preventing leakage.
//   * The Workbench page additionally clears the draft on any pathname
//     change (SideRail Atmosphere / Dossier / Reflection clicks), so
//     navigating away invalidates the draft even before another editor
//     re-mount.

import type { ScopeGroup } from '$lib/state/url-internals';
import type { DiscourseFunction } from '$lib/discourse-function';

export const DRAFT_KEY = 'aer.scope-editor.lastDraft';

export interface ScopeEditorDraft {
  scopes: ScopeGroup[];
  perGroupLock: (DiscourseFunction | null)[];
}

function browserSession(): Storage | null {
  if (typeof window === 'undefined') return null;
  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

export function saveDraft(draft: ScopeEditorDraft): void {
  const s = browserSession();
  if (!s) return;
  try {
    s.setItem(DRAFT_KEY, JSON.stringify(draft));
  } catch {
    // Quota / serialisation error — silently ignore; the Apply itself
    // still works, the user just loses draft-resume on back-nav.
  }
}

export function loadDraft(): ScopeEditorDraft | null {
  const s = browserSession();
  if (!s) return null;
  try {
    const raw = s.getItem(DRAFT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as ScopeEditorDraft;
    if (!Array.isArray(parsed.scopes) || !Array.isArray(parsed.perGroupLock)) return null;
    return parsed;
  } catch {
    return null;
  }
}

export function clearDraft(): void {
  const s = browserSession();
  if (!s) return;
  try {
    s.removeItem(DRAFT_KEY);
  } catch {
    // Ignore.
  }
}
