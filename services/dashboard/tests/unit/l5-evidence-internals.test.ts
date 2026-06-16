import { describe, it, expect } from 'vitest';
import {
  deriveDiffChain,
  selectedDiffPairIndex,
  silentEditSignals,
  nsMarkersFor,
  formatTs,
  sentimentArrow,
  fmtDelta,
  wordDiff,
  walkStepLabel,
  cumulativeLabel,
  lookupStatusLabel
} from '../../src/lib/components/evidence/l5-evidence-internals';
import type { ArticleRevisionEntryDto } from '../../src/lib/api/queries';

// Minimal factory for a revision row — only the fields the pure logic reads.
function mkRev(over: Partial<ArticleRevisionEntryDto> = {}): ArticleRevisionEntryDto {
  return {
    snapshotAt: '2024-01-01T12:00:00Z',
    contentHash: 'abc123',
    revisionIndex: 0,
    trigger: 'cdx_snapshot',
    diffStatus: 'pending',
    deltasComputed: false,
    ...over
  };
}

describe('deriveDiffChain', () => {
  it('returns an empty/null chain for no revisions', () => {
    const c = deriveDiffChain([]);
    expect(c.chainHead).toBeNull();
    expect(c.walkSteps).toEqual([]);
    expect(c.lookupByIndex.size).toBe(0);
    expect(c.changedCount).toBe(0);
    expect(c.pendingCount).toBe(0);
    expect(c.identicalCount).toBe(0);
    expect(c.cumulativeAvailable).toBe(false);
    expect(c.hasEditorialContent).toBe(false);
  });

  it('treats the first array row as the chain head (a single snapshot = head, empty walk)', () => {
    const head = mkRev({ revisionIndex: 0, diffStatus: 'changed' });
    const c = deriveDiffChain([head]);
    expect(c.chainHead).toBe(head);
    expect(c.walkSteps).toEqual([]);
    expect(c.cumulativeAvailable).toBe(true);
  });

  it('walks only non-identical rows after the head; identical re-archivals are dropped from the walk', () => {
    const rows = [
      mkRev({ revisionIndex: 0, diffStatus: 'changed' }), // head — excluded from walk
      mkRev({ revisionIndex: 1, diffStatus: 'identical' }), // dropped
      mkRev({ revisionIndex: 2, diffStatus: 'changed' }), // kept
      mkRev({ revisionIndex: 3, diffStatus: 'pending' }) // kept (might become editorial)
    ];
    const c = deriveDiffChain(rows);
    expect(c.walkSteps.map((r) => r.revisionIndex)).toEqual([2, 3]);
  });

  it('counts every row by diffStatus (head included) and flags editorial content', () => {
    const rows = [
      mkRev({ revisionIndex: 0, diffStatus: 'changed' }),
      mkRev({ revisionIndex: 1, diffStatus: 'identical' }),
      mkRev({ revisionIndex: 2, diffStatus: 'identical' }),
      mkRev({ revisionIndex: 3, diffStatus: 'pending' })
    ];
    const c = deriveDiffChain(rows);
    expect(c.changedCount).toBe(1);
    expect(c.identicalCount).toBe(2);
    expect(c.pendingCount).toBe(1);
    expect(c.hasEditorialContent).toBe(true);
  });

  it('reports no editorial content when every row is an identical re-archival', () => {
    const rows = [
      mkRev({ revisionIndex: 0, diffStatus: 'identical' }),
      mkRev({ revisionIndex: 1, diffStatus: 'identical' })
    ];
    expect(deriveDiffChain(rows).hasEditorialContent).toBe(false);
  });

  it('keys lookupByIndex on revisionIndex, surviving an offset chain (ADR-036 rebuilds)', () => {
    // revision_index is contiguous but can start > 0.
    const rows = [
      mkRev({ revisionIndex: 685, diffStatus: 'changed' }),
      mkRev({ revisionIndex: 686, diffStatus: 'changed' })
    ];
    const c = deriveDiffChain(rows);
    expect(c.chainHead?.revisionIndex).toBe(685);
    expect(c.lookupByIndex.get(686)?.revisionIndex).toBe(686);
    expect(c.walkSteps.map((r) => r.revisionIndex)).toEqual([686]);
  });
});

describe('selectedDiffPairIndex', () => {
  const rows = [
    mkRev({ revisionIndex: 10, diffStatus: 'changed' }),
    mkRev({ revisionIndex: 11, diffStatus: 'changed' }),
    mkRev({ revisionIndex: 12, diffStatus: 'changed' })
  ];
  const chain = deriveDiffChain(rows);

  it('returns the chain-head index in cumulative view', () => {
    expect(selectedDiffPairIndex(chain, 'cumulative', 0)).toBe(10);
    expect(selectedDiffPairIndex(chain, 'cumulative', 5)).toBe(10); // walkPos ignored
  });

  it('returns the walked step index at walkPos in walk view', () => {
    expect(selectedDiffPairIndex(chain, 'walk', 0)).toBe(11);
    expect(selectedDiffPairIndex(chain, 'walk', 1)).toBe(12);
  });

  it('returns -1 when the walk position is out of range', () => {
    expect(selectedDiffPairIndex(chain, 'walk', 9)).toBe(-1);
  });

  it('returns -1 for an empty chain', () => {
    const empty = deriveDiffChain([]);
    expect(selectedDiffPairIndex(empty, 'cumulative', 0)).toBe(-1);
    expect(selectedDiffPairIndex(empty, 'walk', 0)).toBe(-1);
  });
});

