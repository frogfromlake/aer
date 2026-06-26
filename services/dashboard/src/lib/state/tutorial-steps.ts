// Guided-tour step list (pure, node-testable) — the structural backbone of the
// interactive onboarding tour. Copy is NOT here: each step's title/body resolve
// in TutorialOverlay.svelte from a `step.id`→Paraglide-message map, so this file
// stays free of the i18n runtime and unit-tests cleanly (like theme-internals).
//
// Slice 1 (chrome tour): stops on the Atmosphere surface (`/`). Slice 2 adds the
// Workbench segment (`/workbench`) before the closing `outro`; the controller
// navigates per `step.route` and, for `/workbench` steps, seeds the demo panel
// below (`DEMO_WORKBENCH_URL`) so the panel chrome the tour explains is on
// screen. Slice 3 (Reflection) extends the same way.

// A self-contained demo Workbench: one focused Aleph distribution panel over
// Probe 0, SPLIT across its two sources (tagesschau · bundesregierung) so the
// panel renders TWO cells. The split is deliberate: the per-cell action bar
// (Configure-this-cell + Zen + each cell's own "How to read") only appears when
// a panel renders >1 cell, and the tour's cell step explains exactly those. The
// base64url pillar payload is computed once via the app encoder (same codec as
// the e2e seeds) and hardcoded so the pure step list needs no codec import. If
// the compact pillar schema ever changes this can go stale — the panel then
// renders empty and the controller skips the missing targets rather than
// breaking; the workbench tour e2e pins it. The tour snapshots + restores the
// user's real URL around this, so it is never destructive.
const DEMO_ALEPH_SEED =
  'eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbInRhZ2Vzc2NoYXUiXX0seyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbImJ1bmRlc3JlZ2llcnVuZyJdfV0sImMiOiJzIiwidiI6ImRpc3RyaWJ1dGlvbiIsIm0iOiJzZW50aW1lbnRfc2NvcmVfc2VudGl3cyIsImwiOiJnIn1dLCJmaSI6MH1dLCJhdyI6MH0';
export const DEMO_WORKBENCH_URL = `/workbench?activePillar=aleph&aleph=${DEMO_ALEPH_SEED}`;

// The bare Workbench (no pillar state) auto-opens the ScopeEditor in create-mode
// — the surface where scope (probes/sources) is defined before any analysis. The
// `scopeeditor` step lands here; the panel/cell steps land on the seeded demo.
export const SCOPE_EDITOR_URL = '/workbench';

/** Where the explanation card sits relative to its highlighted target.
 *  `center` is a target-less card (welcome / outro / globe overview). */
export type StepPlacement = 'top' | 'bottom' | 'left' | 'right' | 'center';

export interface TutorialStep {
  /** Stable id; also the i18n key stem (`tutorial_<id>_title` / `_body`). */
  readonly id: string;
  /** Route the surface must be on for this step (controller navigates there). */
  readonly route: string;
  /** `data-tutorial-id` of the element to spotlight, or null for a centred card. */
  readonly targetId: string | null;
  /** Card placement relative to the target. */
  readonly placement: StepPlacement;
  /** Exact URL the controller navigates to (defaults to `route`). The two
   *  Workbench modes share the `/workbench` pathname but need different URLs:
   *  the `scopeeditor` step lands on the bare surface (auto-opens the editor),
   *  every panel/cell step lands on the demo-seeded URL. */
  readonly nav?: string;
}

export const TUTORIAL_STEPS: readonly TutorialStep[] = [
  // --- Atmosphere & chrome (Slice 1) ---
  { id: 'welcome', route: '/', targetId: null, placement: 'center' },
  { id: 'surfaces', route: '/', targetId: 'rail-surfaces', placement: 'right' },
  { id: 'dossier', route: '/', targetId: 'rail-dossier', placement: 'right' },
  { id: 'analyses', route: '/', targetId: 'rail-analyses', placement: 'right' },
  { id: 'account', route: '/', targetId: 'rail-account', placement: 'right' },
  { id: 'scopechip', route: '/', targetId: 'scope-chip', placement: 'bottom' },
  { id: 'utilities', route: '/', targetId: 'scope-utilities', placement: 'bottom' },
  { id: 'globe', route: '/', targetId: null, placement: 'center' },
  // --- Workbench (Slice 2) ---
  // The working surface starts with the ScopeEditor (bare /workbench), THEN the
  // panel/cell teaching runs over the demo-seeded split panel.
  {
    id: 'scopeeditor',
    route: '/workbench',
    nav: SCOPE_EDITOR_URL,
    targetId: 'wb-scopeeditor',
    placement: 'left'
  },
  {
    id: 'scopegroups',
    route: '/workbench',
    nav: SCOPE_EDITOR_URL,
    targetId: 'wb-scope-groups',
    placement: 'right'
  },
  {
    id: 'scopeapply',
    route: '/workbench',
    nav: SCOPE_EDITOR_URL,
    targetId: 'wb-scope-apply',
    placement: 'top'
  },
  {
    id: 'pillars',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-pillars',
    placement: 'bottom'
  },
  {
    id: 'panel',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-panel',
    placement: 'left'
  },
  {
    id: 'panelcontrols',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-panelcontrols',
    placement: 'bottom'
  },
  {
    id: 'panellabel',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-panel-label',
    placement: 'bottom'
  },
  {
    id: 'cell',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-cell',
    placement: 'left'
  },
  {
    id: 'addpanel',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-add-panel',
    placement: 'bottom'
  },
  {
    id: 'readingguide',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-reading-guide',
    placement: 'top'
  },
  {
    id: 'zen',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-zen',
    placement: 'bottom'
  },
  {
    id: 'save',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'wb-save',
    placement: 'bottom'
  },
  // Transition into Reflection — highlight the rail anchor BEFORE jumping there,
  // so the surface change is not abrupt. Stays on the demo Workbench (the rail is
  // persistent), then the next stop navigates to /reflection.
  {
    id: 'reflectionnav',
    route: '/workbench',
    nav: DEMO_WORKBENCH_URL,
    targetId: 'rail-reflection',
    placement: 'right'
  },
  // --- Reflection (Slice 3) ---
  { id: 'reflection', route: '/reflection', targetId: 'reflect-overview', placement: 'bottom' },
  {
    id: 'reflectionquestions',
    route: '/reflection',
    targetId: 'reflect-entry',
    placement: 'bottom'
  },
  { id: 'reflectionpapers', route: '/reflection', targetId: 'reflect-papers', placement: 'top' },
  {
    id: 'reflectioncatalogues',
    route: '/reflection',
    targetId: 'reflect-catalogues',
    placement: 'top'
  },
  // --- Close ---
  { id: 'outro', route: '/', targetId: null, placement: 'center' }
] as const;

/** Number of stops in the tour. */
export function stepCount(): number {
  return TUTORIAL_STEPS.length;
}

/** Clamp an index into the valid step range (defensive against stale URLs). */
export function clampStepIndex(i: number): number {
  if (!Number.isFinite(i)) return 0;
  return Math.max(0, Math.min(Math.trunc(i), TUTORIAL_STEPS.length - 1));
}

/** The step at `i`, or null if out of range. */
export function stepAt(i: number): TutorialStep | null {
  return TUTORIAL_STEPS[i] ?? null;
}

/** True when `i` is the final stop (Next → finish). */
export function isLastStep(i: number): boolean {
  return i >= TUTORIAL_STEPS.length - 1;
}
