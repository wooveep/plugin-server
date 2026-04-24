from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from generate_metadata import build_metadata_text, calculate_md5, generate_all_metadata
from pull_plugins import generate_metadata, read_properties


class PluginServerScriptTests(unittest.TestCase):
    def test_calculate_md5_and_metadata_text(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            version_dir = Path(temp_dir) / "demo-plugin" / "1.0.0"
            version_dir.mkdir(parents=True)
            wasm_path = version_dir / "plugin.wasm"
            wasm_path.write_bytes(b"wasm-binary")

            digest = calculate_md5(wasm_path)
            text = build_metadata_text(wasm_path)

            self.assertIn("File: demo-plugin:1.0.0", text)
            self.assertIn(f"MD5: {digest}", text)

    def test_generate_all_metadata_and_pull_plugin_metadata(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            version_dir = Path(temp_dir) / "sample-plugin" / "2.0.0"
            version_dir.mkdir(parents=True)
            wasm_path = version_dir / "plugin.wasm"
            wasm_path.write_bytes(b"sample-wasm")

            generated = generate_all_metadata(Path(temp_dir))
            self.assertEqual(len(generated), 1)
            self.assertTrue((version_dir / "metadata.txt").is_file())

            pull_plugin_dir = Path(temp_dir) / "pulled-plugin"
            pull_plugin_dir.mkdir(parents=True)
            pulled_wasm = pull_plugin_dir / "plugin.wasm"
            pulled_wasm.write_bytes(b"pulled-wasm")
            generate_metadata(str(pull_plugin_dir), "pulled-plugin")

            metadata_text = (pull_plugin_dir / "metadata.txt").read_text(encoding="utf-8")
            self.assertIn("Plugin Name: pulled-plugin", metadata_text)
            self.assertIn("MD5:", metadata_text)

    def test_read_properties_strips_oci_prefix(self) -> None:
        with tempfile.TemporaryDirectory() as temp_dir:
            properties_path = Path(temp_dir) / "plugins.properties"
            properties_path.write_text(
                "demo=oci://registry.example.com/team/demo:1.0.0\n"
                "# comment\n"
                "sample=registry.example.com/team/sample:2.0.0\n",
                encoding="utf-8",
            )

            properties = read_properties(str(properties_path))

            self.assertEqual(
                properties,
                {
                    "demo": "registry.example.com/team/demo:1.0.0",
                    "sample": "registry.example.com/team/sample:2.0.0",
                },
            )


if __name__ == "__main__":
    unittest.main()
