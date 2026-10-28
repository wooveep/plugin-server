import copy
import unittest

import yaml

from artifact_contract import ArtifactError, check_shape, config_schemas, digest, reject_credentials
from mf_canonical import parse_json
from plugin_schema_conversion import convert_spec, encode_yaml_value, validate_extension


def metadata_bytes(global_schema, route_schema=None):
    spec = {"phase": "default", "priority": 1,
            "configSchema": {"openAPIV3Schema": global_schema}}
    if route_schema is not None:
        spec["routeConfigSchema"] = {"openAPIV3Schema": route_schema}
    return yaml.safe_dump({"apiVersion": "1.0.0", "info": {"name": "sensitive-demo", "version": "1.0.0"},
                           "spec": spec}, sort_keys=False).encode()


def validated_schemas(converted):
    metadata = converted["metadata"]
    descriptor = {"plugin_name": metadata["info"]["name"], "plugin_version": metadata["info"]["version"],
                  "config_schema_path": "config.schema.json",
                  "files": [{"path": name, "role": "config_schema"} for name in converted["schemas"]]}
    return config_schemas(descriptor, converted["schemas"])


class SensitiveSchemaConversionTests(unittest.TestCase):
    def test_sensitive_annotations_preserve_scopes_and_write_only(self):
        secret = {"type": "string", "format": "password", "x-sensitive": True, "writeOnly": True}
        global_schema = {"type": "object", "additionalProperties": False,
                         "properties": {"secret/~token": secret, "expression": {"type": "string", "format": "regex"}}}
        route_schema = {"type": "object", "additionalProperties": False,
                        "properties": {"credential": secret, "label": {"type": "string", "x-sensitive": False}}}
        raw = metadata_bytes(global_schema, route_schema)
        converted = convert_spec(raw)
        self.assertEqual(converted, convert_spec(raw))
        self.assertEqual(converted["source_spec_digest"], digest(raw))
        for scope, pointer in (("global", "/properties/secret~1~0token"), ("route", "/properties/credential")):
            self.assertEqual(converted["metadata"]["annotations"][scope][pointer],
                             {"x-sensitive": True, "format": "password"})
        self.assertEqual(converted["metadata"]["annotations"]["route"]["/properties/label"], {"x-sensitive": False})
        schema = parse_json(converted["schemas"]["config.schema.json"])
        self.assertEqual(schema["properties"]["secret/~token"], {"type": "string", "writeOnly": True})
        self.assertEqual(schema["properties"]["expression"]["format"], "regex")
        schemas, registry = validated_schemas(converted)
        route = schemas["urn:modelfusion:payload:route.schema.json"]
        check_shape({"credential": "runtime-value", "label": "public"}, route, registry, "test")
        with self.assertRaises(ArtifactError):
            check_shape({"credential": "runtime-value", "extra": True}, route, registry, "test")

    def test_unknown_annotations_formats_and_bad_marker_types_still_reject(self):
        for field in ({"type": "string", "x-unknown": True},
                      {"type": "string", "x-sensitive": "true"},
                      {"type": "object", "format": "password"}):
            with self.subTest(field=field), self.assertRaises(ArtifactError) as raised:
                convert_spec(metadata_bytes({"type": "object", "properties": {"opaque": field}}))
            self.assertEqual(raised.exception.code, "unsupported_schema_conversion")
        converted = convert_spec(metadata_bytes({"type": "object", "properties": {"opaque": {"type": "string", "format": "unknown-art-format"}}}))
        with self.assertRaises(ArtifactError) as raised:
            validated_schemas(converted)
        self.assertEqual(raised.exception.code, "ART_CONFIG_SCHEMA")
        valid = convert_spec(metadata_bytes({"type": "object", "properties": {"opaque": {"type": "string", "x-sensitive": True}}}))["metadata"]
        for extra in ({"x-future": True}, {"format": "uri"}):
            changed = copy.deepcopy(valid)
            changed["annotations"]["global"]["/properties/opaque"].update(extra)
            with self.subTest(extra=extra), self.assertRaises(ArtifactError) as raised:
                validate_extension(changed)
            self.assertEqual(raised.exception.code, "unsupported_schema_conversion")
        changed = convert_spec(metadata_bytes({"type": "object", "properties": {"opaque": {"type": "string"}}}))
        schema = parse_json(changed["schemas"]["config.schema.json"])
        schema["properties"]["opaque"]["x-sensitive"] = True
        changed["schemas"]["config.schema.json"] = encode_yaml_value(schema)
        with self.assertRaises(ArtifactError) as raised:
            validated_schemas(changed)
        self.assertEqual(raised.exception.code, "ART_CONFIG_SCHEMA")

    def test_sensitive_defaults_and_examples_never_enter_converted_payload(self):
        sentinel = "test-only-value-that-must-not-be-exported"
        for name, marker in (("opaque", {"x-sensitive": True}), ("opaque", {"writeOnly": True}),
                             ("opaque", {"format": "password"}), ("password", {})):
            for keyword in ("default", "example", "examples"):
                field = {"type": "string", **marker, keyword: [sentinel] if keyword == "examples" else sentinel}
                with self.subTest(name=name, marker=marker, keyword=keyword), self.assertRaises(ArtifactError) as raised:
                    convert_spec(metadata_bytes({"type": "object", "properties": {name: field}}))
                self.assertEqual(raised.exception.code, "ART_SECRET")
                self.assertNotIn(sentinel, str(raised.exception))
        for keyword in ("default", "example", "examples"):
            for name in ("password", "bearer_token", "auth_token"):
                value = {"connection": {name: sentinel}}
                schema = {"type": "object", "properties": {"opaque": {"type": "string", "x-sensitive": True}},
                          keyword: [value] if keyword == "examples" else value}
                with self.subTest(keyword=keyword, name=name), self.assertRaises(ArtifactError) as raised:
                    convert_spec(metadata_bytes(schema))
                self.assertEqual(raised.exception.code, "ART_SECRET")
                self.assertNotIn(sentinel, str(raised.exception))
        # No retroactive marker contract is inferred for unmarked legacy scopes.
        legacy_sample = {"password": "legacy-placeholder"}
        legacy = convert_spec(metadata_bytes(
            {"type": "object", "example": legacy_sample},
            {"type": "object", "properties": {"opaque": {"type": "string", "x-sensitive": True}}}))
        self.assertEqual(parse_json(legacy["schemas"]["config.schema.json"])["examples"], [legacy_sample])
        for name in ("bearer_token", "auth_token"):
            with self.assertRaises(ArtifactError):
                reject_credentials({name: sentinel})
            reject_credentials({name: "${CONTROLLED_TOKEN}"})
        converted = convert_spec(metadata_bytes({"type": "object", "properties": {"note": {"type": "string", "x-sensitive": False, "default": "public"}}}))
        self.assertEqual(parse_json(converted["schemas"]["config.schema.json"])["properties"]["note"]["default"], "public")

    def test_sensitive_annotations_keep_nested_schema_positions(self):
        schema = {"type": "object", "allOf": [{"properties": {"opaque": {"type": "string", "format": "password", "writeOnly": True}}}],
                  "propertyNames": {"type": "string", "x-sensitive": False}}
        converted = convert_spec(metadata_bytes(schema))
        self.assertEqual(converted["metadata"]["annotations"]["global"]["/allOf/0/properties/opaque"], {"format": "password"})
        self.assertEqual(converted["metadata"]["annotations"]["global"]["/propertyNames"], {"x-sensitive": False})
        validated_schemas(converted)


if __name__ == "__main__":
    unittest.main()
