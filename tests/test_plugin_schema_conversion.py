import unittest
from pathlib import Path

from artifact_contract import ArtifactError, check_shape, config_schemas, validate_package
from artifact_packaging import package_plugin
from plugin_schema_conversion import convert_spec
from mf_canonical import parse_json

ROOT = Path(__file__).parent / "fixtures/schema-conversion"


class SchemaConversionTests(unittest.TestCase):
    def test_real_key_auth_and_billing_preserve_complete_scope_metadata(self):
        for name in ("key-auth", "ai-billing"):
            raw = (ROOT / (name + ".yaml")).read_bytes()
            first, second = convert_spec(raw), convert_spec(raw)
            self.assertEqual(first, second)
            meta = first["metadata"]
            self.assertEqual(meta["info"]["name"], name)
            self.assertTrue(meta["phase"])
            self.assertEqual(set(first["schemas"]), {"config.schema.json", "route.schema.json"})
            self.assertTrue(meta["annotations"]["global"])
            if name == "ai-billing":
                self.assertEqual(meta["external_resources"]["route"]["x-higress-external-resource"]["domainPath"], "redis_stream.service_name")
            package = package_plugin(spec=raw, wasm=b"\0asm\1\0\0\0", defaults=b"{}", documentation={"docs/README.md": b"Documented test"}, source={"kind": "legacy_unknown", "verification": "unknown"})
            validated = validate_package(package)
            schemas, registry = config_schemas(validated.descriptor, validated.payload)
            route = schemas["urn:modelfusion:payload:route.schema.json"]
            if name == "key-auth":
                check_shape({"allow": []}, route, registry, "test")
                with self.assertRaises(ArtifactError):
                    check_shape({}, route, registry, "test")

    def test_http_callout_metadata_preserves_only_the_declared_http_transport(self):
        import copy
        import yaml
        source = {
            "apiVersion": "1.0.0", "info": {"name": "demo", "version": "1.0.0"},
            "spec": {"phase": "default", "priority": 1, "configSchema": {
                "x-higress-external-resource": {
                    "kind": "http-callout", "registryType": "dns", "domainPath": "console.service_name",
                    "portPath": "console.service_port", "projectedNamePath": "console.service_name", "protocol": "http"},
                "openAPIV3Schema": {"type": "object"}}}}
        encoded = lambda value: yaml.safe_dump(value, sort_keys=False).encode()
        metadata = convert_spec(encoded(source))["metadata"]
        self.assertEqual(metadata["external_resources"]["global"]["x-higress-external-resource"],
                         source["spec"]["configSchema"]["x-higress-external-resource"])
        for field, invalid in (("protocol", "https"), ("protocol", None), ("kind", "unknown"), ("invented", True)):
            candidate = copy.deepcopy(source)
            candidate["spec"]["configSchema"]["x-higress-external-resource"][field] = invalid
            with self.subTest(field=field, value=invalid), self.assertRaises(ArtifactError) as error:
                convert_spec(encoded(candidate))
            self.assertEqual(error.exception.code, "unsupported_schema_conversion")

    def test_unknown_or_lossy_input_fails_closed(self):
        base = b'apiVersion: 1.0.0\ninfo: {name: demo, version: 1.0.0}\nspec:\n  phase: default\n  priority: 1\n  configSchema:\n    openAPIV3Schema: '
        for schema in (b'{type: object, x-unknown: true}', b'{type: string, nullable: true, enum: [x]}',
                       b'{type: string, $id: external}', b'{type: number, exclusiveMinimum: true}',
                       b'{type: string, type: integer}'):
            with self.subTest(schema=schema), self.assertRaises(ArtifactError) as error:
                convert_spec(base + schema)
            self.assertEqual(error.exception.code, "unsupported_schema_conversion")

    def test_exact_decimal_bounds_and_nullable_conversion(self):
        raw = b'apiVersion: 1.0.0\ninfo: {name: demo, version: 1.0.0}\nspec:\n  phase: default\n  priority: 1\n  configSchema:\n    openAPIV3Schema:\n      type: object\n      properties:\n        number: {type: number, minimum: 0.00000000000000000001, exclusiveMinimum: true}\n        text: {type: string, nullable: true}\n'
        converted = convert_spec(raw)
        schema = parse_json(converted["schemas"]["config.schema.json"])
        self.assertEqual(schema["properties"]["text"]["type"], ["string", "null"])
        self.assertIn(b'"exclusiveMinimum":0.00000000000000000001', converted["schemas"]["config.schema.json"])
        self.assertEqual(converted, convert_spec(raw))

    def test_schema_subpositions_share_one_recursive_keyword_table(self):
        raw = b'apiVersion: 1.0.0\ninfo: {name: demo, version: 1.0.0}\nspec:\n  phase: default\n  priority: 1\n  configSchema:\n    openAPIV3Schema:\n      type: object\n      propertyNames: {type: string, x-description-i18n: {en-US: Property}}\n      unevaluatedProperties: {type: string, x-title-i18n: {en-US: Value}}\n'
        converted = convert_spec(raw)
        self.assertEqual(converted["metadata"]["annotations"]["global"]["/propertyNames"]["x-description-i18n"]["en-US"], "Property")
        package_plugin(spec=raw, wasm=b"\0asm\1\0\0\0", defaults=b"{}", documentation={"docs/README.md": b"Docs"}, source={"kind": "legacy_unknown", "verification": "unknown"})

    def test_non_json_yaml_numbers_have_explicit_conversion_rejection(self):
        base = b'apiVersion: 1.0.0\ninfo: {name: demo, version: 1.0.0}\nspec:\n  phase: default\n  priority: 1\n  configSchema:\n    openAPIV3Schema: {type: number, minimum: '
        for value in (b'.inf', b'-.inf', b'.nan'):
            with self.subTest(value=value), self.assertRaises(ArtifactError) as raised:
                convert_spec(base + value + b'}')
            self.assertEqual(raised.exception.code, "unsupported_schema_conversion")


