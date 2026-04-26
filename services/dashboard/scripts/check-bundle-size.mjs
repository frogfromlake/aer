#!/usr/bin/env node
// Enforces the initial-bundle budget from Design Brief §7 (Phase 97 commit 2).
//
// The "initial bundle" is the set of JavaScript assets the browser loads on
// first paint of the landing route. That is, in the static build output,
// every <script src> and <link rel="modulepreload" href> referenced from
// build/index.html. We gzip each file, sum the gzipped sizes, and fail if
// the total exceeds BUDGET_BYTES.
//
// The 180 kB total-initial-bundle budget (Design Brief §7) is enforced at
// Phase 98 when Surface I code lands. Phase 97 enforces an 80 kB shell
// budget (shell + router + runtime) per ROADMAP §Phase 97.

import { gzipSync } from 'node:zlib';
import { readFileSync, readdirSync } from 'node:fs';
import { resolve, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const BUDGET_BYTES = 80 * 1024; // 80 kB gzipped — shell budget (Phase 97)
// Phase 99a — engine chunk (three.js + shaders + landmass loader). Lazy-loaded
// on the WebGL2 path only. The shell budget above remains intact.
const ENGINE_CHUNK_BUDGET_BYTES = 250 * 1024;
// Phase 100a — L3 chart chunk (uPlot + thin Svelte wrapper). Lazy-loaded
// when the user descends to the L3 Analysis panel (preloaded on probe
// hover). Tracked as the second-largest lazy chunk so future view-mode
// growth surfaces as a regression here.
//
// Phase 107 (View-Mode Matrix) raises the second-largest lazy chunk's
// budget from 80 kB to 160 kB to accommodate Observable Plot (the
// presentation library committed by ADR-020 §Visualization Stack and
// Brief §5.9). Plot lands in its own dynamic-import chunk that only
// ships when the user picks the EDA × distribution cell — the
// initial-shell budget stays at 80 kB. d3-force lands in a separate
// (smaller) chunk and does not count here.
const L3_CHUNK_BUDGET_BYTES = 160 * 1024;

const __dirname = dirname(fileURLToPath(import.meta.url));
const BUILD_DIR = resolve(__dirname, '..', 'build');

function fail(msg) {
  console.error(`\x1b[31m✗ bundle-size gate failed:\x1b[0m ${msg}`);
  process.exit(1);
}

let indexHtml;
try {
  indexHtml = readFileSync(resolve(BUILD_DIR, 'index.html'), 'utf8');
} catch (err) {
  fail(`could not read ${BUILD_DIR}/index.html — did you run \`pnpm run build\`? (${err.message})`);
}

const refs = new Set();
for (const m of indexHtml.matchAll(/<script[^>]+src="([^"]+)"/g)) refs.add(m[1]);
for (const m of indexHtml.matchAll(/<link[^>]+rel="modulepreload"[^>]+href="([^"]+)"/g))
  refs.add(m[1]);
for (const m of indexHtml.matchAll(/<link[^>]+href="([^"]+)"[^>]+rel="modulepreload"/g))
  refs.add(m[1]);

if (refs.size === 0) {
  fail('no initial bundle references found in build/index.html');
}

let total = 0;
const rows = [];
for (const ref of refs) {
  const rel = ref.replace(/^\/+/, '');
  const abs = resolve(BUILD_DIR, rel);
  let bytes;
  try {
    bytes = readFileSync(abs);
  } catch (err) {
    fail(`referenced asset missing: ${ref} (${err.message})`);
  }
  const gzipped = gzipSync(bytes).length;
  total += gzipped;
  rows.push({ ref, raw: bytes.length, gzip: gzipped });
}

const fmt = (n) => `${(n / 1024).toFixed(2)} kB`;
console.log('Initial bundle (first-paint assets):');
for (const r of rows.sort((a, b) => b.gzip - a.gzip)) {
  console.log(`  ${fmt(r.gzip).padStart(10)} gz  ${r.ref}`);
}
console.log(`  ${'-'.repeat(10)}`);
console.log(`  ${fmt(total).padStart(10)} gz  total  (budget: ${fmt(BUDGET_BYTES)})`);

if (total > BUDGET_BYTES) {
  fail(
    `initial bundle is ${fmt(total)} gzipped, exceeds budget of ${fmt(BUDGET_BYTES)} by ${fmt(total - BUDGET_BYTES)}`
  );
}

// Phase 99a — engine chunk gate. We treat the largest non-shell chunk under
// build/_app/immutable/chunks/ as the lazy engine chunk (three.js dominates it
// by an order of magnitude over any other lazy chunk). If a future phase ships
// a larger lazy chunk, this gate will surface that regression too — that is
// the intent.
const CHUNKS_DIR = resolve(BUILD_DIR, '_app', 'immutable', 'chunks');
const shellRefs = new Set([...refs].map((r) => r.replace(/^\/+/, '')));
const lazyChunks = [];
try {
  for (const entry of readdirSync(CHUNKS_DIR)) {
    if (!entry.endsWith('.js')) continue;
    const rel = `_app/immutable/chunks/${entry}`;
    if (shellRefs.has(rel)) continue;
    const bytes = readFileSync(join(CHUNKS_DIR, entry));
    const gzipped = gzipSync(bytes).length;
    lazyChunks.push({ ref: rel, raw: bytes.length, gzip: gzipped });
  }
} catch (err) {
  fail(`could not enumerate chunks dir ${CHUNKS_DIR}: ${err.message}`);
}

lazyChunks.sort((a, b) => b.gzip - a.gzip);
const engineChunk = lazyChunks[0];
const l3Chunk = lazyChunks[1];

if (!engineChunk) {
  fail('no lazy chunks found — expected the engine-3d chunk to be present');
}

console.log('');
console.log('Engine chunk (lazy, WebGL2 path only):');
console.log(`  ${fmt(engineChunk.gzip).padStart(10)} gz  ${engineChunk.ref}`);
console.log(`  ${'-'.repeat(10)}`);
console.log(
  `  ${fmt(engineChunk.gzip).padStart(10)} gz  largest lazy chunk  (budget: ${fmt(ENGINE_CHUNK_BUDGET_BYTES)})`
);
if (engineChunk.gzip > ENGINE_CHUNK_BUDGET_BYTES) {
  fail(
    `engine chunk is ${fmt(engineChunk.gzip)} gzipped, exceeds budget of ${fmt(ENGINE_CHUNK_BUDGET_BYTES)} by ${fmt(engineChunk.gzip - ENGINE_CHUNK_BUDGET_BYTES)}`
  );
}

// Phase 100a — L3 chart chunk gate. Enforced only once a second lazy
// chunk exists (i.e. uPlot has shipped). Before Phase 100a lands in
// production there may only be a single lazy chunk, so the absence of
// a second chunk is not a failure.
if (l3Chunk) {
  console.log('');
  console.log('L3 chart chunk (lazy, loaded on L3 descent):');
  console.log(`  ${fmt(l3Chunk.gzip).padStart(10)} gz  ${l3Chunk.ref}`);
  console.log(`  ${'-'.repeat(10)}`);
  console.log(
    `  ${fmt(l3Chunk.gzip).padStart(10)} gz  second-largest lazy chunk  (budget: ${fmt(L3_CHUNK_BUDGET_BYTES)})`
  );
  if (l3Chunk.gzip > L3_CHUNK_BUDGET_BYTES) {
    fail(
      `L3 chart chunk is ${fmt(l3Chunk.gzip)} gzipped, exceeds budget of ${fmt(L3_CHUNK_BUDGET_BYTES)} by ${fmt(l3Chunk.gzip - L3_CHUNK_BUDGET_BYTES)}`
    );
  }
}

console.log('\x1b[32m✔ bundle-size gate passed.\x1b[0m');
