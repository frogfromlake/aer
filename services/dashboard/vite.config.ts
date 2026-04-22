import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig, loadEnv } from 'vite';
import { visualizer } from 'rollup-plugin-visualizer';

export default defineConfig(({ mode }) => {
  // Load the root .env (two directories up) so the proxy target is
  // overridable per-developer via DEV_API_TARGET. The empty prefix loads
  // all vars, not just VITE_* ones.
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
        // Forward /api requests to Traefik so the dev server matches the
        // production request path exactly. Traefik attaches X-API-Key
        // server-side (see bff-api labels in compose.yaml), so no secret
        // is injected here. Requires `make backend-up` first. The
        // self-signed Traefik cert is accepted for local dev (secure:
        // false).
        '/api': {
          target: rootEnv.DEV_API_TARGET || 'https://localhost',
          changeOrigin: true,
          secure: false
        }
      }
    },
    preview: {
      port: 4173,
      strictPort: true
    }
  };
});
