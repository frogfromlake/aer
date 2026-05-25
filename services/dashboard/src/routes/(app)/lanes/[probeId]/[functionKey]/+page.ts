// Phase 122h Slice 8 — Route migration.
// `/lanes/{probeId}/{functionKey}` is retired. The Function Lane shell
// was replaced by three Pillar Shells inside the Workbench. Old bookmarks
// translate as (ADR-033 §7 redirect map):
//   /lanes/{probeId}/{functionKey}?viewMode=cooccurrence_network&…
//     → /workbench?probeId={probeId}&pillar=rhizome&…
// Legacy `viewMode` params map to the new pillar where possible; otherwise
// the redirect lands the user on the default Aleph Pillar with the function
// set as the scope filter. (Phase 130 / ADR-035 removed the Rhizome
// entry-question sub-view, so cooccurrence_network maps to the pillar only.)
import { redirect } from '@sveltejs/kit';
import { pillarForViewMode } from '$lib/viewmodes';
import type { ViewMode } from '$lib/state/url-internals';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ params, url }) => {
  const probeId = params.probeId;
  const functionKey = params.functionKey;

  const out = new URLSearchParams();
  out.set('probeId', probeId);

  // Derive the pillar from the registry SoT (`pillarForViewMode` over
  // `PILLAR_DEFINITIONS`) rather than a parallel map, so the redirect
  // tracks Phase 130 / ADR-035 placement automatically. Unknown legacy
  // view-modes fall back to the default Aleph pillar.
  const legacyViewMode = url.searchParams.get('viewMode');
  const pillar = legacyViewMode ? pillarForViewMode(legacyViewMode as ViewMode) : null;
  if (pillar) {
    out.set('viewingMode', pillar);
    // Keep viewMode for downstream Cells that still read it directly.
    out.set('viewMode', legacyViewMode!);
  } else {
    out.set('viewingMode', 'aleph');
  }

  // Carry the rest of the legacy params through 1:1. `functionKey` itself
  // is preserved as informational state — it will become a Scope-Bar
  // filter chip rather than a path segment.
  for (const [k, v] of url.searchParams.entries()) {
    if (k === 'viewMode') continue;
    out.set(k, v);
  }
  out.set('functionKey', functionKey);

  redirect(308, `/workbench?${out.toString()}`);
};
