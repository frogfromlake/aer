// Phase 123b — cross-lingual relabel: resolving the relabel target language.
//
// The co-occurrence relabel toggle swaps QID-linked node labels from their
// source surface form to the **app's content language**. The relabel target is
// the app UI locale (Phase 144 `locale` rune), clamped to the index's label
// languages — NOT the reader's raw browser locale (relabelling a German UI's
// entities into a browser's French would be inconsistent).
//
// Phase 144 closed the seam this module's old `APP_CONTENT_LANGUAGE` constant
// anticipated: components now pass `locale()` (the rune) through the pure
// `pickViewerLabelLanguage` clamp. The UI locale is en|de; both are in the
// supported label set, so the clamp is the identity for them and the `fr`
// fallback remains available for any future UI locale outside the index set.
//
// The supported set MUST mirror the index build's LANGUAGES list
// (scripts/build/build_wikidata_index.py); adding a label language there (plus
// a rebuild) extends coverage with no code change here.

/** Label languages the baked Wikidata index carries (de/en/fr today). */
export const SUPPORTED_LABEL_LANGUAGES = ['de', 'en', 'fr'] as const;
export type LabelLanguage = (typeof SUPPORTED_LABEL_LANGUAGES)[number];

const DEFAULT_LABEL_LANGUAGE: LabelLanguage = 'en';

/** Pure core: map a raw BCP-47 locale (e.g. "de-DE", "fr") to a supported
 *  label language, or the English default. Lower-cases and takes the primary
 *  subtag so "DE-de", "de_DE", "de-AT" all resolve to "de". Components feed it
 *  the UI `locale()` rune (Phase 144). */
export function pickViewerLabelLanguage(rawLocale: string | undefined | null): LabelLanguage {
  if (!rawLocale) return DEFAULT_LABEL_LANGUAGE;
  const primary = rawLocale.toLowerCase().split(/[-_]/)[0] ?? '';
  return (SUPPORTED_LABEL_LANGUAGES as readonly string[]).includes(primary)
    ? (primary as LabelLanguage)
    : DEFAULT_LABEL_LANGUAGE;
}
