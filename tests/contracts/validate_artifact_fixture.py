"""Read-only offline ART fixture validator. No extraction or product imports."""
from __future__ import annotations

import argparse
import copy
import gzip
import io
import json
import posixpath
import tarfile
import zlib
from pathlib import Path
from typing import Any, NoReturn
from urllib.parse import quote, urldefrag, unquote
from jsonschema.exceptions import SchemaError

from artifact_contract_support import ROOT, ContractError, canonicalize, contract_registry, mf, parse_json, sha256, validate_document


def reject(code: str, stage: str, pointer: str = '', path: str | None = None) -> NoReturn:
    error = ContractError(code, 'offline fixture rule failed', stage, pointer)
    error.member = path
    raise error


def safe_member(name: str) -> bool:
    return bool(name and not name.startswith('/') and '\\' not in name
                and not any(ord(c) < 32 for c in name)
                and all(segment not in ('', '.', '..') for segment in name.split('/')))


def check_wasm(raw: bytes) -> None:
    """Validate complete empty-core/custom-only fixture modules; ABI remains untested."""
    if raw[:8] != b'\x00asm\x01\x00\x00\x00':
        reject('ART_WASM', 'wasm', '/wasm_path')
    offset = 8

    def uint(data: bytes, start: int) -> tuple[int, int]:
        value = 0
        for count in range(5):
            if start + count >= len(data):
                reject('ART_WASM', 'wasm', '/wasm_path')
            byte = data[start + count]
            value |= (byte & 0x7f) << (7 * count)
            if count == 4 and byte & 0xf0:
                reject('ART_WASM', 'wasm', '/wasm_path')
            if not byte & 0x80:
                return value, start + count + 1
        reject('ART_WASM', 'wasm', '/wasm_path')
        raise AssertionError('unreachable')

    while offset < len(raw):
        section_id = raw[offset]
        size, begin = uint(raw, offset + 1)
        end = begin + size
        if section_id > 12 or end > len(raw):
            reject('ART_WASM', 'wasm', '/wasm_path')
        # These inert D0 fixtures deliberately have no executable standard sections.
        # Unsupported complete modules fail closed rather than passing partial parsing.
        if section_id != 0:
            reject('ART_WASM_PROFILE', 'wasm', '/wasm_path')
        name_size, name_start = uint(raw[begin:end], 0)
        name_end = name_start + name_size
        if name_end > size:
            reject('ART_WASM', 'wasm', '/wasm_path')
        try:
            raw[begin + name_start:begin + name_end].decode('utf-8')
        except UnicodeDecodeError:
            reject('ART_WASM', 'wasm', '/wasm_path')
        offset = end


USER_JSON_ERRORS = {'E_UTF8', 'E_JSON', 'E_DUPLICATE_KEY', 'E_SURROGATE', 'E_LIMIT',
                    'E_SCHEMA_SHAPE', 'E_POINTER', 'E_FRAGMENT'}


