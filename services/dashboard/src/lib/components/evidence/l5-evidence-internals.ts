// Pure logic extracted from L5EvidenceReader.svelte (Phase 141 decomposition).
// Everything here is framework-free and unit-tested in
// tests/unit/l5-evidence-internals.test.ts — the editorial-walk / diff-chain
// derivation in particular was previously inline-and-untested (Phase 133), so
// the extraction doubles as its first real coverage. The Svelte components
// (L5EvidenceReader, L5DiffTab, L5NegativeSpaceSection, L5RevisionHistory) wrap
// these in `$derived` and keep only render glue.
import { diffWordsWithSpace, type Change } from 'diff';
import type { ArticleRevisionEntryDto } from '$lib/api/queries';
import type { NSClass } from '$lib/negative-space';
import { m } from '../../paraglide/messages.js';

// ── Diff-chain derivation (Phase 133 — editorial versions only) ──────────────

export type DiffView = 'walk' | 'cumulative';

export interface DiffChain {
  // Chain head = the oldest row by array position (`revisionList[0]`); its diff
  // slot carries the head transition (current article body vs the NEWEST
  // snapshot). `revision_index` is contiguous but NOT guaranteed to start at 0
  // (ADR-036 rebuilds can offset it), so key off array position.
  chainHead: ArticleRevisionEntryDto | null;
  // Editorial walk = every row after the head minus the proven-identical
  // re-archivals. `pending` stays in the walk — it MIGHT be a real change once
  // the sweep computes it; only proven re-archivals are dropped.
  walkSteps: ArticleRevisionEntryDto[];
  lookupByIndex: Map<number, ArticleRevisionEntryDto>;
  changedCount: number;
  pendingCount: number;
  identicalCount: number;
  cumulativeAvailable: boolean;
  // Whether the Diff tab has anything worth showing: at least one pair is an
  // editorial change or still computing. When every capture is an identical
  // re-archival the cumulative view would just restate "nothing changed".
  hasEditorialContent: boolean;
}

export function deriveDiffChain(revisionList: ArticleRevisionEntryDto[]): DiffChain {
  const chainHead = revisionList[0] ?? null;
  const walkSteps = revisionList.slice(1).filter((r) => r.diffStatus !== 'identical');
  return {
    chainHead,
    walkSteps,
    lookupByIndex: new Map(revisionList.map((r) => [r.revisionIndex, r])),
    changedCount: revisionList.filter((r) => r.diffStatus === 'changed').length,
    pendingCount: revisionList.filter((r) => r.diffStatus === 'pending').length,
    identicalCount: revisionList.filter((r) => r.diffStatus === 'identical').length,
    cumulativeAvailable: chainHead !== null,
    hasEditorialContent: revisionList.some(
      (r) => r.diffStatus === 'changed' || r.diffStatus === 'pending'
    )
  };
}

// The revisionIndex the diff query actually fetches for the current view.
export function selectedDiffPairIndex(
  chain: DiffChain,
  diffView: DiffView,
  walkPos: number
): number {
  return diffView === 'cumulative'
    ? (chain.chainHead?.revisionIndex ?? -1)
    : (chain.walkSteps[walkPos]?.revisionIndex ?? -1);
}

// ── Negative-Space (silent-edit) signals ─────────────────────────────────────

// L5's rich NS domain is Silent-Edit: it fires when the article was edited
// after publication, republished under a new URL, or its archive history could
// not be established (a Wayback lookup gap = "we don't know", distinct from
// "no edits"). Each fired signal is listed so the marker is self-explaining
// (WP-003 §5.3.1).
export function silentEditSignals(
  revisionList: ArticleRevisionEntryDto[],
  revisionStatus: string
): string[] {
  const s: string[] = [];
  if (revisionList.some((r) => r.diffStatus === 'changed'))
    s.push(m.evidence_signal_edited_after_publication());
  if (revisionList.some((r) => r.trigger === 'republication_trigger'))
    s.push(m.evidence_signal_republished_new_url());
  if (revisionStatus === 'failed' || revisionStatus === 'no_snapshots')
    s.push(m.evidence_signal_archive_history_unestablished());
  return s;
}

export function nsMarkersFor(signals: string[]): NSClass[] {
  return signals.length > 0 ? ['silent_edit'] : [];
}

// ── Pure formatters ──────────────────────────────────────────────────────────

export function formatTs(iso: string): string {
  try {
    return new Date(iso).toLocaleString('en-CA', {
      dateStyle: 'medium',
      timeStyle: 'short'
    });
  } catch {
    return iso;
  }
}

// Phase 122d.3 — discourse-shift delta helpers for the Revision history rows.
// The deltas are later-in-time minus earlier-in-time (the BFF already orients
// chain-head pairs), so a positive sentiment delta means the article reads more
// positively after the edit. Never re-invert.
export function sentimentArrow(d: number): string {
  if (d > 0.0005) return '▲';
  if (d < -0.0005) return '▼';
  return '→';
}

export function fmtDelta(d: number): string {
  return (d >= 0 ? '+' : '') + d.toFixed(3);
}

// Phase 122d.1 BUG-8 — word-level inline diff for `mod` ops via jsdiff. Returns
// an array of Change records the template renders with red/green spans.
export function wordDiff(before: string, after: string): Change[] {
  return diffWordsWithSpace(before ?? '', after ?? '');
}

// Walk-step label — the (before → after) snapshot dates for a consecutive
// editorial pair. `before` is the preceding chain entry (revisionIndex − 1),
// `after` is this step's snapshot.
export function walkStepLabel(
  lookupByIndex: Map<number, ArticleRevisionEntryDto>,
  step: ArticleRevisionEntryDto | undefined
): string {
  if (!step) return '—';
  const before = lookupByIndex.get(step.revisionIndex - 1);
  const b = before ? new Date(before.snapshotAt).toLocaleDateString('en-CA') : '?';
  const a = new Date(step.snapshotAt).toLocaleDateString('en-CA');
  return `${b} → ${a}`;
}

// Cumulative label — Phase 133: the chain-head compares the current article
// body to the NEWEST snapshot (the latest archived state), i.e. "what the
// publisher changed since the last archive". Read newest → current.
export function cumulativeLabel(revisionList: ArticleRevisionEntryDto[]): string {
  const newest = revisionList[revisionList.length - 1]?.snapshotAt;
  if (!newest) return m.evidence_cumulative_label_no_date();
  return m.evidence_cumulative_label({ date: new Date(newest).toLocaleDateString('en-CA') });
}

export function lookupStatusLabel(status: string): string {
  switch (status) {
    case 'ok':
      return m.evidence_lookup_ok();
    case 'no_snapshots':
      return m.evidence_lookup_no_snapshots();
    case 'failed':
      return m.evidence_lookup_failed();
    case 'skipped':
      return m.evidence_lookup_skipped();
    case 'disabled':
      return m.evidence_lookup_disabled();
    default:
      return m.evidence_lookup_none();
  }
}
