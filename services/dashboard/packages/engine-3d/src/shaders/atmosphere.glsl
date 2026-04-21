// atmosphere.glsl
// Renders the atmospheric glow using rim lighting (Fresnel) and a day/night transition.
// Adapted to adhere to the symbolic, non-prescriptive visual guidelines.

uniform vec3 uSunDirection;
uniform vec3 uHaloColor;
uniform float uIntensity;
uniform float uCameraDistance;

varying vec3 vNormal;
varying vec3 vViewDirection;

void main() {
    // Normalize interpolated vectors
    vec3 normal = normalize(vNormal);
    vec3 viewDir = normalize(vViewDirection);
    vec3 lightDir = normalize(uSunDirection);

    // === FrontSide Volumetric Fresnel ===
    // Calculate how perpendicular the view is to the surface
    float ndotv = max(dot(normal, viewDir), 0.0);
    
    // Base rim effect (strongest at the edge, invisible in the center)
    float rimPower = pow(1.0 - ndotv, 4.0);
    
    // Edge fade. Forces opacity to 0 exactly at the geometric edge (ndotv = 0.0).
    // This softens the outer boundary and makes the atmosphere look like gas, not a solid shell.
    float edgeFade = smoothstep(0.0, 0.2, ndotv);
    
    float fresnel = rimPower * edgeFade;

    // === Sunlight incidence ===
    // Determines how much sunlight hits the surface
    float lightDot = dot(normal, lightDir);

    // === Daylight fade ===
    float dayFade = smoothstep(-0.1, 0.2, lightDot);

    // === Twilight effect ===
    // Occurs right at the terminator line
    float twilight = smoothstep(-0.25, 0.1, lightDot) * (1.0 - dayFade);
    twilight *= 0.8;

    // Combine fades.
    float fade = dayFade + twilight * 0.7;

    // Boost the alpha (* 3.0) to compensate for the new edge fade math
    float alpha = fresnel * fade * uIntensity * 2.0;

    // === Distance-based fade ===
    // Fades the atmosphere slightly as the camera zooms out.
    float distanceFade = smoothstep(1.0, 12.0, uCameraDistance);
    alpha *= (1.0 - distanceFade * 0.6);

    // === Color blending ===
    vec3 dayColor = uHaloColor * 0.7;
    vec3 twilightHue = mix(vec3(1.0, 0.5, 0.3), vec3(0.45, 0.25, 0.65), 0.55); // Orange mixed with purple
    
    vec3 glowColor = mix(dayColor, twilightHue, twilight);

    // Final output
    gl_FragColor = vec4(glowColor * alpha, alpha);
}