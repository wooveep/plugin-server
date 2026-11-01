import json
from pathlib import Path
import runpy
import unittest
from unittest.mock import Mock

from plugin_catalog import selected_entries

ROOT = Path(__file__).resolve().parents[1]
release_catalog = runpy.run_path(str(ROOT / "scripts/verify-release-store.py"))["release_catalog"]


class ReleaseCatalogTests(unittest.TestCase):
    def test_release_requires_model_router_and_every_source_selected_version(self):
        catalog = json.loads((ROOT / "plugins/release/catalog.json").read_bytes())
        metadata = [{"manifest": {"plugin_name": entry["logicalId"], "plugin_version": entry["declaredVersion"]},
                     "metadata": {"info": {"name": entry["logicalId"], "version": entry["declaredVersion"]}}}
                    for entry in selected_entries(catalog)]
        store = Mock()
        store.list_metadata.return_value = [entry for entry in metadata if entry["manifest"]["plugin_name"] != "model-router"]
        with self.assertRaisesRegex(ValueError, "missing=.*model-router"):
            release_catalog(store, catalog)
        store.list_metadata.return_value = metadata
        self.assertEqual(release_catalog(store, catalog)["count"], len(catalog["requiredPlugins"]))

    def test_registered_schema_identity_cannot_diverge_from_immutable_identity(self):
        catalog = {"requiredPlugins": ["router"], "plugins": [
            {"logicalId": "router", "implementation": "go", "releaseEligible": True, "declaredVersion": "2.0.3"}]}
        store = Mock()
        store.list_metadata.return_value = [{"manifest": {"plugin_name": "router", "plugin_version": "2.0.3"},
                                             "metadata": {"info": {"name": "router", "version": "1.0.0"}}}]
        with self.assertRaisesRegex(ValueError, "metadata identity differs"):
            release_catalog(store, catalog)

    def test_previous_registered_versions_are_retained_but_not_selected(self):
        catalog = {"requiredPlugins": ["router"], "plugins": [
            {"logicalId": "router", "implementation": "go", "releaseEligible": True, "declaredVersion": "2.0.3"}]}
        store = Mock()
        store.list_metadata.return_value = [
            {"manifest": {"plugin_name": "router", "plugin_version": version},
             "metadata": {"info": {"name": "router", "version": version}}}
            for version in ["2.0.2", "2.0.3"]]
        result = release_catalog(store, catalog)
        self.assertEqual(result["count"], 1)
        self.assertEqual(result["artifacts"][0]["manifest"]["plugin_version"], "2.0.3")

    def test_unreleased_candidate_does_not_replace_formal_default(self):
        catalog = {"requiredPlugins": ["router"], "plugins": [
            {"logicalId": "router", "implementation": implementation, "releaseEligible": implementation == "go",
             "declaredVersion": version}
            for implementation, version in [("go", "2.0.3"), ("cpp", "3.0.0-beta.1")]]}
        self.assertEqual(selected_entries(catalog)[0]["declaredVersion"], "2.0.3")

    def test_release_image_store_contains_only_selected_packages_and_seed_retains_old_refs(self):
        import subprocess
        import sys
        import tempfile
        from artifact_storage import ArtifactStore
        from artifact_packaging import package_plugin

        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = ArtifactStore(root / "all-plugins")
            for upload, version in [("old", "1.0.0"), ("current", "1.1.0"), ("uploaded", "2.0.0")]:
                spec = json.dumps({"apiVersion": "1.0.0", "info": {"name": "test-plugin", "version": version},
                                   "spec": {"phase": "default", "priority": 1, "configSchema": {
                                       "openAPIV3Schema": {"type": "object", "additionalProperties": False}}}}).encode()
                source.commit(upload, package_plugin(spec=spec, wasm=b"\0asm\1\0\0\0", defaults=b"{}",
                              documentation={"docs/README.md": b"Test release"},
                              source={"kind": "legacy_unknown", "verification": "unknown"}))
            # A newly uploaded higher version does not replace the formal default.
            catalog = root / "catalog.json"
            catalog.write_text(json.dumps({"requiredPlugins": ["test-plugin"], "plugins": [
                {"logicalId": "test-plugin", "implementation": "go", "releaseEligible": True,
                 "declaredVersion": "1.1.0"}]}))
            output = root / "plugin-catalog.json"
            release_root = root / "release-plugins"
            result = subprocess.run([sys.executable, str(ROOT / "scripts/verify-release-store.py"),
                            "--plugins-dir", str(source.root), "--catalog", str(catalog),
                            "--output", str(output), "--release-dir", str(release_root)], capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            release = ArtifactStore(release_root)
            self.assertEqual([item["plugin_version"] for item in release.list_manifests()], ["1.1.0"])
            self.assertFalse((release.root / "test-plugin/1.0.0").exists())
            self.assertFalse((release.root / "test-plugin/2.0.0").exists())
            self.assertEqual(json.loads(output.read_bytes())["artifacts"][0]["manifest"]["plugin_version"], "1.1.0")
            destination = ArtifactStore(root / "live-plugins")
            destination.seed_registered(source.root, [source.manifest("test-plugin", "1.0.0")])
            old_bytes = destination.read_file("test-plugin", "1.0.0", "plugin.wasm")
            destination.seed_registered(release.root, release.list_manifests())
            self.assertEqual(destination.read_file("test-plugin", "1.0.0", "plugin.wasm"), old_bytes)
            self.assertEqual(len(destination.list_manifests()), 2)
