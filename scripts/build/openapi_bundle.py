#!/usr/bin/env python3
"""
OpenAPI bundler for AĒR's two-style ref convention.

Produces a single-file bundle from a modular OpenAPI tree where:
  - Path-level refs may use `#/components/schemas/X` (kin-openapi path-item
    flattening semantics — the way oapi-codegen expects named types).
  - All other refs inside external files use external-file refs
    (`../schemas/X.yaml`).

Redocly/swagger-cli cannot bundle the first form because JSON Reference's `#`
means "current document" — so they choke on `#/components/...` inside
external path files. We emulate kin-openapi's path-item flattening: for each
external file ref we resolve relative to THAT file's directory, so
`../schemas/X.yaml` inside a path file resolves correctly; and we preserve
`#/components/...` refs since after flattening they live in the root.
"""
from __future__ import annotations

import sys
from pathlib import Path
from typing import Any

import yaml


def _load(path: Path) -> Any:
    with path.open("r", encoding="utf-8") as fh:
        return yaml.safe_load(fh)


def _resolve_node(node: Any, current_file: Path, root_dir: Path, schema_files: dict[Path, str]) -> Any:
    """Recursively resolve refs. External-file refs are inlined (or rewritten to
    `#/components/schemas/<name>` if the file matches a registered schema).
    Fragment refs (`#/...`) are kept as-is — valid against the root once this
    subtree is attached there."""
    if isinstance(node, dict):
        if list(node.keys()) == ["$ref"] and isinstance(node["$ref"], str):
            ref = node["$ref"]
            if ref.startswith("#/"):
                return {"$ref": ref}
            target = (current_file.parent / ref).resolve()
            # If this external file is already registered as a named component,
            # rewrite to the canonical pointer.
            if target in schema_files:
                return {"$ref": schema_files[target]}
            # Otherwise inline, resolving refs relative to the target file.
            loaded = _load(target)
            return _resolve_node(loaded, target, root_dir, schema_files)
        return {k: _resolve_node(v, current_file, root_dir, schema_files) for k, v in node.items()}
    if isinstance(node, list):
        return [_resolve_node(item, current_file, root_dir, schema_files) for item in node]
    return node


def bundle(spec_path: Path, out_path: Path) -> None:
    root = _load(spec_path)
    root_dir = spec_path.parent

    # Step 1: register component schemas. Map each external file path -> canonical pointer.
    schema_files: dict[Path, str] = {}
    schemas = root.setdefault("components", {}).setdefault("schemas", {})
    for name, entry in list(schemas.items()):
        if isinstance(entry, dict) and list(entry.keys()) == ["$ref"]:
            ref = entry["$ref"]
            if not ref.startswith("#/"):
                target = (spec_path.parent / ref).resolve()
                schema_files[target] = f"#/components/schemas/{name}"

    # Step 2: resolve each registered schema file, inlining its content.
    for name, entry in list(schemas.items()):
        if isinstance(entry, dict) and list(entry.keys()) == ["$ref"]:
            ref = entry["$ref"]
            if not ref.startswith("#/"):
                target = (spec_path.parent / ref).resolve()
                loaded = _load(target)
                schemas[name] = _resolve_node(loaded, target, root_dir, schema_files)

    # Step 3: inline path-item refs (with their own file as the resolution base).
    paths = root.get("paths", {})
    for path_key, entry in list(paths.items()):
        if isinstance(entry, dict) and list(entry.keys()) == ["$ref"]:
            ref = entry["$ref"]
            if ref.startswith("#/"):
                continue
            target = (spec_path.parent / ref).resolve()
            loaded = _load(target)
            paths[path_key] = _resolve_node(loaded, target, root_dir, schema_files)

    # Step 4: resolve any remaining refs at root (e.g., inside responses).
    root = _resolve_node(root, spec_path, root_dir, schema_files)

    with out_path.open("w", encoding="utf-8") as fh:
        yaml.safe_dump(root, fh, sort_keys=False, allow_unicode=True)


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: openapi_bundle.py <input.yaml> <output.yaml>", file=sys.stderr)
        return 2
    bundle(Path(sys.argv[1]).resolve(), Path(sys.argv[2]).resolve())
    return 0


if __name__ == "__main__":
    sys.exit(main())
