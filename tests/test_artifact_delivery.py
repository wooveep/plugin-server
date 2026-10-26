import io
import json
import subprocess
import sys
import tarfile
import tempfile
import threading
import unittest
from http.server import ThreadingHTTPServer
from pathlib import Path

from artifact_contract import ValidationPolicy, digest, json_bytes, validate_document
from artifact_packaging import prepare_plugin
from plugin_server import PluginRequestHandler

ROOT = Path(__file__).resolve().parents[1]


class ArtifactDeliveryTests(unittest.TestCase):
    def test_verified_package_packaging_zipapp_upload_sync_and_seed_share_policy(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            spec = (ROOT / "tests/fixtures/schema-conversion/key-auth.yaml").read_bytes()
            wasm = b"\0asm\1\0\0\0"
            source = {"kind": "local_build", "verification": "verified", "repo": "test-source", "commit": "a" * 40,
                      "dirty_digest": "sha256:" + "0" * 64, "build_inputs_digest": "sha256:" + "1" * 64}
            candidate = prepare_plugin(spec=spec, wasm=wasm, defaults=b"{}", documentation={"docs/README.md": b"Docs"}, source=source)
            with tarfile.open(fileobj=io.BytesIO(candidate), mode="r:gz") as archive:
                content_digest = validate_document(archive.extractfile("package.json").read())["content_digest"]
            receipt_tuple = (digest(json_bytes(source)), digest(wasm), content_digest)
            policy_value = {"provenance_receipts": [{"source_digest": receipt_tuple[0], "wasm_digest": receipt_tuple[1], "content_digest": receipt_tuple[2]}]}
            policy = ValidationPolicy(provenance_receipts=frozenset((receipt_tuple,)))
            for name, raw in {"spec.yaml": spec, "plugin.wasm": wasm, "defaults.json": b"{}", "README.md": b"Docs", "source.json": json_bytes(source), "policy.json": json_bytes(policy_value)}.items():
                (root / name).write_bytes(raw)
            def run(*args):
                return subprocess.run([sys.executable, *args], cwd=ROOT, check=True, capture_output=True, text=True, timeout=15)
            run("artifact_packaging.py", "--spec", str(root / "spec.yaml"), "--wasm", str(root / "plugin.wasm"), "--defaults", str(root / "defaults.json"),
                "--documentation", str(root / "README.md"), "--source", str(root / "source.json"), "--policy", str(root / "policy.json"), "--output", str(root / "package.tgz"))
            self.assertEqual((root / "package.tgz").read_bytes(), candidate)
            run("scripts/build-upload-cli.py", "--output", str(root / "upload-plugin"))
            handler = PluginRequestHandler.create(root / "http/plugins", "test-only-token", policy=policy)
            server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
            thread = threading.Thread(target=server.serve_forever)
            thread.start()
            try:
                output = run(str(root / "upload-plugin"), "--server", "http://127.0.0.1:" + str(server.server_port), "--token", "test-only-token",
                             "--file", str(root / "package.tgz"), "--name", "key-auth", "--version", "1.0.3", "--upload-id", "explicit", "--policy", str(root / "policy.json"))
                self.assertIn(content_digest, output.stdout)
                self.assertEqual(handler.store.query("explicit")["content_digest"], content_digest)
            finally:
                server.shutdown()
                thread.join(timeout=5)
                server.server_close()
            packages = root / "packages"
            packages.mkdir()
            (packages / "one.tgz").write_bytes(candidate)
            synced = json.loads(run("artifact_import.py", "--source", str(packages), "--destination", str(root / "sync/plugins"), "--policy", str(root / "policy.json")).stdout)
            seeded = json.loads(run("scripts/seed-artifact-packages.py", "--source", str(packages), "--destination", str(root / "seed/plugins"), "--policy", str(root / "policy.json")).stdout)
            self.assertEqual(synced["uploads"][0]["content_digest"], content_digest)
            self.assertEqual(seeded["uploads"][0]["content_digest"], content_digest)
