// Phase 122h Slice 8 — Route migration. Phase 122i revision (R5) update:
// the Dossier was elevated to a top-level surface; legacy bookmarks of
// the per-probe form now redirect to `/dossier?expand=<probeId>` so the
// named probe's card auto-expands on the catalogue page.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ params, url }) => {
  const extra = url.searchParams.toString();
  const probeId = params.probeId;
  const qs = `expand=${encodeURIComponent(probeId)}${extra ? `&${extra}` : ''}`;
  redirect(308, `/dossier?${qs}`);
};
