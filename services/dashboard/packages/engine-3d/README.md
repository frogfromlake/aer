# `@aer/engine-3d`

The AĒR 3D Atmosphere Engine — a vanilla three.js module that renders the
rotating Earth, vector landmasses, the live day/night terminator, and an
atmospheric scattering halo. Framework-agnostic per ADR-020 §5.9: it knows
nothing about Svelte, the BFF, or the rest of the dashboard.

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

const engine = createEngine({ landmassUrl: '/data/landmass.json' });
engine.mount(canvasElement);

// Phase 99b will wire these to live BFF data; in 99a they accept empty arrays.
engine.setProbes([]);
engine.setActivity([]);
engine.setPropagationEvents([]);

engine.flyTo({ latitude: 51.16, longitude: 10.45, durationMs: 1200 });
engine.setBordersVisible(true); // lazy-loads the borders asset on first call

// Tear down (release GL context, dispose geometry/material).
engine.dispose();
```

## Why no satellite texture?

Per Design Brief §3.1, the surface is "restrained silhouettes — no political
borders, no country labels at the highest altitude". A Blue-Marble-style raster
would (a) compete with the discourse signal the globe carries, (b) blur under
the country-level zoom that later phases require, and (c) introduce a
~150-300 ms texture-decode hitch on first paint. Vector landmasses from
Natural Earth 1:50m are crisp at every zoom and add only ~60 kB to the
engine chunk.
