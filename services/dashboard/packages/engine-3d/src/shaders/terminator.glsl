// Terminator + flat-shaded ocean/land fragment shader.
//
// Land/ocean classification is sampled from a baked Signed Distance Field
// (equirectangular, red channel, 0.5 at the coastline). The sphere UV is
// reconstructed from the fragment normal; bilinear filtering across the
// SDF plus `fwidth`-based antialiasing keep the coast pixel-sharp at every
// zoom level without any geometry, seams, or political-border overlays.
// See scripts/bake-landmass.mjs for the bake pipeline.

#ifdef GL_OES_standard_derivatives
#extension GL_OES_standard_derivatives : enable
#endif

precision highp float;

uniform vec3 uSunDirection;     // unit, in the sphere frame (lon=0 → +Z)
uniform vec3 uOceanColor;
uniform vec3 uLandColor;
uniform sampler2D uLandSdf;     // equirectangular SDF; R channel, 0.5 = coast
uniform float uTwilightHalfDeg; // half-width of the twilight band (degrees)

uniform vec3 uRimColor;
uniform float uRimIntensity;
uniform float uNightOceanFactor;
uniform float uNightLandFactor;
uniform float uLandIllumination;

varying vec3 vNormal;
varying vec3 vViewDirection;    // Direction from the camera to the vertex for rim lighting

const float PI = 3.14159265358979323846;

// Mapping must match `latLonToXyz` in scripts/bake-landmass.mjs:
//   lon = 0 → +Z,  lon = +90° → +X,  lat = +90° → +Y.
vec2 sphereUv(vec3 n) {
  float lon = atan(n.x, n.z);                 // [-π, π]
  float lat = asin(clamp(n.y, -1.0, 1.0));    // [-π/2, π/2]
  return vec2(lon / (2.0 * PI) + 0.5, lat / PI + 0.5);
}

void main() {
  vec3 N = normalize(vNormal);
  vec3 viewDir = normalize(vViewDirection);
  vec2 uv = sphereUv(N);
  float sdf = texture2D(uLandSdf, uv).r;

  // Screen-space antialiasing: smooth one fragment of SDF change around the
  // 0.5 isoline. `fwidth` is undefined at the ±180° longitude seam because
  // atan() wraps; clamp to a floor so the seam never bleeds past a pixel.
  float aa = clamp(fwidth(sdf), 1e-4, 0.25);
  float land = smoothstep(0.5 - aa, 0.5 + aa, sdf);

  vec3 base = mix(uOceanColor, uLandColor, land);

  // Day-strength: dot(normal, sun) ∈ [-1, +1]; smooth across ±twilight band.
  float ndotl = dot(N, uSunDirection);
  float halfBand = sin(radians(uTwilightHalfDeg));
  float day = smoothstep(-halfBand, halfBand, ndotl);

  // === Night visibility ===
  // Instead of a flat darkness, ocean drops to darker but land stays at brighter.
  // This guarantees continental readability even on dark monitors.
  float nightFactor = mix(uNightOceanFactor, uNightLandFactor, land);
  vec3 night = base * nightFactor;
  vec3 colour = mix(night, base, day);

  // === Artificial Land Illumination (Emission) ===
  // Adds a configurable inner glow strictly to the landmasses. By using additive
  // blending instead of multiplication, we ensure the land has a global
  // brightness baseline across both the day and night hemispheres.
  colour += uLandColor * uLandIllumination * land;

  // === Subtle Landmass Rim Lighting ===
  // Adds a faint, volumetric glow to the edges of the globe.
  float rimDot = 1.0 - max(dot(viewDir, N), 0.0);
  
  // Instead of dropping instantly at the terminator, we use a softer 
  // fade to prevent the "ring" artifact on the limb.
  float rimSunFade = smoothstep(-0.2, 0.3, ndotl);

  // Only apply to the outer edges (smoothstep 0.5 to 1.0)
  float rimIntensity = smoothstep(0.5, 1.0, rimDot) * uRimIntensity * rimSunFade; 
  
  colour += uRimColor * rimIntensity;

  gl_FragColor = vec4(colour, 1.0);
}