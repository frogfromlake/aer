// Task B — pure label helpers (no runes, no Paraglide) so they are unit-testable
// under node-env Vitest. The reactive registry + locale-bound field labels live
// in $lib/state/labels.svelte.ts, which imports humanizeMachineName from here.

// Proper-noun / acronym casing so humanize never lowercases a model name.
const TOKEN_OVERRIDES: Record<string, string> = {
  bert: 'BERT',
  sentiws: 'SentiWS',
  ws: 'WS',
  url: 'URL',
  de: 'DE',
  en: 'EN',
  fr: 'FR',
  id: 'ID',
  ner: 'NER',
  qid: 'QID',
  ttr: 'TTR'
};

// snake_case → Title Case, preserving known acronyms. The universal fallback when
// no curated (catalogue / Paraglide) label exists; readable but NOT localized.
export function humanizeMachineName(name: string): string {
  if (!name) return '';
  return name
    .split('_')
    .map((w) => TOKEN_OVERRIDES[w] ?? (w ? w.charAt(0).toUpperCase() + w.slice(1) : w))
    .join(' ');
}
