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

varying float vBrightness;
varying float vPulseRate;
varying float vHover;

void main() {
  // gl_PointCoord is (0..1, 0..1) across the disc; recentre to (-1..1).
  vec2 p = gl_PointCoord * 2.0 - 1.0;
  float r2 = dot(p, p);
  if (r2 > 1.0) discard;

  float r = sqrt(r2);

  // Layered falloff: a tight bright core + a broad soft halo.
  float core = smoothstep(0.35, 0.0, r);
  float halo = smoothstep(1.0, 0.15, r) * 0.55;

  // Pulse between ~0.75× and 1.0× so the glow never fully vanishes.
  float pulse = 0.875 + 0.125 * sin(uTime * vPulseRate);

  float intensity = (core + halo) * pulse * vBrightness;

  // Hover adds a small extra halo + core lift — enough to read as
  // feedback without being a visual jump.
  intensity += vHover * (0.25 * halo + 0.15 * core);

  // Radial alpha taper so disc edges blend cleanly against the globe.
  float alpha = intensity * (1.0 - smoothstep(0.8, 1.0, r));

  gl_FragColor = vec4(uGlowColor * intensity, clamp(alpha, 0.0, 1.0));
}
