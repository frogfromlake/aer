// Vite's `?raw` import suffix returns the file contents as a string. The
// engine uses this for `.glsl` shader sources so they ship as inlined string
// literals in the engine chunk (no extra HTTP round-trip, no asset pipeline).

declare module '*.glsl?raw' {
  const src: string;
  export default src;
}

declare module '*.vert.glsl?raw' {
  const src: string;
  export default src;
}
