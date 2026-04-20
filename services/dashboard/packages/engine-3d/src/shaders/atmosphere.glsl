// Atmospheric halo — analytic Rayleigh approximation rendered on a back-side
// sphere slightly larger than the globe. This is the Bruneton-Neyret family
// of approximations, simplified to a single-scattering term: the halo
// brightens with the cosine of the view-angle to the surface normal and
// peaks where the sun grazes the limb.
//
// It is "ornamental and informational" per Brief §1.1: the colour is real
// physics (Rayleigh scattering peaks in blue) and the geometry tracks the
// sun, so the halo always sits on the day-side limb the way Earth's real
// atmosphere does in orbital photographs.

precision highp float;

uniform vec3 uSunDirection;
uniform vec3 uHaloColor;
uniform float uIntensity;

varying vec3 vWorldNormal;
varying vec3 vViewDir;

void main() {
  vec3 n = normalize(vWorldNormal);
  vec3 v = normalize(vViewDir);

  // Limb factor: 1 at the silhouette, 0 facing the camera. fresnel-like.
  float limb = 1.0 - max(dot(n, v), 0.0);
  limb = pow(limb, 2.5);

  // Sun-side factor: stronger where the sunlit hemisphere meets the limb.
  float sunFactor = max(dot(n, uSunDirection), 0.0);

  // Single-scattering Rayleigh phase function (1 + cos² θ) collapsed against
  // the view direction. Soft, isotropic-ish.
  float phase = 0.75 * (1.0 + sunFactor * sunFactor);

  float a = uIntensity * limb * (0.35 + 0.65 * sunFactor) * phase;
  gl_FragColor = vec4(uHaloColor, clamp(a, 0.0, 1.0));
}
