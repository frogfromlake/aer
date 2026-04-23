// Emission-point glow fragment shader (Phase 99b).
//
// Renders a radial-gradient disc per Points vertex. Two layered falloffs
// produce a soft halo around a bright core. A slow sinusoidal pulse
// modulates the brightness; the CPU clamps vPulseRate to <= 2π/4 rad/s
// so the fastest cycle is ~4 s (§1.1 "stillness with motion beneath").
//
// The hovered emission point receives a small additive intensification
// (`vHover == 1.0`) so feedback is legible without inventing a second
// material. No post-processing pass is used — the frame budget stays
// entirely in the fragment stage.
//
// No reach is rendered here. See ROADMAP Phase 99b scope decision.

precision highp float;

uniform float uTime;
uniform vec3  uGlowColor;
uniform float uBrightnessScale;
uniform float uHaloBrightness;
uniform float uOuterRingBrightness;

varying float vBrightness;
varying float vPulseRate;
varying float vHover;
varying float vSelected;

void main() {
  // gl_PointCoord is (0..1, 0..1) across the disc; recentre to (-1..1).
  vec2 p = gl_PointCoord * 2.0 - 1.0;
  float r2 = dot(p, p);
  if (r2 > 1.0) discard;

  float r = sqrt(r2);

  // Exponential falloff for a "shiny" bloom effect.
  // pow() creates a piercing bright core and a smooth, spreading halo.
  float core = pow(max(0.0, 1.0 - r), 8.0) * 1.4;
  float halo = pow(max(0.0, 1.0 - r), 2.5) * uHaloBrightness;

  // Minimum ~8 s baseline breath; active probes run faster via vPulseRate.
  float effectivePulseRate = max(vPulseRate, 0.8);

  // ±12 % amplitude — perceptible but not alarming (§1.1 "stillness with motion beneath").
  float pulse     = 0.88 + 0.12 * sin(uTime * effectivePulseRate);
  // Outer ring lags core by π/2 for a ripple-outward feel.
  float ringPulse = 0.88 + 0.12 * sin(uTime * effectivePulseRate - 1.5708);

  // Layered Gaussian ring centred at r = 0.35.
  float distToRing = r - 0.35;
  float distSq     = distToRing * distToRing;
  float ringSharp  = exp(-1000.0 * distSq);       // razor-thin bright line
  float ringGlow   = exp(-100.0  * distSq) * 0.5; // wider soft glow around it
  
  float outerRing = (ringSharp + ringGlow) * uOuterRingBrightness;
  // -----------------------------

  float coreI     = core      * uBrightnessScale    * pulse     * vBrightness;
  float haloI     = halo                            * pulse     * vBrightness;
  // Apply the phase-shifted ringPulse here instead of the base pulse
  float outerI    = outerRing                       * ringPulse * vBrightness;
  
  float intensity = coreI + haloI + outerI;

  // Active state: triggered by EITHER hover or UI selection
  float activeState = max(vHover, vSelected);

  // Highlight significantly lifts both core and halo
  intensity += activeState * (0.4 * haloI + 0.4 * coreI);

  // Radial alpha taper — extend to 0.3 so the outer halo is not clipped
  // before it reaches the disc edge (the old 0.8 cutoff zeroed alpha at
  // r=0.8, killing the region where the halo spread lives).
  float alpha = intensity * smoothstep(1.0, 0.3, r);

  gl_FragColor = vec4(uGlowColor * intensity, clamp(alpha, 0.0, 1.0));
}