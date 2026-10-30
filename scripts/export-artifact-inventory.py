#!/usr/bin/env python3
"""Export registered digests without rereading complete package payloads."""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_contract import digest, json_bytes
from artifact_storage import ArtifactStore, StorageUnavailable


def export_inventory(store: ArtifactStore, *, include_legacy: bool = False) -> dict:
    artifacts, legacy = [], []
    seen = set()
    for manifest in store.list_manifests():
        name, version = manifest["plugin_name"], manifest["plugin_version"]
        seen.add((name, version))
        files = []
        for entry in manifest["files"]:
            # Full validation and digests belong to upload ingestion. Delivery
            # checks presence/size and reuses the immutable registered digest.
            try:
                size = (store.root / name / version / entry["path"]).stat().st_size
            except OSError as exc:
                raise StorageUnavailable("inventory missing committed payload") from exc
            if str(size) != entry["size_bytes"]:
                raise StorageUnavailable("inventory committed payload size differs")
            files.append({key: entry[key] for key in ("path", "size_bytes", "sha256")})
        for member_name in ("package.json", "metadata.txt"):
            raw = store.read_file(name, version, member_name)
            if raw is None:
                raise StorageUnavailable("inventory missing committed archive/descriptor")
            files.append({"path": member_name, "size_bytes": str(len(raw)), "sha256": digest(raw)})
        try:
            archive_size = (store.root / name / version / "archive.tgz").stat().st_size
        except OSError as exc:
            raise StorageUnavailable("inventory missing committed archive") from exc
        files.append({"path": "archive.tgz", "size_bytes": str(archive_size), "sha256": manifest["archive_digest"]})
        raw_manifest = json_bytes(manifest)
        artifacts.append({"plugin_name": name, "plugin_version": version, "artifact_id": manifest["artifact_id"],
                          "content_digest": manifest["content_digest"], "archive_digest": manifest["archive_digest"],
                          "manifest_digest": digest(raw_manifest), "files": sorted(files, key=lambda e: e["path"])})
    for path in sorted(store.root.glob("*/*/plugin.wasm")):
        name, version = path.parent.parent.name, path.parent.name
        if (name, version) in seen:
            continue
        if not include_legacy:
            raise StorageUnavailable("unregistered Wasm must be ingested before formal delivery")
        if path.is_symlink() or not path.resolve().is_relative_to(store.root) or path.with_name("manifest.json").exists() or path.with_name("package.json").exists():
            raise StorageUnavailable("unregistered/aliased managed version")
        files = [{"path": item.name, "size_bytes": str(item.stat().st_size), "sha256": digest(item.read_bytes())}
                 for item in sorted(path.parent.iterdir()) if item.is_file() and not item.is_symlink()]
        legacy.append({"plugin_name": name, "plugin_version": version, "source_verification": "unknown", "files": files})
    if not artifacts and not legacy:
        raise ValueError("artifact inventory must not be empty")
    result = {"inventory_version": "artifact-inventory.v1", "artifacts": artifacts, "legacy": legacy}
    return {**result, "inventory_digest": digest(json_bytes(result))}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plugins-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--include-legacy", action="store_true", help="Explicit raw-byte audit of unregistered historical files")
    args = parser.parse_args()
    value = export_inventory(ArtifactStore(args.plugins_dir), include_legacy=args.include_legacy)
    with args.output.open("xb") as stream:
        stream.write(json_bytes(value))


if __name__ == "__main__":
    main()
