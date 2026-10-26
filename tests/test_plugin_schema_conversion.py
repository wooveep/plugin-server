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