def original_ai_routing():
    # The existing SDK capability contract, independent of native media D0.
    operation = {
        "id": "openai.responses_to_chat_completions", "clientMethod": "POST", "clientPath": "/v1/responses",
        "upstreamMethod": "POST", "upstreamPath": "/v1/chat/completions",
        "requestTransform": "responses_to_chat_completions", "responseTransform": "chat_completions_to_responses",
        "streamingBehavior": "compatible",
    }
    return {
        "minimumPluginVersion": "1.0.2", "protocolModes": ["auto", "passthrough"],
        "pathScopes": ["legacy_suffixes", "bound_target"],
        "adapters": [
            {"id": adapter_id, "provider": provider, "capability": "openai/v1/responses",
             "operations": [dict(operation)], "usageProfile": "openai.responses.v1", "minimumPluginVersion": "1.0.2"}
            for adapter_id, provider in (("qwen.responses_to_chat_completions", "qwen"), ("responses.to_chat_completions", "*"))
        ],
    }


def routing_spec(routing):
    return {"apiVersion": "1.0.0", "info": {"name": "ai-proxy", "version": "2.0.5"},
            "spec": {"phase": "default", "priority": 100, "x-modelfusion-ai-routing": routing,
                     "configSchema": {"openAPIV3Schema": {"type": "object"}},
                     "routeConfigSchema": {"openAPIV3Schema": {"type": "object"}}}}


