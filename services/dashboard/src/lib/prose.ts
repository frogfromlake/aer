// Long-form prose helpers — Phase 148g.
//
// The content catalogue's `long` registers (and the per-(view×metric) pairing
// prose) are authored as YAML folded scalars (`>`): blank lines between
// paragraphs become single newlines, lines within a paragraph become spaces.
// Rendered in a single element those newlines collapse to whitespace, so the
// reader sees one dense wall of text. Splitting on newlines restores the
// authored paragraph breaks so each can be rendered as its own <p>.

/** Split authored long-form prose into its paragraphs (trimmed, blanks dropped). */
export function splitParagraphs(text: string | null | undefined): string[] {
  if (!text) return [];
  return text
    .split(/\n+/)
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}
