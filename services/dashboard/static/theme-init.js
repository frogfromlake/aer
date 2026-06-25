// Anti-FOUC theme bootstrap. Render-blocking, same-origin (CSP `script-src
// 'self'` forbids an inline app.html script), so it runs BEFORE first paint and
// applies the persisted colour theme without a flash of the prerendered dark
// default. Must stay in sync with src/lib/state/theme-internals.ts.
(function () {
  try {
    var stored = localStorage.getItem('aer.theme');
    var valid = ['system', 'dark', 'light', 'contrast-dark', 'contrast-light'];
    if (!stored || valid.indexOf(stored) === -1) return; // unset/invalid → keep the dark default
    var root = document.documentElement;
    if (stored === 'system') root.removeAttribute('data-theme');
    else root.setAttribute('data-theme', stored);
  } catch {
    // Storage blocked (private mode) — keep the static dark default.
  }
})();
