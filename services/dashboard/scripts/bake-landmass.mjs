#!/usr/bin/env node
// Bake Natural Earth 1:10m land polygons into a Signed-Distance-Field PNG
// consumed by `@aer/engine-3d` at runtime (Phase 99a, SDF-rendered globe).
//
// Why an SDF and not geometry or a Blue-Marble raster:
//   - Triangulated meshes at 10m resolution either lose small islands (at the
//     simplification tolerance needed to keep JSON ≲100 kB) or balloon past
//     the engine's static-asset budget. Either way, coastlines become a
//     string of visible polygon edges when the camera zooms past ~4×.
//   - 4k–8k satellite imagery violates Design Brief §3.1 ("symbolic, not
//     photoreal") and adds 10+ MB of VRAM-expensive texture that still
//     blurs when the camera approaches country level.
//
// An SDF encodes, per pixel, the signed Euclidean distance to the nearest
// coastline. The shader reconstructs a mathematically smooth coastline at
// any zoom level via bilinear filtering and `fwidth`-based antialiasing —
// no geometry, no seams, a single texture fetch per fragment.
//
// Output:
//   static/data/landmass.sdf.png     — equirectangular, 4096×2048, 8-bit.
//                                       Red channel; 0.5 = coast, >0.5 land,
//                                       <0.5 ocean. Saturates at ±RANGE_PX.
//   static/data/landmass.meta.json   — sidecar: encoding parameters + date.
//
// Pipeline (pure Node, no external CLI):
//   1. Fetch Natural Earth 10m land GeoJSON (public-domain).
//   2. Rasterize polygons (and holes) via even-odd scanline fill.
//   3. Horizontally wrap the mask so the ±180° longitude seam is continuous.
//   4. Two 2-D Euclidean Distance Transforms (Felzenszwalb/Huttenlocher,
//      O(n) per axis): one measures ocean-pixel distance to the nearest
//      land pixel, the other measures land-pixel distance to the nearest
//      ocean pixel.
//   5. Signed distance = sqrt(distToOcean) − sqrt(distToLand), encoded into
//      an 8-bit grayscale PNG via `pngjs`.
//
// Provenance: Natural Earth, public-domain (https://www.naturalearthdata.com/).
// Re-bake with `pnpm run bake-landmass` when the source dataset or the
// encoding parameters (WIDTH, HEIGHT, RANGE_PX) change.

