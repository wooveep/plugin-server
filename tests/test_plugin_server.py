import tempfile
import threading
import unittest
import json
import hashlib
import random
import tracemalloc
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
from urllib.request import urlopen
from http.client import HTTPConnection
from http.server import ThreadingHTTPServer
from pathlib import Path
from unittest.mock import Mock, patch

from plugin_server import PluginRequestHandler
from artifact_packaging import build_archive, prepare_plugin
from test_artifact_contract import descriptor, example_payload


class PluginServerTest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.plugins_dir = Path(self.temp_dir.name) / "plugins"
        self.plugins_dir.mkdir()

        handler = PluginRequestHandler.create(
            plugins_dir=self.plugins_dir,
            upload_token="secret-token",
        )
        self.server = ThreadingHTTPServer(("127.0.0.1", 0), handler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        self.host, self.port = self.server.server_address

    def tearDown(self):
        self.server.shutdown()
        self.thread.join(timeout=2)
        self.server.server_close()
        self.temp_dir.cleanup()

    def request(self, method, path, body=None, headers=None):
        conn = HTTPConnection(self.host, self.port, timeout=5)
        conn.request(method, path, body=body, headers=headers or {})
        response = conn.getresponse()
        payload = response.read()
        conn.close()
        return response.status, dict(response.getheaders()), payload

    def test_put_complete_package_writes_immutable_files_and_manifest(self):
        payload = example_payload()
        desc = descriptor(payload, "1.2.3")
        desc["plugin_name"] = "demo"
        archive = build_archive(desc, payload)

        status, _, response_body = self.request(
            "PUT",
            "/plugins/demo/1.2.3/package.tgz",
            body=archive,
            headers={"Authorization": "Bearer secret-token", "Idempotency-Key": "upload-test"},
        )

        self.assertEqual(status, 201, response_body)
        plugin_path = self.plugins_dir / "demo" / "1.2.3" / "plugin.wasm"
        metadata_path = self.plugins_dir / "demo" / "1.2.3" / "metadata.txt"
        self.assertEqual(plugin_path.read_bytes(), payload["plugin.wasm"])
        metadata = metadata_path.read_text()
        self.assertIn("Plugin Name: demo", metadata)
        self.assertIn("Version: 1.2.3", metadata)
        self.assertIn("Size: 8 bytes", metadata)
        self.assertIn("SHA256: sha256:", metadata)
        receipt = json.loads(response_body)
        self.assertEqual(receipt["state"], "committed")
        status, _, raw = self.request("GET", "/plugins/demo/1.2.3/manifest.json")
        self.assertEqual(status, 200)
        self.assertEqual(json.loads(raw), receipt["manifest"])

    def test_raw_wasm_write_cannot_bypass_package_commit(self):
        status, _, _ = self.request("PUT", "/plugins/demo/1.2.3/plugin.wasm", body=b"raw-wasm",
                                    headers={"Authorization": "Bearer secret-token"})
        self.assertEqual(status, 400)
        self.assertFalse((self.plugins_dir / "demo").exists())

    def test_full_package_with_invalid_wasm_is_rejected_before_publication(self):
        payload = example_payload()
        payload["plugin.wasm"] = b"not-wasm"
        archive = build_archive(descriptor(payload), payload)
        status, _, _ = self.request(
            "PUT", "/uploads/invalid-wasm", body=archive,
            headers={"Authorization": "Bearer secret-token"},
        )
        self.assertEqual(status, 422)
        self.assertEqual(self.request("GET", "/plugins/test-plugin/1.0.0/plugin.wasm")[0], 404)

    def test_committed_get_reads_body_once_and_head_reads_no_body(self):
        payload = example_payload()
        archive = build_archive(descriptor(payload), payload)
        status, _, response = self.request(
            "PUT", "/uploads/read-once", body=archive,
            headers={"Authorization": "Bearer secret-token"},
        )
        self.assertEqual(status, 201, response)
        wasm_path = self.plugins_dir / "test-plugin/1.0.0/plugin.wasm"
        original_open = Path.open
        read_bytes = []

        @contextmanager
        def observed_open(target, *args, **kwargs):
            with original_open(target, *args, **kwargs) as stream:
                if target != wasm_path:
                    yield stream
                    return
                observed = Mock(wraps=stream)

                def read(*args, **kwargs):
                    data = stream.read(*args, **kwargs)
                    read_bytes.append(len(data))
                    return data

                def readinto(buffer):
                    size = stream.readinto(buffer)
                    read_bytes.append(size)
                    return size

                observed.read.side_effect = read
                observed.readinto.side_effect = readinto
                yield observed

        path = "/plugins/test-plugin/1.0.0/plugin.wasm"
        with patch.object(Path, "open", observed_open):
            status, headers, body = self.request("GET", path)
            self.assertEqual((status, body), (200, payload["plugin.wasm"]))
            self.assertEqual(headers["Content-Length"], str(len(body)))
            self.assertEqual(sum(read_bytes), len(body), "GET must not scan the body before sending it")
            read_bytes.clear()
            status, headers, body = self.request("HEAD", path)
            self.assertEqual((status, body), (200, b""))
            self.assertEqual(headers["Content-Length"], str(len(payload["plugin.wasm"])))
            self.assertEqual(sum(read_bytes), 0, "HEAD must not read the body")

    def test_batch_metadata_returns_real_numeric_priority_without_revalidation(self):
        archive = prepare_plugin(
            spec=(Path(__file__).parent / "fixtures/schema-conversion/key-auth.yaml").read_bytes(),
            wasm=b"\0asm\1\0\0\0", defaults=b"{}", documentation={"docs/README.md": b"Documented"},
            source={"kind": "legacy_unknown", "verification": "unknown"})
        status, _, body = self.request("PUT", "/uploads/metadata", body=archive,
                                       headers={"Authorization": "Bearer secret-token"})
        self.assertEqual(status, 201, body)
        with patch("artifact_storage.validate_package", side_effect=AssertionError("unexpected package revalidation")):
            status, _, body = self.request("GET", "/artifacts")
        self.assertEqual(status, 200, body)
        entry = json.loads(body)["artifacts"][0]
        self.assertEqual(entry["metadata"]["priority"], 321)
        self.assertEqual(entry["metadata"]["info"]["name"], "key-auth")
        self.assertEqual(entry["manifest"]["plugin_version"], "1.0.3")

    def test_no_directory_listing_or_private_state_download(self):
        for path in ("/", "/plugins/", "/.plugins-artifact-state/state.sqlite3", "/plugins/../.plugins-artifact-state/store.lock"):
            with self.subTest(path=path):
                self.assertEqual(self.request("GET", path)[0], 404)

    def test_upload_query_requires_service_authentication(self):
        self.assertEqual(self.request("GET", "/uploads/unknown")[0], 401)
        self.assertEqual(self.request("GET", "/uploads/unknown", headers={"Authorization": "Bearer secret-token"})[0], 404)

    def test_truncated_or_negative_body_is_rejected(self):
        status, _, _ = self.request("PUT", "/uploads/negative", body=b"", headers={"Authorization": "Bearer secret-token", "Content-Length": "-1"})
        self.assertEqual(status, 400)
        self.assertEqual(self.request("PUT", "/uploads/broken", body=b"invalid", headers={"Authorization": "Bearer secret-token"})[0], 422)

    def test_package_url_identity_cannot_disagree_with_descriptor(self):
        archive = build_archive(descriptor(), example_payload())
        status, _, _ = self.request("PUT", "/plugins/wrong/1.0.0/package.tgz", body=archive,
                                    headers={"Authorization": "Bearer secret-token", "Idempotency-Key": "wrong"})
        self.assertEqual(status, 422)
        self.assertFalse((self.plugins_dir / "wrong").exists())

    def test_semver_build_metadata_and_special_wasm_path_url_round_trip(self):
        payload = example_payload()
        wasm = payload.pop("plugin.wasm")
        name = "bin/plugin?#% 空格.wasm"
        payload[name] = wasm
        desc = descriptor(payload, "1.0.0+build.1")
        desc["wasm_path"] = name
        for item in desc["files"]:
            if item["path"] == name:
                item["role"] = "wasm"
        self.server.RequestHandlerClass.store.base_url = f"http://{self.host}:{self.port}"
        status, _, raw = self.request("PUT", "/uploads/special", body=build_archive(desc, payload), headers={"Authorization": "Bearer secret-token"})
        self.assertEqual(status, 201, raw)
        url = json.loads(raw)["manifest"]["download_url"]
        with urlopen(url, timeout=3) as response:
            self.assertEqual(response.read(), wasm)

    def test_put_plugin_without_token_is_rejected(self):
        status, _, _ = self.request(
            "PUT",
            "/plugins/demo/1.2.3/plugin.wasm",
            body=b"wasm",
        )

        self.assertEqual(status, 401)
        self.assertFalse((self.plugins_dir / "demo").exists())

    def test_put_plugin_rejects_invalid_plugin_path(self):
        status, _, _ = self.request(
            "PUT",
            "/plugins/../bad/1.2.3/plugin.wasm",
            body=b"wasm",
            headers={"Authorization": "Bearer secret-token"},
        )

        self.assertEqual(status, 400)
        self.assertFalse((self.plugins_dir.parent / "bad").exists())

    def test_get_serves_uploaded_plugin(self):
        plugin_dir = self.plugins_dir / "demo" / "1.2.3"
        plugin_dir.mkdir(parents=True)
        plugin_dir.joinpath("plugin.wasm").write_bytes(b"download-me")

        status, headers, payload = self.request(
            "GET",
            "/plugins/demo/1.2.3/plugin.wasm",
        )

        self.assertEqual(status, 200)
        self.assertEqual(payload, b"download-me")
        normalized_headers = {key.lower(): value for key, value in headers.items()}
        self.assertEqual(normalized_headers["content-type"], "application/wasm")

    def test_parallel_cold_downloads_keep_bounded_memory_exact_bytes_and_size_refusal(self):
        payload = example_payload()
        payload["docs/README.md"] = random.Random(7).randbytes(4 * 1024 * 1024).hex().encode()
        archive = build_archive(descriptor(payload), payload)
        status, _, raw = self.request("PUT", "/uploads/parallel-cold-read", body=archive,
                                       headers={"Authorization": "Bearer secret-token"})
        self.assertEqual(status, 201, raw)
        expected_size = len(payload["docs/README.md"])
        expected_digest = hashlib.sha256(payload["docs/README.md"]).hexdigest()
        path = "/plugins/test-plugin/1.0.0/docs/README.md"
        readers = threading.Barrier(16)

        def download(_):
            connection = HTTPConnection(self.host, self.port, timeout=20)
            try:
                connection.request("GET", path)
                response = connection.getresponse()
                self.assertEqual(response.status, 200)
                self.assertEqual(response.getheader("Content-Length"), str(expected_size))
                # Concurrent cold consumers hold their bodies until all headers
                # arrive, as Gateway startup does; no client buffers a whole file.
                readers.wait(timeout=20)
                observed, digest = 0, hashlib.sha256()
                while chunk := response.read(65536):
                    observed += len(chunk)
                    digest.update(chunk)
                return observed, digest.hexdigest()
            finally:
                connection.close()

        tracemalloc.start()
        try:
            with ThreadPoolExecutor(max_workers=16) as pool:
                results = list(pool.map(download, range(16)))
            _, peak = tracemalloc.get_traced_memory()
        finally:
            tracemalloc.stop()
        self.assertEqual(results, [(expected_size, expected_digest)] * 16)
        self.assertLess(peak, expected_size * 4,
                        "parallel downloads must not retain a whole file per response")
        status, headers, body = self.request("HEAD", path)
        self.assertEqual((status, headers["Content-Length"], body), (200, str(expected_size), b""))
        (self.plugins_dir / "test-plugin/1.0.0/docs/README.md").write_bytes(b"corrupted")
        self.assertEqual(self.request("GET", path)[0], 503)


if __name__ == "__main__":
    unittest.main()
