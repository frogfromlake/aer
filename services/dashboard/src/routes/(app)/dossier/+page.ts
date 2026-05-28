// Phase 123a — The Dossier is no longer a top-level route; it opens as a
// global overlay driven by URL state (see DossierOverlay.svelte). This
// route is retained only to redirect legacy bookmarks/deep-links to the
// root surface with the equivalent overlay grammar:
//   /dossier                    → /?dossier=open
//   /dossier?expand=<id>        → /?probe=<id>
//   /dossier?selectedProbes=…   → /?dossier=open&selectedProbes=…
// The `?from`/`?to` window params are preserved when present.
import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = ({ url }) => {
  const src = url.searchParams;
  const out = new URLSearchParams();

  const expand = src.get('expand');
  if (expand) {
    out.set('probe', expand);
  } else {
    out.set('dossier', 'open');
  }

  const selected = src.get('selectedProbes');
  if (selected) out.set('selectedProbes', selected);
  const from = src.get('from');
  if (from) out.set('from', from);
  const to = src.get('to');
  if (to) out.set('to', to);

  redirect(308, `/?${out.toString()}`);
};
