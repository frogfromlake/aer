// Phase 122h Slice 8 — Route migration.
// Bare `/lanes` (no probe selected) is redirected to `/workbench` which
// renders its own empty-scope prompt inviting the user to pick a probe
// on the Atmosphere.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ url }) => {
  const qs = url.searchParams.toString();
  redirect(308, `/workbench${qs ? `?${qs}` : ''}`);
};
