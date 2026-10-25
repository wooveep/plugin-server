#!/usr/bin/env python3
"""Pinned, offline D0 test-asset validator. No product modules are imported."""
from __future__ import annotations

import json
import argparse
import hashlib
import importlib.metadata
import re
import sys
import tempfile
from datetime import date
from decimal import Decimal
from pathlib import Path, PurePosixPath
from typing import Any
from urllib.parse import unquote, urldefrag

try:
    from jsonschema import Draft202012Validator, FormatChecker, validators
    from referencing import Registry, Resource
    from referencing.jsonschema import DRAFT202012
except ImportError:
    summary = {'executed': 0, 'matched': 0, 'failed': 1, 'missing': 1}
    failure = {'report_format_version': 'd0-report-v1', 'evidence_level': 'offline_fixture',
               'status': 'blocked', 'product_execution': 'not_run', 'summary': summary,
               'results': [{'case_id': 'environment', 'matched': False,
                            'actual': {'verdict': 'reject', 'error_code': 'E_DEPENDENCY', 'stage': 'environment'}}]}
    if '--report' in sys.argv and sys.argv.index('--report') + 1 < len(sys.argv):
        destination = Path(sys.argv[sys.argv.index('--report') + 1])
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(json.dumps(failure, indent=2) + '\n')
    print('E_DEPENDENCY: prepare the hash-locked offline environment', file=sys.stderr)
    sys.exit(2)

MAX_INPUT_BYTES = 1_048_576
MAX_OUTPUT_BYTES = 4_194_304
MAX_NUMBER_CHARS = 10_000
MAX_DEPTH = 128
MAX_ITEMS = 10_000
TOOL_ROOT = Path(__file__).resolve().parents[2]
DIALECT = 'https://json-schema.org/draft/2020-12/schema'
URN = 'urn:modelfusion:contracts:v1:'
VALIDATORS = {'d0-assertions.v1', 'mf-json.v1', 'd0-resolver.v1'}
RESOLVER_MUTATIONS = {'unknown_urn', 'duplicate_urn', 'missing_file', 'digest_mismatch',
                      'path_escape', 'absolute_path', 'symlink_escape', 'network_ref',
                      'unknown_fragment', 'dynamic_ref', 'duplicate_path', 'wrong_dialect',
                      'missing_asset', 'unknown_validator', 'unused_bad_ref', 'duplicate_anchor',
                      'legacy_definition_ref', 'legacy_dependency_ref', 'fragment_target_ref',
                      'annotation_data', 'empty_selection', 'missing_pointer', 'semantic_input_ref'}
OPERATORS = {'eq', 'ne', 'unique', 'contains', 'join', 'cardinality',
             'int_compare', 'int_add', 'int_sub', 'key_intersection', 'partial_order'}


class ContractError(Exception):
    def __init__(self, code: str, detail: str, stage: str = 'parse', pointer: str = ''):
        super().__init__(detail)
        self.code, self.stage, self.pointer = code, stage, pointer


def parse_json(raw: bytes) -> Any:
    if len(raw) > MAX_INPUT_BYTES:
        raise ContractError('E_LIMIT', 'input byte limit exceeded')
    try:
        text = raw.decode('utf-8')
    except UnicodeDecodeError as error:
        raise ContractError('E_UTF8', 'invalid UTF-8') from error

    def pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in items:
            if key in result:
                raise ContractError('E_DUPLICATE_KEY', 'duplicate object key')
            result[key] = value
        return result

    def invalid_constant(value: str) -> Any:
        raise ContractError('E_JSON', 'non-JSON number')

    try:
        result = json.loads(text, parse_float=Decimal, parse_int=Decimal,
                            parse_constant=invalid_constant, object_pairs_hook=pairs)
    except (ValueError, RecursionError) as error:
        raise ContractError('E_JSON' if isinstance(error, ValueError) else 'E_LIMIT',
                            'invalid JSON or excessive nesting') from error

    def check(value: Any, depth: int = 0) -> None:
        if depth > MAX_DEPTH:
            raise ContractError('E_LIMIT', 'nesting limit exceeded')
        if isinstance(value, str) and any(0xD800 <= ord(c) <= 0xDFFF for c in value):
            raise ContractError('E_SURROGATE', 'isolated surrogate')
        if isinstance(value, (list, dict)) and len(value) > MAX_ITEMS:
            raise ContractError('E_LIMIT', 'container item limit exceeded')
        if isinstance(value, dict):
            for key, item in value.items():
                check(key, depth + 1)
                check(item, depth + 1)
        elif isinstance(value, list):
            for item in value:
                check(item, depth + 1)
    check(result)
    return result


def canonicalize(raw: bytes) -> bytes:
    """mf-json.v1, with exact decimal parsing and bounded exponent expansion."""
    chunks: list[bytes] = []
    size = 0

    def append(value: str) -> None:
        nonlocal size
        chunk = value.encode('utf-8')
        size += len(chunk)
        if size > MAX_OUTPUT_BYTES:
            raise ContractError('E_LIMIT', 'canonical output limit exceeded')
        chunks.append(chunk)

    def encode(value: Any) -> None:
        if value is None:
            append('null')
        elif isinstance(value, bool):
            append('true' if value else 'false')
        elif isinstance(value, Decimal):
            if not value:
                append('0')
            else:
                _, digits, exponent = value.as_tuple()
                if not isinstance(exponent, int) or len(digits) + abs(exponent) > MAX_NUMBER_CHARS:
                    raise ContractError('E_LIMIT', 'expanded number limit exceeded')
                number = format(value, 'f')
                append(number.rstrip('0').rstrip('.') if '.' in number else number)
        elif isinstance(value, str):
            append('"' + ''.join('\\u%04x' % ord(c) if ord(c) < 32 else
                                  '\\' + c if c in ('"', '\\') else c for c in value) + '"')
        elif isinstance(value, list):
            append('[')
            for index, item in enumerate(value):
                if index:
                    append(',')
                encode(item)
            append(']')
        else:
            append('{')
            for index, key in enumerate(sorted(value)):
                if index:
                    append(',')
                encode(key)
                append(':')
                encode(value[key])
            append('}')
    encode(parse_json(raw))
    return b''.join(chunks)


