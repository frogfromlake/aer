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
import { readFileSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const BUDGET_BYTES = 80 * 1024; // 80 kB gzipped

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
console.log('\x1b[32m✔ bundle-size gate passed.\x1b[0m');