def validate_defaults(descriptor: dict[str, Any], payload: dict[str, bytes]) -> None:
    declared = {f['path']: f['role'] for f in descriptor['files']}
    pending = [descriptor['config_schema_path']]
    schemas: dict[str, Any] = {}
    seen = set()
    while pending:
        path = pending.pop()
        if path in seen:
            continue
        seen.add(path)
        try:
            schema = mf.schema_numbers(parse_json(payload[path]))
            if not isinstance(schema, dict) or schema.get('$schema') != mf.DIALECT:
                reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
            mf.Draft202012Validator.check_schema(schema)
        except ContractError as error:
            if error.code == 'ART_CONFIG_SCHEMA':
                raise
            if error.code in USER_JSON_ERRORS and error.stage not in ('tool', 'environment'):
                reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
            raise
        except SchemaError:
            reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
        schema = copy.deepcopy(schema)
        resource_id = 'urn:modelfusion:fixture:payload:' + quote(path, safe='')
        for node in mf.schema_nodes(schema):
            if '$dynamicRef' in node or '$dynamicAnchor' in node or ('$id' in node and node is not schema):
                reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
            if '$ref' not in node:
                continue
            base, fragment = urldefrag(node['$ref'])
            if base == 'urn:modelfusion:contracts:v1:common':
                continue
            if ':' in base or base.startswith('/') or '\\' in base:
                reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
            target = posixpath.normpath(posixpath.join(posixpath.dirname(path), base)) if base else path
            if not safe_member(target) or target not in payload or declared.get(target) != 'config_schema':
                reject('ART_CONFIG_SCHEMA', 'schema', '/config_schema_path', path)
            pending.append(target)
            node['$ref'] = 'urn:modelfusion:fixture:payload:' + quote(target, safe='') + ('#' + fragment if fragment else '')
        schema['$id'] = resource_id
        schemas[resource_id] = schema
    contract_registry()
    # Reuse the fixed public resolver for URI decoding, schema pointers and anchors.
    resolver = mf.Bundle(ROOT / 'contracts/vendor/ocloudmodelfusion/bundle-manifest.json')
    resolver.schemas = {**resolver.schemas, **schemas}
    for uri, schema in schemas.items():
        for node in mf.schema_nodes(schema):
            if '$anchor' in node:
                identity = (uri, node['$anchor'])
                if identity in resolver.anchors:
                    reject('ART_CONFIG_REFERENCE', 'schema', '/config_schema_path')
                resolver.anchors[identity] = node
    for uri, schema in schemas.items():
        for node in mf.schema_nodes(schema):
            if '$ref' in node:
                try:
                    resolver.resolve(node['$ref'], uri)
                    # The fixed resolver has validated escaping and the target. Normalize
                    # that URI spelling for the schema engine's anchor lookup adapter.
                    base, fragment = urldefrag(node['$ref'])
                    decoded = unquote(fragment, errors='strict')
                    # JSON Pointer lookup already decodes once inside the engine.
                    # Only its anchor lookup needs the spelling adapter.
                    if decoded and not decoded.startswith('/'):
                        node['$ref'] = (base or uri) + '#' + decoded
                except ContractError as error:
                    if error.code in ('E_FRAGMENT', 'E_UNKNOWN_REF', 'E_REF_URI') and error.stage not in ('tool', 'environment'):
                        reject('ART_CONFIG_REFERENCE', 'schema', '/config_schema_path')
                    raise
    registry = mf.registry_for(resolver.schemas)
    try:
        value = parse_json(payload[descriptor['defaults_path']])
        reference = 'urn:modelfusion:fixture:payload:' + quote(descriptor['config_schema_path'], safe='')
        mf.check_shape(value, {'$ref': reference}, registry)
    except ContractError as error:
        if error.code in USER_JSON_ERRORS and error.stage not in ('tool', 'environment'):
            reject('ART_DEFAULTS', 'defaults', '/defaults_path', descriptor['defaults_path'])
        raise


