"""Pure, bounded validation of immutable artifact.v1 packages.

No extraction, network access, module execution, or storage writes occur here.
Policy (environment references and provenance receipts) belongs to the trusted
service, never to the submitted package. Static Wasm validation does not certify
Proxy-Wasm ABI compatibility or gateway behavior.
"""
from __future__ import annotations

import copy
import gzip
import io
import json
import posixpath
import re
import tarfile
import zlib
from dataclasses import dataclass, field
from decimal import Decimal
from functools import lru_cache
from fractions import Fraction
from importlib.resources import files as resource_files
from pathlib import Path
from types import MappingProxyType
from typing import Any, Mapping
from urllib.parse import quote, unquote, urldefrag

from jsonschema import Draft202012Validator, FormatChecker, validators
from jsonschema.exceptions import SchemaError
from jsonschema.exceptions import ValidationError
from referencing import Registry, Resource
from referencing.exceptions import Unresolvable
from referencing.jsonschema import DRAFT202012
from wasmtime import Engine, Module, WasmtimeError

from mf_canonical import ContractError, canonicalize, parse_json, schema_numbers, sha256, utc_timestamp

DIALECT = "https://json-schema.org/draft/2020-12/schema"
COMMON = "urn:modelfusion:contracts:v1:common"
FIELDS = ("canonicalization_version", "schema_version", "plugin_name", "plugin_version",
          "files", "wasm_path", "config_schema_path", "defaults_path",
          "documentation_paths", "environment_refs", "source")
SCHEMA_MAP_KEYS = ("$defs", "properties", "patternProperties", "dependentSchemas")
SCHEMA_SINGLE_KEYS = ("items", "contains", "additionalProperties", "unevaluatedProperties",
                      "unevaluatedItems", "propertyNames", "not", "if", "then", "else", "contentSchema")
SCHEMA_ARRAY_KEYS = ("prefixItems", "allOf", "anyOf", "oneOf")
SCHEMA_KEYWORDS = frozenset("""
    $schema $id $ref $anchor $defs $comment type enum const title description
    default examples deprecated readOnly writeOnly multipleOf minimum maximum
    exclusiveMinimum exclusiveMaximum minLength maxLength pattern format
    minItems maxItems uniqueItems contains minContains maxContains prefixItems
    items minProperties maxProperties required properties patternProperties
    additionalProperties propertyNames dependentRequired dependentSchemas
    unevaluatedItems unevaluatedProperties allOf anyOf oneOf not if then else
    contentEncoding contentMediaType contentSchema x-higress
""".split())
EXECUTABLE_SUFFIXES = frozenset((".sh", ".bash", ".py", ".js", ".mjs", ".exe", ".so", ".bat"))
SECRET_KEYS = frozenset(("password", "api_key", "apikey", "api-key", "access_token",
                         "secret_key", "client_secret", "authorization"))


class ArtifactError(ContractError):
    """A determinate package/policy rejection; I/O failures are not invalid input."""

    def __init__(self, code: str, detail: str, stage: str = "semantic", pointer: str = ""):
        super().__init__(code, detail, stage, pointer)


@dataclass(frozen=True)
class ValidationPolicy:
    max_archive_bytes: int = 64 * 1024 * 1024
    max_tar_bytes: int = 96 * 1024 * 1024
    max_member_bytes: int = 64 * 1024 * 1024
    max_payload_bytes: int = 80 * 1024 * 1024
    max_members: int = 128
    allowed_environment_refs: tuple[tuple[str, str, bool], ...] = ()
    # Independent receipts bind the source, actual Wasm and complete content.
    # Approving a source alone must not authorize unrelated output bytes.
    provenance_receipts: frozenset[tuple[str, str, str]] = frozenset()
    forbidden_markers: tuple[bytes, ...] = (b"SYNTHETIC_SECRET_DO_NOT_PUBLISH",)


