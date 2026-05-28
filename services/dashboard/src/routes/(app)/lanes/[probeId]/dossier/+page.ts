// Phase 123a — Route migration. The Dossier is now a global overlay, not a
// top-level route; legacy per-probe bookmarks redirect to the root surface
// with `?probe=<probeId>`, which opens the mini Dossier overlay focused on
// that probe.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ params, url }) => {
  const params2 = new URLSearchParams(url.searchParams);
  params2.delete('expand');
  params2.set('probe', params.probeId);
  redirect(308, `/?${params2.toString()}`);
};
