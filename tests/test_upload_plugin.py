import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import upload_plugin


class UploadPluginCliTest(unittest.TestCase):
    def test_upload_plugin_sends_put_request_with_token_and_file_body(self):
        with tempfile.NamedTemporaryFile() as wasm_file:
            wasm_file.write(b"\x00asm-cli")
            wasm_file.flush()

            with patch("upload_plugin.urlopen") as urlopen:
                response = urlopen.return_value.__enter__.return_value
                response.status = 201
                response.read.return_value = b"uploaded\n"

                upload_plugin.upload_plugin(
                    server="http://localhost:8080/",
                    token="secret-token",
                    plugin_file=Path(wasm_file.name),
                    name="demo",
                    version="1.2.3",
                )

        request = urlopen.call_args.args[0]
        self.assertEqual(
            request.full_url,
            "http://localhost:8080/plugins/demo/1.2.3/plugin.wasm",
        )
        self.assertEqual(request.get_method(), "PUT")
        self.assertEqual(request.data, b"\x00asm-cli")
        self.assertEqual(request.headers["Authorization"], "Bearer secret-token")
        self.assertEqual(request.headers["Content-type"], "application/wasm")

    def test_upload_plugin_rejects_missing_file(self):
        with self.assertRaisesRegex(FileNotFoundError, "missing.wasm"):
            upload_plugin.upload_plugin(
                server="http://localhost:8080",
                token="secret-token",
                plugin_file=Path("missing.wasm"),
                name="demo",
                version="1.2.3",
            )


if __name__ == "__main__":
    unittest.main()
