#!/usr/bin/env node
// i18n parity gate (Phase 144 / ADR-042). Fails CI when the per-feature
// Paraglide message files drift out of EN⇄DE parity, so a missing translation
// is caught at lint time instead of being served as silent English.
//
// Checks, across messages/{en,de}/*.json:
//   1. Key parity — every key present in one locale exists in the other.
//   2. No duplicate keys within a locale. The messageFormat plugin merges all
//      files into one flat namespace (last-file-wins), so a duplicated key is a
//      silent override bug; keys are feature-prefixed to stay globally unique.
//   3. No forgotten translation — a value that is blank in a NON-base locale
//      while the base locale has content is a stub. A value blank in the BASE
//      locale is treated as an intentional empty fragment (e.g. a plural-suffix
//      message whose singular form is the empty string) and is allowed as long
//      as the other locale is also blank.
//
// Pure Node, no deps. Run via `pnpm run i18n-parity` (part of `pnpm run lint`).
import { readdirSync, readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..');
const MESSAGES = join(ROOT, 'messages');
const LOCALES = ['en', 'de'];

/** @returns {{ values: Map<string, string>, dupes: string[] }} */
function loadLocale(locale) {
  const dir = join(MESSAGES, locale);
  const values = new Map();
  const dupes = [];
  const files = readdirSync(dir)
    .filter((f) => f.endsWith('.json'))
    .sort();
  for (const file of files) {
    const json = JSON.parse(readFileSync(join(dir, file), 'utf8'));
    for (const [key, value] of Object.entries(json)) {
      if (key === '$schema') continue;
      if (values.has(key)) dupes.push(`${key} (re-declared in ${locale}/${file})`);
      values.set(key, typeof value === 'string' ? value : String(value));
    }
  }
  return { values, dupes };
}

const isBlank = (v) => typeof v === 'string' && v.trim() === '';

const loaded = Object.fromEntries(LOCALES.map((l) => [l, loadLocale(l)]));
const errors = [];

// 1. Key parity (pairwise against the base locale 'en').
const [base, ...others] = LOCALES;
const baseKeys = [...loaded[base].values.keys()];
for (const other of others) {
  const missingInOther = baseKeys.filter((k) => !loaded[other].values.has(k));
  const missingInBase = [...loaded[other].values.keys()].filter((k) => !loaded[base].values.has(k));
  if (missingInOther.length)
    errors.push(`Keys in ${base} but missing in ${other}:\n  ${missingInOther.join('\n  ')}`);
  if (missingInBase.length)
    errors.push(`Keys in ${other} but missing in ${base}:\n  ${missingInBase.join('\n  ')}`);
}

// 2. Duplicate keys per locale.
for (const l of LOCALES) {
  if (loaded[l].dupes.length)
    errors.push(`Duplicate keys in ${l}:\n  ${loaded[l].dupes.join('\n  ')}`);
}

// 3. Forgotten translations: a non-base locale value blank while the base has
//    content (an empty base value is an intentional fragment, allowed).
for (const other of others) {
  const stubs = baseKeys.filter(
    (k) => !isBlank(loaded[base].values.get(k)) && isBlank(loaded[other].values.get(k))
  );
  if (stubs.length)
    errors.push(
      `Blank ${other} values whose ${base} has content (untranslated):\n  ${stubs.join('\n  ')}`
    );
}

const sizes = LOCALES.map((l) => `${l}: ${loaded[l].values.size} keys`).join(' · ');
if (errors.length) {
  console.error('✖ i18n parity check failed:\n');
  console.error(errors.join('\n\n'));
  console.error(`\n${sizes}`);
  process.exit(1);
}

console.log(`✔ i18n parity OK — ${sizes}`);
