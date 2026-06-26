// URL read/write API + enum parsers. Types/consts in ./url-types (re-exported);
// compact codec in ./url-codec. Stable import path $lib/state/url-internals. (Phase 141.)
export * from './url-types';
import type { UrlState } from './url-types';
import { clampLocale, NORMALIZATIONS, RESOLUTIONS, VIEWING_MODES } from './url-types';
import { encodePillarState, decodePillarState } from './url-codec';

// SEC-093 — an RFC-3339-style instant: a full date, at least `THH:mm`, and a
// `Z`/`±hh:mm` designator. Requiring the time + offset rejects ambiguous
// partials (`2026`, `2026-04`, `2026-04-01`) that `new Date()` would otherwise
// coerce into a valid-but-unintended window from a malformed/hand-edited URL.
const ISO_INSTANT = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:\d{2})$/;

function parseIso(v: string | null): string | null {
  if (v === null) return null;
  if (!ISO_INSTANT.test(v)) return null;
  // The regex pins the shape; new Date still rejects impossible calendar values
  // (e.g. month 13) and toISOString normalises to the canonical UTC form.
  const d = new Date(v);
  return Number.isNaN(d.getTime()) ? null : d.toISOString();
}

function parseEnum<T extends string>(v: string | null, allowed: readonly T[]): T | null {
  if (v === null) return null;
  return (allowed as readonly string[]).includes(v) ? (v as T) : null;
}

export function readFromSearch(search: string): UrlState {
  const p = new URLSearchParams(search);
  // Phase 122k — single canonical grammar. Pillar-state base64url is the
  // only form for Workbench state; `?selectedProbes=…` carries the
  // Atmos/Modal selection consumed by Dossier and Workbench.
  const alephRaw = p.get('aleph');
  const epistemeRaw = p.get('episteme');
  const rhizomeRaw = p.get('rhizome');
  const aleph = alephRaw !== null ? decodePillarState(alephRaw) : null;
  const episteme = epistemeRaw !== null ? decodePillarState(epistemeRaw) : null;
  const rhizome = rhizomeRaw !== null ? decodePillarState(rhizomeRaw) : null;
  // The pillars wrapper is populated whenever ANY pillar key is present in
  // the URL — even if decoding failed (the per-pillar slot is then null).
  // This preserves the diagnostic "pillar URL with malformed payload"
  // case so the dashboard can render a refusal surface rather than
  // silently falling back to an empty Workbench.
  const hasPillars = alephRaw !== null || epistemeRaw !== null || rhizomeRaw !== null;
  return {
    from: parseIso(p.get('from')),
    to: parseIso(p.get('to')),
    // Phase 144 — UI locale deep-link. Lenient parse ("de-DE" → "de").
    lang: clampLocale(p.get('lang')),
    resolution: parseEnum(p.get('resolution'), RESOLUTIONS),
    normalization: parseEnum(p.get('normalization'), NORMALIZATIONS),
    activePillar: parseEnum(p.get('activePillar'), VIEWING_MODES),
    pillars: hasPillars ? { aleph, episteme, rhizome } : null,
    selectedProbes: parseIdList(p.get('selectedProbes')),
    dossier: p.get('dossier') === 'open' ? 'open' : null,
    account: p.get('account') === 'open' ? 'open' : null,
    admin: p.get('admin') === 'open' ? 'open' : null,
    about: p.get('about') === 'open' ? 'open' : null,
    guide: p.get('guide') === 'open' ? 'open' : null,
    analyses: p.get('analyses') === 'open' ? 'open' : p.get('analyses') === 'save' ? 'save' : null,
    savedAnalysis: p.get('savedAnalysis') || null
  };
}

function parseIdList(v: string | null): string[] {
  if (!v) return [];
  return v
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s.length > 0);
}

