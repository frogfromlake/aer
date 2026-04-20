// Vertex shader shared by ocean and landmass meshes. We pass the unit normal
// of the sphere through to the fragment shader for the terminator dot-product.

varying vec3 vNormal;

void main() {
  vNormal = normalize(normalMatrix * normal);
  gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
}
