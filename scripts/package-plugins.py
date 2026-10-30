#!/usr/bin/env python3
"""Prepare reviewable candidates, then ingest approved uploads exactly once."""

import argparse
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from artifact_contract import digest, json_bytes, load_policy, validate_document
from artifact_packaging import prepare_plugin
from artifact_storage import ArtifactStore, StorageConflict
from mf_canonical import parse_json
from plugin_catalog import select_releases, source_inputs


def prepare(root: Path, plugins: list[str] | None = None) -> None:
    repository = subprocess.check_output(["git", "remote", "get-url", "origin"], cwd=root, text=True).strip()
    reviews = []
    selected = select_releases(root)
    if plugins:
        if set(plugins) - {source.name for source in selected}:
            raise ValueError("preparation requires known formal plugin names")
        selected = [source for source in selected if source.name in plugins]
    for source in selected:
        output = root / "build/plugins" / source.name / source.version
        receipt = parse_json((output / "build-receipt.json").read_bytes())
        inputs = source_inputs(root, source)
        if (receipt["plugin_name"], receipt["plugin_version"], receipt["implementation"]) != (
                source.name, source.version, source.implementation) or receipt["source_inputs"] != inputs:
            raise ValueError("build receipt does not describe current source inputs: " + source.name)
        commit = receipt["source_commit"]
        tracked = subprocess.check_output(["git", "ls-tree", "-r", "--name-only", commit, "--", *inputs], cwd=root, text=True).splitlines()
        if set(tracked) != set(inputs):
            raise ValueError("build inputs must all exist in the recorded Git commit: " + source.name)
        subprocess.run(["git", "diff", "--exit-code", "--quiet", commit, "--", *inputs], cwd=root, check=True)
        wasm = (root / receipt["artifact"]).read_bytes()
        if digest(wasm) != receipt["sha256"] or len(wasm) != receipt["size_bytes"]:
            raise ValueError("compiler output differs from build receipt: " + source.name)
        metadata = source.metadata_directory
        documentation = {"docs/" + path.name: path.read_bytes() for path in sorted(metadata.glob("*.md"))}
        if not documentation:
            documentation = {"docs/" + path.name: path.read_bytes() for path in sorted(source.directory.glob("*.md"))}
        origin = {"kind": "local_build", "verification": "verified", "repo": repository,
                  "commit": commit, "dirty_digest": digest(b""),
                  "build_inputs_digest": digest(json_bytes(inputs))}
        environment = parse_json((metadata / "environment.json").read_bytes())
        archive = prepare_plugin(spec=(metadata / "spec.yaml").read_bytes(), wasm=wasm,
                                 defaults=(metadata / "defaults.json").read_bytes(), documentation=documentation,
                                 source=origin, environment_refs=environment)
        # Inspect our generated descriptor only. Full archive, payload, schema and
        # Wasm validation occurs at ingestion, using an independent trusted policy.
        with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as package:
            descriptor = package.extractfile("package.json").read()
        declared = validate_document(descriptor)
        destination = root / "build/packages" / source.name / source.version
        destination.mkdir(parents=True, exist_ok=True)
        (destination / "package.tgz").write_bytes(archive)
        review = {"plugin_name": source.name, "plugin_version": source.version,
                  "source_commit": commit, "source_inputs_digest": origin["build_inputs_digest"],
                  "build_receipt_digest": digest((output / "build-receipt.json").read_bytes()),
                  "toolchain": receipt["toolchain"], "source_digest": digest(json_bytes(origin)),
                  "wasm_digest": receipt["sha256"], "content_digest": declared["content_digest"],
                  "archive_digest": digest(archive), "environment_refs": environment,
                  "package": str((destination / "package.tgz").relative_to(root))}
        (destination / "review.json").write_bytes(json_bytes(review))
        reviews.append(review)
        print(json.dumps({"prepared": source.name, "version": source.version}), flush=True)
    (root / "build/packages/review.json").write_bytes(json_bytes({"review_version": 1, "candidates": reviews}))


def ingest(root: Path, policy_file: Path, base_url: str) -> None:
    policy = load_policy(policy_file)
    store = ArtifactStore(root / "dist/plugins", policy=policy, public_base_url=base_url)
    existing = {(item["plugin_name"], item["plugin_version"]): item for item in store.list_manifests()}
    for source in select_releases(root):
        candidate = root / "build/packages" / source.name / source.version
        review = parse_json((candidate / "review.json").read_bytes())
        registered = existing.get((source.name, source.version))
        if registered is not None:
            if registered["content_digest"] != review["content_digest"]:
                raise StorageConflict("version_conflict")
            print(json.dumps({"registered": source.name, "reused": True, "artifact_id": registered["artifact_id"]}), flush=True)
            continue
        raw = (candidate / "package.tgz").read_bytes()
        if digest(raw) != review["archive_digest"]:
            raise ValueError("candidate changed since preparation: " + source.name)
        receipt = store.commit("release-" + review["content_digest"].split(":")[1], raw,
                               created_by="plugin-server-source-release", expected_identity=(source.name, source.version))
        print(json.dumps({"registered": source.name, "artifact_id": receipt["artifact_id"]}), flush=True)
    shutil.copyfile(policy_file, root / "dist/policy.json")
    subprocess.run([sys.executable, str(root / "scripts/verify-release-store.py"),
                    "--plugins-dir", str(store.root), "--catalog", str(root / "plugins/release/catalog.json"),
                    "--output", str(root / "dist/plugin-catalog.json")], cwd=root, check=True)
    inventory = root / "dist/artifact-inventory.json"
    inventory.unlink(missing_ok=True)
    subprocess.run([sys.executable, str(root / "scripts/export-artifact-inventory.py"),
                    "--plugins-dir", str(store.root), "--output", str(inventory)], cwd=root, check=True)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("prepare", "ingest"))
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--policy", type=Path)
    parser.add_argument("--plugin", action="append", help="Prepare only named plugins; image verification still requires the full catalog")
    parser.add_argument("--base-url", default="http://higress-plugin-server.higress-system.svc:80")
    args = parser.parse_args()
    if args.action == "prepare":
        prepare(args.root.resolve(), args.plugin)
    else:
        if args.policy is None:
            parser.error("ingestion requires an independently reviewed --policy")
        if args.plugin:
            parser.error("use the upload entry for a single plugin; local release ingestion requires the full catalog")
        ingest(args.root.resolve(), args.policy.resolve(), args.base_url)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
