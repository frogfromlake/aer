// Identity display helpers — pure, dependency-free, so they unit-test cleanly
// and can be reused by the avatar, the account header, and the saved-analyses
// owner column. Names are an optional identity layer (added in Phase 148e); when
// absent these helpers fall back to the email so a brand-new or nameless account
// still renders something sensible.

export interface IdentityLike {
  firstName?: string | null | undefined;
  lastName?: string | null | undefined;
  email?: string | null | undefined;
}

/** First Unicode code point of a string (umlaut-safe; `Array.from` splits on
 *  code points, not UTF-16 units). Empty string in → empty string out. */
function firstCodePoint(s: string): string {
  for (const cp of s) return cp;
  return '';
}

/** Initials from the local-part of an email. `anna.schmidt@x` → `AS`;
 *  `nelixposteo@x` → `NE`. Returns `''` for an empty/blank email. */
function initialsFromEmail(email: string): string {
  const local = email.trim().split('@')[0] ?? '';
  if (!local) return '';
  const parts = local.split(/[._\-+]/).filter(Boolean);
  if (parts.length >= 2) {
    return (firstCodePoint(parts[0] ?? '') + firstCodePoint(parts[1] ?? '')).toUpperCase();
  }
  // Single token: take the first two code points.
  const cps = Array.from(local);
  return (cps.slice(0, 2).join('') || cps[0] || '').toUpperCase();
}

/** Up to two uppercase initials for an avatar disc. Prefers given + family
 *  name (first code point of each); falls back to the email local-part. Never
 *  throws; returns `''` only when neither a name nor an email is present. */
export function initials(id: IdentityLike): string {
  const first = (id.firstName ?? '').trim();
  const last = (id.lastName ?? '').trim();
  if (first || last) {
    const a = firstCodePoint(first);
    const b = firstCodePoint(last);
    return (a + b || a || b).toUpperCase();
  }
  return initialsFromEmail(id.email ?? '');
}

/** Human display name: `First Last` when a name is present, otherwise the
 *  email. Used as the primary owner label (email stays as the secondary line). */
export function displayName(id: IdentityLike): string {
  const first = (id.firstName ?? '').trim();
  const last = (id.lastName ?? '').trim();
  const full = [first, last].filter(Boolean).join(' ');
  return full || (id.email ?? '').trim();
}

/** True when the identity carries a real (non-blank) given or family name —
 *  i.e. `displayName` returns a name rather than the email fallback. */
export function hasName(id: IdentityLike): boolean {
  return Boolean((id.firstName ?? '').trim() || (id.lastName ?? '').trim());
}
