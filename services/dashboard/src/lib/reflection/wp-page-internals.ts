// Pure helpers for the Working-Paper reader page (Phase 141 decomposition). The
// `+page.svelte` shell and its region children (WpBreadcrumb / WpTableOfContents
// / WpPaperBody) own only the Svelte markup; the TOC split, the scroll-target id
// candidates, and the Back-to-Workbench href builder live here so they are
// unit-testable in isolation.
import type { PaperSection } from './md';

// The TOC and the body iterate different slices of the parsed sections: the TOC
// shows numbered, non-appendix sections (main) and appendix sections separately;
// the body renders all of them in order. Splitting once keeps both consistent.
export function splitSections(sections: readonly PaperSection[]): {
  main: PaperSection[];
  appendix: PaperSection[];
} {
  return {
    main: sections.filter((s) => !s.isAppendix && s.number),
    appendix: sections.filter((s) => s.isAppendix)
  };
}

// The `?section=` deep-link is matched against two candidate element ids (exact
// section number with dots → dashes, then an appendix id), tried in order.
export function scrollTargetIds(sectionParam: string): string[] {
  return [`section-${sectionParam.replace(/\./g, '-')}`, `appendix-${sectionParam.toLowerCase()}`];
}

// Back-to-Workbench href from the referrer params (Phase 113c / 122h). A probe is
// required; the function and pillar are best-effort enrichments (pillar defaults
// to 'aleph'). Returns null when there is no probe to return to.
export function buildBackToWorkbenchHref(args: {
  probe: string | null;
  fn: string | null;
  pillar: string | null;
}): string | null {
  if (!args.probe) return null;
  const params = new URLSearchParams();
  params.set('probeId', args.probe);
  if (args.fn) params.set('functionKey', args.fn);
  params.set('viewingMode', args.pillar ?? 'aleph');
  return `/workbench?${params.toString()}`;
}
