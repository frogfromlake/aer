// Terminator + flat-shaded ocean/land fragment shader.
//
// Day side: full uOceanColor (or uLandColor, set per-mesh via uIsLand).
// Night side: same colors deeply darkened — the surface is symbolic, not
// photoreal, so no city-lights overlay.
// Twilight: smoothstep across an angular band centred on the terminator.

precision highp float;

uniform vec3 uSunDirection;     // unit, in the sphere frame (lon=0 → +Z)
uniform vec3 uOceanColor;
uniform vec3 uLandColor;
uniform float uIsLand;          // 0.0 ocean, 1.0 land
uniform float uTwilightHalfDeg; // half-width of the twilight band (degrees)

varying vec3 vNormal;

void main() {
  vec3 base = mix(uOceanColor, uLandColor, uIsLand);

  // Day-strength: dot(normal, sun) ∈ [-1, +1]; smooth across ±twilight band.
  float ndotl = dot(normalize(vNormal), uSunDirection);
  float halfBand = sin(radians(uTwilightHalfDeg));
  float day = smoothstep(-halfBand, halfBand, ndotl);

  // Night side keeps a faint hint of the base hue (~12% luminance) so
  // landmasses remain readable at the dark hemisphere — but never a
  // photoreal-style city-light overlay.
  vec3 night = base * 0.12;
  vec3 colour = mix(night, base, day);

  gl_FragColor = vec4(colour, 1.0);
}
