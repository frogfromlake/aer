// "How to read this" — legacy export-path entry point (Phase 131, localized
// Phase 144c; refactored Phase 148f).
//
// The implementation moved into `reading-guide.ts` (the Reading Guide composer),
// which owns both the legacy flat `composeHowToRead(...) => string[]` used by
// every cell's CellExport payload AND the richer `composeReadingGuide(...)` for
// the on-screen guide. This module stays a thin re-export so the ~16 existing
// import sites — and `tests/unit/how-to-read.test.ts`, the byte-identical
// compatibility gate — keep working unchanged.

export { composeHowToRead, type HowToReadFacts } from './reading-guide';
