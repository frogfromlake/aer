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

// Phase 148e — split a resolved metric display label into its subject noun and
// its model sub-slot. The content catalogue encodes the model after a ` · `
// separator, used ONLY for the sentiment family ("Sentiment Score · BERT
// Multilingual"); every other metric label carries no `·`, so it returns the
// whole label as the subject with a null model. Pure (operates on an already-
// resolved label string) so it is node-testable.
const MODEL_SEPARATOR = ' · ';
export function splitSubjectAndModel(label: string): { subject: string; model: string | null } {
  const idx = label.indexOf(MODEL_SEPARATOR);
  if (idx === -1) return { subject: label, model: null };
  return {
    subject: label.slice(0, idx),
    model: label.slice(idx + MODEL_SEPARATOR.length) || null
  };
}
