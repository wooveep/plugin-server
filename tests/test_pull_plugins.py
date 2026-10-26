import tempfile
import unittest
from pathlib import Path

from artifact_import import ImportRejected
from pull_plugins import process_plugin


class PullPluginsTest(unittest.TestCase):
    def test_raw_local_file_does_not_bypass_source_lock_and_full_package(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "plugins/demo/2.0.0/plugin.wasm"
            path.parent.mkdir(parents=True)
            path.write_bytes(b"legacy bytes")
            with self.assertRaises(ImportRejected) as error:
                process_plugin(temp, "demo", "registry.example.com/demo:2.0.0", "2.0.0")
            self.assertEqual(error.exception.code, "complete_package_and_source_lock_required")
            self.assertEqual(path.read_bytes(), b"legacy bytes")
            self.assertFalse(path.with_name("metadata.txt").exists())