def sha256(raw: bytes) -> str:
    return hashlib.sha256(raw).hexdigest()


def read_bytes(path: Path) -> bytes:
    try:
        if path.stat().st_size > 16 * MAX_INPUT_BYTES:
            raise ContractError('E_LIMIT', 'asset byte limit exceeded')
        return path.read_bytes()
    except OSError as error:
        raise ContractError('E_FILE', 'missing or unreadable file') from error


def safe_path(root: Path, relative: str) -> Path:
    if (not isinstance(relative, str) or not re.fullmatch(r'[A-Za-z0-9_./-]+', relative)
            or any(p in ('', '.', '..') for p in relative.split('/'))
            or PurePosixPath(relative).is_absolute()):
        raise ContractError('E_PATH', 'path must stay within the explicit root')
    current = root
    for part in relative.split('/'):
        current = current / part
        if current.is_symlink():
            raise ContractError('E_PATH', 'symlink paths are forbidden')
    if not current.resolve().is_relative_to(root.resolve()):
        raise ContractError('E_PATH', 'path escaped root')
    return current


def pointer(value: Any, path: str, code: str = 'E_POINTER') -> Any:
    if path == '':
        return value
    if not isinstance(path, str) or not path.startswith('/'):
        raise ContractError(code, 'invalid JSON Pointer', 'semantic')
    for part in path[1:].split('/'):
        if re.search(r'~(?![01])', part):
            raise ContractError(code, 'invalid JSON Pointer escape', 'semantic', path)
        key = part.replace('~1', '/').replace('~0', '~')
        if isinstance(value, dict) and key in value:
            value = value[key]
        elif isinstance(value, list) and re.fullmatch(r'0|[1-9][0-9]*', key) and len(key) < 10 and int(key) < len(value):
            value = value[int(key)]
        else:
            raise ContractError(code, 'JSON Pointer target is absent', 'semantic', path)
    return value


def selected_records(raw: bytes, selection: dict[str, Any]) -> list[Any]:
    if selection['mode'] == 'ndjson_records':
        records = [parse_json(row) for row in raw.splitlines() if row.strip()]
        if not records or len(records) > MAX_ITEMS:
            raise ContractError('E_NO_CHECKS', 'empty or oversized NDJSON selection')
        return records
    value = parse_json(raw)
    try:
        return [pointer(value, selection['pointer']) if selection['mode'] == 'json_pointer' else value]
    except ContractError as error:
        raise ContractError('E_SELECTION', 'fixture JSON Pointer target is absent', 'parse', selection['pointer']) from error


FORMATS = FormatChecker()


@FORMATS.checks('mf-utc')
def utc_timestamp(value: Any) -> bool:
    if not isinstance(value, str):
        return True  # JSON Schema type owns non-strings.
    match = re.fullmatch(r'(\d{4})-(\d{2})-(\d{2})[Tt](\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:[Zz]|\+00:00)', value, re.ASCII)
    if not match:
        return False
    year, month, day, hour, minute, second = map(int, match.groups())
    try:
        date(year, month, day)
    except ValueError:
        return False
    return hour < 24 and minute < 60 and (second < 60 or (second == 60 and hour == 23 and minute == 59))


SchemaValidator = validators.extend(Draft202012Validator, type_checker=Draft202012Validator.TYPE_CHECKER.redefine(
    'integer', lambda checker, value: not isinstance(value, bool) and
    (isinstance(value, int) or isinstance(value, Decimal) and value == value.to_integral_value())))


def registry_for(schemas: dict[str, Any]) -> Registry:
    return Registry().with_resources((uri, Resource(contents=schema, specification=DRAFT202012))
                                     for uri, schema in schemas.items())


def check_shape(value: Any, schema: Any, registry: Registry, code: str = 'E_SCHEMA_SHAPE') -> None:
    error = next(SchemaValidator(schema, registry=registry, format_checker=FORMATS).iter_errors(value), None)
    if error:
        location = '/' + '/'.join(str(v).replace('~', '~0').replace('/', '~1') for v in error.absolute_path)
        raise ContractError(code, 'schema keyword failed: ' + str(error.validator), 'shape', location)


def tool_schemas() -> dict[str, Any]:
    result = {}
    for name in ('common', 'fixture-validation', 'bundle', 'lock'):
        schema = parse_json(read_bytes(TOOL_ROOT / 'contracts/v1' / (name + '.schema.json')))
        result[schema['$id']] = schema
    schema = parse_json(read_bytes(TOOL_ROOT / 'scripts/contracts/assertions.schema.json'))
    result[schema['$id']] = schema
    return result


def schema_numbers(value: Any) -> Any:
    """The schema engine requires native integral keyword values; never use floats."""
    if isinstance(value, Decimal) and value == value.to_integral_value():
        if abs(value.adjusted()) > MAX_NUMBER_CHARS:
            raise ContractError('E_LIMIT', 'schema integer limit exceeded')
        return int(value)
    if isinstance(value, dict):
        return {key: schema_numbers(item) for key, item in value.items()}
    if isinstance(value, list):
        return [schema_numbers(item) for item in value]
    return value