import { writeFileSync, mkdirSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { PNG } from 'pngjs';

const RAW = 'https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson';
const LAND_URL = `${RAW}/ne_10m_land.geojson`;

// Equirectangular: WIDTH must equal 2 * HEIGHT so one pixel spans the same
// number of degrees on both axes — the EDT is then isotropic in pixel space,
// which is what the shader expects when it interprets the sample as distance.
const WIDTH = 4096;
const HEIGHT = 2048;

// Half-width of the smoothing band, in pixels. Distances beyond ±RANGE_PX
// saturate to 0.0 / 1.0, so the full 8-bit dynamic range is spent on the
// coastal strip the shader actually needs. 32 px at 4096 width ≈ 2.8° ≈
// 310 km at the equator — ample for `fwidth`-AA and future coastal-glow
// effects without wasting precision deep in the Pacific or Siberia.
const RANGE_PX = 32;

// Horizontal padding used during the EDT so the wrap boundary at ±180°
// longitude cannot influence any pixel within the kept output.
const PAD_X = RANGE_PX * 2;

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');
const OUT_DIR = resolve(ROOT, 'static', 'data');

async function fetchJson(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
  return res.json();
}

function* iterPolygons(geojson) {
  for (const feat of geojson.features) {
    const g = feat.geometry;
    if (!g) continue;
    if (g.type === 'Polygon') yield g.coordinates;
    else if (g.type === 'MultiPolygon') for (const poly of g.coordinates) yield poly;
  }
}

// Even-odd scanline fill. Outer rings and holes share the same pass — hole
// edges flip parity at each intersection, so nested rings fill correctly
// without a separate subtraction step.
function rasterize(geojson, width, height) {
  const mask = new Uint8Array(width * height);
  const scanlines = Array.from({ length: height }, () => []);

  const lonToX = (lon) => ((lon + 180) / 360) * width;
  const latToY = (lat) => ((90 - lat) / 180) * height;

  let edgeCount = 0;
  for (const polygon of iterPolygons(geojson)) {
    for (const ring of polygon) {
      if (ring.length < 4) continue;
      for (let i = 0; i < ring.length - 1; i++) {
        const [lon1, lat1] = ring[i];
        const [lon2, lat2] = ring[i + 1];
        addEdge(scanlines, lonToX(lon1), latToY(lat1), lonToX(lon2), latToY(lat2), height);
        edgeCount++;
      }
    }
  }

  for (let y = 0; y < height; y++) {
    const xs = scanlines[y];
    if (xs.length < 2) continue;
    xs.sort((a, b) => a - b);
    for (let i = 0; i + 1 < xs.length; i += 2) {
      const xStart = Math.max(0, Math.ceil(xs[i]));
      const xEnd = Math.min(width, Math.ceil(xs[i + 1]));
      const row = y * width;
      for (let x = xStart; x < xEnd; x++) mask[row + x] = 1;
    }
  }

  return { mask, edgeCount };
}

function addEdge(scanlines, x1, y1, x2, y2, height) {
  if (y1 === y2) return; // horizontal edge: no parity flip in even-odd fill
  const yMin = Math.min(y1, y2);
  const yMax = Math.max(y1, y2);
  const yStart = Math.max(0, Math.ceil(yMin));
  // yMax is exclusive: a vertex shared between two edges must flip parity
  // exactly once, so the lower edge owns the intersection and the upper
  // edge skips it.
  const yEnd = Math.min(height - 1, Math.floor(yMax - 1e-9));
  if (yEnd < yStart) return;
  const slope = (x2 - x1) / (y2 - y1);
  for (let y = yStart; y <= yEnd; y++) {
    scanlines[y].push(x1 + (y - y1) * slope);
  }
}

// Felzenszwalb/Huttenlocher 1-D squared Euclidean distance transform.
// `f[i]` is 0 at source pixels and +Inf elsewhere. `d`, `v`, `z` are
// pre-allocated scratch buffers so the 2-D wrapper avoids per-row churn.
function edt1d(f, n, d, v, z) {
  let k = 0;
  v[0] = 0;
  z[0] = -Infinity;
  z[1] = Infinity;
  for (let q = 1; q < n; q++) {
    let s = (f[q] + q * q - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
    while (s <= z[k]) {
      k--;
      s = (f[q] + q * q - (f[v[k]] + v[k] * v[k])) / (2 * q - 2 * v[k]);
    }
    k++;
    v[k] = q;
    z[k] = s;
    z[k + 1] = Infinity;
  }
  k = 0;
  for (let q = 0; q < n; q++) {
    while (z[k + 1] < q) k++;
    d[q] = (q - v[k]) * (q - v[k]) + f[v[k]];
  }
}

// 2-D squared EDT: for every pixel, distance² to the nearest pixel whose
// mask value equals `target`. Two 1-D sweeps (columns, then rows).
function edt2d(mask, width, height, target) {
  const INF = 1e20;
  const out = new Float64Array(width * height);
  const maxDim = Math.max(width, height);
  const f = new Float64Array(maxDim);
  const d = new Float64Array(maxDim);
  const v = new Int32Array(maxDim);
  const z = new Float64Array(maxDim + 1);

  for (let x = 0; x < width; x++) {
    for (let y = 0; y < height; y++) {
      f[y] = mask[y * width + x] === target ? 0 : INF;
    }
    edt1d(f, height, d, v, z);
    for (let y = 0; y < height; y++) out[y * width + x] = d[y];
  }

  for (let y = 0; y < height; y++) {
    const row = y * width;
    for (let x = 0; x < width; x++) f[x] = out[row + x];
    edt1d(f, width, d, v, z);
    for (let x = 0; x < width; x++) out[row + x] = d[x];
  }

  return out;
}

function wrapHorizontally(mask, width, height, pad) {
  const newW = width + 2 * pad;
  const out = new Uint8Array(newW * height);
  for (let y = 0; y < height; y++) {
    const srcRow = y * width;
    const dstRow = y * newW;
    for (let i = 0; i < pad; i++) out[dstRow + i] = mask[srcRow + (width - pad + i)];
    for (let x = 0; x < width; x++) out[dstRow + pad + x] = mask[srcRow + x];
    for (let i = 0; i < pad; i++) out[dstRow + pad + width + i] = mask[srcRow + i];
  }
  return out;
}

function cropHorizontally(buf, paddedW, height, pad, width) {
  const out = new Float64Array(width * height);
  for (let y = 0; y < height; y++) {
    const src = y * paddedW + pad;
    const dst = y * width;
    for (let x = 0; x < width; x++) out[dst + x] = buf[src + x];
  }
  return out;
}

function encodeSdfGray(distToOcean2, distToLand2, width, height, rangePx) {
  const out = Buffer.alloc(width * height);
  const invRange = 1 / (2 * rangePx);
  for (let i = 0; i < width * height; i++) {
    // Signed distance: +positive inside land, negative in ocean, 0 at coast.
    //   land pixel  → distToOcean > 0, distToLand = 0 → signed = +distToOcean
    //   ocean pixel → distToOcean = 0, distToLand > 0 → signed = −distToLand
    const signed = Math.sqrt(distToOcean2[i]) - Math.sqrt(distToLand2[i]);
    let v = 0.5 + signed * invRange;
    if (v < 0) v = 0;
    else if (v > 1) v = 1;
    out[i] = Math.round(v * 255);
  }
  return out;
}

function fmt(n) {
  return `${(n / 1024).toFixed(1)} kB`;
}

async function main() {
  mkdirSync(OUT_DIR, { recursive: true });

  console.log(`Fetching ${LAND_URL}`);
  const landGeo = await fetchJson(LAND_URL);

  console.log(`Rasterizing ${WIDTH}×${HEIGHT} land mask…`);
  const { mask, edgeCount } = rasterize(landGeo, WIDTH, HEIGHT);
  console.log(`  ${edgeCount} edges rasterised`);

  console.log(`Wrapping ±${PAD_X} columns for seam continuity…`);
  const paddedW = WIDTH + 2 * PAD_X;
  const padded = wrapHorizontally(mask, WIDTH, HEIGHT, PAD_X);

  // `edt2d(mask, target)` measures distance to the nearest pixel whose mask
  // value equals `target` — so `target=1` yields "distance to the nearest
  // land pixel" (zero on land, positive in ocean), and `target=0` yields
  // "distance to the nearest ocean pixel" (zero in ocean, positive on land).
  // To get a signed distance that is +positive inside land, subtract the
  // distance-to-land from the distance-to-ocean.
  console.log('EDT: distance to nearest ocean (positive on land)…');
  const distToOcean = cropHorizontally(
    edt2d(padded, paddedW, HEIGHT, 0),
    paddedW,
    HEIGHT,
    PAD_X,
    WIDTH
  );

  console.log('EDT: distance to nearest land (positive in ocean)…');
  const distToLand = cropHorizontally(
    edt2d(padded, paddedW, HEIGHT, 1),
    paddedW,
    HEIGHT,
    PAD_X,
    WIDTH
  );

  console.log('Encoding signed distance field…');
  const gray = encodeSdfGray(distToOcean, distToLand, WIDTH, HEIGHT, RANGE_PX);

  // 8-bit grayscale output (colorType 0). pngjs's grayscale packer reads
  // one byte per pixel from `png.data`, so we overwrite the constructor's
  // default RGBA-sized buffer with our tightly-packed grayscale buffer —
  // otherwise pngjs reads RGBA-striped bytes as if they were grayscale
  // pixels, producing a characteristic 4-wide stride artefact.
  const png = new PNG({
    width: WIDTH,
    height: HEIGHT,
    colorType: 0,
    inputColorType: 0,
    inputHasAlpha: false,
    bitDepth: 8
  });
  png.data = gray;
  const buffer = PNG.sync.write(png, {
    deflateLevel: 9,
    deflateStrategy: 3,
    colorType: 0,
    inputColorType: 0,
    inputHasAlpha: false,
    bitDepth: 8
  });
  const outPath = resolve(OUT_DIR, 'landmass.sdf.png');
  writeFileSync(outPath, buffer);
  console.log(`  landmass.sdf.png: ${fmt(buffer.length)}`);

  const meta = {
    source: 'Natural Earth 1:10m (public-domain) — https://www.naturalearthdata.com/',
    builtAt: new Date().toISOString().slice(0, 10),
    encoding: 'sdf-equirectangular',
    width: WIDTH,
    height: HEIGHT,
    rangePx: RANGE_PX,
    rangeDeg: (RANGE_PX * 360) / WIDTH,
    coastValue: 0.5,
    note: 'Grayscale red channel. value > 0.5 = land, value < 0.5 = ocean. Distance saturates at ±rangePx pixels from the coast.'
  };
  writeFileSync(resolve(OUT_DIR, 'landmass.meta.json'), JSON.stringify(meta, null, 2) + '\n');
  console.log('Done.');
}

await main();
