// Phase 123b — cross-lingual relabel: resolving the relabel target language.
//
// The co-occurrence relabel toggle swaps QID-linked node labels from their
// source surface form to the **app's content language**. The dashboard is
// currently localized to a single language (English — content fetches default
// to `en`); there is no per-user locale selector yet. So the relabel target is
// the app content language, NOT the reader's browser locale (relabelling an
// English UI's entities into a browser's German would be inconsistent).
//
// `APP_CONTENT_LANGUAGE` is the single seam: when the app gains a real locale
// selector, point this (and the content-fetch `locale` default) at that store
// and the relabel follows automatically — no other change here.
//
// The supported set MUST mirror the index build's LANGUAGES list
// (scripts/build/build_wikidata_index.py); adding a label language there (plus
// a rebuild) extends coverage with no code change here.

/** Label languages the baked Wikidata index carries (de/en/fr today). */
export const SUPPORTED_LABEL_LANGUAGES = ['de', 'en', 'fr'] as const;
export type LabelLanguage = (typeof SUPPORTED_LABEL_LANGUAGES)[number];

const DEFAULT_LABEL_LANGUAGE: LabelLanguage = 'en';

/** The app's current content language. Single source of truth for the relabel
 *  target until a locale selector exists; today the dashboard is English-only.
 */
export const APP_CONTENT_LANGUAGE: LabelLanguage = 'en';

/** Pure core: map a raw BCP-47 locale (e.g. "de-DE", "fr") to a supported
 *  label language, or the English default. Lower-cases and takes the primary
 *  subtag so "DE-de", "de_DE", "de-AT" all resolve to "de". Retained as the
 *  clamp a future locale store will feed its value through. */
export function pickViewerLabelLanguage(rawLocale: string | undefined | null): LabelLanguage {
  if (!rawLocale) return DEFAULT_LABEL_LANGUAGE;
  const primary = rawLocale.toLowerCase().split(/[-_]/)[0] ?? '';
  return (SUPPORTED_LABEL_LANGUAGES as readonly string[]).includes(primary)
    ? (primary as LabelLanguage)
    : DEFAULT_LABEL_LANGUAGE;
}

/** The relabel target language: the app content language, clamped to the
 *  index's label languages. */
export function viewerLabelLanguage(): LabelLanguage {
  return pickViewerLabelLanguage(APP_CONTENT_LANGUAGE);
}
