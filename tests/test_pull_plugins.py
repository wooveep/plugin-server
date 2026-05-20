import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import pull_plugins


class PullPluginsTest(unittest.TestCase):
    def test_process_plugin_uses_existing_local_wasm_without_downloading(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            base_path = Path(temp_dir)
            plugin_dir = base_path / "plugins" / "demo" / "2.0.0"
            plugin_dir.mkdir(parents=True)
            plugin_dir.joinpath("plugin.wasm").write_bytes(b"\x00asm-local")

            with patch(
                "pull_plugins.subprocess.run",
                side_effect=AssertionError("download should not be called"),
            ):
                success = pull_plugins.process_plugin(
                    str(base_path),
                    "demo",
                    "registry.example.com/plugins/demo:2.0.0",
                    "2.0.0",
                )

            self.assertTrue(success)
            metadata = plugin_dir.joinpath("metadata.txt").read_text()
            self.assertIn("Plugin Name: demo", metadata)
            self.assertIn("Size: 10 bytes", metadata)


if __name__ == "__main__":
    unittest.main()
