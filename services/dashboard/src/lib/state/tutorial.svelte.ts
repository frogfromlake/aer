// Guided-tour runtime state (Svelte 5 rune). Single source of truth for whether
// the interactive onboarding tour is running and which stop it is on. The
// browser-only rune shell; the pure step list + helpers live in
// `tutorial-steps.ts` (node-tested). Persistence mirrors the locale/theme runes:
// a `localStorage 'aer.guide_completed'` flag so a finished/skipped tour does
// not auto-start again (auto-start sequencing itself is wired in a later slice).

import { browser } from '$app/environment';
import { setLeaveGuardSuppressed } from '$lib/workbench/dirty.svelte';
import { clampStepIndex, isLastStep, stepAt, type TutorialStep } from './tutorial-steps';

const STORAGE_KEY = 'aer.guide_completed';

let active = $state(false);
let index = $state(0);

// The URL (path + query) the user was on when the tour started. The Workbench
// segment navigates to a seeded demo panel; restoring this on finish keeps the
// tour NON-DESTRUCTIVE (a user mid-analysis returns exactly where they were).
let returnTo: string | null = null;

// First-visit handshake: the WelcomeAmbient sets this once it has played (or was
// already seen this session), so auto-start never collides with the greeting.
let welcomeSettled = $state(false);

/** Marked by WelcomeAmbient once the greeting is done (or already seen). */
export function settleWelcomeForTour(): void {
  welcomeSettled = true;
}

/** Reactive: has the welcome greeting cleared, so the tour may auto-start? */
export function welcomeSettledForTour(): boolean {
  return welcomeSettled;
}

/** The path+query to restore when the tour ends (null once consumed). */
export function tourReturnTo(): string | null {
  return returnTo;
}

/** Clear the stored return URL (called by the overlay after restoring). */
export function clearTourReturnTo(): void {
  returnTo = null;
}

/** Reactive: is the tour currently running? */
export function isTourActive(): boolean {
  return active;
}

/** Reactive: the current stop index. */
export function tourStepIndex(): number {
  return index;
}

/** Reactive: the current step, or null when the tour is not running. */
export function currentTourStep(): TutorialStep | null {
  return active ? stepAt(index) : null;
}

/** Whether the user has already finished or skipped the tour (persisted). */
export function tourCompleted(): boolean {
  if (!browser) return true;
  try {
    return localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    // Storage blocked (private mode) → treat as completed so we never nag.
    return true;
  }
}

function markCompleted(): void {
  if (!browser) return;
  try {
    localStorage.setItem(STORAGE_KEY, '1');
  } catch {
    /* best-effort */
  }
}

/** Start (or restart) the tour from the first stop. Snapshots the current URL so
 *  the seeded Workbench demo can be undone when the tour ends. */
export function startTour(): void {
  if (browser) returnTo = window.location.pathname + window.location.search;
  index = 0;
  active = true;
  // Suppress the Workbench leave-guard for the whole run — the tour's seeded
  // demo is ephemeral and its real URL is restored on finish (the overlay lifts
  // the suppression after that restore navigation completes).
  setLeaveGuardSuppressed(true);
}

/** Advance to the next stop, or finish if on the last. */
export function nextTourStep(): void {
  if (isLastStep(index)) {
    finishTour();
    return;
  }
  index = clampStepIndex(index + 1);
}

/** Step back to the previous stop (no-op on the first). */
export function prevTourStep(): void {
  index = clampStepIndex(index - 1);
}

/** End the tour and persist that it has been seen. */
export function finishTour(): void {
  active = false;
  markCompleted();
}
