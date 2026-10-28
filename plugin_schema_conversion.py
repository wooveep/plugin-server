"""Versioned, fail-closed conversion of Higress metadata to artifact payloads."""
from __future__ import annotations

import copy
import json
import posixpath
import re
from decimal import Decimal, InvalidOperation
from functools import lru_cache
from importlib.resources import files as resource_files
from typing import Any
from urllib.parse import unquote, urldefrag

import yaml

from artifact_contract import (ArtifactError, DIALECT, SCHEMA_KEYWORDS, SCHEMA_MAP_KEYS, SCHEMA_SINGLE_KEYS, SCHEMA_ARRAY_KEYS, SECRET_KEYS, COMMON, digest,
                               check_shape, contract_registry, registry_for, reject_credentials, schema_nodes, schema_positions)
from mf_canonical import MAX_DEPTH, MAX_INPUT_BYTES, canonicalize, parse_json, schema_numbers

VERSION = "higress-conversion.v1"
UI_KEYS = frozenset(("x-title-i18n", "x-description-i18n", "x-scope", "x-sensitive"))
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
def _yaml_decimal(loader, node):
    try:
        value = Decimal(node.value)
    except InvalidOperation as exc:
        raise ArtifactError("unsupported_schema_conversion", "non-JSON YAML number", "conversion") from exc
    if not value.is_finite():
        unsupported("non-finite YAML number")
    return value


StrictLoader.add_constructor("tag:yaml.org,2002:float", _yaml_decimal)


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
    property_path = pointer.rsplit("/", 2)
    named_secret = (len(property_path) == 3 and property_path[-2] == "properties"
                    and property_path[-1].replace("~1", "/").replace("~0", "~").lower() in SECRET_KEYS)
    if (named_secret or schema.get("x-sensitive") is True or schema.get("writeOnly") is True
            or schema.get("format") == "password") and any(key in schema for key in ("default", "example", "examples")):
        raise ArtifactError("ART_SECRET", "sensitive schema fields cannot contain defaults or examples", "conversion", pointer)
    result = {}
    for key, value in schema.items():
        escaped = key.replace("~", "~0").replace("/", "~1")
        if key in UI_KEYS:
            annotations.setdefault(pointer, {})[key] = value
        elif key == "format" and value == "password":
            if schema.get("type") != "string":
                unsupported("password UI format requires string type")
            # OpenAPI's password widget is display metadata, not a validation
            # format. Preserve it without widening the JSON Schema format set.
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


def _reject_marked_value(value: Any) -> None:
    if isinstance(value, dict):
        for item in value.values():
            _reject_marked_value(item)
    elif isinstance(value, list):
        for item in value:
            _reject_marked_value(item)
    else:
        # Reuse the original literal/reference policy without guessing the
        # credential's actual property name or exposing its value in errors.
        reject_credentials({"password": value})


