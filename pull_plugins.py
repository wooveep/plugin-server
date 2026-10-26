"""Import one explicitly pinned OCI Wasm as a complete immutable package."""
from __future__ import annotations

import argparse
import subprocess
import tempfile
from pathlib import Path

from artifact_contract import ValidationPolicy, digest, json_bytes, validate_package
from artifact_import import ImportRejected, read_oci_wasm
from artifact_packaging import prepare_plugin
from artifact_storage import ArtifactStore
from mf_canonical import parse_json


def process_plugin(base_path, plugin_name, plugin_url, version, *, source_lock=None,
                   layout=None, spec=None, defaults=None, documentation=None):
    if source_lock is None or layout is None or spec is None or defaults is None or documentation is None:
        raise ImportRejected("complete_package_and_source_lock_required")
    lock = source_lock
    if (lock["plugin_name"], lock["plugin_version"], lock["original_ref"]) != (plugin_name, version, plugin_url):
        raise ImportRejected("oci_source_lock_identity")
    wasm, source = read_oci_wasm(Path(layout), original_ref=plugin_url,
                                 manifest_digest=lock["manifest_digest"], layer_digest=lock["layer_digest"])
    if digest(wasm) != lock["wasm_digest"] or digest(json_bytes(source)) != lock["source_receipt"]:
        raise ImportRejected("oci_source_lock_digest")
    policy = ValidationPolicy(provenance_receipts=frozenset(((lock["source_receipt"], lock["wasm_digest"], lock["content_digest"]),)))
    raw = prepare_plugin(spec=spec, wasm=wasm, defaults=defaults, documentation=documentation, source=source)
    validated = validate_package(raw, policy)
    if (validated.descriptor["plugin_name"], validated.descriptor["plugin_version"]) != (plugin_name, version):
        raise ImportRejected("oci_package_identity")
    store = ArtifactStore(Path(base_path) / "plugins", policy=policy)
    return store.commit("oci:" + validated.content_digest.removeprefix("sha256:"), raw)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-lock", type=Path, required=True)
    parser.add_argument("--layout", type=Path, help="existing OCI layout; otherwise fetch the pinned digest with ORAS")
    parser.add_argument("--spec", type=Path, required=True)
    parser.add_argument("--defaults", type=Path, required=True)
    parser.add_argument("--documentation", type=Path, required=True)
    parser.add_argument("--destination", type=Path, required=True, help="parent of plugins/")
    args = parser.parse_args()
    lock = parse_json(args.source_lock.read_bytes())
    with tempfile.TemporaryDirectory(prefix="artifact-oci-") as temporary:
        layout = args.layout or Path(temporary)
        if args.layout is None:
            original = lock["original_ref"].removeprefix("oci://")
            pinned = original.split("@")[0] + "@" + lock["manifest_digest"]
            subprocess.run(["oras", "cp", pinned, "--to-oci-layout", str(layout)], check=True,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        result = process_plugin(args.destination, lock["plugin_name"], lock["original_ref"], lock["plugin_version"],
                                source_lock=lock, layout=layout, spec=args.spec.read_bytes(),
                                defaults=args.defaults.read_bytes(), documentation={"docs/README.md": args.documentation.read_bytes()})
    print(json_bytes(result).decode())


if __name__ == "__main__":
    main()
