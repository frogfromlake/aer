import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  DRAFT_KEY,
  clearDraft,
  loadDraft,
  saveDraft,
  type ScopeEditorDraft
} from '../../src/lib/workbench/scope-editor-draft';

// Phase 122k §11 / 142 — the ScopeEditor draft is a one-shot sessionStorage
// snapshot for back-nav resume. The module reads `window.sessionStorage`, so
// these tests stub `window` with a small in-memory Storage shim and pin the
// round-trip, the no-window guard, and every defensive parse path.

function memoryStorage(): Storage {
  const map = new Map<string, string>();
  return {
    getItem: (k) => (map.has(k) ? map.get(k)! : null),
    setItem: (k, v) => void map.set(k, String(v)),
    removeItem: (k) => void map.delete(k),
    clear: () => map.clear(),
    key: (i) => Array.from(map.keys())[i] ?? null,
    get length() {
      return map.size;
    }
  } as Storage;
}

const DRAFT: ScopeEditorDraft = {
  scopes: [{ probeIds: ['probe-0'], sourceIds: ['tagesschau'] }],
  perGroupLock: ['epistemic_authority']
};

afterEach(() => vi.unstubAllGlobals());

describe('with a working sessionStorage', () => {
  let storage: Storage;
  beforeEach(() => {
    storage = memoryStorage();
    vi.stubGlobal('window', { sessionStorage: storage });
  });

  it('round-trips a draft through save → load', () => {
    saveDraft(DRAFT);
    expect(storage.getItem(DRAFT_KEY)).toContain('probe-0');
    expect(loadDraft()).toEqual(DRAFT);
  });

  it('loadDraft returns null when nothing was saved', () => {
    expect(loadDraft()).toBeNull();
  });

  it('clearDraft removes the stored entry (the one-shot consume)', () => {
    saveDraft(DRAFT);
    clearDraft();
    expect(loadDraft()).toBeNull();
  });

  it('loadDraft returns null on corrupt JSON', () => {
    storage.setItem(DRAFT_KEY, '{not json');
    expect(loadDraft()).toBeNull();
  });

  it('loadDraft rejects a structurally-invalid draft (missing arrays)', () => {
    storage.setItem(DRAFT_KEY, JSON.stringify({ scopes: 'oops', perGroupLock: [] }));
    expect(loadDraft()).toBeNull();
    storage.setItem(DRAFT_KEY, JSON.stringify({ scopes: [], perGroupLock: 'oops' }));
    expect(loadDraft()).toBeNull();
  });
});

describe('without a browser window (SSR / node)', () => {
  beforeEach(() => vi.stubGlobal('window', undefined));

  it('save/load/clear are safe no-ops', () => {
    expect(() => saveDraft(DRAFT)).not.toThrow();
    expect(loadDraft()).toBeNull();
    expect(() => clearDraft()).not.toThrow();
  });
});

describe('with a throwing sessionStorage accessor', () => {
  beforeEach(() => {
    vi.stubGlobal('window', {
      get sessionStorage(): Storage {
        throw new DOMException('blocked', 'SecurityError');
      }
    });
  });

  it('treats a throwing accessor as no storage (private-mode / blocked)', () => {
    expect(() => saveDraft(DRAFT)).not.toThrow();
    expect(loadDraft()).toBeNull();
    expect(() => clearDraft()).not.toThrow();
  });
});

describe('with a quota-throwing setItem', () => {
  beforeEach(() => {
    const throwing = {
      ...memoryStorage(),
      setItem: () => {
        throw new DOMException('QuotaExceededError');
      },
      getItem: () => null,
      removeItem: () => {
        throw new Error('blocked');
      }
    } as Storage;
    vi.stubGlobal('window', { sessionStorage: throwing });
  });

  it('swallows a setItem quota error (the Apply still works)', () => {
    expect(() => saveDraft(DRAFT)).not.toThrow();
  });

  it('swallows a removeItem error in clearDraft', () => {
    expect(() => clearDraft()).not.toThrow();
  });
});