describe('silentEditSignals', () => {
  it('fires the edited-after-publication signal on a changed row', () => {
    const s = silentEditSignals([mkRev({ diffStatus: 'changed' })], 'ok');
    expect(s).toEqual(['edited after publication (paragraph or headline change)']);
  });

  it('fires the republished signal on a republication_trigger row', () => {
    const s = silentEditSignals([mkRev({ trigger: 'republication_trigger' })], 'ok');
    expect(s).toContain('republished under a new URL');
  });

  it('fires the archive-gap signal for failed and no_snapshots statuses', () => {
    expect(silentEditSignals([], 'failed')).toContain(
      'archive history could not be established (Wayback gap)'
    );
    expect(silentEditSignals([], 'no_snapshots')).toContain(
      'archive history could not be established (Wayback gap)'
    );
  });

  it('returns no signals for a clean, fully-archived article', () => {
    expect(silentEditSignals([mkRev({ diffStatus: 'identical' })], 'ok')).toEqual([]);
  });

  it('accumulates multiple distinct signals', () => {
    const s = silentEditSignals(
      [mkRev({ diffStatus: 'changed', trigger: 'republication_trigger' })],
      'failed'
    );
    expect(s).toHaveLength(3);
  });
});

describe('nsMarkersFor', () => {
  it('maps any signals to the silent_edit class', () => {
    expect(nsMarkersFor(['x'])).toEqual(['silent_edit']);
  });
  it('maps no signals to no markers', () => {
    expect(nsMarkersFor([])).toEqual([]);
  });
});

describe('sentimentArrow', () => {
  it('points up above the threshold, down below it, flat within', () => {
    expect(sentimentArrow(0.01)).toBe('▲');
    expect(sentimentArrow(-0.01)).toBe('▼');
    expect(sentimentArrow(0)).toBe('→');
    expect(sentimentArrow(0.0004)).toBe('→'); // within the dead band
  });
});

describe('fmtDelta', () => {
  it('prefixes a sign and fixes three decimals', () => {
    expect(fmtDelta(0.5)).toBe('+0.500');
    expect(fmtDelta(-0.5)).toBe('-0.500');
    expect(fmtDelta(0)).toBe('+0.000');
  });
});

describe('wordDiff', () => {
  it('marks added and removed tokens between two strings', () => {
    const ops = wordDiff('the cat sat', 'the dog sat');
    expect(ops.some((o) => o.removed && o.value.includes('cat'))).toBe(true);
    expect(ops.some((o) => o.added && o.value.includes('dog'))).toBe(true);
  });
  it('treats nullish input as empty (no throw)', () => {
    expect(() => wordDiff(undefined as unknown as string, 'x')).not.toThrow();
  });
});

describe('walkStepLabel', () => {
  it('returns an em dash for a missing step', () => {
    expect(walkStepLabel(new Map(), undefined)).toBe('—');
  });

  it('uses "?" for the before-date when the predecessor is absent', () => {
    const step = mkRev({ revisionIndex: 5, snapshotAt: '2024-03-02T12:00:00Z' });
    expect(walkStepLabel(new Map(), step).startsWith('? → ')).toBe(true);
  });

  it('renders before → after when the predecessor is present', () => {
    const prev = mkRev({ revisionIndex: 4, snapshotAt: '2024-03-01T12:00:00Z' });
    const step = mkRev({ revisionIndex: 5, snapshotAt: '2024-03-02T12:00:00Z' });
    const label = walkStepLabel(new Map([[4, prev]]), step);
    expect(label).toContain(' → ');
    expect(label.startsWith('?')).toBe(false);
  });
});

describe('cumulativeLabel', () => {
  it('falls back when there is no snapshot', () => {
    expect(cumulativeLabel([])).toBe('Latest snapshot → current article');
  });
  it('names the newest snapshot and reads toward the current article', () => {
    const label = cumulativeLabel([
      mkRev({ revisionIndex: 0, snapshotAt: '2024-01-01T12:00:00Z' }),
      mkRev({ revisionIndex: 1, snapshotAt: '2024-06-15T12:00:00Z' })
    ]);
    expect(label.startsWith('Latest snapshot ')).toBe(true);
    expect(label.endsWith(' → current article')).toBe(true);
  });
});

describe('lookupStatusLabel', () => {
  it('maps each known status to its prose', () => {
    expect(lookupStatusLabel('ok')).toMatch(/CDX returned/);
    expect(lookupStatusLabel('no_snapshots')).toMatch(/not yet archived/);
    expect(lookupStatusLabel('failed')).toMatch(/lookup failed/);
    expect(lookupStatusLabel('skipped')).toMatch(/canonical URL missing/);
    expect(lookupStatusLabel('disabled')).toMatch(/disabled/);
  });
  it('falls back for the empty/unknown status', () => {
    expect(lookupStatusLabel('')).toMatch(/No revision metadata/);
  });
});

describe('formatTs', () => {
  it('formats a valid ISO timestamp (year preserved, TZ-safe)', () => {
    expect(formatTs('2024-01-15T12:00:00Z')).toContain('2024');
  });
});
