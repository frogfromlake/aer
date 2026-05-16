// Workbench route — analytical surface (ADR-033). The Workbench is a
// single SPA route that switches between three Pillar Shells (Aleph /
// Episteme / Rhizome) based on `url.viewingMode`. URL params (`probes`,
// `functions`, `pillar`, `metric`, `view`, `layer`, `from`, `to`,
// `normalization`, `sourceIds`) carry scope through the SPA.
export const prerender = false;
export const ssr = false;
