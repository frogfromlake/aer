#!/usr/bin/env node
// Working-Paper sync (Phase 144 / ADR-042). Copies the per-locale Working-Paper
// markdown from the methodology source of truth into the dashboard's static
// assets, normalising the verbose source filenames to wp-NNN.md:
//
//   docs/methodology/{en,de}/WP-NNN-{locale}-<slug>.md
//     → services/dashboard/static/content/papers/{en,de}/wp-NNN.md
//
// The generated files ARE committed: the dashboard Docker build context is the
// services/dashboard/ tree only (docs/ is not copied), so the static papers
// must already exist at image-build time. Run `pnpm run sync-papers` whenever a
// Working Paper changes; `--check` (used in CI/lint) fails on drift, mirroring
// the openapi codegen drift gate.
import { mkdirSync, readdirSync, readFileSync, rmSync, writeFileSync, existsSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const SRC = join(ROOT, '..', '..', 'docs', 'methodology');
const OUT = join(ROOT, 'static', 'content', 'papers');
const LOCALES = ['en', 'de'];
const checkOnly = process.argv.includes('--check');

const FILE_RE = /^WP-(\d+)-[a-z]{2}-.*\.md$/i;

let drift = false;
for (const locale of LOCALES) {
  const srcDir = join(SRC, locale);
  const outDir = join(OUT, locale);
  if (!checkOnly) {
    rmSync(outDir, { recursive: true, force: true });
    mkdirSync(outDir, { recursive: true });
  }
  const files = readdirSync(srcDir).filter((f) => FILE_RE.test(f));
  for (const file of files) {
    const num = file.match(FILE_RE)[1];
    const target = join(outDir, `wp-${num}.md`);
    const content = readFileSync(join(srcDir, file), 'utf8');
    if (checkOnly) {
      const current = existsSync(target) ? readFileSync(target, 'utf8') : null;
      if (current !== content) {
        console.error(`✖ papers out of sync: ${locale}/wp-${num}.md differs from ${file}`);
        drift = true;
      }
    } else {
      writeFileSync(target, content);
    }
  }
}

if (checkOnly && drift) {
  console.error('Run `pnpm run sync-papers` and commit the result.');
  process.exit(1);
}
console.log(`✔ Working papers ${checkOnly ? 'in sync' : 'synced'} (${LOCALES.join(', ')}).`);
