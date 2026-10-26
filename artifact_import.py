"""One complete-package import protocol for upload, sync, OCI and image seeds."""
from __future__ import annotations

import argparse
import gzip
import io
import json
import re
import tarfile
import tempfile
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

from artifact_contract import (ValidationPolicy, digest, json_bytes, load_policy,
                               safe_path, validate_document, validate_package, validate_wasm)
from artifact_storage import ArtifactStore
from mf_canonical import parse_json


class UploadUnknown(Exception):
    def __init__(self, upload_id):
        self.upload_id = upload_id
        super().__init__("upload outcome unknown; query/replay the same upload_id: " + upload_id)


class ImportRejected(Exception):
    def __init__(self, code):
        self.code = code
        super().__init__(code)


class ArtifactClient:
    def __init__(self, server, token, *, timeout=30, opener=urlopen, policy=None):
        if not re.fullmatch(r"https?://[^\s]+", server):
            raise ValueError("an explicit HTTP(S) artifact server is required")
        self.server, self.token = server.rstrip("/"), token
        self.timeout, self.opener = timeout, opener
        self.policy = policy or ValidationPolicy()

    def _request(self, method, upload_id, raw=None):
        request = Request(self.server + "/uploads/" + quote(upload_id, safe=""), method=method, data=raw,
                          headers={"Authorization": "Bearer " + self.token, "Content-Type": "application/gzip"})
        with self.opener(request, timeout=self.timeout) as response:
            return parse_json(response.read(1_048_577))

    def query(self, upload_id):
        try:
            return self._request("GET", upload_id)
        except HTTPError as error:
            if error.code == 404:
                error.close()
                return None
            raise

    @staticmethod
    def _committed(receipt, upload_id, expected_digest):
        if not isinstance(receipt, dict) or receipt.get("upload_id") != upload_id:
            raise ImportRejected("invalid_upload_receipt")
        if receipt.get("state") == "rejected":
            raise ImportRejected(receipt.get("error", "rejected"))
        if receipt.get("state") != "committed":
            return None
        manifest = receipt.get("manifest")
        if not isinstance(manifest, dict):
            raise ImportRejected("invalid_upload_receipt")
        checked = validate_document(json_bytes(manifest), "manifest")
        if checked["content_digest"] != expected_digest or receipt.get("content_digest") != expected_digest or receipt.get("artifact_id") != manifest["artifact_id"]:
            raise ImportRejected("upload_receipt_identity_mismatch")
        return receipt

    def commit(self, upload_id, raw):
        expected = validate_package(raw, self.policy).content_digest
        for attempt in range(2):
            try:
                receipt = self._request("PUT", upload_id, raw)
                committed = self._committed(receipt, upload_id, expected)
                if committed is not None:
                    return committed
            except HTTPError as error:
                if error.code < 500:
                    try:
                        code = parse_json(error.read(1_048_577)).get("error", "http_rejection")
                    finally:
                        error.close()
                    raise ImportRejected(code) from None
                error.close()
            except (URLError, OSError):
                pass
            # A lost ACK is neither rejection nor a reason to allocate a new key.
            try:
                receipt = self.query(upload_id)
                if receipt is not None:
                    committed = self._committed(receipt, upload_id, expected)
                    if committed is not None:
                        return committed
                    if receipt.get("content_digest") not in (None, expected):
                        raise ImportRejected("idempotency_conflict")
            except (URLError, OSError):
                raise UploadUnknown(upload_id) from None
            if attempt:
                break
        raise UploadUnknown(upload_id)


def sync_packages(source: Path, store: ArtifactStore) -> list[dict]:
    paths = sorted(Path(source).glob("*.tgz"))
    if not paths:
        raise ImportRejected("no_complete_packages")
    # Preflight all packages before changing the destination. Every actual commit
    # still performs its own validation and immutable-version check.
    items = [(path, path.read_bytes()) for path in paths]
    for _, raw in items:
        validate_package(raw, store.policy)
    return [store.commit("sync:" + digest(raw).removeprefix("sha256:"), raw) for _, raw in items]


