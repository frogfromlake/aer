import { describe, expect, it } from 'vitest';

import {
  splitSections,
  scrollTargetIds,
  buildBackToWorkbenchHref
} from '../../src/lib/reflection/wp-page-internals';
import type { PaperSection } from '../../src/lib/reflection/md';

function section(over: Partial<PaperSection>): PaperSection {
  return { number: '', title: '', id: 'intro', html: '', isAppendix: false, ...over };
}

describe('splitSections', () => {
  it('puts numbered non-appendix sections in main, appendix sections in appendix', () => {
    const sections = [
      section({ number: '', id: 'intro' }), // dropped from main (no number)
      section({ number: '1', title: 'Scope', id: 'section-1' }),
      section({ number: '2', title: 'Findings', id: 'section-2' }),
      section({ number: 'A', title: 'Appendix A', id: 'appendix-a', isAppendix: true })
    ];
    const { main, appendix } = splitSections(sections);
    expect(main.map((s) => s.id)).toEqual(['section-1', 'section-2']);
    expect(appendix.map((s) => s.id)).toEqual(['appendix-a']);
  });

  it('returns empty slices for an empty section list', () => {
    expect(splitSections([])).toEqual({ main: [], appendix: [] });
  });
});

describe('scrollTargetIds', () => {
  it('builds the section-id candidate (dots → dashes) and the appendix-id candidate', () => {
    expect(scrollTargetIds('5.3')).toEqual(['section-5-3', 'appendix-5.3']);
  });

  it('lower-cases the appendix candidate', () => {
    expect(scrollTargetIds('B')).toEqual(['section-B', 'appendix-b']);
  });
});

describe('buildBackToWorkbenchHref', () => {
  it('returns null when there is no probe', () => {
    expect(buildBackToWorkbenchHref({ probe: null, fn: 'x', pillar: 'aleph' })).toBeNull();
  });

  it('builds the full href with function + pillar', () => {
    expect(
      buildBackToWorkbenchHref({ probe: 'probe-0', fn: 'epistemic_authority', pillar: 'episteme' })
    ).toBe('/workbench?probeId=probe-0&functionKey=epistemic_authority&viewingMode=episteme');
  });

  it('omits functionKey when absent and defaults the pillar to aleph', () => {
    expect(buildBackToWorkbenchHref({ probe: 'probe-0', fn: null, pillar: null })).toBe(
      '/workbench?probeId=probe-0&viewingMode=aleph'
    );
  });
});
