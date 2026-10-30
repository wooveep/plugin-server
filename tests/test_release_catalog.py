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
