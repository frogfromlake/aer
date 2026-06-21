// Reads the single-use auth token (invite / password-reset) from the URL
// fragment, e.g. `#token=abc` (SEC-009). The token rides in the fragment, never
// the query string, so it is never sent to a server and never lands in access
// logs. Accepts the fragment with or without the leading `#`.
export function tokenFromHash(hash: string): string {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash;
  return new URLSearchParams(raw).get('token') ?? '';
}