@dataclass(frozen=True)
class ValidatedPackage:
    descriptor_bytes: bytes
    canonical_bytes: bytes
    content_digest: str
    archive_digest: str
    payload: Mapping[str, bytes] = field(repr=False)

    @property
    def descriptor(self) -> dict[str, Any]:
        # Return a fresh value so callers cannot change validated identity in place.
        return parse_json(self.descriptor_bytes)


def load_policy(path: Path | None) -> ValidationPolicy:
    if path is None:
        return ValidationPolicy()
    value = parse_json(Path(path).read_bytes())
    if not isinstance(value, dict) or set(value) - {"allowed_environment_refs", "provenance_receipts"}:
        raise ValueError("invalid trusted artifact policy")
    allowed = tuple((r["name"], r["kind"], r["required"]) for r in value.get("allowed_environment_refs", []))
    receipts = frozenset((r["source_digest"], r["wasm_digest"], r["content_digest"]) for r in value.get("provenance_receipts", []))
    if any(not isinstance(name, str) or not name or kind not in ("secret", "endpoint", "setting") or not isinstance(required, bool) for name, kind, required in allowed):
        raise ValueError("invalid environment reference policy")
    if any(not isinstance(d, str) or re.fullmatch(r"sha256:[0-9a-f]{64}", d) is None for receipt in receipts for d in receipt):
        raise ValueError("invalid provenance receipt")
    return ValidationPolicy(allowed_environment_refs=allowed, provenance_receipts=receipts)


def digest(raw: bytes) -> str:
    return "sha256:" + sha256(raw)


def json_bytes(value: Any) -> bytes:
    """Encode protocol records (whose numeric quantities are decimal strings)."""
    return canonicalize(json.dumps(value, ensure_ascii=False, separators=(",", ":"),
                                   allow_nan=False).encode("utf-8"))


def safe_path(name: str) -> bool:
    return bool(isinstance(name, str) and name and not name.startswith("/")
                and "\\" not in name and not any(ord(c) < 32 or ord(c) == 127 for c in name)
                and all(part not in ("", ".", "..") for part in name.split("/")))


FORMATS = FormatChecker()
FORMATS.checks("mf-utc")(utc_timestamp)


def exact_multiple_of(validator, divisor, instance, schema):
    if validator.is_type(instance, "number") and (Fraction(instance) / Fraction(divisor)).denominator != 1:
        yield ValidationError("value is not an exact multiple of the declared divisor")


SchemaValidator = validators.extend(
    Draft202012Validator,
    validators={"multipleOf": exact_multiple_of},
    type_checker=Draft202012Validator.TYPE_CHECKER.redefine(
        "integer", lambda checker, value: not isinstance(value, bool) and
        (isinstance(value, int) or isinstance(value, Decimal) and value == value.to_integral_value())),
)


def schema_positions(schema: Any, pointer: str = ""):
    """Enumerate schema locations, including boolean schemas, without instance data."""
    if not isinstance(schema, (dict, bool)):
        return
    yield pointer, schema
    if not isinstance(schema, dict):
        return
    for key in SCHEMA_MAP_KEYS:
        for name, child in schema.get(key, {}).items():
            escaped = name.replace("~", "~0").replace("/", "~1")
            yield from schema_positions(child, pointer + "/" + key + "/" + escaped)
    for key in SCHEMA_SINGLE_KEYS:
        if key in schema:
            yield from schema_positions(schema[key], pointer + "/" + key)
    for key in SCHEMA_ARRAY_KEYS:
        for index, child in enumerate(schema.get(key, [])):
            yield from schema_positions(child, pointer + "/" + key + "/" + str(index))


def schema_nodes(schema: Any):
    for _, node in schema_positions(schema):
        if isinstance(node, dict):
            yield node


def registry_for(schemas: Mapping[str, Any]) -> Registry:
    # Registry has no retrieval callback: missing resources never cause network I/O.
    return Registry().with_resources((uri, Resource(schema, DRAFT202012)) for uri, schema in schemas.items())


