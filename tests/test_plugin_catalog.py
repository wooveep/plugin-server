import json
from pathlib import Path
import tempfile
import subprocess
import runpy
import unittest
from unittest.mock import patch

from plugin_catalog import PluginSource, Version, load_catalog, select_releases, source_inputs
from plugin_schema_conversion import convert_spec
from artifact_contract import validate_defaults


class PluginCatalogTests(unittest.TestCase):
    def test_package_identity_drift_is_rejected_before_writing_a_candidate_review(self):
        from artifact_contract import digest

        root = Path(__file__).resolve().parents[1]
        prepare = runpy.run_path(str(root / "scripts/package-plugins.py"))["prepare"]
        for name, version in [("example", "1.0.0"), ("other-plugin", "1.1.0")]:
            with self.subTest(name=name, version=version), tempfile.TemporaryDirectory() as temporary:
                work = Path(temporary)
                metadata = work / "metadata"
                metadata.mkdir()
                (metadata / "README.md").write_text("Release fixture")
                (metadata / "defaults.json").write_text("{}")
                (metadata / "environment.json").write_text("[]")
                (metadata / "spec.yaml").write_text(json.dumps({
                    "apiVersion": "1.0.0", "info": {"name": name, "version": version},
                    "spec": {"phase": "default", "priority": 1, "configSchema": {
                        "openAPIV3Schema": {"type": "object", "additionalProperties": False}}},
                }))
                output = work / "build/plugins/example/1.1.0"
                output.mkdir(parents=True)
                wasm = output / "plugin.wasm"
                wasm.write_bytes(b"\0asm\1\0\0\0")
                (output / "build-receipt.json").write_text(json.dumps({
                    "plugin_name": "example", "plugin_version": "1.1.0", "implementation": "go",
                    "source_commit": "a" * 40, "source_inputs": {}, "artifact": str(wasm.relative_to(work)),
                    "sha256": digest(wasm.read_bytes()), "size_bytes": wasm.stat().st_size,
                }))
                source = PluginSource("example", "go", metadata, "1.1.0", True, "wasm", None, metadata)
                with patch.dict(prepare.__globals__, {
                    "select_releases": lambda _: [source], "source_inputs": lambda *_: {},
                }), patch("subprocess.check_output", side_effect=["https://example.invalid/plugins", ""]), patch("subprocess.run"):
                    with self.assertRaisesRegex(ValueError, "metadata identity differs from selected source VERSION"):
                        prepare(work)
                self.assertFalse((work / "build/packages").exists())
                self.assertFalse((work / "dist/plugins").exists())

    def test_generated_wasm_test_output_does_not_change_release_inputs(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "plugins/release").mkdir(parents=True)
            directory = root / "plugins/go"
            directory.mkdir()
            (directory / "VERSION").write_text("1.0.0")
            (directory / "main.go").write_text("package main\n")
            (directory / "embedded.wasm").write_bytes(b"embedded source asset")
            (root / "plugins/release/catalog.json").write_text(json.dumps({
                "schemaVersion": 1, "requiredPlugins": ["example"], "plugins": [{
                    "logicalId": "example", "implementation": "go", "sourceDir": "plugins/go",
                    "declaredVersion": "1.0.0", "releaseEligible": True,
                    "artifactInputs": ["plugins/go/**"],
                }],
            }))
            source = select_releases(root)[0]
            original = source_inputs(root, source)
            self.assertIn("plugins/go/embedded.wasm", original)
            (directory / "wasm-unit-test.wasm").write_bytes(b"generated test binary")
            self.assertEqual(source_inputs(root, source), original)
            (directory / "main.go").write_text("package main\nfunc main() {}\n")
            self.assertNotEqual(source_inputs(root, source), original)

    def test_formal_inventory_covers_upstream_and_custom_sources_with_matching_metadata(self):
        root = Path(__file__).resolve().parents[1]
        lock = json.loads((root / "plugins/upstream-lock.json").read_bytes())
        original = json.loads(subprocess.check_output(
            ["git", "show", lock["tracking_commit"] + ":release/catalog.json"], cwd=root))
        upstream = {item["logicalId"] for item in original["plugins"] if item["releaseEligible"]}
        upstream.update(item["key"] for item in original["consumerInventories"]["pluginServer"])
        customized = json.loads((root / "plugins/release/custom-integration.json").read_bytes())
        required = upstream | {item["plugin"] for item in customized["release_decisions"]}
        selected = select_releases(root)
        self.assertLessEqual(required, {source.name for source in selected})
        for source in selected:
            with self.subTest(plugin=source.name):
                converted = convert_spec((source.metadata_directory / "spec.yaml").read_bytes())
                self.assertEqual((converted["metadata"]["info"]["name"], converted["metadata"]["info"]["version"]),
                                 (source.name, source.version))
                payload = {**converted["schemas"], "defaults.json": (source.metadata_directory / "defaults.json").read_bytes()}
                validate_defaults({"plugin_name": source.name, "plugin_version": source.version,
                                   "files": [{"path": name, "role": "config_schema"} for name in converted["schemas"]],
                                   "config_schema_path": "config.schema.json", "defaults_path": "defaults.json"}, payload)

    def test_semver_orders_numeric_components_and_prerelease_identifiers(self):
        values = ["2.0.9", "2.0.10", "2.1.0-alpha", "2.1.0-alpha.2",
                  "2.1.0-alpha.10", "2.1.0-beta", "2.1.0"]
        self.assertEqual(sorted(values, key=Version.parse), values)
        self.assertEqual(Version.parse("2.1.0+first"), Version.parse("2.1.0+second"))
        for invalid in ["2.01.0", "2.0", "2.0.0-01", "latest"]:
            with self.subTest(version=invalid), self.assertRaises(ValueError):
                Version.parse(invalid)

    def test_default_selection_keeps_unknown_legacy_versions_visible(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "plugins/release").mkdir(parents=True)
            for name, version in [("go", "2.0.10"), ("rust", "2.0.9"), ("cpp", None)]:
                directory = root / "plugins" / name
                directory.mkdir()
                if version:
                    (directory / "VERSION").write_text(version)
            catalog = {"schemaVersion": 1, "requiredPlugins": ["model-router"], "plugins": [
                {"logicalId": "model-router", "implementation": name, "sourceDir": "plugins/" + name,
                 "declaredVersion": {"go": "2.0.10", "rust": "2.0.9", "cpp": None}[name],
                 "releaseEligible": name != "cpp", **({"unmanagedReason": "legacy source has no VERSION"} if name == "cpp" else {})}
                for name in ["go", "rust", "cpp"]
            ]}
            path = root / "plugins/release/catalog.json"
            path.write_text(json.dumps(catalog))
            self.assertEqual(select_releases(root)[0].implementation, "go")
            self.assertIsNone(load_catalog(root)[1][2].version)
            catalog["requiredPlugins"].append("missing-plugin")
            path.write_text(json.dumps(catalog))
            with self.assertRaisesRegex(ValueError, "missing=missing-plugin"):
                select_releases(root)

    def test_equal_versions_do_not_silently_choose_different_implementations(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            (root / "plugins/release").mkdir(parents=True)
            entries = []
            for implementation in ["go", "cpp"]:
                directory = root / "plugins" / implementation
                directory.mkdir()
                (directory / "VERSION").write_text("2.0.3")
                entries.append({"logicalId": "mapper", "implementation": implementation,
                                "sourceDir": "plugins/" + implementation, "releaseEligible": True,
                                "declaredVersion": "2.0.3"})
            (root / "plugins/release/catalog.json").write_text(json.dumps(
                {"schemaVersion": 1, "requiredPlugins": ["mapper"], "plugins": entries}))
            with self.assertRaisesRegex(ValueError, "same-version"):
                select_releases(root)
