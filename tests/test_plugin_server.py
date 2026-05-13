import tempfile
import threading
import unittest
from http.client import HTTPConnection
from http.server import ThreadingHTTPServer
from pathlib import Path

from plugin_server import PluginRequestHandler


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

    def test_put_plugin_wasm_with_token_writes_file_and_metadata(self):
        wasm = b"\x00asm-built-plugin"

        status, _, payload = self.request(
            "PUT",
            "/plugins/demo/1.2.3/plugin.wasm",
            body=wasm,
            headers={"Authorization": "Bearer secret-token"},
        )

        self.assertEqual(status, 201, payload)
        plugin_path = self.plugins_dir / "demo" / "1.2.3" / "plugin.wasm"
        metadata_path = self.plugins_dir / "demo" / "1.2.3" / "metadata.txt"
        self.assertEqual(plugin_path.read_bytes(), wasm)
        metadata = metadata_path.read_text()
        self.assertIn("Plugin Name: demo", metadata)
        self.assertIn("Version: 1.2.3", metadata)
        self.assertIn("Size: 17 bytes", metadata)
        self.assertIn("MD5: ", metadata)

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


if __name__ == "__main__":
    unittest.main()
