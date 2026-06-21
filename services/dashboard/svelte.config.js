import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'build',
      assets: 'build',
      fallback: 'index.html',
      precompress: false,
      strict: true
    }),
    prerender: {
      handleHttpError: 'warn'
    },
    // Content-Security-Policy (SEC-021). `mode: 'hash'` lets SvelteKit hash its
    // own inline bootstrap/hydration scripts, so `script-src 'self'` (no
    // 'unsafe-inline') does not break the app — the framework-blessed way to a
    // strict script policy for a static SPA. All scripts are bundled (no
    // third-party CDN). `style-src` keeps 'unsafe-inline' because Svelte/Plot
    // emit inline `style=` attributes that cannot be hashed reliably.
    // `frame-ancestors` is intentionally absent: it is ignored in the <meta>
    // CSP that adapter-static emits, and clickjacking is already covered by the
    // nginx `X-Frame-Options: DENY` header.
    csp: {
      mode: 'hash',
      directives: {
        'default-src': ['self'],
        'script-src': ['self'],
        'style-src': ['self', 'unsafe-inline'],
        'img-src': ['self', 'data:', 'blob:'],
        'font-src': ['self'],
        'connect-src': ['self'],
        'worker-src': ['self', 'blob:'],
        'object-src': ['none'],
        'base-uri': ['self'],
        'form-action': ['self']
      }
    }
  }
};

export default config;
