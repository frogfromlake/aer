# Engine asset cache (`static/data/`)

Pre-baked assets consumed by `@aer/engine-3d` at runtime. These files are
committed because they are the result of a deterministic, one-shot bake
from a public-domain source — re-baking on every CI run would require
network access and produce byte-identical output.

## Files

| File                 | Purpose                                                                                               |
| -------------------- | ----------------------------------------------------------------------------------------------------- |
| `landmass.sdf.png`   | Equirectangular Signed Distance Field of global land polygons. Red channel; 0.5 at the coastline.     |
| `landmass.meta.json` | Sidecar with the exact bake parameters (resolution, SDF range, source dataset) and the ISO bake date. |

## Provenance

All data derives from **Natural Earth 1:10m** (public-domain) — see
<https://www.naturalearthdata.com/>. The fetch URL and exact processing
parameters live in `services/dashboard/scripts/bake-landmass.mjs`.

## Re-baking

```sh
cd services/dashboard
pnpm run bake-landmass
```

Re-bake when:

- The SDF resolution or range parameters are intentionally changed.
- Natural Earth ships a new version of the source dataset.
- Rendering regressions at high zoom turn out to stem from the baked
  texture rather than the shader.

Commit the resulting PNG together with the `landmass.meta.json` update so
reviewers can read the bake parameters in the same diff.
