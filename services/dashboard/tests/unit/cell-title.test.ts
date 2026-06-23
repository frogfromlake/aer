import { describe, it, expect } from 'vitest';
import { composeCellTitle, type CellTitleSpec } from '../../src/lib/presentations/cell-title';
import { resolveScopeLabel } from '../../src/lib/presentations/scope-label';

const base: CellTitleSpec = {
  presentation: 'Distribution',
  subject: { kind: 'single', label: 'Sentiment' },
  scope: { kind: 'single', label: 'Bundesregierung' },
  idSeed: 'dist-sentiment'
};

describe('composeCellTitle', () => {
  it('passes through the core slots and defaults model to null', () => {
    const t = composeCellTitle(base);
    expect(t.presentation).toBe('Distribution');
    expect(t.subject).toEqual({ kind: 'single', label: 'Sentiment' });
    expect(t.scope).toEqual({ kind: 'single', label: 'Bundesregierung' });
    expect(t.model).toBeNull();
    expect(t.qualifiers).toEqual([]);
  });

  it('keeps a non-empty model but nulls a blank one', () => {
    expect(composeCellTitle({ ...base, model: 'BERT Multilingual' }).model).toBe(
      'BERT Multilingual'
    );
    expect(composeCellTitle({ ...base, model: '   ' }).model).toBeNull();
    expect(composeCellTitle({ ...base, model: null }).model).toBeNull();
  });

  it('drops empty/nullish qualifiers and defaults tone to muted', () => {
    const t = composeCellTitle({
      ...base,
      qualifiers: [{ label: 'daily' }, null, undefined, { label: '   ' }, { label: 'r = 0.42' }]
    });
    expect(t.qualifiers.map((q) => q.label)).toEqual(['daily', 'r = 0.42']);
    expect(t.qualifiers.every((q) => q.tone === 'muted')).toBe(true);
  });

  it('sorts badge-toned qualifiers to the tail, preserving relative order', () => {
    const t = composeCellTitle({
      ...base,
      qualifiers: [
        { label: 'Tier 2', tone: 'tier' },
        { label: 'daily' },
        { label: 'Silver', tone: 'layer' },
        { label: 'Top 200' }
      ]
    });
    expect(t.qualifiers.map((q) => q.label)).toEqual(['daily', 'Top 200', 'Tier 2', 'Silver']);
  });

  it('preserves pair subjects and scope relations verbatim', () => {
    const t = composeCellTitle({
      ...base,
      presentation: 'Lead–Lag',
      subject: { kind: 'none' },
      scope: { kind: 'pair', left: 'Bundesregierung', right: 'franceinfo', relation: 'vs' }
    });
    expect(t.subject.kind).toBe('none');
    expect(t.scope).toEqual({
      kind: 'pair',
      left: 'Bundesregierung',
      right: 'franceinfo',
      relation: 'vs'
    });
  });
});

describe('resolveScopeLabel', () => {
  const probes = [
    {
      probeId: 'probe-0-de-institutional-web',
      displayName: 'Germany · Institutional',
      shortName: 'DE Inst'
    },
    {
      probeId: 'probe-1-fr-institutional-web',
      displayName: 'France · Institutional',
      shortName: null
    }
  ];

  it('resolves a probe id to shortName, then displayName, then id', () => {
    expect(resolveScopeLabel('probe-0-de-institutional-web', probes)).toBe('DE Inst');
    expect(resolveScopeLabel('probe-1-fr-institutional-web', probes)).toBe(
      'France · Institutional'
    );
  });

  it('returns a source name (or unknown id) verbatim', () => {
    expect(resolveScopeLabel('bundesregierung', probes)).toBe('bundesregierung');
    expect(resolveScopeLabel('probe-9-unknown', probes)).toBe('probe-9-unknown');
  });

  it('falls back to the id when both display labels are absent', () => {
    expect(resolveScopeLabel('p', [{ probeId: 'p' }])).toBe('p');
  });
});
