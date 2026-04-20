#!/usr/bin/env node
// Bake Natural Earth land + admin-0 boundary GeoJSON into compact static assets
// consumed by `@aer/engine-3d` at runtime.
//
// Why: the 3D Atmosphere globe (Phase 99a) renders landmasses as a triangulated
// vector mesh on the sphere — never as satellite imagery (Design Brief §3.1).
// This bake step downloads the Natural Earth source, normalises winding,
// triangulates land polygons via earcut, and writes:
//
//   static/data/landmass.json   — { positions: number[], indices: number[] }
//                                   positions are unit-sphere Cartesian (xyz),
//                                   indices reference triangles.
//   static/data/borders.json    — { positions: number[] } as gl.LINES pairs
//                                   (lazy-loaded on first toggle).
//
// Provenance: Natural Earth, public-domain (https://www.naturalearthdata.com/).
// Re-bake with `pnpm run bake-landmass` whenever the source data changes or
// the simplification tolerance is adjusted.

import { writeFileSync, mkdirSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import earcut from 'earcut';

const RAW = 'https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson';
const LAND_URL = `${RAW}/ne_50m_land.geojson`;
const BORDERS_URL = `${RAW}/ne_50m_admin_0_boundary_lines_land.geojson`;

// Sphere radius is 1.0 in engine space; landmass mesh sits at +EPS to avoid
// z-fighting with the ocean sphere.
const LAND_RADIUS = 1.0015;
const BORDER_RADIUS = 1.0025;

// Douglas-Peucker tolerance in degrees. 0.35° ≈ 39 km at equator and yields
// ~60 kB gzipped — the chosen balance for Phase 99a's 1.2×–8× zoom range.
// Tightening to 0.25° pushes the asset to ~80 kB gzipped with visibly finer
// coastlines; loosening to 0.5° drops to ~40 kB but small islands begin to
// flatten. Re-bake when later phases push the zoom past country-level.
const SIMPLIFY_TOLERANCE_DEG = 0.35;

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, '..');
const OUT_DIR = resolve(ROOT, 'static', 'data');

async function fetchJson(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`HTTP ${res.status} fetching ${url}`);
  return res.json();
}

// Convert lon/lat (degrees) → unit sphere (x,y,z) using the standard mapping
// used by three.js SphereGeometry's UV unwrap (lon=0 → +Z, lat=0 → equator).
function latLonToXyz(lonDeg, latDeg, radius) {
  const lon = (lonDeg * Math.PI) / 180;
  const lat = (latDeg * Math.PI) / 180;
  const cosLat = Math.cos(lat);
  return [radius * cosLat * Math.sin(lon), radius * Math.sin(lat), radius * cosLat * Math.cos(lon)];
}

// Iterative Ramer-Douglas-Peucker (degree-space). Cheap and good enough for
// 1:50m at the chosen tolerance; we don't need the geodesic version.
function simplifyRing(ring, tol) {
  if (ring.length < 4) return ring;
  const tol2 = tol * tol;
  const keep = new Uint8Array(ring.length);
  keep[0] = 1;
  keep[ring.length - 1] = 1;
  const stack = [[0, ring.length - 1]];
  while (stack.length) {
    const [a, b] = stack.pop();
    let maxD2 = 0;
    let maxI = -1;
    const pa = ring[a];
    const pb = ring[b];
    for (let i = a + 1; i < b; i++) {
      const p = ring[i];
      const d2 = perpendicularDist2(p, pa, pb);
      if (d2 > maxD2) {
        maxD2 = d2;
        maxI = i;
      }
    }
    if (maxI !== -1 && maxD2 > tol2) {
      keep[maxI] = 1;
      stack.push([a, maxI], [maxI, b]);
    }
  }
  const out = [];
  for (let i = 0; i < ring.length; i++) if (keep[i]) out.push(ring[i]);
  return out;
}

function perpendicularDist2(p, a, b) {
  const dx = b[0] - a[0];
  const dy = b[1] - a[1];
  if (dx === 0 && dy === 0) {
    const ex = p[0] - a[0];
    const ey = p[1] - a[1];
    return ex * ex + ey * ey;
  }
  const t = ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / (dx * dx + dy * dy);
  const tc = Math.max(0, Math.min(1, t));
  const ex = a[0] + tc * dx - p[0];
  const ey = a[1] + tc * dy - p[1];
  return ex * ex + ey * ey;
}