def check_environment() -> None:
    """No installation or fallback. A prepared hash-locked environment is required."""
    lock = read_bytes(TOOL_ROOT / 'scripts/contracts/requirements.lock')
    for package, version in re.findall(rb'^([a-zA-Z0-9_-]+)==([^\s\\]+)', lock, re.MULTILINE):
        try:
            actual = importlib.metadata.version(package.decode())
        except importlib.metadata.PackageNotFoundError as error:
            raise ContractError('E_DEPENDENCY', 'missing locked tool dependency') from error
        if actual != version.decode():
            raise ContractError('E_DEPENDENCY', 'installed dependency version differs from tool lock')
    meta_root = TOOL_ROOT / 'contracts/v1/vendor'
    meta_lock = parse_json(read_bytes(meta_root / 'json-schema.lock.json'))
    if meta_lock['schema_dialect'] != DIALECT or not meta_lock['resources']:
        raise ContractError('E_DIALECT', 'incomplete local metaschema lock')
    seen = set()
    for item in meta_lock['resources']:
        path = safe_path(meta_root, item['path'])
        raw = read_bytes(path)
        if sha256(raw) != item['sha256']:
            raise ContractError('E_DIGEST', 'metaschema byte digest differs')
        schema = parse_json(raw)
        if schema['$id'] != item['uri'] or item['uri'] in seen:
            raise ContractError('E_DIALECT', 'invalid local metaschema identity')
        seen.add(item['uri'])
    if DIALECT not in seen or len(seen) != 8:
        raise ContractError('E_DIALECT', 'missing Draft 2020-12 metaschema resource')


def schema_nodes(schema: Any):
    """Walk schema positions only; defaults and examples are instance data."""
    if not isinstance(schema, dict):
        return
    yield schema
    for key in ('$defs', 'definitions', 'dependencies', 'properties', 'patternProperties', 'dependentSchemas'):
        for child in schema.get(key, {}).values():
            yield from schema_nodes(child)
    for key in ('items', 'contains', 'additionalProperties', 'unevaluatedProperties',
                'unevaluatedItems', 'propertyNames', 'not', 'if', 'then', 'else', 'contentSchema'):
        if key in schema:
            yield from schema_nodes(schema[key])
    for key in ('prefixItems', 'allOf', 'anyOf', 'oneOf'):
        for child in schema.get(key, []):
            yield from schema_nodes(child)


