// Phase 122h Slice 8 — Route migration.
// `/lanes/{probeId}/dossier` is retired; the Probe Dossier now lives at
// `/dossier/{probeId}` (ADR-033 §7 redirect map). Old bookmarks and
// referrer links continue to work via this 308 redirect.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ params, url }) => {
  const qs = url.searchParams.toString();
  const probeId = params.probeId;
  redirect(308, `/dossier/${probeId}${qs ? `?${qs}` : ''}`);
};
