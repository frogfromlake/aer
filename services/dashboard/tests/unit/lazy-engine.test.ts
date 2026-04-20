import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

// Hard boundary: the shell route must NEVER statically import three.js or the
// engine package. The engine module is dynamic-imported inside AtmosphereCanvas,
// which is the only path that pulls three into a code-split chunk.

const repoRoot = resolve(__dirname, '..', '..');

async function read(rel: string) {
  return readFile(resolve(repoRoot, rel), 'utf8');
}

describe('engine lazy-import boundary', () => {
  it('shell route does not statically import three or @aer/engine-3d (besides /capability)', async () => {
    const src = await read('src/routes/+page.svelte');
    // Static `import ... from 'three'` is forbidden.
    expect(src).not.toMatch(/from\s+['"]three['"]/);
    // The engine package may only be referenced via the side-effect-free /capability subpath.
    const engineImports = src.match(/from\s+['"]@aer\/engine-3d[^'"]*['"]/g) ?? [];
    for (const line of engineImports) {
      expect(line).toMatch(/@aer\/engine-3d\/capability/);
    }
  });

  it('AtmosphereCanvas only references the engine via dynamic import', async () => {
    const src = await read('src/lib/components/atmosphere/AtmosphereCanvas.svelte');
    // The bare `import ... from '@aer/engine-3d'` line is allowed only for `import type`.
    const staticImports = src.match(/^\s*import\s+(?!type\b)[^;]*from\s+['"]@aer\/engine-3d['"]/gm);
    expect(staticImports).toBeNull();
    expect(src).toMatch(/await\s+import\(['"]@aer\/engine-3d['"]\)/);
  });

  it('WebGLFallback does not import the engine or three', async () => {
    const src = await read('src/lib/components/atmosphere/WebGLFallback.svelte');
    expect(src).not.toMatch(/from\s+['"]three['"]/);
    expect(src).not.toMatch(/from\s+['"]@aer\/engine-3d/);
  });
});
