// Observable Plot a11y normalisation (Phase 128).
//
// Plot tags its mark / axis / rule `<g>` groups with `aria-label="rect"`,
// `aria-label="rule"`, `aria-label="tip"`, etc. An `aria-label` on a `<g>`
// that has no `role` is *prohibited* (WCAG 4.1.2 → axe rule `aria-prohibited-attr`,
// serious). Those labels are mark-type noise, not a text alternative: every
// AĒR cell already gives its `<figure>` a meaningful `aria-label` and renders a
// `<dl class="summary">` (or equivalent) carrying the data textually. Stripping
// the prohibited group labels leaves the SVG clean for assistive tech without
// losing any information.
//
// `host.appendChild(sanitizePlotA11y(node))` is the intended call site — the
// function mutates in place AND returns the node so it can wrap the existing
// appendChild expression with a single-line change.

/** True when this element is a `<g>` carrying an `aria-label` but no `role`
 *  (the prohibited combination Observable Plot emits). Exported for unit tests. */
export function hasProhibitedAria(el: Element): boolean {
  return (
    el.tagName.toLowerCase() === 'g' && el.hasAttribute('aria-label') && !el.hasAttribute('role')
  );
}

/**
 * Removes the prohibited `aria-label` from every roleless `<g>` under (and
 * including) `root`. Returns `root` for call-site chaining. Null-safe.
 */
export function sanitizePlotA11y<T extends Element | null | undefined>(root: T): T {
  if (!root) return root;
  if (hasProhibitedAria(root)) root.removeAttribute('aria-label');
  for (const g of root.querySelectorAll('g[aria-label]:not([role])')) {
    g.removeAttribute('aria-label');
  }
  return root;
}
