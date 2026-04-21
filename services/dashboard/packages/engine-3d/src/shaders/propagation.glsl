// Propagation-arc fragment shader (Phase 99b — reserved slot).
//
// This file exists to lock in the shader-level plumbing for cross-probe
// propagation arcs. No geometry is drawn while `setPropagationEvents()`
// is empty (the engine short-circuits long before a draw call), so this
// program is authored as a no-op placeholder until multi-probe
// propagation data arrives in a later phase.
//
// Deliberately not exported through index.ts — dead code is opt-in only.

precision highp float;

void main() {
  // Discard everything so even a stray draw call costs zero fragments.
  discard;
}
