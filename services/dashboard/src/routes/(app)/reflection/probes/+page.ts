// The probe aggregate enumerates the live ProbeRegistry and fetches each
// dossier client-side, so it renders client-side like the other dynamic
// Reflection pages rather than being prerendered to a static loading shell.
export const prerender = false;
export const ssr = false;
