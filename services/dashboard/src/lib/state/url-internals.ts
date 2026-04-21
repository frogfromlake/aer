// Pure URL (de)serialisation helpers backing `url.svelte.ts`. Kept rune-
// free in their own module so vitest can import them without a Svelte
// compiler pass. The runes-based store lives in `url.svelte.ts` and
// re-exports these for component-side use.

export type Resolution = '5min' | 'hourly' | 'daily' | 'weekly' | 'monthly';
export type ViewingMode = 'aleph' | 'episteme' | 'rhizome';

export interface UrlState {
  from: string | null;
  to: string | null;
  probe: string | null;
  resolution: Resolution | null;
  viewingMode: ViewingMode | null;
}

export const EMPTY_URL_STATE: UrlState = {
  from: null,
  to: null,
  probe: null,
  resolution: null,
  viewingMode: null
};

const RESOLUTIONS: readonly Resolution[] = ['5min', 'hourly', 'daily', 'weekly', 'monthly'];
const VIEWING_MODES: readonly ViewingMode[] = ['aleph', 'episteme', 'rhizome'];

function parseIso(v: string | null): string | null {
  if (v === null) return null;
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

function parseEnum<T extends string>(v: string | null, allowed: readonly T[]): T | null {
  if (v === null) return null;
  return (allowed as readonly string[]).includes(v) ? (v as T) : null;
}

export function readFromSearch(search: string): UrlState {
  const p = new URLSearchParams(search);
  return {
    from: parseIso(p.get('from')),
    to: parseIso(p.get('to')),
    probe: p.get('probe'),
    resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
    viewingMode: parseEnum(p.get('viewingMode'), VIEWING_MODES)
  };
}

export function writeToSearch(state: UrlState): string {
  const p = new URLSearchParams();
  if (state.from) p.set('from', state.from);
  if (state.to) p.set('to', state.to);
  if (state.probe) p.set('probe', state.probe);
  if (state.resolution) p.set('resolution', state.resolution);
  if (state.viewingMode) p.set('viewingMode', state.viewingMode);
  const qs = p.toString();
  return qs.length === 0 ? '' : `?${qs}`;
}
