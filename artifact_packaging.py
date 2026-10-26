"""Deterministic complete artifact.v1 packaging; no registry or storage writes."""
from __future__ import annotations

import argparse
import gzip
import io
import json
import tarfile
from pathlib import Path
from typing import Any

from artifact_contract import (ArtifactError, ValidationPolicy, digest, json_bytes, load_policy,
                               safe_path, validate_package)
from mf_canonical import parse_json
from plugin_schema_conversion import convert_spec


def build_archive(descriptor: dict[str, Any], payload: dict[str, bytes]) -> bytes:
    """Stable order, metadata, modes, gzip timestamp and header, from original bytes."""
    members = {"package.json": json_bytes(descriptor), **payload}
    if "package.json" in payload or not all(safe_path(name) for name in members):
        raise ArtifactError("ART_ARCHIVE_PATH", "unsafe/reserved payload path")
    content = io.BytesIO()
    with tarfile.open(fileobj=content, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for name, raw in sorted(members.items()):
            member = tarfile.TarInfo(name)
            member.size, member.mode, member.mtime = len(raw), 0o644, 0
            archive.addfile(member, io.BytesIO(raw))
    compressed = io.BytesIO()
    with gzip.GzipFile(fileobj=compressed, mode="wb", filename="", mtime=0, compresslevel=9) as stream:
        stream.write(content.getvalue())
    return compressed.getvalue()


def prepare_plugin(*, spec: bytes, wasm: bytes, defaults: bytes, documentation: dict[str, bytes],
                   source: dict[str, Any], environment_refs: list[dict[str, Any]] | None = None,
                   policy: ValidationPolicy | None = None) -> bytes:
    """Assemble candidate bytes; validation/commit still requires trusted receipts."""
    converted = convert_spec(spec)
    metadata = converted["metadata"]
    payload = {"plugin.wasm": wasm, "defaults.json": defaults, **converted["schemas"]}
    roles = {name: "config_schema" for name in converted["schemas"]}
    roles.update({"plugin.wasm": "wasm", "defaults.json": "defaults"})
    for name, raw in documentation.items():
        if not name.startswith("docs/") or not safe_path(name) or name in payload:
            raise ArtifactError("ART_DOCUMENTATION", "documentation must use unique docs/ paths")
        payload[name], roles[name] = raw, "documentation"
    if not documentation:
        raise ArtifactError("ART_DOCUMENTATION", "at least one documentation file is required")
    descriptor = {
        "canonicalization_version": "artifact-json.v1", "schema_version": "artifact.v1",
        "plugin_name": metadata["info"]["name"], "plugin_version": metadata["info"]["version"],
        "files": [{"path": name, "role": roles[name], "size_bytes": str(len(raw)), "sha256": digest(raw)} for name, raw in sorted(payload.items())],
        "wasm_path": "plugin.wasm", "config_schema_path": "config.schema.json", "defaults_path": "defaults.json",
        "documentation_paths": sorted(documentation), "environment_refs": environment_refs or [], "source": source,
    }
    result = build_archive(descriptor, payload)
    return result


def package_plugin(**kwargs) -> bytes:
    """Prepare and validate a package against an independently supplied policy."""
    result = prepare_plugin(**kwargs)
    validate_package(result, kwargs.get("policy"))
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--wasm", type=Path, required=True)
    parser.add_argument("--defaults", type=Path, required=True)
    parser.add_argument("--documentation", type=Path, required=True)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--policy", type=Path, help="independent trusted provenance/environment receipts")
    args = parser.parse_args()
    result = package_plugin(spec=args.spec.read_bytes(), wasm=args.wasm.read_bytes(), defaults=args.defaults.read_bytes(),
                            documentation={"docs/README.md": args.documentation.read_bytes()}, source=parse_json(args.source.read_bytes()), policy=load_policy(args.policy))
    # Refuse to replace a prepared delivery; publishing is an independent action.
    with args.output.open("xb") as stream:
        stream.write(result)
    validated = validate_package(result, load_policy(args.policy))
    print(json.dumps({"content_digest": validated.content_digest, "archive_digest": validated.archive_digest}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