function* iterPolygons(geojson) {
  for (const feat of geojson.features) {
    const g = feat.geometry;
    if (!g) continue;
    if (g.type === 'Polygon') yield g.coordinates;
    else if (g.type === 'MultiPolygon') for (const poly of g.coordinates) yield poly;
  }
}

function bakeLandmass(geojson) {
  const positions = [];
  const indices = [];
  let vertexOffset = 0;

  for (const polygon of iterPolygons(geojson)) {
    // polygon = [outerRing, hole1, hole2, ...]; each ring = [[lon,lat], ...]
    const flat = [];
    const holes = [];
    let cursor = 0;

    for (let r = 0; r < polygon.length; r++) {
      const raw = polygon[r];
      // GeoJSON closes rings; drop the duplicate end vertex for earcut.
      const open = raw.slice(0, raw.length - 1);
      const simplified = simplifyRing(open, SIMPLIFY_TOLERANCE_DEG);
      if (simplified.length < 3) continue;
      if (r > 0) holes.push(cursor);
      for (const [lon, lat] of simplified) flat.push(lon, lat);
      cursor += simplified.length;
    }

    if (flat.length < 6) continue;
    const tris = earcut(flat, holes.length ? holes : undefined, 2);

    // Project ring vertices onto the sphere and append to global buffers.
    const ringVertCount = flat.length / 2;
    for (let i = 0; i < ringVertCount; i++) {
      const lon = flat[i * 2];
      const lat = flat[i * 2 + 1];
      const [x, y, z] = latLonToXyz(lon, lat, LAND_RADIUS);
      positions.push(x, y, z);
    }
    for (const idx of tris) indices.push(vertexOffset + idx);
    vertexOffset += ringVertCount;
  }

  return { positions, indices };
}

function bakeBorders(geojson) {
  const positions = [];
  for (const feat of geojson.features) {
    const g = feat.geometry;
    if (!g) continue;
    const lines =
      g.type === 'LineString' ? [g.coordinates] : g.type === 'MultiLineString' ? g.coordinates : [];
    for (const line of lines) {
      const simplified = simplifyRing(line, SIMPLIFY_TOLERANCE_DEG);
      // Emit gl.LINES pairs.
      for (let i = 0; i < simplified.length - 1; i++) {
        const [lonA, latA] = simplified[i];
        const [lonB, latB] = simplified[i + 1];
        const a = latLonToXyz(lonA, latA, BORDER_RADIUS);
        const b = latLonToXyz(lonB, latB, BORDER_RADIUS);
        positions.push(a[0], a[1], a[2], b[0], b[1], b[2]);
      }
    }
  }
  return { positions };
}

function fmt(n) {
  return `${(n / 1024).toFixed(1)} kB`;
}

function roundArray(arr, digits) {
  // Reduce JSON size: 6 significant digits is well under 1 m on a 6,371 km globe.
  const f = 10 ** digits;
  return arr.map((v) => Math.round(v * f) / f);
}

async function main() {
  mkdirSync(OUT_DIR, { recursive: true });

  console.log(`Fetching ${LAND_URL}`);
  const landGeo = await fetchJson(LAND_URL);
  const land = bakeLandmass(landGeo);
  land.positions = roundArray(land.positions, 5);
  const landJson = JSON.stringify(land);
  writeFileSync(resolve(OUT_DIR, 'landmass.json'), landJson);
  console.log(
    `  landmass.json: ${land.positions.length / 3} verts, ${land.indices.length / 3} tris, ${fmt(landJson.length)}`
  );

  console.log(`Fetching ${BORDERS_URL}`);
  const bordersGeo = await fetchJson(BORDERS_URL);
  const borders = bakeBorders(bordersGeo);
  borders.positions = roundArray(borders.positions, 5);
  const bordersJson = JSON.stringify(borders);
  writeFileSync(resolve(OUT_DIR, 'borders.json'), bordersJson);
  console.log(`  borders.json:  ${borders.positions.length / 3} verts, ${fmt(bordersJson.length)}`);

  // Tiny sidecar so the runtime can sanity-check what it loaded.
  const meta = {
    source: 'Natural Earth 1:50m (public-domain) — https://www.naturalearthdata.com/',
    builtAt: new Date().toISOString().slice(0, 10),
    simplifyToleranceDeg: SIMPLIFY_TOLERANCE_DEG,
    landRadius: LAND_RADIUS,
    borderRadius: BORDER_RADIUS
  };
  writeFileSync(resolve(OUT_DIR, 'landmass.meta.json'), JSON.stringify(meta, null, 2) + '\n');
  console.log('Done.');
}

await main();
