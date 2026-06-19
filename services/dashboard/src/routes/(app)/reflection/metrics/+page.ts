// The metric aggregate enumerates the live /metrics/available catalogue and
// fetches each provenance record client-side, so it renders client-side like the
// other dynamic Reflection pages rather than being prerendered.
export const prerender = false;
export const ssr = false;
