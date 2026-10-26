import copy
import gzip
import io
import json
import tarfile
import unittest
from dataclasses import replace
from pathlib import Path

from artifact_contract import (ValidationPolicy, digest, json_bytes,
                               validate_document, validate_package)
from artifact_packaging import build_archive
from mf_canonical import ContractError, canonicalize

ROOT = Path(__file__).resolve().parents[1]


def example_payload():
    return {"plugin.wasm": b"\0asm\1\0\0\0", "config.schema.json":
            b'{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}',
            "defaults.json": b"{}", "docs/README.md": b"Test package.\n"}


def descriptor(payload=None, version="1.0.0"):
    payload = payload or example_payload()
    roles = {"plugin.wasm": "wasm", "config.schema.json": "config_schema", "defaults.json": "defaults", "docs/README.md": "documentation"}
    return {"canonicalization_version": "artifact-json.v1", "schema_version": "artifact.v1",
            "plugin_name": "test-plugin", "plugin_version": version,
            "files": [{"path": name, "size_bytes": str(len(raw)), "sha256": digest(raw), "role": roles.get(name, "config_schema")} for name, raw in sorted(payload.items())],
            "wasm_path": "plugin.wasm", "config_schema_path": "config.schema.json", "defaults_path": "defaults.json",
            "documentation_paths": ["docs/README.md"], "environment_refs": [],
            "source": {"kind": "legacy_unknown", "verification": "unknown"}}


def example_archive(payload=None, version="1.0.0"):
    payload = payload or example_payload()
    return build_archive(descriptor(payload, version), payload)


