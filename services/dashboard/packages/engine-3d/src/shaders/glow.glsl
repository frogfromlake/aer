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
// Phase 123c (Issue 2) — colour of the selection reticle (AĒR turquoise).
uniform vec3  uSelectColor;

varying float vBrightness;
varying float vPulseRate;
varying float vHover;
varying float vSelected;
varying float vPointSize;

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

  vec3  color = uGlowColor * intensity;

  // Phase 123c (Issue 2) — selection reticle. Selected glyphs get a crisp
  // turquoise target marker: two concentric rings plus four cardinal dots
  // between them. Rendered with its own colour + alpha (independent of the
  // glow taper) so multi-selected probes stand out at a glance. The existing
  // hover/selection glow lift above is kept. Satellites never set vSelected,
  // so they never show a reticle.
  if (vSelected > 0.5) {
    float ringPulseSel = 0.85 + 0.15 * sin(uTime * 1.6);
    // Issue 2 (refinement) — crisp-edged rings + dots. A Gaussian fades and
    // reads as blurry when the glyph is large at close zoom; a thin band with
    // a minimal antialias edge (smoothstep) holds a sharp boundary at every
    // size. `aa` is the only soft part (1–2 px of anti-aliasing).
    // Phase 135 — keep the reticle SHARP at every camera distance. The widths
    // below are in point-coord units, where r spans the point's `vPointSize`
    // device pixels (1 px ≈ 2/vPointSize in r). At max camera distance the
    // point shrinks to ~20 px, so the fine fixed widths (0.009 ≈ 0.09 px) fall
    // far below one pixel and the rasteriser aliases them ("pixelated"). Floor
    // each width to a minimum SCREEN size so lines stay ≥ ~1–2 px when far,
    // while still collapsing to the original fine lines when close (large point).
    float pxToR = 2.0 / max(1.0, vPointSize);  // r-units per device pixel
    float aa      = max(0.010, 1.0  * pxToR);  // ≥ 1 px antialias
    float ringHW  = max(0.009, 0.75 * pxToR);  // ≥ ~1.5 px ring line
    float dotHW   = max(0.020, 1.0  * pxToR);  // ≥ 2 px dot band
    float ringInner = smoothstep(ringHW + aa, ringHW, abs(r - 0.60));
    float ringOuter = smoothstep(ringHW + aa, ringHW, abs(r - 0.86));
    // Four dots at the cardinal angles (0/90/180/270°): a crisp radial band
    // gated by a tight angular window where |cos(2θ)| ≈ 1.
    float ang  = atan(p.y, p.x);
    float radialBand = smoothstep(dotHW + aa, dotHW, abs(r - 0.73));
    float angularGate = smoothstep(0.986, 0.997, abs(cos(ang * 2.0)));
    float dots = radialBand * angularGate;
    float reticle = (ringInner + ringOuter + dots) * ringPulseSel;
    color += uSelectColor * reticle;
    alpha  = max(alpha, reticle);
  }

  gl_FragColor = vec4(color, clamp(alpha, 0.0, 1.0));
}