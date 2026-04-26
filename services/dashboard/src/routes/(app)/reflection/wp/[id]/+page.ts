import type { PageLoad } from './$types';
import { getPaperMeta, paperContentUrl } from '$lib/reflection/papers';
import { renderPaper } from '$lib/reflection/md';

export const prerender = false;
export const ssr = false;

export const load: PageLoad = async ({ params, fetch }) => {
  const id = (params.id ?? '').toLowerCase();
  const meta = getPaperMeta(id);
  if (!meta) return { paper: null };

  let rendered = null;
  try {
    const res = await fetch(paperContentUrl(id));
    if (res.ok) {
      const raw = await res.text();
      rendered = renderPaper(raw);
    }
  } catch {
    // Network unavailable — page renders with metadata only
  }

  return { paper: { meta, rendered } };
};
