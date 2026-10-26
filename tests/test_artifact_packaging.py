import unittest
from pathlib import Path

from artifact_contract import validate_package
from artifact_packaging import package_plugin


class ArtifactPackagingTests(unittest.TestCase):
    def test_identical_inputs_produce_identical_complete_archive_bytes(self):
        inputs = dict(spec=(Path(__file__).parent / "fixtures/schema-conversion/key-auth.yaml").read_bytes(),
                      wasm=b"\0asm\1\0\0\0", defaults=b"{}", documentation={"docs/README.md": b"Documented"},
                      source={"kind": "legacy_unknown", "verification": "unknown"})
        first = package_plugin(**inputs)
        self.assertEqual(first, package_plugin(**inputs))
        validated = validate_package(first)
        self.assertEqual(len(validated.descriptor["files"]), 5)
        inputs["documentation"] = {"docs/README.md": b"New documented content"}
        self.assertNotEqual(validate_package(package_plugin(**inputs)).content_digest, validated.content_digest)