class ArtifactContractTests(unittest.TestCase):
    def assert_code(self, code, action):
        with self.assertRaises(ContractError) as raised:
            action()
        self.assertEqual(raised.exception.code, code)

    def test_frozen_artifact_projection_goldens(self):
        for name in ("descriptor-valid", "descriptor-reordered"):
            root = ROOT / "contracts/fixtures/v1/artifact/canonical"
            result = validate_document((root / (name + ".projection.json")).read_bytes())
            self.assertEqual(result["canonical_bytes"], (root / "base.json").read_bytes())
            self.assertEqual(result["content_digest"], json.loads((root / (name + ".canonical.json")).read_bytes())["expected_digest"])

    def test_public_canonical_exact_numbers_unicode_and_control_bytes(self):
        raw = '{"😀":1.20e1,"\ue000":1e-3,"control":"\\b\\n\\r\\t","z":-0.0,"s":"001"}'.encode()
        expected = '{"control":"\\u0008\\u000a\\u000d\\u0009","s":"001","z":0,"\ue000":0.001,"😀":12}'.encode()
        self.assertEqual(canonicalize(raw), expected)
        self.assertNotEqual(canonicalize('"é"'.encode()), canonicalize('"é"'.encode()))
        for raw, code in ((b'{"a":1,"a":2}', "E_DUPLICATE_KEY"), (b'"\\ud800"', "E_SURROGATE"), (b'NaN', "E_JSON"), (b'1e1000000', "E_LIMIT"), (b'"\xff"', "E_UTF8")):
            with self.subTest(raw=raw):
                self.assert_code(code, lambda: canonicalize(raw))

    def test_full_package_and_compression_independent_identity(self):
        raw = example_archive()
        different = gzip.compress(gzip.decompress(raw), mtime=123456)
        first, second = validate_package(raw), validate_package(different)
        self.assertEqual(first.content_digest, second.content_digest)
        self.assertNotEqual(first.archive_digest, second.archive_digest)
        copy = first.descriptor
        copy["plugin_version"] = "9.0.0"
        self.assertEqual(first.descriptor["plugin_version"], "1.0.0")
        with self.assertRaises(TypeError):
            first.payload["plugin.wasm"] = b"invalid"

    def test_schema_docs_defaults_and_source_all_change_identity(self):
        original = validate_package(example_archive()).content_digest
        for name, replacement in (("docs/README.md", b"Changed docs"), ("defaults.json", b"{ }"),
                                  ("config.schema.json", example_payload()["config.schema.json"] + b"\n")):
            payload = example_payload()
            payload[name] = replacement
            self.assertNotEqual(validate_package(example_archive(payload)).content_digest, original)
        desc = descriptor()
        desc["source"] = {"kind": "local_build", "verification": "unknown", "repo": "repo", "commit": "a" * 40,
                          "dirty_digest": "sha256:" + "0" * 64, "build_inputs_digest": "sha256:" + "1" * 64}
        self.assertNotEqual(validate_package(build_archive(desc, example_payload())).content_digest, original)

    def test_verified_source_requires_trusted_independent_policy(self):
        desc = descriptor()
        desc["source"] = {"kind": "oci_import", "verification": "verified", "original_ref": "registry:9980/demo@sha256:" + "a" * 64,
                          "manifest_digest": "sha256:" + "a" * 64, "layer_digest": "sha256:" + "b" * 64}
        raw = build_archive(desc, example_payload())
        self.assert_code("ART_SOURCE_EVIDENCE", lambda: validate_package(raw))
        content_digest = validate_document(json_bytes(desc))["content_digest"]
        policy = ValidationPolicy(provenance_receipts=frozenset(((digest(json_bytes(desc["source"])), digest(example_payload()["plugin.wasm"]), content_digest),)))
        self.assertEqual(validate_package(raw, policy).descriptor["source"]["verification"], "verified")
        changed = example_payload()
        changed["docs/README.md"] = b"Source receipt must not approve changed output"
        changed_desc = descriptor(changed)
        changed_desc["source"] = desc["source"]
        self.assert_code("ART_SOURCE_EVIDENCE", lambda: validate_package(build_archive(changed_desc, changed), policy))

    def test_duplicate_roles_paths_environment_and_server_fields_rejected(self):
        for expected, mutate in (
                ("ART_DUPLICATE_PATH", lambda d: d["files"].append(copy.deepcopy(d["files"][0]))),
                ("ART_ROLE_REFERENCE", lambda d: d.update(wasm_path="defaults.json")),
                ("ART_DESCRIPTOR", lambda d: d.update(artifact_id="client-supplied")),
                ("ART_DUPLICATE_ENV", lambda d: d.update(environment_refs=[{"name": "A", "kind": "setting", "required": False}] * 2)),
                ("ART_DESCRIPTOR", lambda d: d.update(plugin_version="latest"))):
            desc = descriptor()
            mutate(desc)
            with self.subTest(expected=expected):
                self.assert_code(expected, lambda: validate_document(json_bytes(desc)))

    def test_missing_extra_wrong_hash_and_wrong_length_rejected(self):
        payload = example_payload()
        desc = descriptor(payload)
        payload["docs/README.md"] = b"x"
        self.assert_code("ART_FILE_SIZE", lambda: validate_package(build_archive(desc, payload)))
        payload["docs/README.md"] = b"X" * int(desc["files"][2]["size_bytes"]) if desc["files"][2]["path"] == "docs/README.md" else b"X" * len(example_payload()["docs/README.md"])
        self.assert_code("ART_FILE_DIGEST", lambda: validate_package(build_archive(desc, payload)))
        payload = example_payload()
        payload["hidden.txt"] = b"hidden"
        self.assert_code("ART_FILE_SET", lambda: validate_package(build_archive(desc, payload)))

    def test_bounded_archive_and_invalid_wasm_rejected(self):
        raw = example_archive()
        self.assert_code("ART_LIMIT", lambda: validate_package(raw, replace(ValidationPolicy(), max_archive_bytes=len(raw) - 1)))
        self.assert_code("ART_LIMIT", lambda: validate_package(raw, replace(ValidationPolicy(), max_tar_bytes=100)))
        self.assert_code("ART_ARCHIVE", lambda: validate_package(raw[:-3]))
        for wasm in (b"not-wasm", b"\0asm\1\0\0\0\1\xff", b"\0asm\1\0\0\0\xff\0"):
            payload = example_payload()
            payload["plugin.wasm"] = wasm
            self.assert_code("ART_WASM", lambda: validate_package(example_archive(payload)))

    def test_unsafe_tar_member_kinds_paths_and_hidden_trailing_data(self):
        for name, kind, mode, code in (("../escape", tarfile.REGTYPE, 0o644, "ART_ARCHIVE_PATH"),
                                      ("/escape", tarfile.REGTYPE, 0o644, "ART_ARCHIVE_PATH"),
                                      ("file", tarfile.SYMTYPE, 0o644, "ART_ARCHIVE_LINK"),
                                      ("file", tarfile.LNKTYPE, 0o644, "ART_ARCHIVE_LINK"),
                                      ("run.sh", tarfile.REGTYPE, 0o644, "ART_ARCHIVE_EXECUTABLE"),
                                      ("run", tarfile.REGTYPE, 0o755, "ART_ARCHIVE_EXECUTABLE")):
            out = io.BytesIO()
            with tarfile.open(fileobj=out, mode="w") as archive:
                member = tarfile.TarInfo(name)
                member.type, member.mode = kind, mode
                archive.addfile(member, io.BytesIO(b""))
            self.assert_code(code, lambda: validate_package(gzip.compress(out.getvalue())))
        tar = bytearray(gzip.decompress(example_archive()))
        tar[-1200] = 1
        self.assert_code("ART_ARCHIVE", lambda: validate_package(gzip.compress(tar)))

    def test_defaults_and_unreachable_schema_references_are_checked(self):
        payload = example_payload()
        payload["defaults.json"] = b'{"unexpected":true}'
        self.assert_code("ART_DEFAULTS", lambda: validate_package(example_archive(payload)))
        for ref in ("https://example.com/schema", "missing.json#/$defs/x", "#/missing"):
            schema = json.loads(example_payload()["config.schema.json"])
            schema["$defs"] = {"unused": {"$ref": ref}}
            payload = example_payload()
            payload["config.schema.json"] = json_bytes(schema)
            self.assert_code("ART_CONFIG_REFERENCE", lambda: validate_package(example_archive(payload)))

    def test_local_schema_closure_and_anchor(self):
        payload = example_payload()
        payload["config.schema.json"] = b'{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"rev":{"$ref":"types.json#integer"}}}'
        payload["types.json"] = b'{"$schema":"https://json-schema.org/draft/2020-12/schema","$anchor":"integer","type":"integer"}'
        payload["defaults.json"] = b'{"rev":9007199254740993}'
        validate_package(example_archive(payload))
        payload["defaults.json"] = b'{"rev":1.1}'
        self.assert_code("ART_DEFAULTS", lambda: validate_package(example_archive(payload)))

    def test_reference_into_instance_data_is_not_a_schema_location(self):
        for target in ("#/default", "#/examples/0"):
            payload = example_payload()
            payload["config.schema.json"] = json_bytes({"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": target,
                                                        "default": {"type": "string", "minimum": "bad"}, "examples": [{"typo": "ignored"}]})
            payload["defaults.json"] = b'"text"'
            self.assert_code("ART_CONFIG_REFERENCE", lambda: validate_package(example_archive(payload)))

    def test_multiple_of_uses_exact_math_beyond_decimal_context_precision(self):
        payload = example_payload()
        payload["config.schema.json"] = b'{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"number","multipleOf":0.1}'
        payload["defaults.json"] = b'9999999999999999999999999999.1'
        validate_package(example_archive(payload))
        payload["defaults.json"] = b'9999999999999999999999999999.11'
        self.assert_code("ART_DEFAULTS", lambda: validate_package(example_archive(payload)))

    def test_percent_encoded_schema_pointer_decodes_once(self):
        payload = example_payload()
        payload["config.schema.json"] = b'{"$schema":"https://json-schema.org/draft/2020-12/schema","$ref":"#/$defs/percent%25key","$defs":{"percent%key":{"type":"object"}}}'
        validate_package(example_archive(payload))

    def test_nonterminating_reference_is_a_protocol_rejection(self):
        payload = example_payload()
        payload["config.schema.json"] = b'{"$schema":"https://json-schema.org/draft/2020-12/schema","$ref":"#"}'
        self.assert_code("ART_CONFIG_REFERENCE", lambda: validate_package(example_archive(payload)))

    def test_literal_secrets_and_unapproved_environment_references(self):
        payload = example_payload()
        schema = json.loads(payload["config.schema.json"])
        schema["additionalProperties"] = True
        payload["config.schema.json"] = json_bytes(schema)
        payload["defaults.json"] = b'{"password":"real-credential-value"}'
        self.assert_code("ART_SECRET", lambda: validate_package(example_archive(payload)))
        desc = descriptor()
        desc["environment_refs"] = [{"name": "CONSOLE_ENDPOINT", "kind": "endpoint", "required": True}]
        raw = build_archive(desc, example_payload())
        self.assert_code("ART_ENVIRONMENT_REF", lambda: validate_package(raw))
        validate_package(raw, ValidationPolicy(allowed_environment_refs=(("CONSOLE_ENDPOINT", "endpoint", True),)))


if __name__ == "__main__":
    unittest.main()
