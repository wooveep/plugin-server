import importlib.util
import tempfile
import unittest
from pathlib import Path

from artifact_storage import ArtifactStore, StorageUnavailable
from test_artifact_contract import example_archive

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("producer_inventory", ROOT / "scripts/export-artifact-inventory.py")
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)


class ArtifactInventoryTests(unittest.TestCase):
    def test_nonempty_complete_inventory_verifies_actual_payload_bytes(self):
        with tempfile.TemporaryDirectory() as temporary:
            store = ArtifactStore(Path(temporary) / "plugins")
            with self.assertRaises(ValueError):
                inventory.export_inventory(store)
            receipt = store.commit("inventory", example_archive())
            result = inventory.export_inventory(store)
            self.assertEqual(len(result["artifacts"]), 1)
            self.assertEqual(len(result["artifacts"][0]["files"]), 7)
            self.assertEqual(result["artifacts"][0]["content_digest"], receipt["content_digest"])
            (store.root / "test-plugin/1.0.0/plugin.wasm").write_bytes(b"wrong")
            with self.assertRaises(StorageUnavailable):
                inventory.export_inventory(store)

    def test_missing_archive_cannot_receive_success_inventory(self):
        with tempfile.TemporaryDirectory() as temporary:
            store = ArtifactStore(Path(temporary) / "plugins")
            store.commit("inventory", example_archive())
            (store.root / "test-plugin/1.0.0/archive.tgz").unlink()
            with self.assertRaises(StorageUnavailable):
                inventory.export_inventory(store)
