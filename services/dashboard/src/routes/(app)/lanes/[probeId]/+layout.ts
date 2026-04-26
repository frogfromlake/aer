// Dynamic probe routes are never prerendered — the probe ID comes from
// client navigation (globe click or direct URL). The static adapter's
// fallback: 'index.html' serves these routes via the SPA shell.
export const prerender = false;
export const ssr = false;
