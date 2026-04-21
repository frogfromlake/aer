# `@aer/engine-3d`

The AĒR 3D Atmosphere Engine — a vanilla three.js module that renders the
rotating Earth, the live day/night terminator, and an atmospheric-scattering
halo. Framework-agnostic per ADR-020 §5.9: it knows nothing about Svelte,
the BFF, or the rest of the dashboard.

The engine is consumed via a narrow imperative API (`AtmosphereEngine` in
`src/index.ts`). The shell mounts it onto a `<canvas>` and drives it; the
engine never reaches back.

## Public API (Phase 99a)

```ts
import { createEngine, hasWebGL2 } from '@aer/engine-3d';

if (!hasWebGL2()) {
  // Render the static text fallback instead.
  return;
}

const engine = createEngine({ landSdfUrl: '/data/landmass.sdf.png' });
engine.mount(canvasElement);

// Phase 99b will wire these to live BFF data; in 99a they accept empty arrays.
engine.setProbes([]);
engine.setActivity([]);
engine.setPropagationEvents([]);

engine.flyTo({ latitude: 51.16, longitude: 10.45, durationMs: 1200 });

// Tear down (release GL context, dispose geometry/material).
engine.dispose();
```

## Landmasses: Signed Distance Field, not geometry

Land/ocean classification is read from a pre-baked equirectangular Signed
Distance Field (red channel, 0.5 at the coast). The shader reconstructs a
mathematically smooth coastline via bilinear filtering + `fwidth`-based
antialiasing, so coasts stay pixel-sharp at every zoom level with a single
texture fetch per fragment. The globe itself is a single sphere mesh —
there is no land geometry.

Why not triangulated landmasses or satellite imagery:

- Triangulated 10 m meshes either lose small islands (at the simplification
  tolerance needed to stay under the static-asset budget) or balloon past
  it. Either way, polygon edges become visible when the camera zooms past
  ~4×.
- 4k–8k satellite rasters violate Design Brief §3.1 ("symbolic, not
  photoreal") and add 10+ MB of VRAM-expensive texture that still blurs
  near the surface. Region identity comes from probe-bound activity lighting
  up the continental fill (Phase 99b), not from political borders that shift
  over time.

The SDF is produced by `services/dashboard/scripts/bake-landmass.mjs` from
Natural Earth 1:10m data and committed at
`services/dashboard/static/data/landmass.sdf.png`. Re-bake with
`pnpm run bake-landmass` when the source dataset or the encoding parameters
(`WIDTH`, `HEIGHT`, `RANGE_PX`) change.