class Bundle:
    def __init__(self, manifest_path: Path, source_root: Path | None = None,
                 export_map_path: Path | None = None, defer_refs: bool = False):
        if manifest_path.is_symlink():
            raise ContractError('E_PATH', 'manifest symlink is forbidden')
        self.manifest_path = manifest_path.resolve()
        self.raw_manifest = read_bytes(self.manifest_path)
        self.manifest = parse_json(self.raw_manifest)
        trusted = tool_schemas()
        check_shape(self.manifest, trusted[URN + 'bundle'], registry_for(trusted), 'E_MANIFEST')
        self.root = self.manifest_path.parent
        self.mapping: dict[str, str] | None = None
        if source_root is not None or export_map_path is not None:
            if source_root is None or export_map_path is None:
                raise ContractError('E_EXPORT_MAP', 'source root and export map are both required')
            self.root = source_root.resolve()
            mapping = parse_json(read_bytes(export_map_path))
            if mapping.get('export_format_version') != 'd0-export-v1' or not isinstance(mapping.get('files'), list):
                raise ContractError('E_EXPORT_MAP', 'unknown export-map format')
            self.mapping = {}
            sources: set[str] = set()
            for item in mapping['files']:
                source, target = item['source'], item['path']
                safe_path(self.root, source)
                safe_path(self.root, target)
                if target in self.mapping or source in sources or not allowed_source(source, target):
                    raise ContractError('E_EXPORT_MAP', 'duplicate or non-whitelisted export')
                self.mapping[target] = source
                sources.add(source)
        self.files: dict[str, bytes] = {}
        self.schemas: dict[str, Any] = {}
        self.assets: dict[str, dict[str, Any]] = {}
        for group, key in [('entries', 'schema_id'), ('fixtures', 'case_id'), ('assets', 'asset_id')]:
            ids: set[str] = set()
            for item in self.manifest[group]:
                if item[key] in ids:
                    raise ContractError('E_DUPLICATE_URN' if group == 'entries' else 'E_DUPLICATE_ID', 'duplicate manifest identity')
                ids.add(item[key])
                path = item['path']
                safe_path(self.root, path)
                if path in self.files or path == 'bundle-manifest.json':
                    raise ContractError('E_DUPLICATE_PATH', 'each package file must be registered once')
                if self.mapping is not None and path not in self.mapping:
                    raise ContractError('E_EXPORT_MAP', 'manifest file missing from export map')
                file_path = safe_path(self.root, self.mapping[path] if self.mapping is not None else path)
                raw = read_bytes(file_path)
                if sha256(raw) != item['sha256']:
                    raise ContractError('E_DIGEST', 'registered file digest differs', pointer=path)
                self.files[path] = raw
                if group == 'entries':
                    schema = schema_numbers(parse_json(raw))
                    if not isinstance(schema, dict) or schema.get('$id') != item[key]:
                        raise ContractError('E_SCHEMA_ID', 'schema ID differs from manifest')
                    if schema.get('$schema') != DIALECT:
                        raise ContractError('E_DIALECT', 'schema dialect is unsupported')
                    try:
                        Draft202012Validator.check_schema(schema)
                    except Exception as error:
                        raise ContractError('E_SCHEMA', 'invalid JSON Schema') from error
                    self.schemas[item[key]] = schema
                elif group == 'assets':
                    self.assets[item[key]] = item
        if self.mapping is not None and set(self.mapping) != set(self.files):
            raise ContractError('E_EXPORT_MAP', 'export map must exactly match the complete manifest')
        self.registry = registry_for(self.schemas)
        self.anchors: dict[tuple[str, str], Any] = {}
        for uri, schema in self.schemas.items():
            for node in schema_nodes(schema):
                if '$dynamicRef' in node or '$dynamicAnchor' in node:
                    raise ContractError('E_DYNAMIC_REF', 'dynamic business references are unsupported')
                if node is not schema and ('$id' in node or '$schema' in node):
                    raise ContractError('E_SCHEMA_ID', 'nested schema identities are unsupported')
                if '$anchor' in node:
                    anchor = node['$anchor']
                    if (not re.fullmatch(r'[A-Za-z_][-A-Za-z0-9._]*', anchor)
                            or (uri, anchor) in self.anchors):
                        raise ContractError('E_FRAGMENT', 'invalid or duplicate schema anchor')
                    self.anchors[(uri, anchor)] = node
        if not defer_refs:
            self.check_references()

    def resolve(self, ref: str, origin: str = '') -> Any:
        base, fragment = urldefrag(ref)
        if not base:
            base = origin
        if base.startswith(('https:', 'http:')):
            raise ContractError('E_NETWORK_REF', 'network schema references are forbidden')
        if not base.startswith('urn:'):
            raise ContractError('E_REF_URI', 'schema reference must be an explicit URN or same-document fragment')
        if base not in self.schemas:
            raise ContractError('E_UNKNOWN_REF', 'unknown schema URN')
        if re.search(r'%(?![0-9a-fA-F]{2})', fragment):
            raise ContractError('E_FRAGMENT', 'invalid URI fragment escape')
        try:
            fragment = unquote(fragment, errors='strict')
            result = pointer(self.schemas[base], fragment, 'E_FRAGMENT') if fragment.startswith('/') or not fragment else self.anchors[(base, fragment)]
        except (KeyError, UnicodeError, ContractError) as error:
            raise ContractError('E_FRAGMENT', 'unknown schema fragment') from error
        if not isinstance(result, (dict, bool)):
            raise ContractError('E_FRAGMENT', 'reference does not point to a schema')
        return result

    def check_references(self) -> None:
        pending = []
        trusted = tool_schemas()
        trusted_registry = registry_for(trusted)
        for uri, schema in self.schemas.items():
            for node in schema_nodes(schema):
                pending.append((uri, node))
        for fixture in self.manifest['fixtures']:
            validation = fixture['validation']
            if len(set(fixture['asset_refs'])) != len(fixture['asset_refs']):
                raise ContractError('E_ASSET_REF', 'duplicate asset reference')
            if any(ref not in self.assets for ref in fixture['asset_refs']):
                raise ContractError('E_ASSET_REF', 'missing fixture asset reference')
            if validation['kind'] == 'json_schema':
                base, _ = urldefrag(validation['schema_ref'])
                pending.append((base, self.resolve(validation['schema_ref'])))
            elif validation['validator_id'] not in VALIDATORS:
                raise ContractError('E_VALIDATOR', 'unknown fixed validator')
            elif ((validation['kind'] == 'semantic') != (validation['validator_id'] == 'd0-assertions.v1')):
                raise ContractError('E_VALIDATOR', 'validator kind differs from fixed interface')
            if validation['kind'] == 'raw_case' and validation['selection']['mode'] != 'document':
                raise ContractError('E_VALIDATOR_INPUT', 'raw validators require an explicit whole-document input')
            if validation['kind'] == 'raw_case':
                spec = parse_json(self.files[fixture['path']])
                if not isinstance(spec, dict):
                    raise ContractError('E_VALIDATOR_INPUT', 'fixed raw validator requires an instruction object')
                if validation['validator_id'] == 'mf-json.v1':
                    if (set(spec) != {'input_asset', 'expected_asset', 'expected_digest'}
                            or not isinstance(spec['expected_digest'], str)
                            or not re.fullmatch(r'sha256:[0-9a-f]{64}', spec['expected_digest'])):
                        raise ContractError('E_VALIDATOR_INPUT', 'invalid canonical vector instruction')
                    self.asset_bytes(spec['input_asset'], fixture)
                    self.asset_bytes(spec['expected_asset'], fixture)
                elif (set(spec) != {'mutation'} or not isinstance(spec['mutation'], str)
                      or spec['mutation'] not in RESOLVER_MUTATIONS):
                    raise ContractError('E_VALIDATOR_INPUT', 'invalid fixed resolver instruction')
            elif validation['kind'] == 'semantic':
                for record in selected_records(self.files[fixture['path']], validation['selection']):
                    check_shape(record, trusted[URN + 'assertions'], trusted_registry, 'E_ASSERTION_SPEC')
                    for item in record['inputs']:
                        asset = self.assets.get(item['asset_id'])
                        if not asset or asset['path'] != item['path'] or asset['sha256'] != item['sha256']:
                            raise ContractError('E_INPUT', 'assertion input is not a frozen registered asset')
                        self.asset_bytes(item['asset_id'], fixture)
                        if 'schema_ref' in item:
                            base, _ = urldefrag(item['schema_ref'])
                            pending.append((base, self.resolve(item['schema_ref'])))
        seen: set[tuple[str, int]] = set()
        while pending:
            uri, target = pending.pop()
            if isinstance(target, dict) and (uri, id(target)) not in seen:
                try:
                    Draft202012Validator.check_schema(schema_numbers(target))
                except Exception as error:
                    raise ContractError('E_SCHEMA', 'referenced target is not a valid schema') from error
            for node in schema_nodes(target):
                identity = (uri, id(node))
                if identity in seen:
                    continue
                seen.add(identity)
                if '$dynamicRef' in node or '$dynamicAnchor' in node:
                    raise ContractError('E_DYNAMIC_REF', 'dynamic business references are unsupported')
                if node is not self.schemas[uri] and ('$id' in node or '$schema' in node):
                    raise ContractError('E_SCHEMA_ID', 'nested schema identities are unsupported')
                if '$ref' in node:
                    base, _ = urldefrag(node['$ref'])
                    pending.append((base or uri, self.resolve(node['$ref'], uri)))

    def validate(self, value: Any, ref: str) -> None:
        # Use a reference wrapper so nested local #refs retain the resource's base URI.
        self.resolve(ref)
        check_shape(value, {'$ref': ref}, self.registry)

    def run_cases(self, selected: str | None = None) -> list[dict[str, Any]]:
        fixtures = self.manifest['fixtures']
        if 'common-catalog' in self.assets:
            catalog = parse_json(self.files[self.assets['common-catalog']['path']])
            required = catalog.get('required_cases', [])
            expected = {f['case_id']: (f['validation']['expected_stage'], frozen(f['expected'])) for f in fixtures}
            actual = {row['case_id']: (row['expected_stage'], frozen(row['expected'])) for row in required}
            if not required or len(required) != len(actual) or actual != expected:
                raise ContractError('E_COVERAGE', 'frozen common catalog and manifest cases differ')
        if selected is not None:
            selected = selected.replace('contracts/v1/cases/', 'fixtures/mf/').rstrip('/')
            fixtures = [f for f in fixtures if f['path'] == selected or f['path'].startswith(selected + '/')]
        if not fixtures:
            raise ContractError('E_NO_CHECKS', 'no registered fixtures selected')
        return [match_result(f['case_id'], f['expected'], f['validation']['expected_stage'],
                             lambda f=f: self.run_fixture(f),
                             permit_tool_rejection=f['validation']['kind'] == 'raw_case',
                             permit_decode_rejection=f['validation']['kind'] == 'json_schema') for f in fixtures]

    def run_fixture(self, fixture: dict[str, Any]) -> str:
        validation = fixture['validation']
        raw = self.files[fixture['path']]
        if validation['kind'] == 'raw_case':
            spec = parse_json(raw)
            if validation['validator_id'] == 'mf-json.v1':
                source = self.asset_bytes(spec['input_asset'], fixture)
                actual = canonicalize(source)
                expected = self.asset_bytes(spec['expected_asset'], fixture)
                if actual != expected or 'sha256:' + sha256(actual) != spec['expected_digest']:
                    raise ContractError('E_CANONICAL_BYTES', 'canonical bytes differ from independent golden', 'canonical')
                return 'canonical'
            return self.resolver_case(spec)
        records = selected_records(raw, validation['selection'])
        for record in records:
            if validation['kind'] == 'json_schema':
                self.validate(record, validation['schema_ref'])
            else:
                trusted = tool_schemas()
                check_shape(record, trusted[URN + 'assertions'], registry_for(trusted), 'E_ASSERTION_SPEC')
                inputs = {}
                for item in record['inputs']:
                    asset = self.assets.get(item['asset_id'])
                    if (not asset or asset['path'] != item['path'] or asset['sha256'] != item['sha256']
                            or item['input_id'] in inputs):
                        raise ContractError('E_INPUT', 'assertion input is not a frozen registered asset')
                    data = parse_json(self.asset_bytes(item['asset_id'], fixture))
                    if 'schema_ref' in item:
                        self.validate(data, item['schema_ref'])
                    inputs[item['input_id']] = data
                execute_assertions(record['assertions'], inputs)
        return 'shape' if validation['kind'] == 'json_schema' else 'semantic'

    def asset_bytes(self, asset_id: str, fixture: dict[str, Any]) -> bytes:
        if not isinstance(asset_id, str) or asset_id not in fixture['asset_refs'] or asset_id not in self.assets:
            raise ContractError('E_ASSET_REF', 'undeclared auxiliary fixture input')
        return self.files[self.assets[asset_id]['path']]

    def resolver_case(self, spec: dict[str, Any]) -> str:
        """Fixed parser fault injection; the package cannot supply executable code."""
        mutation = spec.get('mutation')
        if set(spec) != {'mutation'} or mutation not in RESOLVER_MUTATIONS:
            raise ContractError('E_VALIDATOR_INPUT', 'unknown resolver mutation')
        with tempfile.TemporaryDirectory(prefix='d0-parser-') as directory:
            root = Path(directory) / 'package'
            root.mkdir()
            for path, raw in self.files.items():
                output = safe_path(root, path)
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_bytes(raw)
            manifest = json.loads(self.raw_manifest)
            entry = manifest['entries'][0]
            if mutation == 'duplicate_urn':
                manifest['entries'].append(dict(entry))
            elif mutation == 'duplicate_path':
                manifest['assets'].append({'asset_id': 'duplicate', 'path': entry['path'], 'sha256': entry['sha256'], 'media_type': 'application/json'})
            elif mutation == 'missing_file':
                (root / entry['path']).unlink()
            elif mutation == 'digest_mismatch':
                (root / entry['path']).write_bytes(b'{}')
            elif mutation in {'path_escape', 'absolute_path'}:
                entry['path'] = '../outside.json' if mutation == 'path_escape' else '/outside.json'
            elif mutation == 'symlink_escape':
                (root / entry['path']).unlink()
                outside = Path(directory) / 'outside.json'
                outside.write_text('{}')
                (root / entry['path']).symlink_to(outside)
            elif mutation == 'missing_asset':
                manifest['fixtures'][0]['asset_refs'].append('absent')
            elif mutation == 'unknown_validator':
                manifest['fixtures'][0]['validation'] = {'kind': 'raw_case', 'validator_id': 'unknown.v1', 'selection': {'mode': 'document'}, 'expected_stage': 'parse'}
            elif mutation == 'wrong_dialect':
                manifest['schema_dialect'] = 'https://json-schema.org/draft-07/schema'
            else:
                schema = json.loads((root / entry['path']).read_bytes())
                if mutation == 'dynamic_ref':
                    schema['$dynamicRef'] = '#ignored'
                elif mutation == 'unused_bad_ref':
                    schema.setdefault('$defs', {})['Unused'] = {'$ref': 'urn:absent'}
                elif mutation == 'duplicate_anchor':
                    schema.setdefault('$defs', {})['Unused'] = {'$anchor': 'revision'}
                elif mutation == 'legacy_definition_ref':
                    schema['definitions'] = {'Unused': {'$ref': 'https://invalid.example/schema'}}
                elif mutation == 'legacy_dependency_ref':
                    schema['dependencies'] = {'unused': {'$ref': 'urn:absent'}}
                elif mutation in {'fragment_target_ref', 'annotation_data', 'semantic_input_ref'}:
                    schema['default'] = {'$ref': 'urn:absent'}
                    if mutation == 'fragment_target_ref':
                        schema.setdefault('$defs', {})['Indirect'] = {'$ref': '#/default'}
                    elif mutation == 'semantic_input_ref':
                        schema['default'] = {'type': 'object', 'properties': {'unused': {'$ref': 'urn:absent'}}}
                        fixture = next(f for f in manifest['fixtures'] if f['case_id'] == 'assertions-pointer-input')
                        target = root / fixture['path']
                        data = json.loads(target.read_bytes())
                        data['inputs'][0]['schema_ref'] = entry['schema_id'] + '#/default'
                        payload = (json.dumps(data) + '\n').encode()
                        target.write_bytes(payload)
                        fixture['sha256'] = sha256(payload)
                elif mutation in {'empty_selection', 'missing_pointer'}:
                    pass
                else:
                    schema['$ref'] = {'network_ref': 'https://invalid.example/schema',
                                      'unknown_urn': 'urn:absent',
                                      'unknown_fragment': entry['schema_id'] + '#/absent'}[mutation]
                raw = (json.dumps(schema, ensure_ascii=False, indent=2) + '\n').encode()
                (root / entry['path']).write_bytes(raw)
                entry['sha256'] = sha256(raw)
            (root / 'bundle-manifest.json').write_text(json.dumps(manifest))
            modified = Bundle(root / 'bundle-manifest.json')
            if mutation in {'empty_selection', 'missing_pointer'}:
                fixture = modified.manifest['fixtures'][0]
                fixture['validation']['selection'] = {'mode': 'ndjson_records'} if mutation == 'empty_selection' else {'mode': 'json_pointer', 'pointer': '/absent'}
                modified.files[fixture['path']] = b'\n' if mutation == 'empty_selection' else b'{}'
                modified.run_fixture(fixture)
        return 'parse'


