// Phase 123a — Route migration. The Dossier is now a global overlay, not a
// top-level route; legacy per-probe bookmarks redirect to the root surface
// opening the Dossier catalogue overlay with that probe in the selection
// cart (the overlay auto-expands selected probes).
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ params, url }) => {
  const out = new URLSearchParams(url.searchParams);
  out.delete('expand');
  out.set('dossier', 'open');
  out.set('selectedProbes', params.probeId);
  redirect(308, `/?${out.toString()}`);
};