export function writeToSearch(state: UrlState): string {
  const p = new URLSearchParams();
  if (state.from) p.set('from', state.from);
  if (state.to) p.set('to', state.to);
  // Phase 144 — preserve the UI locale across every URL mutation (this
  // function rebuilds the query string from scratch). Emitted only when
  // explicitly pinned; the rune's localStorage/navigator fallback covers the
  // unpinned case so a clean URL stays clean.
  if (state.lang) p.set('lang', state.lang);
  if (state.resolution) p.set('resolution', state.resolution);
  // Phase 122k — single canonical Workbench grammar.
  if (state.pillars) {
    if (state.activePillar) p.set('activePillar', state.activePillar);
    if (state.pillars.aleph) p.set('aleph', encodePillarState(state.pillars.aleph));
    if (state.pillars.episteme) p.set('episteme', encodePillarState(state.pillars.episteme));
    if (state.pillars.rhizome) p.set('rhizome', encodePillarState(state.pillars.rhizome));
  } else if (state.activePillar) {
    // ActivePillar without a pillar payload is meaningful on the Dossier
    // and Atmos surfaces (it lets the user pre-select which pillar a
    // future Workbench will open into) — emit it so a reload restores it.
    p.set('activePillar', state.activePillar);
  }
  // Normalization is omitted when raw (the default) so the URL stays
  // clean for the Level-1 view.
  if (state.normalization && state.normalization !== 'raw') {
    p.set('normalization', state.normalization);
  }
  // Phase 122k — probe selection set.
  if (state.selectedProbes.length > 0) {
    p.set('selectedProbes', state.selectedProbes.join(','));
  }
  // Phase 123a — Dossier overlay state.
  if (state.dossier === 'open') p.set('dossier', 'open');
  // Phase 134 — account / admin overlays.
  if (state.account === 'open') p.set('account', 'open');
  if (state.admin === 'open') p.set('admin', 'open');
  // Phase 149 — About AĒR overlay.
  if (state.about === 'open') p.set('about', 'open');
  // Guided tour launch trigger (consumed + cleared by the TutorialOverlay).
  if (state.guide === 'open') p.set('guide', 'open');
  // Phase 135 — analyses overlay.
  if (state.analyses) p.set('analyses', state.analyses);
  // Phase 135 — preserve the "loaded saved analysis" marker across mutations.
  if (state.savedAnalysis) p.set('savedAnalysis', state.savedAnalysis);
  const qs = p.toString();
  return qs.length === 0 ? '' : `?${qs}`;
}

// ---------------------------------------------------------------------------
// Phase 122i / ADR-034 — PillarState encoder / decoder.
//
// Encoded shape uses short keys so the URL stays under the 8 KiB cap even
// for the realistic worst-case of 4 windows × 4 panels × multiple scope
// groups. Base64url (RFC 4648 §5) keeps the payload URL-safe; no gzip is
// applied because typical states stay well under 2 KiB and gzip would
// force asynchronous CompressionStream usage in what is otherwise a
// synchronous read/write path. Hand-rolled type guards validate every
// nested field on decode; malformed input returns `null` and the
// rune-store renders a refusal surface.
//
// Short-key map:
//   w  → windows[]              p   → panels[]            s  → scopes[]
//   pi → probeIds[]             si  → sourceIds[]
//   c  → composition ("m"|"s")  v   → view (full string)  m  → metric
//   l  → layer ("g"|"s")        r   → resolution          n  → normalization
//   tN → topN                   L   → locked (1)          lr → lockedReason
//   lf → lockedFunction         fi  → focusedPanelIndex
//   aw → activeWindowIndex      bn  → bins                ch → channels
//   sb → showBand=false (0)     fs  → forceStrength       dl → displayLanguage=viewer (1)
//   co → cellOverrides (Phase 126; map cellKey → CompactCellOverride)
//
// Phase 126 note — inside a CompactCellOverride the enum levers (`sb`, `sc`,
// `dl`) are PRESENCE-encoded (0 or 1, only when overridden) rather than
// default-omitted like the panel-level keys: an override must be able to set a
// lever to its non-default value AND to its default value, so "absent" has to
// mean "inherit", not "default".
//
// `c` and `l` get a one-letter alphabet because they're the only
// well-bounded enums where one-letter keys aid compression; `v`/`r`/`n`
// keep their canonical strings because their alphabets carry semantic
// value that aids URL debuggability.
// ---------------------------------------------------------------------------

export { encodePillarState, decodePillarState };