def allowed_source(source: str, target: str) -> bool:
    if source.startswith('contracts/v1/cases/'):
        return target == source.replace('contracts/v1/cases/', 'fixtures/mf/', 1) and not source.endswith(('.py', '.sh', '.js'))
    if re.fullmatch(r'contracts/v1/[a-z0-9-]+\.schema\.json', source):
        return target == source.replace('contracts/', '', 1)
    if source == 'scripts/contracts/assertions.schema.json':
        return target == 'verification/assertions.schema.json'
    if source == 'contracts/v1/canonicalization-profile.json':
        return target == 'verification/canonicalization-profile.json'
    return source.startswith('contracts/verification/') and target == source.replace('contracts/', '', 1) and source.endswith(('.json', '.ndjson', '.txt'))


def match_result(case_id: str, expected: dict[str, Any], expected_stage: str, action,
                 permit_tool_rejection: bool = False, permit_decode_rejection: bool = False) -> dict[str, Any]:
    try:
        stage = action()
        actual = {'verdict': 'accept', 'error_code': None, 'stage': stage, 'pointer': ''}
    except ContractError as error:
        actual = {'verdict': 'reject', 'error_code': error.code, 'stage': error.stage, 'pointer': error.pointer}
    code = actual['error_code']
    allowed = (code is None or not str(code).startswith('E_') or permit_tool_rejection
               or permit_decode_rejection and code in {'E_SCHEMA_SHAPE', 'E_UTF8', 'E_JSON', 'E_DUPLICATE_KEY', 'E_SURROGATE', 'E_LIMIT'})
    return {'case_id': case_id, 'expected': dict(expected, stage=expected_stage), 'actual': actual,
            'matched': allowed and actual['verdict'] == expected['verdict'] and code == expected['error_code'] and actual['stage'] == expected_stage}


