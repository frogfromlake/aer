// Workbench leave-guard dirty tracking — Phase 127.
//
// The configured analysis lives entirely in the URL. "Dirty" = the current
// deep-link differs from the last clean baseline (the state at entry, or the
// last explicit save). The comparison runs ONLY at navigation time (in the
// page's `beforeNavigate`), so there is no continuous reactive effect — the
// guard costs nothing until the user actually leaves the Workbench.
//
// Baseline is reset to "clean" at three points: when the Workbench is entered
// or a saved analysis is loaded (the page effect keyed on `savedAnalysis`), and
// on a successful Save (the AnalysesOverlay create/update paths).

let baseline = $state<string | null>(null);

/** Mark the given deep-link as the clean reference (entry, load, or save). */
export function setCleanBaseline(deepLink: string): void {
  baseline = deepLink;
}

/** True when the current deep-link diverges from the clean baseline. */
export function isWorkbenchDirty(currentDeepLink: string): boolean {
  return baseline !== null && currentDeepLink !== baseline;
}

// Guided-tour escape hatch. The tour seeds an EPHEMERAL demo Workbench (and
// briefly visits the bare surface for the ScopeEditor step) and snapshots +
// restores the user's real URL itself, so its programmatic navigation must not
// trip the unsaved-work confirm modal. The tour suppresses the guard for its
// whole run; the workbench `beforeNavigate` consults this before anything else.
let suppressed = false;

/** Suppress (or re-enable) the leave-guard. Set by the guided tour. */
export function setLeaveGuardSuppressed(value: boolean): void {
  suppressed = value;
}

/** True while the leave-guard is suppressed (tour running). */
export function isLeaveGuardSuppressed(): boolean {
  return suppressed;
}