def check_shape(value: Any, schema: Any, registry: Registry, code: str) -> None:
    try:
        error = next(SchemaValidator(schema, registry=registry, format_checker=FORMATS).iter_errors(value), None)
    except Unresolvable as exc:
        raise ArtifactError("ART_CONFIG_REFERENCE", "unresolvable schema reference", "schema") from exc
    except RecursionError as exc:
        raise ArtifactError("ART_CONFIG_REFERENCE", "schema reference evaluation did not terminate", "schema") from exc
    if error is not None:
        pointer = "/" + "/".join(str(v).replace("~", "~0").replace("/", "~1") for v in error.absolute_path)
        raise ArtifactError(code, "schema keyword failed: " + str(error.validator), "shape", pointer)


@lru_cache(maxsize=1)
def contract_registry() -> tuple[dict[str, Any], Registry]:
    schemas = {}
    for name in ("common", "package-descriptor", "artifact-manifest"):
        schema = schema_numbers(parse_json(resource_files("artifact_schemas").joinpath(f"{name}.schema.json").read_bytes()))
        Draft202012Validator.check_schema(schema)
        schemas[schema["$id"]] = schema
    return schemas, registry_for(schemas)


def validate_document(raw: bytes, kind: str = "descriptor") -> dict[str, Any]:
    if kind not in ("descriptor", "manifest"):
        raise ValueError("expected descriptor or manifest")
    value = parse_json(raw)
    schemas, registry = contract_registry()
    uri = "urn:modelfusion:contracts:v1:" + ("package-descriptor" if kind == "descriptor" else "artifact-manifest")
    check_shape(value, schemas[uri], registry, "ART_DESCRIPTOR")
    by_path = {}
    for entry in value["files"]:
        name = entry["path"]
        if not safe_path(name):
            raise ArtifactError("ART_ARCHIVE_PATH", "unsafe declared payload path")
        if name in by_path:
            raise ArtifactError("ART_DUPLICATE_PATH", "duplicate declared payload path")
        if name.split("/")[0] in ("package.json", "manifest.json", "archive.tgz", "metadata.txt"):
            raise ArtifactError("ART_DESCRIPTOR_SELF_REFERENCE", "reserved service/descriptor path")
        by_path[name] = entry
    for name in by_path:
        if any("/".join(name.split("/")[:i]) in by_path for i in range(1, len(name.split("/")))):
            raise ArtifactError("ART_ARCHIVE_PATH", "payload file is also used as a directory")
    if len({ref["name"] for ref in value["environment_refs"]}) != len(value["environment_refs"]):
        raise ArtifactError("ART_DUPLICATE_ENV", "duplicate environment reference")
    for name, role in (("wasm_path", "wasm"), ("config_schema_path", "config_schema"), ("defaults_path", "defaults")):
        if by_path.get(value[name], {}).get("role") != role:
            raise ArtifactError("ART_ROLE_REFERENCE", "selected path has wrong/missing role", pointer="/" + name)
    if sum(e["role"] == "wasm" for e in by_path.values()) != 1:
        raise ArtifactError("ART_ROLE_CARDINALITY", "exactly one Wasm is required")
    if sum(e["role"] == "defaults" for e in by_path.values()) != 1:
        raise ArtifactError("ART_ROLE_CARDINALITY", "exactly one defaults file is required")
    docs = {e["path"] for e in by_path.values() if e["role"] == "documentation"}
    if docs != set(value["documentation_paths"]):
        raise ArtifactError("ART_ROLE_REFERENCE", "documentation references differ")
    projection = {key: value[key] for key in FIELDS}
    projection["files"] = sorted(value["files"], key=lambda e: e["path"])
    projection["environment_refs"] = sorted(value["environment_refs"], key=lambda e: e["name"])
    projection["documentation_paths"] = sorted(value["documentation_paths"])
    normalized = json_bytes(projection)
    content_digest = digest(normalized)
    if kind == "manifest" and value["content_digest"] != content_digest:
        raise ArtifactError("ART_CONTENT_DIGEST", "content digest does not match descriptor projection")
    return {"document": value, "canonical_bytes": normalized, "content_digest": content_digest}