def _check_sensitive_samples(roots: dict[str, Any], samples: list[tuple[str, Any, Any]]) -> None:
    # Only declared schema positions are reference targets. No retrieval callback,
    # instance-data traversal as schemas, or inferred cross-scope inheritance.
    common_schemas, _ = contract_registry()
    common = {COMMON: common_schemas[COMMON]}
    documents = {**common, **roots}
    locations = {name: dict(schema_positions(root)) for name, root in documents.items()}
    remaining = MAX_INPUT_BYTES

    def resolve(reference: Any, document: str) -> tuple[str, Any]:
        if not isinstance(reference, str):
            unsupported("invalid sensitive sample schema reference")
        base, fragment = urldefrag(reference)
        try:
            if re.search(r"%(?![0-9a-fA-F]{2})", base + fragment):
                unsupported("invalid sensitive sample schema reference")
            base, fragment = unquote(base, errors="strict"), unquote(fragment, errors="strict")
        except UnicodeError:
            unsupported("invalid sensitive sample schema reference")
        target = document if not base else base if base in common else posixpath.normpath(base)
        if target not in locations or (fragment.startswith("/") and re.search(r"~(?![01])", fragment)):
            unsupported("unresolved sensitive sample schema reference")
        matches = ([locations[target][fragment]] if fragment in locations[target] else
                   [node for node in locations[target].values()
                    if fragment and not fragment.startswith("/") and isinstance(node, dict)
                    and node.get("$anchor") == fragment])
        if len(matches) != 1:
            unsupported("unresolved sensitive sample schema reference")
        return target, matches[0]

    marked_cache = {}

    def marked(root: Any, document: str) -> bool:
        nonlocal remaining
        root_token = (document, id(root))
        if root_token in marked_cache:
            return marked_cache[root_token]
        pending, seen = [(document, root)], set()
        while pending:
            name, value = pending.pop()
            for node in schema_nodes(value):
                token = (name, id(node))
                if token in seen:
                    continue
                seen.add(token)
                remaining -= 1
                if remaining < 0:
                    unsupported("sensitive sample traversal exceeds conversion budget")
                if (node.get("x-sensitive") is True or node.get("writeOnly") is True
                        or node.get("format") == "password"):
                    marked_cache[root_token] = True
                    return True
                if "$ref" in node:
                    pending.append(resolve(node["$ref"], name))
        marked_cache[root_token] = False
        return False

    def visit(schema: Any, sample: Any, document: str, seen: set, depth: int = 0) -> None:
        nonlocal remaining
        if not isinstance(schema, dict) or (document, id(schema), id(sample)) in seen:
            return
        seen.add((document, id(schema), id(sample)))
        remaining -= 1
        if remaining < 0 or depth > MAX_DEPTH:
            unsupported("sensitive sample traversal exceeds conversion budget")
        if (schema.get("x-sensitive") is True or schema.get("writeOnly") is True
                or schema.get("format") == "password"):
            _reject_marked_value(sample)
        for key in ("if", "then", "else", "not", "unevaluatedProperties", "unevaluatedItems", "propertyNames", "contentSchema"):
            if key in schema and marked(schema[key], document):
                unsupported("sensitive sample has an ambiguous instance position")

        def child(node: Any, value: Any, name: str = document) -> None:
            visit(node, value, name, seen, depth + 1)

        if "$ref" in schema:
            target, node = resolve(schema["$ref"], document)
            child(node, sample, target)
        # Positive compositions conservatively retain every declared sensitive
        # path; a public alternative cannot erase another branch's UI marker.
        for key in ("allOf", "anyOf", "oneOf"):
            for node in schema.get(key, []):
                child(node, sample)
        if isinstance(sample, dict):
            covered = set()
            for name, node in schema.get("properties", {}).items():
                if name in sample:
                    covered.add(name)
                    child(node, sample[name])
            for pattern, node in schema.get("patternProperties", {}).items():
                try:
                    names = [name for name in sample if re.search(pattern, name)]
                except re.error:
                    unsupported("sensitive sample has an invalid property pattern")
                for name in names:
                    covered.add(name)
                    child(node, sample[name])
            for name in sample.keys() - covered:
                child(schema.get("additionalProperties"), sample[name])
            for name, node in schema.get("dependentSchemas", {}).items():
                if name in sample:
                    child(node, sample)
        if isinstance(sample, list):
            prefix = schema.get("prefixItems", [])
            for node, value in zip(prefix, sample):
                child(node, value)
            for value in sample[len(prefix):]:
                child(schema.get("items"), value)
            for value in sample:
                child(schema.get("contains"), value)

    for document, node, sample in samples:
        if marked(node, document):
            visit(node, sample, document, set())


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
    roots, pending_samples = {}, []
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
        roots[path] = wrapper["openAPIV3Schema"]
        schema = _convert_schema(wrapper["openAPIV3Schema"], annotations[scope])
        if not isinstance(schema, dict):
            unsupported("top-level plugin schema must be an object")
        # Scan ancestor samples only for scopes which explicitly declare this
        # sensitive-field contract. Unmarked legacy scopes keep their existing
        # conversion behavior; sensitive fields themselves are checked above.
        has_sensitive_marker = False
        samples = []
        for node in schema_nodes(wrapper["openAPIV3Schema"]):
            has_sensitive_marker |= (node.get("x-sensitive") is True or node.get("writeOnly") is True
                                     or node.get("format") == "password")
            for key in ("default", "example", "examples"):
                if key in node:
                    values = node[key] if key == "examples" and isinstance(node[key], list) else [node[key]]
                    samples.extend((node, sample) for sample in values)
        if has_sensitive_marker:
            for _, sample in samples:
                reject_credentials(sample)
        pending_samples.extend((path, node, sample) for node, sample in samples)
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
    _check_sensitive_samples(roots, pending_samples)
    schemas["config.schema.json"]["x-higress"] = extension
    return {"schemas": {name: encode_yaml_value(schema) for name, schema in schemas.items()},
            "metadata": extension, "source_spec_digest": digest(raw)}
