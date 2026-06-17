import { describe, expect, it } from 'vitest';

import { PRESENTATIONS } from '../../src/lib/presentations/registry-data';

// Phase 141 / 142 — the presentation-definition data table is the SoT for which
// presentations exist and what config each declares. These tests pin the table's
// structural invariants (unique ids, lazy loaders, the metric/field contract)
// and exercise every loadComponent closure so a stale entry surfaces here.

describe('PRESENTATIONS table', () => {
  it('has unique presentation ids', () => {
    const ids = PRESENTATIONS.map((p) => p.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('registers the load-bearing presentations referenced by the Pillars', () => {
    const ids = PRESENTATIONS.map((p) => p.id);
    for (const id of [
      'time_series',
      'distribution',
      'cooccurrence_network',
      'metric_scatter',
      'topic_distribution',
      'topic_evolution'
    ]) {
      expect(ids).toContain(id);
    }
  });

  it('every entry carries a non-empty label + description and a lazy loadComponent', () => {
    for (const p of PRESENTATIONS) {
      expect(p.label.length).toBeGreaterThan(0);
      expect(p.description.length).toBeGreaterThan(0);
      expect(typeof p.loadComponent).toBe('function');
    }
  });

  it('metric_scatter is channel-driven (usesMetric:false) so it stays out of the single-metric map', () => {
    const scatter = PRESENTATIONS.find((p) => p.id === 'metric_scatter');
    expect(scatter?.usesMetric).toBe(false);
  });

  it('only time_series advertises overlay support', () => {
    const overlayable = PRESENTATIONS.filter((p) => p.supportsOverlay).map((p) => p.id);
    expect(overlayable).toEqual(['time_series']);
  });

  it('every loadComponent is a distinct loader returning a Promise', () => {
    // The arrow bodies are `() => import('./SomeCell.svelte')`. node-env Vitest
    // cannot resolve the .svelte target (that wiring is the E2E suite's
    // contract), so this asserts only what IS verifiable here: each loader is a
    // real function that returns a thenable, and no two presentations share the
    // same loader reference (a copy-paste that points two cells at one closure).
    const loaders = PRESENTATIONS.map((p) => p.loadComponent);
    for (const load of loaders) {
      expect(typeof load).toBe('function');
      const pr = load();
      expect(typeof (pr as Promise<unknown>).then).toBe('function');
      void (pr as Promise<unknown>).catch(() => {}); // swallow the unresolved .svelte import
    }
    expect(new Set(loaders).size).toBe(PRESENTATIONS.length);
  });
});
