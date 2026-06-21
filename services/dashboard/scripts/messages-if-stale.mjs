#!/usr/bin/env node
// Compile Paraglide i18n messages ONLY when the sources changed.
//
// `pnpm run messages` is prepended to lint/check/knip/test:unit for standalone
// safety, so a full `make fe-lint` would otherwise run `paraglide-js compile`
// 3× (~3.8s each) every time — even on commits that touch no message file.
// This guard makes the compile a near-instant no-op when the generated output
// is already newer than every input, so:
//   * within one fe-lint run, only the first call compiles; the rest skip;
//   * commits that don't touch i18n skip the compile entirely.
//
// Correctness: it over-compiles on ambiguity (output missing, or any input
// newer) and only skips when the output is provably fresh — it never silently
// skips a needed compile. The Vite build path is unaffected (it uses
// paraglideVitePlugin, not this script). The generated output is gitignored.
import { execSync } from 'node:child_process';
import { statSync, readdirSync, existsSync } from 'node:fs';
import { join } from 'node:path';

const ROOT = process.cwd(); // services/dashboard (pnpm sets cwd to the package dir)
const INPUTS = ['messages', 'project.inlang']; // sources; `cache` dirs excluded below
const OUTPUT = 'src/lib/paraglide';

// Latest mtime under a path, skipping volatile `cache` dirs (inlang writes a
// cache under project.inlang/ during compile — including it would make the
// guard always think the inputs changed).
function latestMtimeMs(path) {
  const st = statSync(path);
  if (!st.isDirectory()) return st.mtimeMs;
  let max = st.mtimeMs;
  for (const entry of readdirSync(path, { withFileTypes: true })) {
    if (entry.isDirectory() && entry.name === 'cache') continue;
    max = Math.max(max, latestMtimeMs(join(path, entry.name)));
  }
  return max;
}

const outPath = join(ROOT, OUTPUT);
let reason = null;
if (!existsSync(outPath)) {
  reason = 'output missing';
} else {
  const outM = latestMtimeMs(outPath);
  let inM = 0;
  for (const inp of INPUTS) {
    const p = join(ROOT, inp);
    if (existsSync(p)) inM = Math.max(inM, latestMtimeMs(p));
  }
  if (inM > outM) reason = 'messages changed';
}

if (reason) {
  console.log(`ℹ [messages] ${reason} → compiling paraglide…`);
  execSync('pnpm run messages:compile', { stdio: 'inherit', cwd: ROOT });
} else {
  console.log('✔ [messages] up-to-date — skipping paraglide compile');
}
