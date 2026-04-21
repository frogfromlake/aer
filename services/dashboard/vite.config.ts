import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';

export default defineConfig({
  plugins: [
    sveltekit(),
    visualizer({
      filename: 'build/stats.html',
      gzipSize: true,
      brotliSize: true,
      template: 'treemap'
    })
  ],
  server: {
    port: 5173,
    strictPort: true,
    fs: {
      // Allow the entire dashboard service root: the `packages/engine-3d`
      // workspace sibling imports via `@aer/engine-3d` and the SvelteKit
      // runtime needs to read the root `package.json` for its app module
      // resolution. Anything above the dashboard service (the AĒR monorepo
      // root, other services) stays excluded.
      allow: ['.']
    }
  },
  preview: {
    port: 4173,
    strictPort: true
  }
});
