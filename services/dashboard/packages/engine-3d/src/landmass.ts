// Loader for the baked Natural Earth landmass + borders assets. Wraps the
// fetch and validates the JSON shape so a corrupt asset surfaces as a typed
// error rather than a NaN-filled BufferGeometry.

import { BufferAttribute, BufferGeometry } from 'three';

export interface LandmassMesh {
  positions: Float32Array;
  indices: Uint32Array;
}

export interface BordersMesh {
  positions: Float32Array;
}

interface LandmassPayload {
  positions: number[];
  indices: number[];
}

interface BordersPayload {
  positions: number[];
}

function isLandmassPayload(v: unknown): v is LandmassPayload {
  if (typeof v !== 'object' || v === null) return false;
  const obj = v as Record<string, unknown>;
  return Array.isArray(obj.positions) && Array.isArray(obj.indices);
}

function isBordersPayload(v: unknown): v is BordersPayload {
  if (typeof v !== 'object' || v === null) return false;
  return Array.isArray((v as Record<string, unknown>).positions);
}

export async function loadLandmass(url: string, signal?: AbortSignal): Promise<LandmassMesh> {
  const init: RequestInit = signal ? { signal } : {};
  const res = await fetch(url, init);
  if (!res.ok) throw new Error(`landmass fetch failed: HTTP ${res.status}`);
  const json: unknown = await res.json();
  if (!isLandmassPayload(json)) throw new Error('landmass asset has unexpected shape');
  return {
    positions: Float32Array.from(json.positions),
    indices: Uint32Array.from(json.indices)
  };
}

export async function loadBorders(url: string, signal?: AbortSignal): Promise<BordersMesh> {
  const init: RequestInit = signal ? { signal } : {};
  const res = await fetch(url, init);
  if (!res.ok) throw new Error(`borders fetch failed: HTTP ${res.status}`);
  const json: unknown = await res.json();
  if (!isBordersPayload(json)) throw new Error('borders asset has unexpected shape');
  return { positions: Float32Array.from(json.positions) };
}

export function landmassToGeometry(mesh: LandmassMesh): BufferGeometry {
  const geom = new BufferGeometry();
  geom.setAttribute('position', new BufferAttribute(mesh.positions, 3));
  geom.setIndex(new BufferAttribute(mesh.indices, 1));
  geom.computeVertexNormals();
  return geom;
}

export function bordersToGeometry(mesh: BordersMesh): BufferGeometry {
  const geom = new BufferGeometry();
  geom.setAttribute('position', new BufferAttribute(mesh.positions, 3));
  return geom;
}
