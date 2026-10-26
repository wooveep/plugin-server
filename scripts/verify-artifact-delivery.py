#!/usr/bin/env python3
"""Verify an immutable local image over real HTTP and a container restart."""
from __future__ import annotations

import argparse
import json
import secrets
import socket
import subprocess
import sys
import time
import uuid
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_contract import digest, json_bytes
from artifact_packaging import build_archive


def run(*args):
    return subprocess.run(args, check=True, capture_output=True, text=True, timeout=60).stdout


def fixture_archive(documentation=b"Runtime delivery check"):
    payload = {"plugin.wasm": b"\0asm\1\0\0\0", "config.schema.json": b'{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}',
               "defaults.json": b"{}", "docs/README.md": documentation}
    roles = {"plugin.wasm": "wasm", "config.schema.json": "config_schema", "defaults.json": "defaults", "docs/README.md": "documentation"}
    descriptor = {"canonicalization_version": "artifact-json.v1", "schema_version": "artifact.v1", "plugin_name": "artifact-runtime-upload", "plugin_version": "0.0.1",
                  "files": [{"path": name, "role": roles[name], "size_bytes": str(len(raw)), "sha256": digest(raw)} for name, raw in sorted(payload.items())],
                  "wasm_path": "plugin.wasm", "config_schema_path": "config.schema.json", "defaults_path": "defaults.json", "documentation_paths": ["docs/README.md"],
                  "environment_refs": [], "source": {"kind": "legacy_unknown", "verification": "unknown"}}
    return build_archive(descriptor, payload)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", required=True)
    parser.add_argument("--output", type=Path, required=True, help="new evidence directory")
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    container = "artifact-p1-" + uuid.uuid4().hex[:12]
    token = secrets.token_urlsafe(24)
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    base = f"http://127.0.0.1:{port}"
    image_id = run("docker", "image", "inspect", args.image, "--format", "{{.Id}}").strip()
    report = {"evidence_level": "actual_container_http", "image_id": image_id, "gateway_abi": "not_run", "default_k3s": "not_run", "result": "execution_error", "cases": []}

    def request(method, path, raw=None, authorized=False):
        headers = {"Authorization": "Bearer " + token} if authorized else {}
        try:
            with urlopen(Request(base + path, data=raw, headers=headers, method=method), timeout=10) as response:
                return response.status, response.read()
        except HTTPError as error:
            try:
                return error.code, error.read()
            finally:
                error.close()

    def ready():
        for attempt in range(50):
            try:
                if request("GET", "/uploads/not-created", authorized=True)[0] == 404:
                    return
            except (OSError, URLError):
                pass
            time.sleep(0.1)
        raise RuntimeError("container HTTP endpoint did not start")

    try:
        run("docker", "run", "-d", "--name", container, "-p", f"127.0.0.1:{port}:8080", "-e", "PLUGIN_SERVER_UPLOAD_TOKEN=" + token,
            "-e", "PLUGIN_SERVER_PUBLIC_BASE_URL=" + base, image_id)
        ready()
        inventory_raw = run("docker", "exec", container, "cat", "/opt/plugin-server/artifact-inventory.json").encode()
        inventory = json.loads(inventory_raw)
        (args.output / "image-inventory.json").write_bytes(inventory_raw)
        actual = []
        for entry in inventory["artifacts"] + inventory["legacy"]:
            prefix = "/plugins/" + quote(entry["plugin_name"], safe="") + "/" + quote(entry["plugin_version"], safe="") + "/"
            if "manifest_digest" in entry:
                status, raw = request("GET", prefix + "manifest.json")
                if status != 200 or digest(raw) != entry["manifest_digest"]:
                    raise AssertionError("image manifest HTTP readback mismatch")
                actual.append({"path": prefix + "manifest.json", "status": status, "sha256": digest(raw), "size_bytes": str(len(raw))})
            for expected in entry["files"]:
                path = prefix + quote(expected["path"], safe="/")
                status, raw = request("GET", path)
                if status != 200 or digest(raw) != expected["sha256"] or str(len(raw)) != expected["size_bytes"]:
                    raise AssertionError("image payload HTTP readback mismatch: " + path)
                actual.append({"path": path, "status": status, "sha256": digest(raw), "size_bytes": str(len(raw))})
        if not actual:
            raise AssertionError("empty expected image inventory")
        (args.output / "http-files.json").write_bytes(json_bytes(actual))
        report["cases"].append({"case_id": "image-inventory-http", "result": "passed", "file_count": str(len(actual)), "complete_artifacts": str(len(inventory["artifacts"])), "legacy_versions": str(len(inventory["legacy"]))})
        archive = fixture_archive()
        status, raw = request("PUT", "/uploads/runtime-upload", archive, authorized=True)
        if status != 201:
            raise AssertionError("complete package upload failed")
        receipt = json.loads(raw)
        (args.output / "runtime-upload.json").write_bytes(raw)
        with urlopen(receipt["manifest"]["download_url"], timeout=5) as response:
            if response.read() != b"\0asm\1\0\0\0":
                raise AssertionError("returned download URL does not serve uploaded bytes")
        if request("PUT", "/uploads/runtime-upload", fixture_archive(b"Changed"), authorized=True)[0] != 409:
            raise AssertionError("idempotency conflict was not rejected")
        if request("PUT", "/uploads/other-upload", fixture_archive(b"Changed"), authorized=True)[0] != 409:
            raise AssertionError("version conflict was not rejected")
        if request("PUT", "/uploads/no-auth", archive)[0] != 401:
            raise AssertionError("unauthenticated write accepted")
        if request("GET", "/.plugins-artifact-state/state.sqlite3")[0] != 404:
            raise AssertionError("private state was downloadable")
        report["cases"].append({"case_id": "upload-conflicts-auth-private-stage", "result": "passed"})
        run("docker", "restart", container)
        ready()
        status, after = request("GET", "/uploads/runtime-upload", authorized=True)
        if status != 200 or json.loads(after) != receipt:
            raise AssertionError("upload receipt did not survive restart")
        status, replay = request("PUT", "/uploads/runtime-upload", archive, authorized=True)
        if status != 201 or json.loads(replay) != receipt:
            raise AssertionError("same intent changed after restart")
        report["cases"].append({"case_id": "persistent-container-restart", "result": "passed"})
        report["result"] = "passed"
    finally:
        subprocess.run(["docker", "rm", "-f", "-v", container], capture_output=True, timeout=30)
        report["files"] = [{"path": path.name, "sha256": digest(path.read_bytes()), "size_bytes": str(path.stat().st_size)} for path in sorted(args.output.iterdir()) if path.is_file()]
        (args.output / "acceptance.json").write_bytes(json_bytes(report))
    print(json.dumps(report))


if __name__ == "__main__":
    main()