def config_schemas(descriptor: dict[str, Any], payload: Mapping[str, bytes]) -> tuple[dict[str, Any], Registry]:
    """Validate and bind every local schema, including unused reference branches."""
    declared = {e["path"] for e in descriptor["files"] if e["role"] == "config_schema"}
    schemas = {}
    references = []
    for name in declared:
        canonicalize(payload[name])  # enforce exact-number expansion limits before schema arithmetic
        value = schema_numbers(parse_json(payload[name]))
        if not isinstance(value, dict) or value.get("$schema") != DIALECT:
            raise ArtifactError("ART_CONFIG_SCHEMA", "standalone Draft 2020-12 schema required", "schema")
        try:
            Draft202012Validator.check_schema(value)
        except SchemaError as exc:
            raise ArtifactError("ART_CONFIG_SCHEMA", "invalid schema", "schema") from exc
        value = copy.deepcopy(value)
        uri = "urn:modelfusion:payload:" + quote(name, safe="")
        anchors = set()
        for node in schema_nodes(value):
            if set(node) - SCHEMA_KEYWORDS or (node is not value and ("$id" in node or "x-higress" in node)):
                raise ArtifactError("ART_CONFIG_SCHEMA", "unsupported schema keyword or nested resource", "schema")
            if "format" in node and node["format"] not in FORMATS.checkers:
                raise ArtifactError("ART_CONFIG_SCHEMA", "unsupported schema format", "schema")
            if "$anchor" in node:
                if node["$anchor"] in anchors:
                    raise ArtifactError("ART_CONFIG_REFERENCE", "duplicate schema anchor", "schema")
                anchors.add(node["$anchor"])
            if "$ref" in node:
                base, fragment = urldefrag(node["$ref"])
                if base == COMMON:
                    target_uri = COMMON
                else:
                    if ":" in base or base.startswith("/") or "\\" in base:
                        raise ArtifactError("ART_CONFIG_REFERENCE", "non-local schema reference", "schema")
                    if re.search(r"%(?![0-9a-fA-F]{2})", base):
                        raise ArtifactError("ART_CONFIG_REFERENCE", "invalid URI path escape", "schema")
                    try:
                        base = unquote(base, errors="strict")
                    except UnicodeError as exc:
                        raise ArtifactError("ART_CONFIG_REFERENCE", "invalid URI path encoding", "schema") from exc
                    target = posixpath.normpath(posixpath.join(posixpath.dirname(name), base)) if base else name
                    if not safe_path(target) or target not in declared:
                        raise ArtifactError("ART_CONFIG_REFERENCE", "undeclared schema reference", "schema")
                    target_uri = "urn:modelfusion:payload:" + quote(target, safe="")
                if re.search(r"%(?![0-9a-fA-F]{2})", fragment):
                    raise ArtifactError("ART_CONFIG_REFERENCE", "invalid percent escape", "schema")
                try:
                    fragment = unquote(fragment, errors="strict")
                except UnicodeError as exc:
                    raise ArtifactError("ART_CONFIG_REFERENCE", "invalid fragment encoding", "schema") from exc
                if fragment.startswith("/") and re.search(r"~(?![01])", fragment):
                    raise ArtifactError("ART_CONFIG_REFERENCE", "invalid JSON Pointer escape", "schema")
                references.append((node, target_uri, fragment))
                node["$ref"] = target_uri + ("#" + quote(fragment, safe="/~") if fragment else "")
        value["$id"] = uri
        schemas[uri] = value
    common_schemas, _ = contract_registry()
    bound = {COMMON: common_schemas[COMMON], **schemas}
    locations = {uri: dict(schema_positions(schema)) for uri, schema in bound.items()}
    anchors = {uri: {node["$anchor"] for node in schema_nodes(schema) if "$anchor" in node} for uri, schema in bound.items()}
    for _, target_uri, fragment in references:
        valid = fragment in locations[target_uri] if not fragment or fragment.startswith("/") else fragment in anchors[target_uri]
        if not valid:
            raise ArtifactError("ART_CONFIG_REFERENCE", "reference target is not a declared schema location", "schema")
    registry = registry_for(bound)
    for uri, schema in schemas.items():
        for node in schema_nodes(schema):
            if "$ref" in node:
                try:
                    registry.resolver(uri).lookup(node["$ref"])
                except Unresolvable as exc:
                    raise ArtifactError("ART_CONFIG_REFERENCE", "unresolvable local schema reference", "schema") from exc
        if "x-higress" in schema:
            # Validation is local to avoid a circular import at module import time.
            from plugin_schema_conversion import validate_extension
            validate_extension(schema["x-higress"])
            extension = schema["x-higress"]
            if (extension["info"]["name"], extension["info"]["version"]) != (descriptor["plugin_name"], descriptor["plugin_version"]):
                raise ArtifactError("ART_CONFIG_SCHEMA", "metadata identity differs from package", "schema")
            for key in ("global_schema_path", "route_schema_path"):
                if extension[key] is not None and extension[key] not in declared:
                    raise ArtifactError("ART_CONFIG_REFERENCE", "metadata schema path is absent", "schema")
            if extension["global_schema_path"] != descriptor["config_schema_path"]:
                raise ArtifactError("ART_CONFIG_SCHEMA", "selected global schema differs", "schema")
    return schemas, registry


