"""Versioned, fail-closed conversion of Higress metadata to artifact payloads."""
from __future__ import annotations

import copy
import json
from decimal import Decimal
from functools import lru_cache
from importlib.resources import files as resource_files
from typing import Any

import yaml

from artifact_contract import (ArtifactError, DIALECT, SCHEMA_KEYWORDS, SCHEMA_MAP_KEYS, SCHEMA_SINGLE_KEYS, SCHEMA_ARRAY_KEYS, digest,
                               check_shape, contract_registry, registry_for)
from mf_canonical import canonicalize, parse_json, schema_numbers

VERSION = "higress-conversion.v1"
UI_KEYS = frozenset(("x-title-i18n", "x-description-i18n", "x-scope"))
SCHEMA_MAPS = frozenset(SCHEMA_MAP_KEYS)
SCHEMA_SINGLES = frozenset(SCHEMA_SINGLE_KEYS)
SCHEMA_ARRAYS = frozenset(SCHEMA_ARRAY_KEYS)


def unsupported(detail: str):
    raise ArtifactError("unsupported_schema_conversion", detail, "conversion")


class StrictLoader(yaml.SafeLoader):
    pass


def _mapping(loader, node):
    result = {}
    for key_node, value_node in node.value:
        key = loader.construct_object(key_node, deep=True)
        if not isinstance(key, str) or key in result:
            unsupported("duplicate/non-string YAML mapping key")
        result[key] = loader.construct_object(value_node, deep=True)
    return result


StrictLoader.add_constructor("tag:yaml.org,2002:map", _mapping)
# YAML non-decimal numbers/dates are not JSON. Aliases are permitted but recursive
# or oversized expansions are rejected by the bounded JSON canonicalizer below.
StrictLoader.add_constructor("tag:yaml.org,2002:float", lambda loader, node: Decimal(node.value))


def _json_text(value: Any) -> str:
    """Emit exact YAML decimal scalars; never round through a binary float."""
    if isinstance(value, Decimal):
        if not value.is_finite():
            unsupported("non-finite YAML number")
        return str(value)
    if isinstance(value, list):
        return "[" + ",".join(_json_text(v) for v in value) + "]"
    if isinstance(value, dict):
        return "{" + ",".join(json.dumps(k, ensure_ascii=False) + ":" + _json_text(v) for k, v in value.items()) + "}"
    return json.dumps(value, ensure_ascii=False, allow_nan=False)


def encode_yaml_value(value: Any) -> bytes:
    try:
        return canonicalize(_json_text(value).encode())
    except (ValueError, TypeError, RecursionError) as exc:
        unsupported("YAML value is not a bounded JSON value")
        raise AssertionError from exc


@lru_cache(maxsize=1)
def extension_schema():
    schema = schema_numbers(parse_json(resource_files("artifact_schemas").joinpath("higress-conversion.schema.json").read_bytes()))
    schemas, _ = contract_registry()
    return schema, registry_for({**schemas, schema["$id"]: schema})


def validate_extension(value: Any) -> None:
    schema, registry = extension_schema()
    check_shape(value, schema, registry, "unsupported_schema_conversion")


