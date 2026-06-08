// Open-redirect guard for the `?redirect=` login parameter (Phase 134 /
// ADR-040). Only same-app, absolute-path targets are allowed; anything that
// could leave the origin (protocol-relative `//host`, absolute URLs, or a
// non-path) falls back to the app root.

export function safeRedirect(raw: string | null | undefined): string {
  if (!raw) return '/';
  // Must be an absolute in-app path and NOT protocol-relative (`//evil.com`).
  if (!raw.startsWith('/') || raw.startsWith('//') || raw.startsWith('/\\')) return '/';
  return raw;
}
