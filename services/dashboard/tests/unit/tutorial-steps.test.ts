import { describe, it, expect } from 'vitest';

import {
  TUTORIAL_STEPS,
  stepCount,
  clampStepIndex,
  stepAt,
  isLastStep,
  DEMO_WORKBENCH_URL,
  SCOPE_EDITOR_URL
} from '../../src/lib/state/tutorial-steps';

// Guided tour — pin the pure step-list invariants the controller relies on
// (TutorialOverlay.svelte is browser-only, covered by E2E per ADR-041).

describe('TUTORIAL_STEPS', () => {
  it('has at least the Slice-1 chrome tour and starts with welcome / ends with outro', () => {
    expect(TUTORIAL_STEPS.length).toBeGreaterThanOrEqual(9);
    expect(TUTORIAL_STEPS[0]?.id).toBe('welcome');
    expect(TUTORIAL_STEPS[TUTORIAL_STEPS.length - 1]?.id).toBe('outro');
  });

  it('every step has a unique id', () => {
    const ids = TUTORIAL_STEPS.map((s) => s.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it('centre-placed steps are target-less and targeted steps are not centred', () => {
    for (const s of TUTORIAL_STEPS) {
      if (s.placement === 'center') expect(s.targetId).toBeNull();
      else expect(s.targetId).not.toBeNull();
    }
  });

  it('every step names a route', () => {
    for (const s of TUTORIAL_STEPS) expect(s.route.startsWith('/')).toBe(true);
  });

  it('covers all three surfaces (atmosphere, workbench, reflection)', () => {
    const routes = new Set(TUTORIAL_STEPS.map((s) => s.route));
    expect(routes.has('/')).toBe(true);
    expect(routes.has('/workbench')).toBe(true);
    expect(routes.has('/reflection')).toBe(true);
  });

  it('teaches scope before panels: scopeeditor precedes pillars/panel/cell', () => {
    const idx = (id: string) => TUTORIAL_STEPS.findIndex((s) => s.id === id);
    expect(idx('scopeeditor')).toBeGreaterThan(idx('globe'));
    for (const after of ['pillars', 'panel', 'cell', 'save']) {
      expect(idx('scopeeditor')).toBeLessThan(idx(after));
    }
  });

  it('bridges Workbench → Reflection with a rail-anchor transition stop', () => {
    const idx = (id: string) => TUTORIAL_STEPS.findIndex((s) => s.id === id);
    const nav = TUTORIAL_STEPS.find((s) => s.id === 'reflectionnav');
    expect(nav?.targetId).toBe('rail-reflection');
    expect(nav?.route).toBe('/workbench'); // still on the WB; rail is persistent
    expect(idx('save')).toBeLessThan(idx('reflectionnav'));
    expect(idx('reflectionnav')).toBeLessThan(idx('reflection'));
  });

  it('breaks Reflection into overview → questions → papers → catalogues', () => {
    const idx = (id: string) => TUTORIAL_STEPS.findIndex((s) => s.id === id);
    const order = ['reflection', 'reflectionquestions', 'reflectionpapers', 'reflectioncatalogues'];
    for (const id of order) expect(idx(id)).toBeGreaterThanOrEqual(0);
    for (let i = 1; i < order.length; i++) {
      expect(idx(order[i]!)).toBeGreaterThan(idx(order[i - 1]!));
    }
  });

  it('uses the bare Workbench for the scope steps and the seeded demo for panel steps', () => {
    // The scope steps drive the auto-opening ScopeEditor (no pillar seed); every
    // other workbench step rides the demo seed (so panel/cell chrome exists).
    for (const id of ['scopeeditor', 'scopegroups', 'scopeapply']) {
      const s = TUTORIAL_STEPS.find((step) => step.id === id);
      expect(s?.nav).toBe(SCOPE_EDITOR_URL);
      expect(s?.nav).not.toContain('aleph=');
    }
    for (const id of ['pillars', 'panel', 'panelcontrols', 'cell', 'save']) {
      const s = TUTORIAL_STEPS.find((step) => step.id === id);
      expect(s?.nav).toBe(DEMO_WORKBENCH_URL);
    }
  });

  it('walks the ScopeEditor in order: intro → choose probes/sources → create panel → pillars', () => {
    const idx = (id: string) => TUTORIAL_STEPS.findIndex((s) => s.id === id);
    expect(idx('scopeeditor')).toBeLessThan(idx('scopegroups'));
    expect(idx('scopegroups')).toBeLessThan(idx('scopeapply'));
    expect(idx('scopeapply')).toBeLessThan(idx('pillars'));
  });
});

describe('DEMO_WORKBENCH_URL', () => {
  it('is a seeded Aleph workbench URL (so workbench steps have panel chrome)', () => {
    expect(DEMO_WORKBENCH_URL).toContain('/workbench?');
    expect(DEMO_WORKBENCH_URL).toContain('activePillar=aleph');
    expect(DEMO_WORKBENCH_URL).toContain('aleph=');
  });

  it('seeds a SPLIT panel over two sources (so the cell step shows per-cell chrome)', () => {
    const seed = new URL(`http://x${DEMO_WORKBENCH_URL}`).searchParams.get('aleph')!;
    const json = Buffer.from(seed.replace(/-/g, '+').replace(/_/g, '/'), 'base64').toString('utf8');
    const decoded = JSON.parse(json);
    const panel = decoded.w[0].p[0];
    expect(panel.c).toBe('s'); // split composition
    expect(panel.s).toHaveLength(2); // two scope groups → two cells
    expect(panel.s.map((g: { si: string[] }) => g.si[0])).toEqual([
      'tagesschau',
      'bundesregierung'
    ]);
  });
});

describe('SCOPE_EDITOR_URL', () => {
  it('is the bare workbench (no pillar seed → ScopeEditor auto-opens)', () => {
    expect(SCOPE_EDITOR_URL).toBe('/workbench');
    expect(SCOPE_EDITOR_URL).not.toContain('aleph=');
  });
});

describe('stepCount', () => {
  it('matches the array length', () => {
    expect(stepCount()).toBe(TUTORIAL_STEPS.length);
  });
});

describe('clampStepIndex', () => {
  it('clamps below, above, and passes through valid indices', () => {
    expect(clampStepIndex(-5)).toBe(0);
    expect(clampStepIndex(0)).toBe(0);
    expect(clampStepIndex(3)).toBe(3);
    expect(clampStepIndex(999)).toBe(TUTORIAL_STEPS.length - 1);
  });
  it('coerces non-finite / fractional input', () => {
    expect(clampStepIndex(Number.NaN)).toBe(0);
    expect(clampStepIndex(2.9)).toBe(2);
  });
});

describe('stepAt', () => {
  it('returns the step or null out of range', () => {
    expect(stepAt(0)?.id).toBe('welcome');
    expect(stepAt(-1)).toBeNull();
    expect(stepAt(TUTORIAL_STEPS.length)).toBeNull();
  });
});

describe('isLastStep', () => {
  it('is true only on (and past) the final index', () => {
    expect(isLastStep(0)).toBe(false);
    expect(isLastStep(TUTORIAL_STEPS.length - 1)).toBe(true);
    expect(isLastStep(TUTORIAL_STEPS.length + 2)).toBe(true);
  });
});