def _convert_schema(schema: Any, annotations: dict[str, Any], pointer: str = "") -> Any:
    if isinstance(schema, bool):
        return schema
    if not isinstance(schema, dict):
        unsupported("schema node must be an object or boolean")
    if set(schema) - SCHEMA_KEYWORDS - UI_KEYS - {"nullable", "example"}:
        unsupported("unrecognized schema keyword: " + ",".join(sorted(set(schema) - SCHEMA_KEYWORDS - UI_KEYS - {"nullable", "example"})))
    if "x-higress" in schema or "$id" in schema or "$schema" in schema:
        unsupported("nested resource/dialect declaration in OpenAPI schema")
    result = {}
    for key, value in schema.items():
        escaped = key.replace("~", "~0").replace("/", "~1")
        if key in UI_KEYS:
            annotations.setdefault(pointer, {})[key] = value
        elif key in SCHEMA_MAPS:
            if not isinstance(value, dict):
                unsupported("invalid schema map")
            result[key] = {name: _convert_schema(child, annotations, pointer + "/" + escaped + "/" + name.replace("~", "~0").replace("/", "~1")) for name, child in value.items()}
        elif key in SCHEMA_SINGLES:
            result[key] = _convert_schema(value, annotations, pointer + "/" + escaped)
        elif key in SCHEMA_ARRAYS:
            if not isinstance(value, list):
                unsupported("invalid schema array")
            result[key] = [_convert_schema(child, annotations, pointer + "/" + escaped + "/" + str(index)) for index, child in enumerate(value)]
        elif key == "example":
            if "examples" in schema:
                unsupported("ambiguous example/examples")
            result["examples"] = [value]
        elif key != "nullable":
            result[key] = copy.deepcopy(value)
    if "nullable" in schema:
        if not isinstance(schema["nullable"], bool):
            unsupported("nullable must be boolean")
        if schema["nullable"]:
            if not isinstance(schema.get("type"), str) or any(k in schema for k in ("$ref", "enum", "const", "allOf", "anyOf", "oneOf", "not")):
                unsupported("nullable with reference/composition/enum has ambiguous semantics")
            result["type"] = [schema["type"], "null"]
    for direction in ("Minimum", "Maximum"):
        exclusive = "exclusive" + direction
        bound = direction.lower()
        if isinstance(result.get(exclusive), bool):
            flag = result.pop(exclusive)
            if flag:
                if bound not in result:
                    unsupported("exclusive bound without bound value")
                result[exclusive] = result.pop(bound)
    return result


def convert_spec(raw: bytes) -> dict[str, Any]:
    if len(raw) > 1_048_576:
        unsupported("spec exceeds conversion budget")
    try:
        # Reject alias cycles before traversing/serializing and reject non-JSON YAML.
        parsed = yaml.load(raw.decode("utf-8"), Loader=StrictLoader)
        value = parse_json(encode_yaml_value(parsed))
    except (yaml.YAMLError, UnicodeError, ValueError, RecursionError) as exc:
        raise ArtifactError("unsupported_schema_conversion", "invalid YAML metadata", "conversion") from exc
    if not isinstance(value, dict) or set(value) != {"apiVersion", "info", "spec"} or value["apiVersion"] != "1.0.0":
        unsupported("unsupported metadata wrapper")
    info, spec = value["info"], value["spec"]
    if not isinstance(spec, dict) or set(spec) - {"phase", "priority", "configSchema", "routeConfigSchema", "externalResources"}:
        unsupported("unknown spec field")
    schemas = {}
    annotations = {"global": {}, "route": {}}
    external = {"global": {}, "route": {}}
    for scope, wrapper_key, path in (("global", "configSchema", "config.schema.json"), ("route", "routeConfigSchema", "route.schema.json")):
        if wrapper_key not in spec:
            if scope == "global":
                unsupported("global schema is required")
            continue
        wrapper = spec[wrapper_key]
        if not isinstance(wrapper, dict) or set(wrapper) - {"openAPIV3Schema", "x-higress-external-resource", "x-higress-external-resources"}:
            unsupported("unknown schema wrapper field")
        if "openAPIV3Schema" not in wrapper:
            unsupported("missing OpenAPI schema")
        if "x-higress-external-resource" in wrapper and "x-higress-external-resources" in wrapper:
            unsupported("ambiguous external resource declarations")
        external[scope] = {key: copy.deepcopy(item) for key, item in wrapper.items() if key != "openAPIV3Schema"}
        schema = _convert_schema(wrapper["openAPIV3Schema"], annotations[scope])
        if not isinstance(schema, dict):
            unsupported("top-level plugin schema must be an object")
        schemas[path] = {"$schema": DIALECT, **schema}
    if "externalResources" in spec:
        external["global"]["externalResources"] = copy.deepcopy(spec["externalResources"])
    extension = {
        "schema_version": VERSION, "converter_version": VERSION,
        "info": copy.deepcopy(info), "phase": spec.get("phase"), "priority": spec.get("priority"),
        "global_schema_path": "config.schema.json",
        "route_schema_path": "route.schema.json" if "route.schema.json" in schemas else None,
        "external_resources": external, "annotations": annotations, "source_spec_digest": digest(raw),
    }
    validate_extension(extension)
    schemas["config.schema.json"]["x-higress"] = extension
    return {"schemas": {name: encode_yaml_value(schema) for name, schema in schemas.items()},
            "metadata": extension, "source_spec_digest": digest(raw)}
