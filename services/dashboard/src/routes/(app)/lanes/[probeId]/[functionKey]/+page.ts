// Phase 122h Slice 8 — Route migration.
// `/lanes/{probeId}/{functionKey}` is retired. The Function Lane shell
// was replaced by three Pillar Shells inside the Workbench. Old bookmarks
// translate as (ADR-033 §7 redirect map):
//   /lanes/{probeId}/{functionKey}?viewMode=cooccurrence_network&…
//     → /workbench?probeId={probeId}&pillar=rhizome&view=actors-topics&…
// Legacy `viewMode` params map to the new (pillar, view) tuple where
// possible; otherwise the redirect lands the user on the default Aleph
// Pillar with the function set as the scope filter.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

const LEGACY_VIEWMODE_TO_PILLAR: Record<
  string,
  { pillar: 'aleph' | 'episteme' | 'rhizome'; view?: string }
> = {
  time_series: { pillar: 'aleph' },
  distribution: { pillar: 'aleph' },
  topic_distribution: { pillar: 'episteme' },
  topic_evolution: { pillar: 'episteme' },
  cooccurrence_network: { pillar: 'rhizome', view: 'actors-topics' }
};

export const load: PageLoad = ({ params, url }) => {
  const probeId = params.probeId;
  const functionKey = params.functionKey;

  const out = new URLSearchParams();
  out.set('probeId', probeId);

  const legacyViewMode = url.searchParams.get('viewMode');
  const mapping = legacyViewMode ? LEGACY_VIEWMODE_TO_PILLAR[legacyViewMode] : undefined;
  if (mapping) {
    out.set('viewingMode', mapping.pillar);
    if (mapping.view) out.set('view', mapping.view);
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
