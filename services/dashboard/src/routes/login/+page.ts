// Client-only, pre-auth login page. No SSR / prerender — it does a live
// /auth/me check and talks to the BFF.
export const ssr = false;
export const prerender = false;
