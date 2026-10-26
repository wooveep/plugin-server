import tempfile
import threading
import unittest
import io
import tarfile
from http.server import ThreadingHTTPServer
from pathlib import Path
from urllib.error import URLError

from artifact_contract import digest, json_bytes, validate_document
from artifact_packaging import prepare_plugin
from artifact_import import ArtifactClient, ImportRejected, UploadUnknown, read_oci_wasm, sync_packages
from artifact_storage import ArtifactStore
from plugin_server import PluginRequestHandler
from pull_plugins import process_plugin
from test_artifact_contract import example_archive, example_payload


def oci_layout(root, *, media="application/vnd.module.wasm.content.layer.v1+wasm", layer=None):
    layer = example_payload()["plugin.wasm"] if layer is None else layer
    layer_digest = digest(layer)
    manifest = json_bytes({"schemaVersion": 2, "layers": [{"mediaType": media, "digest": layer_digest, "size": len(layer)}]})
    manifest_digest = digest(manifest)
    blobs = root / "blobs/sha256"
    blobs.mkdir(parents=True)
    (blobs / layer_digest[7:]).write_bytes(layer)
    (blobs / manifest_digest[7:]).write_bytes(manifest)
    (root / "index.json").write_bytes(json_bytes({"manifests": [{"digest": "sha256:" + "0" * 64, "size": 0}, {"digest": manifest_digest, "size": len(manifest)}]}))
    return manifest_digest, layer_digest


class ArtifactImportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)

    def tearDown(self):
        self.temp.cleanup()

    def test_lost_ack_queries_same_intent_and_returns_original_manifest(self):
        handler = PluginRequestHandler.create(self.root / "plugins", "service-token")
        class LostAckHandler(handler):
            lost = False
            def _reply(self, status, raw, content_type="application/json"):
                if self.command == "PUT" and status == 201 and not type(self).lost:
                    type(self).lost = True
                    self.close_connection = True
                    return
                super()._reply(status, raw, content_type)
        server = ThreadingHTTPServer(("127.0.0.1", 0), LostAckHandler)
        thread = threading.Thread(target=server.serve_forever)
        thread.start()
        try:
            client = ArtifactClient("http://127.0.0.1:" + str(server.server_port), "service-token", timeout=2)
            first = client.commit("same-intent", example_archive())
            second = client.commit("same-intent", example_archive())
            self.assertEqual(first, second)
            self.assertEqual(client.query("same-intent"), first)
        finally:
            server.shutdown()
            thread.join(timeout=5)
            server.server_close()

    def test_unreachable_query_remains_unknown_without_changing_key(self):
        calls = []
        def offline(request, **kwargs):
            calls.append((request.method, request.full_url))
            raise URLError("offline")
        with self.assertRaises(UploadUnknown) as error:
            ArtifactClient("http://example.invalid", "private", opener=offline).commit("fixed-key", example_archive())
        self.assertEqual(error.exception.upload_id, "fixed-key")
        self.assertEqual([method for method, _ in calls], ["PUT", "GET"])
        self.assertTrue(all(url.endswith("/uploads/fixed-key") for _, url in calls))

    def test_partial_ack_json_and_short_http_body_query_original_commit(self):
        for mode in ("invalid_json", "short_http_body"):
            with self.subTest(mode=mode):
                handler = PluginRequestHandler.create(self.root / mode / "plugins", "service-token")
                class PartialAckHandler(handler):
                    lost = False
                    queries = 0
                    def _reply(self, status, raw, content_type="application/json"):
                        if self.command == "GET" and self.path.startswith("/uploads/"):
                            type(self).queries += 1
                        if self.command == "PUT" and status == 201 and not type(self).lost:
                            type(self).lost = True
                            if mode == "invalid_json":
                                return super()._reply(status, raw[:20], content_type)
                            self.send_response(status)
                            self.send_header("Content-Length", str(len(raw)))
                            self.end_headers()
                            self.wfile.write(raw[:20])
                            self.close_connection = True
                            return
                        super()._reply(status, raw, content_type)
                server = ThreadingHTTPServer(("127.0.0.1", 0), PartialAckHandler)
                thread = threading.Thread(target=server.serve_forever)
                thread.start()
                try:
                    client = ArtifactClient("http://127.0.0.1:" + str(server.server_port), "service-token", timeout=2)
                    receipt = client.commit("partial-ack", example_archive())
                    self.assertEqual(receipt["state"], "committed")
                    self.assertEqual(PartialAckHandler.queries, 1)
                    self.assertEqual(receipt, handler.store.query("partial-ack"))
                finally:
                    server.shutdown()
                    thread.join(timeout=5)
                    server.server_close()

    def test_sync_preflights_all_packages_and_retains_old_versions(self):
        source = self.root / "source"
        source.mkdir()
        (source / "one.tgz").write_bytes(example_archive())
        (source / "two.tgz").write_bytes(b"invalid")
        store = ArtifactStore(self.root / "plugins")
        with self.assertRaises(Exception):
            sync_packages(source, store)
        self.assertIsNone(store.manifest("test-plugin", "1.0.0"))
        (source / "two.tgz").unlink()
        first = sync_packages(source, store)[0]
        self.assertEqual(sync_packages(source, store)[0], first)
        (source / "two.tgz").write_bytes(example_archive(version="1.0.1"))
        sync_packages(source, store)
        self.assertEqual(store.manifest("test-plugin", "1.0.0"), first["manifest"])

    def test_oci_exact_manifest_selection_ports_tag_at_digest_and_blob_identity(self):
        layout = self.root / "layout"
        manifest_digest, layer_digest = oci_layout(layout)
        original = "registry.example:9980/plugins/demo:tag@" + manifest_digest
        wasm, source = read_oci_wasm(layout, original_ref=original, manifest_digest=manifest_digest, layer_digest=layer_digest)
        self.assertEqual(wasm, example_payload()["plugin.wasm"])
        self.assertEqual(source["original_ref"], original)
        self.assertEqual(source["manifest_digest"], manifest_digest)
        self.assertEqual(source["layer_digest"], layer_digest)
        (layout / "blobs/sha256" / layer_digest[7:]).write_bytes(b"tampered")
        with self.assertRaises(ImportRejected) as error:
            read_oci_wasm(layout, original_ref=original, manifest_digest=manifest_digest, layer_digest=layer_digest)
        self.assertEqual(error.exception.code, "oci_digest_mismatch")

    def test_oci_non_wasm_layer_and_mismatched_ref_rejected(self):
        layout = self.root / "layout"
        manifest_digest, layer_digest = oci_layout(layout, media="application/json")
        with self.assertRaises(ImportRejected):
            read_oci_wasm(layout, original_ref="registry/demo@sha256:" + "f" * 64, manifest_digest=manifest_digest, layer_digest=layer_digest)
        with self.assertRaises(ImportRejected) as error:
            read_oci_wasm(layout, original_ref="registry/demo:tag", manifest_digest=manifest_digest, layer_digest=layer_digest)
        self.assertEqual(error.exception.code, "unsupported_oci_layer")

    def test_oci_pinned_source_lock_commits_through_same_store(self):
        layout = self.root / "layout"
        manifest_digest, layer_digest = oci_layout(layout)
        ref = "registry.example:9980/key-auth:1.0.3@" + manifest_digest
        _, source = read_oci_wasm(layout, original_ref=ref, manifest_digest=manifest_digest, layer_digest=layer_digest)
        lock = {"plugin_name": "key-auth", "plugin_version": "1.0.3", "original_ref": ref,
                "manifest_digest": manifest_digest, "layer_digest": layer_digest,
                "wasm_digest": digest(example_payload()["plugin.wasm"]), "source_receipt": digest(json_bytes(source))}
        spec = (Path(__file__).parent / "fixtures/schema-conversion/key-auth.yaml").read_bytes()
        candidate = prepare_plugin(spec=spec, wasm=example_payload()["plugin.wasm"], defaults=b"{}", documentation={"docs/README.md": b"Pinned package"}, source=source)
        with tarfile.open(fileobj=io.BytesIO(candidate), mode="r:gz") as archive:
            lock["content_digest"] = validate_document(archive.extractfile("package.json").read())["content_digest"]
        receipt = process_plugin(self.root / "destination", "key-auth", ref, "1.0.3", source_lock=lock, layout=layout,
                                  spec=spec,
                                  defaults=b"{}", documentation={"docs/README.md": b"Pinned package"})
        self.assertEqual(receipt["state"], "committed")
        self.assertEqual(receipt["manifest"]["source"], source)