def frozen(value: Any) -> Any:
    """JSON value equality, including the distinction between booleans and numbers."""
    if isinstance(value, dict):
        return ('object', tuple((k, frozen(v)) for k, v in sorted(value.items())))
    if isinstance(value, list):
        return ('array', tuple(frozen(v) for v in value))
    if isinstance(value, bool):
        return ('bool', value)
    if isinstance(value, (int, Decimal)):
        return ('number', Decimal(value))
    return (type(value).__name__, value)


def execute_assertions(assertions: list[dict[str, Any]], inputs: dict[str, Any]) -> None:
    if not assertions or len(assertions) > MAX_ITEMS:
        raise ContractError('E_NO_CHECKS', 'assertions must be nonempty and bounded', 'semantic')

    def operand(spec: dict[str, Any]) -> Any:
        if set(spec) == {'literal'}:
            return spec['literal']
        if set(spec) != {'input', 'pointer'} or spec['input'] not in inputs:
            raise ContractError('E_INPUT', 'assertion input is missing', 'semantic')
        return pointer(inputs[spec['input']], spec['pointer'])

    def collection(spec: dict[str, Any]) -> list[Any]:
        value = operand(spec)
        if not isinstance(value, list):
            raise ContractError('E_ASSERTION_TYPE', 'expected a collection', 'semantic')
        if len(value) > MAX_ITEMS:
            raise ContractError('E_LIMIT', 'collection limit exceeded', 'semantic')
        return value

    def integer(spec: dict[str, Any]) -> int:
        value = operand(spec)
        if not isinstance(value, str) or not re.fullmatch(r'0|-?[1-9][0-9]*', value, re.ASCII):
            raise ContractError('E_ASSERTION_TYPE', 'expected a canonical decimal integer string', 'semantic')
        if len(value.lstrip('-')) > 4096:
            raise ContractError('E_LIMIT', 'integer digit limit exceeded', 'semantic')
        return int(value)

    def keys(rows: list[Any], paths: list[str]) -> list[Any]:
        if not paths:
            raise ContractError('E_ASSERTION_TYPE', 'complete key paths are required', 'semantic')
        return [tuple(frozen(pointer(row, path)) for path in paths) for row in rows]

    for index, assertion in enumerate(assertions):
        op = assertion.get('op')
        if op not in OPERATORS:
            raise ContractError('E_OPERATOR', 'unknown assertion operator', 'semantic')
        try:
            ok = False
            if op in {'eq', 'ne'}:
                equal = frozen(operand(assertion['left'])) == frozen(operand(assertion['right']))
                ok = equal if op == 'eq' else not equal
            elif op == 'unique':
                values = keys(collection(assertion['collection']), assertion['keys'])
                ok = len(values) == len(set(values))
            elif op == 'contains':
                ok = frozen(operand(assertion['value'])) in {frozen(v) for v in collection(assertion['collection'])}
            elif op == 'cardinality':
                ok = assertion['min'] <= len(collection(assertion['collection'])) <= assertion['max']
            elif op in {'int_add', 'int_sub', 'int_compare'}:
                left_integer, right_integer = integer(assertion['left']), integer(assertion['right'])
                if op == 'int_compare':
                    comparison = assertion['comparison']
                    if comparison not in {'eq', 'ne', 'lt', 'le', 'gt', 'ge'}:
                        raise ContractError('E_OPERATOR', 'unknown integer comparison', 'semantic')
                    ok = {'eq': left_integer == right_integer, 'ne': left_integer != right_integer,
                          'lt': left_integer < right_integer, 'le': left_integer <= right_integer,
                          'gt': left_integer > right_integer, 'ge': left_integer >= right_integer}[comparison]
                else:
                    ok = (left_integer + right_integer if op == 'int_add' else left_integer - right_integer) == integer(assertion['result'])
            elif op in {'join', 'key_intersection'}:
                if len(assertion['left_keys']) != len(assertion['right_keys']):
                    raise ContractError('E_ASSERTION_TYPE', 'join key arity differs', 'semantic')
                left = keys(collection(assertion['left']), assertion['left_keys'])
                right = keys(collection(assertion['right']), assertion['right_keys'])
                if op == 'join':
                    counts: dict[Any, int] = {}
                    for key in right:
                        counts[key] = counts.get(key, 0) + 1
                    ok = all(assertion['min'] <= counts.get(key, 0) <= assertion['max'] for key in left)
                else:
                    result = collection(assertion['result'])
                    if any(not isinstance(row, list) or len(row) != len(assertion['left_keys']) for row in result):
                        raise ContractError('E_ASSERTION_TYPE', 'intersection result must contain complete key tuples', 'semantic')
                    expected = {tuple(frozen(v) for v in row) for row in result}
                    ok = set(left) & set(right) == expected
            elif op == 'partial_order':
                nodes = collection(assertion['nodes'])
                edges = collection(assertion['edges'])
                if (not nodes or len(nodes) > 2048 or any(not isinstance(n, str) or not n for n in nodes)
                        or len(nodes) != len(set(nodes))):
                    raise ContractError('E_ASSERTION_TYPE', 'graph nodes must be unique nonempty IDs', 'semantic')
                graph: dict[str, set[str]] = {n: set() for n in nodes}
                indegree = dict.fromkeys(nodes, 0)
                for pair in edges:
                    if not isinstance(pair, list) or len(pair) != 2 or any(not isinstance(n, str) or n not in graph for n in pair):
                        raise ContractError('E_ASSERTION_TYPE', 'graph edge endpoints must exist', 'semantic')
                    start, end = pair
                    if end not in graph[start]:
                        graph[start].add(end)
                        indegree[end] += 1
                ready = [n for n in nodes if not indegree[n]]
                visited = 0
                while ready:
                    node = ready.pop()
                    visited += 1
                    for child in graph[node]:
                        indegree[child] -= 1
                        if not indegree[child]:
                            ready.append(child)
                ok = visited == len(nodes)
                required = collection(assertion['required'])
                if any(not isinstance(n, str) for n in required):
                    raise ContractError('E_ASSERTION_TYPE', 'required graph nodes must be IDs', 'semantic')
                ok = ok and set(required) <= set(nodes)
                for pair in collection(assertion['before']):
                    if not isinstance(pair, list) or len(pair) != 2 or any(not isinstance(n, str) or n not in graph for n in pair):
                        raise ContractError('E_ASSERTION_TYPE', 'reachability endpoints must exist', 'semantic')
                    start, end = pair
                    reached: set[str] = set()
                    pending = list(graph[start])
                    while pending:
                        node = pending.pop()
                        if node not in reached:
                            reached.add(node)
                            pending.extend(graph[node] - reached)
                    ok = ok and end in reached and start != end
            if not ok:
                raise ContractError(assertion['error_code'], 'declarative assertion failed', 'semantic', '/assertions/' + str(index))
        except (KeyError, TypeError, ValueError) as error:
            raise ContractError('E_ASSERTION_TYPE', 'invalid assertion operands', 'semantic', '/assertions/' + str(index)) from error


