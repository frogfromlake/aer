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
    const src = await read('src/routes/(app)/+page.svelte');
    // Static `import ... from 'three'` is forbidden.
    expect(src).not.toMatch(/from\s+['"]three['"]/);
    // The engine package may only be referenced via the side-effect-free /capability subpath.
    // `import type` statements are erased at compile time and are exempt — they bring no runtime.
    const engineImports =
      src.match(/^\s*import\s+(?!type\b)[^;]*from\s+['"]@aer\/engine-3d[^'"]*['"]/gm) ?? [];
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

  it('shell route does not statically import uplot (L3 chart chunk must stay lazy)', async () => {
    const src = await read('src/routes/(app)/+page.svelte');
    expect(src).not.toMatch(/from\s+['"]uplot['"]/);
  });

  it('L3 panel references uplot only through TimeSeriesChart (no direct static import)', async () => {
    const src = await read('src/lib/components/L3AnalysisPanel.svelte');
    expect(src).not.toMatch(/from\s+['"]uplot['"]/);
  });

  it('TimeSeriesChart only references uplot via dynamic import', async () => {
    const src = await read('src/lib/components/TimeSeriesChart.svelte');
    // The runtime module itself must be awaited, not statically imported.
    // The CSS sidecar (`uplot/dist/uPlot.min.css`) is exempt — a static
    // CSS import is what Vite needs to emit the style sidecar and does
    // not pull the JS runtime into the importer's chunk.
    const staticJsImports = src.match(/^\s*import\s+(?!type\b)[^;]*from\s+['"]uplot['"]\s*;/gm);
    expect(staticJsImports).toBeNull();
    expect(src).toMatch(/await\s+import\(['"]uplot['"]\)/);
  });

  it('WebGLFallback does not statically import the engine runtime or three', async () => {
    const src = await read('src/lib/components/atmosphere/WebGLFallback.svelte');
    expect(src).not.toMatch(/from\s+['"]three['"]/);
    // `import type` is erased at compile time and brings no runtime — exempt.
    const engineImports =
      src.match(/^\s*import\s+(?!type\b)[^;]*from\s+['"]@aer\/engine-3d[^'"]*['"]/gm) ?? [];
    expect(engineImports).toEqual([]);
  });
});