def validate_defaults(descriptor: dict[str, Any], payload: Mapping[str, bytes]) -> None:
    schemas, registry = config_schemas(descriptor, payload)
    uri = "urn:modelfusion:payload:" + quote(descriptor["config_schema_path"], safe="")
    canonicalize(payload[descriptor["defaults_path"]])
    check_shape(schema_numbers(parse_json(payload[descriptor["defaults_path"]])), schemas[uri], registry, "ART_DEFAULTS")


def reject_credentials(value: Any) -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if key.lower() in SECRET_KEYS and isinstance(item, str) and item and not re.fullmatch(r"\$\{[A-Z_][A-Z0-9_]*\}", item):
                raise ArtifactError("ART_SECRET", "literal credentials must use controlled references", "policy")
            reject_credentials(item)
    elif isinstance(value, list):
        for item in value:
            reject_credentials(item)


@lru_cache(maxsize=1)
def wasm_engine() -> Engine:
    return Engine()


def validate_wasm(raw: bytes) -> None:
    if raw[:8] != b"\x00asm\x01\x00\x00\x00":
        raise ArtifactError("ART_WASM", "expected a binary core WebAssembly module", "wasm")
    try:
        Module.validate(wasm_engine(), raw)
    except WasmtimeError as exc:
        raise ArtifactError("ART_WASM", "invalid WebAssembly module", "wasm") from exc