def source_bundle(path: Path) -> Bundle:
    if path.resolve() == TOOL_ROOT / 'contracts/v1/contract-manifest.json':
        return Bundle(path, TOOL_ROOT, TOOL_ROOT / 'contracts/v1/export-map.json')
    if path.name != 'bundle-manifest.json':
        raise ContractError('E_MANIFEST', 'exported package root requires bundle-manifest.json')
    return Bundle(path)


def validate_vendor(lock_path: Path, repo_root: Path) -> list[dict[str, Any]]:
    lock = parse_json(read_bytes(lock_path))
    trusted = tool_schemas()
    check_shape(lock, trusted[URN + 'lock'], registry_for(trusted), 'E_LOCK')
    packages = []
    identities: set[tuple[str, str]] = set()
    roots: set[str] = set()
    schemas: dict[str, Any] = {}
    anchors: dict[tuple[str, str], Any] = {}
    for entry in lock['packages']:
        identity = (entry['producer_project'], entry['bundle_version'])
        if identity in identities or entry['vendor_root'] in roots:
            raise ContractError('E_LOCK', 'duplicate package or vendor root')
        identities.add(identity)
        roots.add(entry['vendor_root'])
        root = safe_path(repo_root, entry['vendor_root'])
        manifest_path = safe_path(root, 'bundle-manifest.json')
        if sha256(read_bytes(manifest_path)) != entry['manifest_sha256']:
            raise ContractError('E_DIGEST', 'vendor manifest digest differs from lock')
        bundle = Bundle(manifest_path, defer_refs=True)
        if (bundle.manifest['producer_project'], bundle.manifest['bundle_version']) != identity:
            raise ContractError('E_LOCK', 'vendor identity differs from lock')
        if len(set(entry['imported_schema_ids'])) != len(entry['imported_schema_ids']) or not set(entry['imported_schema_ids']) <= bundle.schemas.keys():
            raise ContractError('E_LOCK', 'imported schema IDs are absent or duplicated')
        if schemas.keys() & bundle.schemas.keys():
            raise ContractError('E_DUPLICATE_URN', 'a schema URN has multiple package sources')
        schemas.update(bundle.schemas)
        anchors.update(bundle.anchors)
        packages.append(bundle)
    rows = []
    registry = registry_for(schemas)
    for bundle in packages:
        bundle.schemas, bundle.anchors, bundle.registry = schemas, anchors, registry
        bundle.check_references()
        rows.extend(bundle.run_cases())
    return rows


