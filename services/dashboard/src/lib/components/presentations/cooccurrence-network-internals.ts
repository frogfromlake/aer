// Pure helpers backing CoOccurrenceNetworkCell.svelte (Phase 118 / 121b).
// Kept rune-free in their own module so vitest can import them under the
// default `node` environment without a Svelte compiler pass — same pattern
// as methodology-tray-internals.ts.

const WIKIPEDIA_LINK_LANG_DEFAULT = 'en';

export function wikidataHref(qid: string): string {
  return `https://www.wikidata.org/wiki/${encodeURIComponent(qid)}`;
}

export function wikipediaHref(qid: string, lang: string = WIKIPEDIA_LINK_LANG_DEFAULT): string {
  return `https://www.wikidata.org/wiki/Special:GoToLinkedPage/${lang}wiki/${encodeURIComponent(qid)}`;
}

export function hasExternalLinks(node: { wikidataQid?: string | null }): boolean {
  return typeof node.wikidataQid === 'string' && node.wikidataQid.length > 0;
}
