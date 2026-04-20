# Engine asset cache (`static/data/`)

Pre-baked vector assets consumed by `@aer/engine-3d` at runtime. These files
are committed because they are the result of a deterministic, one-shot bake
from a public-domain source — re-baking on every CI run would require network
access and produce byte-identical output.

## Files

| File | Purpose |
|------|---------|
| `landmass.json` | Triangulated land mesh (positions on the unit sphere + indices). Fed to the globe as a `BufferGeometry`. |
| `landmass.meta.json` | Sidecar with the exact bake parameters and the ISO date the bake ran. |
| `borders.json` | Country borders as `gl.LINES` vertex pairs. Lazy-loaded only when `engine.setBordersVisible(true)` is called. |

## Provenance

All data derives from **Natural Earth 1:50m** (public-domain) — see
<https://www.naturalearthdata.com/>. The fetch URLs and exact processing
parameters live in `services/dashboard/scripts/bake-landmass.mjs`.

## Re-baking

```sh
cd services/dashboard
pnpm run bake-landmass
```

Re-bake when:

- The simplification tolerance is intentionally changed.
- Natural Earth ships a new version of the source dataset.
- A later phase requires higher resolution (e.g. zoom past country level).

Commit the resulting JSON files together with the `landmass.meta.json`
update so reviewers can read the bake parameters in the same diff.