def standalone_assertions(path: Path, input_paths: list[Path]) -> list[dict[str, Any]]:
    spec = parse_json(read_bytes(path))
    trusted = tool_schemas()
    check_shape(spec, trusted[URN + 'assertions'], registry_for(trusted), 'E_ASSERTION_SPEC')
    supplied: dict[Path, bytes] = {}
    for input_path in input_paths:
        if input_path.is_symlink() or input_path.resolve() in supplied:
            raise ContractError('E_INPUT', 'duplicate or symlink input')
        supplied[input_path.resolve()] = read_bytes(input_path)
    inputs: dict[str, Any] = {}
    for item in spec['inputs']:
        frozen_path = safe_path(path.resolve().parent, item['path']).resolve()
        if frozen_path not in supplied or sha256(supplied[frozen_path]) != item['sha256'] or item['input_id'] in inputs:
            raise ContractError('E_INPUT', 'assertion input missing, duplicated or changed')
        data = parse_json(supplied.pop(frozen_path))
        if 'schema_ref' in item:
            # Standalone mode deliberately permits only the trusted common/core schemas.
            ref = item['schema_ref']
            base, _ = urldefrag(ref)
            if base not in trusted:
                raise ContractError('E_UNKNOWN_REF', 'standalone schema requires a locked bundle')
            check_shape(data, {'$ref': ref}, registry_for(trusted))
        inputs[item['input_id']] = data
    if supplied:
        raise ContractError('E_INPUT', 'undeclared auxiliary inputs')
    def action() -> str:
        execute_assertions(spec['assertions'], inputs)
        return 'semantic'
    return [match_result(path.stem, {'verdict': 'accept', 'error_code': None}, 'semantic', action)]


def input_receipts(args: Any) -> list[dict[str, Any]]:
    receipts = []
    for name in ('manifest', 'lock', 'assertions'):
        path = getattr(args, name, None)
        if path is not None:
            raw = read_bytes(path)
            item = {'kind': name, 'path': str(path), 'sha256': sha256(raw)}
            data = parse_json(raw)
            if name == 'manifest':
                for key in ('bundle_version', 'producer_project', 'source_revision'):
                    item[key] = data.get(key)
            receipts.append(item)
    for path in getattr(args, 'input', []):
        receipts.append({'kind': 'input', 'path': str(path), 'sha256': sha256(read_bytes(path))})
    return receipts


def write_report(path: Path, rows: list[dict[str, Any]], receipts: list[dict[str, Any]] | None = None) -> bool:
    failed = sum(not row['matched'] for row in rows)
    report = {'report_format_version': 'd0-report-v1', 'evidence_level': 'offline_fixture',
              'status': 'passed' if rows and not failed else 'failed',
              'product_execution': 'not_run', 'PG_execution': 'not_run',
              'input_receipts': receipts or [],
              'tool': {'validator_id': 'd0-assertions.v1', 'python': sys.version.split()[0],
                       'validator_sha256': sha256(read_bytes(Path(__file__))),
                       'requirements_lock_sha256': sha256(read_bytes(TOOL_ROOT / 'scripts/contracts/requirements.lock'))},
              'summary': {'executed': len(rows), 'matched': len(rows) - failed, 'failed': failed,
                          'missing': sum(not row['matched'] and row['actual']['error_code'] in {'E_NO_CHECKS', 'E_FILE', 'E_INPUT', 'E_DEPENDENCY'} for row in rows)},
              'results': rows}
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    print(json.dumps(report['summary'], sort_keys=True))
    return bool(rows) and failed == 0


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    for name in ('self-test', 'cases', 'assertions', 'vendor'):
        cmd = sub.add_parser(name)
        cmd.add_argument('--offline', action='store_true', required=True)
        cmd.add_argument('--report', type=Path, required=True)
        if name in ('self-test', 'cases'):
            cmd.add_argument('--manifest', type=Path, required=True)
            cmd.add_argument('--cases')
        elif name == 'vendor':
            cmd.add_argument('--lock', type=Path, required=True)
            cmd.add_argument('--repo-root', type=Path, default=Path.cwd())
        else:
            cmd.add_argument('--assertions', type=Path, required=True)
            cmd.add_argument('--input', type=Path, action='append', default=[])
    args = parser.parse_args()
    receipts: list[dict[str, Any]] = []
    try:
        check_environment()
        receipts = input_receipts(args)
        if args.command in ('self-test', 'cases'):
            bundle = source_bundle(args.manifest)
            rows = bundle.run_cases(args.cases)
        elif args.command == 'vendor':
            rows = validate_vendor(args.lock, args.repo_root.resolve())
        else:
            rows = standalone_assertions(args.assertions, args.input)
    except ContractError as error:
        rows = [{'case_id': 'preflight', 'expected': {'verdict': 'accept', 'error_code': None},
                 'actual': {'verdict': 'reject', 'error_code': error.code, 'stage': error.stage, 'pointer': error.pointer}, 'matched': False}]
    except Exception:
        # Invalid tool input must never turn into a skipped check or a passing negative case.
        rows = [{'case_id': 'tool', 'expected': {'verdict': 'accept', 'error_code': None},
                 'actual': {'verdict': 'reject', 'error_code': 'E_TOOL', 'stage': 'tool', 'pointer': ''}, 'matched': False}]
    return 0 if write_report(args.report, rows, receipts) else 1


if __name__ == '__main__':
    sys.exit(main())