def _oci_blob(layout: Path, expected_digest: str) -> bytes:
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", expected_digest):
        raise ImportRejected("invalid_oci_digest")
    path = Path(layout) / "blobs/sha256" / expected_digest.removeprefix("sha256:")
    if path.is_symlink() or path.stat().st_size > 64 * 1024 * 1024:
        raise ImportRejected("invalid_oci_blob")
    raw = path.read_bytes()
    if digest(raw) != expected_digest:
        raise ImportRejected("oci_digest_mismatch")
    return raw


def read_oci_wasm(layout: Path, *, original_ref: str, manifest_digest: str, layer_digest: str) -> tuple[bytes, dict]:
    """Select exact manifest and layer; never use first-manifest/first-Wasm rules."""
    if any(c.isspace() for c in original_ref) or original_ref.count("@") > 1 or ("@" in original_ref and not original_ref.endswith("@" + manifest_digest)):
        raise ImportRejected("oci_reference_mismatch")
    index = parse_json((Path(layout) / "index.json").read_bytes())
    manifests = index.get("manifests", [])
    matches = [entry for entry in manifests if entry.get("digest") == manifest_digest]
    if len(matches) != 1:
        raise ImportRejected("oci_manifest_selection")
    manifest_raw = _oci_blob(layout, manifest_digest)
    if str(len(manifest_raw)) != str(matches[0].get("size")):
        raise ImportRejected("oci_manifest_size")
    manifest = parse_json(manifest_raw)
    layers = [entry for entry in manifest.get("layers", []) if entry.get("digest") == layer_digest]
    if len(layers) != 1:
        raise ImportRejected("oci_layer_selection")
    layer = _oci_blob(layout, layer_digest)
    if str(len(layer)) != str(layers[0].get("size")):
        raise ImportRejected("oci_layer_size")
    media = layers[0].get("mediaType")
    if media == "application/vnd.module.wasm.content.layer.v1+wasm":
        wasm = layer
    elif media in ("application/vnd.oci.image.layer.v1.tar+gzip", "application/vnd.docker.image.rootfs.diff.tar.gzip"):
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(layer)) as compressed:
                expanded = compressed.read(80 * 1024 * 1024 + 1)
            if len(expanded) > 80 * 1024 * 1024:
                raise ImportRejected("oci_layer_limit")
            candidates = []
            with tarfile.open(fileobj=io.BytesIO(expanded), mode="r:") as archive:
                for index, member in enumerate(archive):
                    if index >= 128 or not safe_path(member.name) or not member.isfile() or member.size > 64 * 1024 * 1024:
                        raise ImportRejected("unsafe_oci_layer")
                    if member.name.endswith(".wasm"):
                        stream = archive.extractfile(member)
                        candidates.append(stream.read(member.size + 1))
            if len(candidates) != 1:
                raise ImportRejected("ambiguous_oci_wasm")
            wasm = candidates[0]
        except (tarfile.TarError, EOFError, OSError) as exc:
            raise ImportRejected("invalid_oci_layer") from exc
    else:
        raise ImportRejected("unsupported_oci_layer")
    validate_wasm(wasm)
    source = {"kind": "oci_import", "verification": "verified", "original_ref": original_ref,
              "manifest_digest": manifest_digest, "layer_digest": layer_digest}
    return wasm, source


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True, help="directory of complete .tgz packages")
    parser.add_argument("--destination", type=Path, required=True)
    parser.add_argument("--public-base-url", default="http://higress-plugin-server.higress-system.svc:8080")
    parser.add_argument("--dry-run", action="store_true")
    parser.add_argument("--policy", type=Path)
    parser.add_argument("--prune", action="store_true", help="rejected: retention requires Console coordination")
    args = parser.parse_args()
    if args.prune:
        raise ImportRejected("retention_coordination_required")
    with tempfile.TemporaryDirectory(prefix="artifact-preflight-") as temp:
        destination = Path(temp) / "plugins" if args.dry_run else args.destination
        results = sync_packages(args.source, ArtifactStore(destination, public_base_url=args.public_base_url, policy=load_policy(args.policy)))
    print(json.dumps({"committed": len(results), "uploads": results}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
