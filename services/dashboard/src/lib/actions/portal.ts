// Phase 149 (Zen) — move a node to <body> for the duration of its life.
//
// A `position: fixed` overlay only escapes the SideRail (z-index 450) if it is
// NOT trapped inside a lower stacking context further up the workbench tree.
// Re-parenting the node to <body> puts it in the root stacking context, so its
// own z-index wins outright. Svelte keeps updating the node in place after the
// move (only the DOM parent changes), and the node is removed on destroy.
export function portal(node: HTMLElement) {
  document.body.appendChild(node);
  return {
    destroy() {
      node.parentNode?.removeChild(node);
    }
  };
}
