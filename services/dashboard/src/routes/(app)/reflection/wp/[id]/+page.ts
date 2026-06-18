import type { PageLoad } from './$types';
import { getPaperMeta, paperContentUrl } from '$lib/reflection/papers';
import { renderPaper } from '$lib/reflection/md';
import { locale } from '$lib/state/locale.svelte';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = async ({ params, fetch, depends }) => {
  // Phase 144 — re-run this load when the UI locale changes. A locale switch
  // writes `?lang=` via history.replaceState (bypassing the router), so the
  // page wouldn't reload on its own; setLocale calls `invalidate('app:locale')`
  // to force the active locale's Working-Paper markdown to be re-fetched.
  depends('app:locale');
  const id = (params.id ?? '').toLowerCase();
  const meta = getPaperMeta(id);
  if (!meta) return { paper: null };

  let rendered = null;
  try {
    const res = await fetch(paperContentUrl(id, locale()));
    if (res.ok) {
      const raw = await res.text();
      rendered = renderPaper(raw);
    }
  } catch {
    // Network unavailable — page renders with metadata only
  }

  return { paper: { meta, rendered } };
};
