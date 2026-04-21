// Vertex shader shared by ocean and landmass meshes. We pass the unit normal
// of the sphere and the view direction through to the fragment shader.

varying vec3 vNormal;
varying vec3 vViewDirection;

void main() {
  // Calculate world position to determine the camera view direction
  vec4 worldPosition = modelMatrix * vec4(position, 1.0);
  
  // Normal in world space
  vNormal = normalize((modelMatrix * vec4(normal, 0.0)).xyz);
  
  // Direction from the camera to the vertex for Rim Lighting
  vViewDirection = normalize(cameraPosition - worldPosition.xyz);

  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}