def validate_package(path: Path, *, case_id: str, policy: dict[str, Any] | None = None) -> dict[str, Any]:
    report: dict[str, Any] = {'report_format_version': 'artifact-validation-report-v1',
                              'case_id': case_id, 'evidence_level': 'offline_fixture',
                              'product_execution': 'not_run', 'gateway_abi': 'not_run',
                              'wasm_validation_profile': 'empty-core-custom-sections-v1',
                              'result': 'execution_error', 'validation_stage': 'execution', 'errors': [],
                              'statistics': {'archive_bytes': 0, 'members': 0, 'payload_bytes': 0}}
    try:
        if policy is None:
            policy = json.loads((ROOT / 'contracts/fixtures/v1/artifact/package-cases.json').read_bytes())['policy']
        with path.open('rb') as source:
            raw = source.read(policy['max_archive_bytes'] + 1)
        report['statistics']['archive_bytes'] = len(raw)
        if len(raw) > policy['max_archive_bytes']:
            report['statistics']['archive_bytes'] = path.stat().st_size
            report['archive_digest_status'] = 'not_computed_over_limit'
            reject('ART_LIMIT', 'limits')
        report['archive_digest'] = 'sha256:' + sha256(raw)
        try:
            with gzip.GzipFile(fileobj=io.BytesIO(raw)) as compressed:
                tar_bytes = compressed.read(policy['max_tar_bytes'] + 1)
            if len(tar_bytes) > policy['max_tar_bytes']:
                reject('ART_LIMIT', 'limits')
            if len(tar_bytes) % 512 or tar_bytes[-1024:] != bytes(1024):
                reject('ART_ARCHIVE', 'archive')
            members: dict[str, bytes] = {}
            payload_end = 0
            with tarfile.open(fileobj=io.BytesIO(tar_bytes), mode='r:') as archive:
                for member in archive:
                    if member.offset_data + member.size > len(tar_bytes) - 1024:
                        reject('ART_ARCHIVE', 'archive')
                    report['statistics']['members'] += 1
                    if report['statistics']['members'] > policy['max_members'] or member.size > policy['max_member_bytes']:
                        reject('ART_LIMIT', 'limits')
                    if not safe_member(member.name):
                        reject('ART_ARCHIVE_PATH', 'archive')
                    if member.name in members:
                        reject('ART_ARCHIVE_DUPLICATE', 'archive', path=member.name)
                    if not member.isfile() or member.type not in (tarfile.REGTYPE, tarfile.AREGTYPE):
                        reject('ART_ARCHIVE_LINK', 'archive')
                    if member.mode & 0o111 or Path(member.name).suffix in ('.sh', '.py', '.js', '.exe'):
                        reject('ART_ARCHIVE_EXECUTABLE', 'archive')
                    stream = archive.extractfile(member)
                    if stream is None:
                        reject('ART_ARCHIVE', 'archive')
                    content = stream.read(member.size + 1)
                    if len(content) != member.size:
                        reject('ART_ARCHIVE', 'archive')
                    members[member.name] = content
                    payload_end = member.offset_data + ((member.size + 511) // 512) * 512
                    if member.name != 'package.json':
                        report['statistics']['payload_bytes'] += len(content)
                        if report['statistics']['payload_bytes'] > policy['max_total_payload_bytes']:
                            reject('ART_LIMIT', 'limits')
            if len(tar_bytes) - payload_end < 1024 or any(tar_bytes[payload_end:]):
                reject('ART_ARCHIVE', 'archive')
        except (tarfile.TarError, EOFError, gzip.BadGzipFile, zlib.error):
            reject('ART_ARCHIVE', 'archive')
        if 'package.json' not in members:
            reject('ART_DESCRIPTOR', 'descriptor')
        report['descriptor_digest'] = 'sha256:' + sha256(members['package.json'])
        validated = validate_document(members['package.json'], 'descriptor')
        descriptor = validated['document']
        files = {entry['path']: entry for entry in descriptor['files']}
        if set(members) != set(files) | {'package.json'}:
            reject('ART_FILE_SET', 'payload', '/files')
        for name, entry in files.items():
            if str(len(members[name])) != entry['size_bytes']:
                reject('ART_FILE_SIZE', 'payload', '/files', name)
            if 'sha256:' + sha256(members[name]) != entry['sha256']:
                reject('ART_FILE_DIGEST', 'payload', '/files', name)
        allowed = {json.dumps(ref, sort_keys=True) for ref in policy['allowed_environment_refs']}
        if any(json.dumps(ref, sort_keys=True) not in allowed for ref in descriptor['environment_refs']):
            reject('ART_ENVIRONMENT_REF', 'policy', '/environment_refs')
        if any(policy['synthetic_forbidden_marker'].encode() in content for content in members.values()):
            reject('ART_SYNTHETIC_SECRET', 'policy')
        source_digest = 'sha256:' + sha256(canonicalize(json.dumps(descriptor['source'], separators=(',', ':')).encode()))
        if descriptor['source']['verification'] == 'verified' and source_digest not in policy['approved_source_digests']:
            reject('ART_SOURCE_EVIDENCE', 'provenance', '/source')
        check_wasm(members[descriptor['wasm_path']])
        validate_defaults(descriptor, members)
        report.update(result='valid', validation_stage='complete', normalized_content_digest=validated['content_digest'])
        report['expected_files'] = [{'path': 'package.json', 'size_bytes': str(len(members['package.json'])), 'sha256': report['descriptor_digest']}] + descriptor['files']
    except ContractError as error:
        tool_fault = error.stage in ('tool', 'environment') or (not error.code.startswith('ART_') and error.code not in USER_JSON_ERRORS)
        report.update(result='execution_error' if tool_fault else 'invalid',
                      validation_stage='execution' if tool_fault else error.stage)
        item = {'code': error.code, 'pointer': error.pointer}
        if getattr(error, 'member', None):
            item['path'] = error.member
        report['errors'] = [item]
    except OSError:
        report.update(result='execution_error', validation_stage='execution', errors=[{'code': 'ART_IO', 'pointer': ''}])
    except Exception:
        report.update(result='execution_error', validation_stage='execution', errors=[{'code': 'ART_TOOL', 'pointer': ''}])
    return report


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--offline', action='store_true', required=True)
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--case-id', required=True)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    report = validate_package(args.archive, case_id=args.case_id)
    args.report.parent.mkdir(parents=True, exist_ok=True)
    args.report.write_text(json.dumps(report, indent=2) + '\n')
    return 0 if report['result'] == 'valid' else 2 if report['result'] == 'execution_error' else 1


if __name__ == '__main__':
    raise SystemExit(main())
