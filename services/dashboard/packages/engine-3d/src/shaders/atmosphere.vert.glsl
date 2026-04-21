// Calculates the position and view direction for the atmospheric halo, 
// passing them to the fragment shader for volumetric rim lighting calculations.

varying vec3 vNormal;
varying vec3 vViewDirection;

void main() {
    // Calculate the world position of the vertex
    vec4 worldPosition = modelMatrix * vec4(position, 1.0);

    // Transform the normal from object space to world space
    vNormal = normalize((modelMatrix * vec4(normal, 0.0)).xyz);

    // Calculate the direction from the camera to the vertex
    vViewDirection = normalize(cameraPosition - worldPosition.xyz);

    // Calculate the final clip space position
    gl_Position = projectionMatrix * viewMatrix * worldPosition;
}