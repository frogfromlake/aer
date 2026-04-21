import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';

export default defineConfig(({ mode }) => {
  // Load the root .env (two directories up) so the dev proxy can inject
  // BFF_API_KEY without requiring a separate dashboard/.env file.
  // The empty prefix loads all vars, not just VITE_* ones.
  const rootEnv = loadEnv(mode, '../..', '');

  return {
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
      },
      proxy: {
        // Forward /api requests to Traefik (which routes to the BFF) so the
        // Vite dev server doesn't 404 on API calls. Requires the full Docker
        // stack to be running (`make up`). The self-signed Traefik cert is
        // accepted for local dev only (secure: false).
        '/api': {
          target: rootEnv.DEV_API_TARGET || 'https://localhost',
          changeOrigin: true,
          secure: false,
          headers: {
            ...(rootEnv.BFF_API_KEY ? { 'X-API-Key': rootEnv.BFF_API_KEY } : {})
          }
        }
      }
    },
    preview: {
      port: 4173,
      strictPort: true
    }
  };
});