def validate_package(raw: bytes, policy: ValidationPolicy | None = None) -> ValidatedPackage:
    policy = policy or ValidationPolicy()
    if len(raw) > policy.max_archive_bytes:
        raise ArtifactError("ART_LIMIT", "archive exceeds configured budget", "limits")
    try:
        with gzip.GzipFile(fileobj=io.BytesIO(raw)) as stream:
            tar_bytes = stream.read(policy.max_tar_bytes + 1)
        if len(tar_bytes) > policy.max_tar_bytes:
            raise ArtifactError("ART_LIMIT", "expanded archive exceeds configured budget", "limits")
        if len(tar_bytes) % 512 or len(tar_bytes) < 1024 or tar_bytes[-1024:] != bytes(1024):
            raise ArtifactError("ART_ARCHIVE", "truncated tar archive", "archive")
        members = {}
        total = 0
        payload_end = 0
        with tarfile.open(fileobj=io.BytesIO(tar_bytes), mode="r:") as archive:
            for member in archive:
                if member.offset != payload_end or member.offset_data != member.offset + 512:
                    raise ArtifactError("ART_ARCHIVE", "extended or hidden tar headers are unsupported", "archive")
                if not safe_path(member.name):
                    raise ArtifactError("ART_ARCHIVE_PATH", "unsafe archive member", "archive")
                if member.name in members:
                    raise ArtifactError("ART_ARCHIVE_DUPLICATE", "duplicate archive member", "archive")
                if not member.isfile() or member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE) or member.pax_headers:
                    raise ArtifactError("ART_ARCHIVE_LINK", "only plain regular members are accepted", "archive")
                if member.mode & 0o111 or Path(member.name).suffix.lower() in EXECUTABLE_SUFFIXES:
                    raise ArtifactError("ART_ARCHIVE_EXECUTABLE", "executable archive member", "archive")
                if len(members) >= policy.max_members or member.size > policy.max_member_bytes:
                    raise ArtifactError("ART_LIMIT", "archive member budget exceeded", "limits")
                if member.offset_data + member.size > len(tar_bytes) - 1024:
                    raise ArtifactError("ART_ARCHIVE", "truncated member", "archive")
                content = archive.extractfile(member)
                if content is None:
                    raise ArtifactError("ART_ARCHIVE", "unreadable member", "archive")
                payload = content.read(member.size + 1)
                if len(payload) != member.size:
                    raise ArtifactError("ART_ARCHIVE", "short member", "archive")
                members[member.name] = payload
                total += len(payload)
                if total > policy.max_payload_bytes:
                    raise ArtifactError("ART_LIMIT", "payload budget exceeded", "limits")
                payload_end = member.offset_data + ((member.size + 511) // 512) * 512
        if len(tar_bytes) - payload_end < 1024 or any(tar_bytes[payload_end:]):
            raise ArtifactError("ART_ARCHIVE", "unexpected hidden tar members", "archive")
    except (tarfile.TarError, EOFError, gzip.BadGzipFile, zlib.error) as exc:
        raise ArtifactError("ART_ARCHIVE", "invalid compressed archive", "archive") from exc
    if "package.json" not in members:
        raise ArtifactError("ART_DESCRIPTOR", "missing root package.json", "descriptor")
    validated = validate_document(members["package.json"])
    descriptor = validated["document"]
    declared = {e["path"]: e for e in descriptor["files"]}
    if set(members) != set(declared) | {"package.json"}:
        raise ArtifactError("ART_FILE_SET", "undeclared or missing payload", "payload")
    for name, entry in declared.items():
        if str(len(members[name])) != entry["size_bytes"]:
            raise ArtifactError("ART_FILE_SIZE", "payload size mismatch", "payload")
        if digest(members[name]) != entry["sha256"]:
            raise ArtifactError("ART_FILE_DIGEST", "payload digest mismatch", "payload")
    if any((r["name"], r["kind"], r["required"]) not in policy.allowed_environment_refs for r in descriptor["environment_refs"]):
        raise ArtifactError("ART_ENVIRONMENT_REF", "unapproved environment reference", "policy")
    source = descriptor["source"]
    provenance = (digest(json_bytes(source)), digest(members[descriptor["wasm_path"]]), validated["content_digest"])
    if source["verification"] == "verified" and provenance not in policy.provenance_receipts:
        raise ArtifactError("ART_SOURCE_EVIDENCE", "verified source needs independent trusted receipt", "provenance")
    if any(marker in content for content in members.values() for marker in policy.forbidden_markers):
        raise ArtifactError("ART_SECRET", "forbidden credential marker", "policy")
    reject_credentials(parse_json(members[descriptor["defaults_path"]]))
    validate_defaults(descriptor, members)
    validate_wasm(members[descriptor["wasm_path"]])
    return ValidatedPackage(members["package.json"], validated["canonical_bytes"],
                            validated["content_digest"], digest(raw), MappingProxyType(members))
