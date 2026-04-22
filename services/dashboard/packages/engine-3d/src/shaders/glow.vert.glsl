// Emission-point glow vertex shader (Phase 99b).
//
// Consumed by a Points draw call. Each vertex is one emission point on
// the globe. The fragment shader receives:
//   - vBrightness:  per-point core brightness (0..1)
//   - vPulseRate:   pulse rate in rad/s, already clamped on the CPU to
//                    honour §1.1 ("stillness with motion beneath")
//   - vHover:       0.0 / 1.0 — the one hot point under the cursor gets
//                    1.0 so the fragment can intensify its glow.
//
// Point size is computed in screen space from distance-to-camera with
// a fixed world-space disc diameter, so a glow's apparent size stays
// stable as the user orbits.

attribute float aPulseRate;
attribute float aCoreBrightness;
attribute float aHover;
attribute float aSelected;

uniform float uPixelRatio;
uniform float uPointWorldSize;

varying float vBrightness;
varying float vPulseRate;
varying float vHover;
varying float vSelected;

void main() {
  vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
  gl_Position = projectionMatrix * mvPosition;

  // gl_PointSize grows linearly as the camera approaches the surface.
  // The `-mvPosition.z` factor keeps the disc's *world* diameter fixed;
  // `uPixelRatio` keeps it HiDPI-stable.
  gl_PointSize = uPointWorldSize * uPixelRatio / max(0.0001, -mvPosition.z);

  vBrightness = aCoreBrightness;
  vPulseRate = aPulseRate;
  vHover = aHover;
  vSelected = aSelected;
}