class AiRoutingMetadataTests(unittest.TestCase):
    def test_original_routing_contract_survives_conversion_package_and_manifest_readback(self):
        import copy
        import tempfile
        import yaml
        from artifact_contract import digest, validate_document
        from artifact_storage import ArtifactStore
        routing = original_ai_routing()
        raw = yaml.safe_dump(routing_spec(routing), sort_keys=False).encode()
        converted = convert_spec(raw)
        self.assertEqual(converted["metadata"]["ai_routing"], routing)
        global_schema = parse_json(converted["schemas"]["config.schema.json"])
        self.assertEqual(global_schema["x-higress"]["ai_routing"], routing)
        self.assertNotIn("x-higress", parse_json(converted["schemas"]["route.schema.json"]))
        args = {"spec": raw, "wasm": b"\0asm\1\0\0\0", "defaults": b"{}",
                "documentation": {"docs/README.md": b"Metadata contract fixture; no gateway support attestation"},
                "source": {"kind": "legacy_unknown", "verification": "unknown"}}
        package = package_plugin(**args)
        validated = validate_package(package)
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "plugins"
            receipt = ArtifactStore(root).commit("original-routing", package)
            manifest = receipt["manifest"]
            validate_document(__import__("artifact_contract").json_bytes(manifest), "manifest")
            stored = ArtifactStore(root).read_file("ai-proxy", "2.0.5", "config.schema.json")
            self.assertEqual(stored, converted["schemas"]["config.schema.json"])
            member = next(file for file in manifest["files"] if file["path"] == "config.schema.json")
            self.assertEqual(member["sha256"], digest(stored))
            self.assertEqual(member["size_bytes"], str(len(stored)))
            self.assertEqual(manifest["content_digest"], validated.content_digest)
            self.assertEqual(ArtifactStore(root).manifest("ai-proxy", "2.0.5"), manifest)
            self.assertNotIn("ai_routing", manifest)
        changed = copy.deepcopy(routing)
        changed["pathScopes"] = ["legacy_suffixes"]
        args["spec"] = yaml.safe_dump(routing_spec(changed), sort_keys=False).encode()
        self.assertNotEqual(validated.content_digest, validate_package(package_plugin(**args)).content_digest)
        # A valid capability edit still needs a new original member/content hash.
        # ValidatedPackage retains archive members including package.json;
        # the builder receives only descriptor-listed payload and owns that reserved file.
        payload = {entry["path"]: validated.payload[entry["path"]] for entry in validated.descriptor["files"]}
        original_schema = payload["config.schema.json"]
        payload["config.schema.json"] = original_schema.replace(b"1.0.2", b"1.0.3")
        self.assertNotEqual(payload["config.schema.json"], original_schema)
        self.assertEqual(len(payload["config.schema.json"]), len(original_schema))
        from artifact_packaging import build_archive
        tampered_archive = build_archive(validated.descriptor, payload)
        with self.assertRaises(ArtifactError) as raised:
            validate_package(tampered_archive)
        self.assertEqual(raised.exception.code, "ART_FILE_DIGEST")

    def test_only_the_existing_closed_routing_extension_is_accepted(self):
        import yaml
        for variant in ("unknown_spec", "unknown_contract", "unknown_adapter", "unknown_operation", "missing_minimum",
                        "bad_minimum", "bad_scope", "bad_mode", "missing_usage", "invalid_transform", "null"):
            source = routing_spec(original_ai_routing())
            contract = source["spec"]["x-modelfusion-ai-routing"]
            if variant == "unknown_spec":
                source["spec"]["x-unreviewed-capability"] = True
            elif variant == "unknown_contract":
                contract["invented"] = True
            elif variant == "unknown_adapter":
                contract["adapters"][0]["invented"] = True
            elif variant == "unknown_operation":
                contract["adapters"][0]["operations"][0]["invented"] = True
            elif variant == "missing_minimum":
                contract.pop("minimumPluginVersion")
            elif variant == "bad_minimum":
                contract["minimumPluginVersion"] = "latest"
            elif variant == "bad_scope":
                contract["pathScopes"] = ["managed_operation"]
            elif variant == "bad_mode":
                contract["protocolModes"] = ["qwen30_media"]
            elif variant == "missing_usage":
                contract["adapters"][0].pop("usageProfile")
            elif variant == "invalid_transform":
                contract["adapters"][0]["operations"][0]["requestTransform"] = {}
            else:
                source["spec"]["x-modelfusion-ai-routing"] = None
            with self.subTest(variant=variant), self.assertRaises(ArtifactError) as raised:
                convert_spec(yaml.safe_dump(source, sort_keys=False).encode())
            self.assertEqual(raised.exception.code, "unsupported_schema_conversion")
        legacy = routing_spec(original_ai_routing())
        legacy["spec"].pop("x-modelfusion-ai-routing")
        self.assertNotIn("ai_routing", convert_spec(yaml.safe_dump(legacy, sort_keys=False).encode())["metadata"])
