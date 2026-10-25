"""Offline ART test boundary; shared byte algorithms are pinned MF tool imports."""
from __future__ import annotations

import hashlib
import importlib.util
import json
from functools import lru_cache
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
URN = 'urn:modelfusion:contracts:v1:'
FIELDS = ('canonicalization_version', 'schema_version', 'plugin_name', 'plugin_version',
          'files', 'wasm_path', 'config_schema_path', 'defaults_path',
          'documentation_paths', 'environment_refs', 'source')


def verify_tool_files() -> None:
    lock = json.loads((ROOT / 'tests/contracts/tool-manifest.json').read_bytes())
    if lock.get('source_revision') != '753704b29a6ea40815d45dcbbce16cc0d91bace2' or len(lock.get('files', [])) != 16:
        raise RuntimeError('incomplete fixed MF test tool manifest')
    for entry in lock['files']:
        path = ROOT / entry['path']
        if path.is_symlink() or hashlib.sha256(path.read_bytes()).hexdigest() != entry['sha256']:
            raise RuntimeError('fixed MF test tool bytes differ')


verify_tool_files()
_TOOL_PATH = ROOT / 'tests/contracts/vendor/mf_tools/scripts/contracts/validate_bundle.py'
_SPEC = importlib.util.spec_from_file_location('art_pinned_mf_validator', _TOOL_PATH)
assert _SPEC is not None and _SPEC.loader is not None
mf = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(mf)
ContractError = mf.ContractError
parse_json = mf.parse_json
canonicalize = mf.canonicalize
sha256 = mf.sha256


@lru_cache(maxsize=1)
def contract_registry() -> tuple[dict[str, Any], Any]:
    verify_tool_files()
    mf.check_environment()
    rows = mf.validate_vendor(ROOT / 'contracts/vendor/lock.json', ROOT)
    if not rows or not all(row['matched'] for row in rows):
        raise ContractError('E_COMMON', 'fixed common corpus differs', 'tool')
    schemas = mf.tool_schemas()
    for name in ('package-descriptor', 'artifact-manifest'):
        schema = mf.schema_numbers(parse_json((ROOT / 'contracts/v1' / (name + '.schema.json')).read_bytes()))
        mf.Draft202012Validator.check_schema(schema)
        schemas[schema['$id']] = schema
    registry = mf.registry_for(schemas)
    # Resolve every declared reference, including unused branches, without network retrieval.
    for schema in schemas.values():
        for node in mf.schema_nodes(schema):
            if '$ref' in node:
                reference = node['$ref']
                base, _, fragment = reference.partition('#')
                if base.startswith(('https:', 'http:')):
                    raise ContractError('E_NETWORK_REF', 'network schema reference')
                target = schemas.get(base or schema['$id'])
                if target is None:
                    raise ContractError('E_UNKNOWN_REF', 'unlocked schema reference')
                if fragment:
                    mf.pointer(target, fragment, 'E_FRAGMENT')
    return schemas, registry


def validate_document(raw: bytes, kind: str, *, check_digest: bool = True) -> dict[str, Any]:
    """Validate descriptor or server receipt and return exact projected content identity."""
    schemas, registry = contract_registry()
    if kind not in ('descriptor', 'manifest'):
        raise ValueError('document kind must be descriptor or manifest')
    value = parse_json(raw)
    schema_id = URN + ('package-descriptor' if kind == 'descriptor' else 'artifact-manifest')
    mf.check_shape(value, {'$ref': schema_id}, registry)
    files = value['files']
    by_path: dict[str, dict[str, Any]] = {}
    for entry in files:
        if entry['path'] in by_path:
            raise ContractError('ART_DUPLICATE_PATH', 'duplicate declared file path', 'semantic', '/files')
        by_path[entry['path']] = entry
    if len({e['name'] for e in value['environment_refs']}) != len(value['environment_refs']):
        raise ContractError('ART_DUPLICATE_ENV', 'duplicate environment reference', 'semantic', '/environment_refs')
    for field, role in [('wasm_path', 'wasm'), ('config_schema_path', 'config_schema'), ('defaults_path', 'defaults')]:
        if by_path.get(value[field], {}).get('role') != role:
            raise ContractError('ART_ROLE_REFERENCE', 'selected path has wrong or missing role', 'semantic', '/' + field)
    for role in ('wasm',):
        if sum(entry['role'] == role for entry in files) != 1:
            raise ContractError('ART_ROLE_CARDINALITY', 'exactly one Wasm must be selected', 'semantic', '/files')
    docs = {entry['path'] for entry in files if entry['role'] == 'documentation'}
    if docs != set(value['documentation_paths']):
        raise ContractError('ART_ROLE_REFERENCE', 'documentation references differ from declarations', 'semantic', '/documentation_paths')
    if any(entry['path'] == 'package.json' for entry in files):
        raise ContractError('ART_DESCRIPTOR_SELF_REFERENCE', 'descriptor cannot list itself', 'semantic', '/files')
    projection = {name: value[name] for name in FIELDS}
    projection['files'] = sorted(files, key=lambda item: item['path'])
    projection['environment_refs'] = sorted(value['environment_refs'], key=lambda item: item['name'])
    projection['documentation_paths'] = sorted(value['documentation_paths'])
    normalized = canonicalize(json.dumps(projection, ensure_ascii=False, separators=(',', ':')).encode())
    digest = 'sha256:' + sha256(normalized)
    if kind == 'manifest' and check_digest and value['content_digest'] != digest:
        raise ContractError('ART_CONTENT_DIGEST', 'server content digest differs', 'canonical', '/content_digest')
    return {'document': value, 'canonical_bytes': normalized, 'content_digest': digest}


def manifest_bundle(root: Path, common_root: Path | None = None) -> Any:
    """Bind a complete ART export and an independent complete common package."""
    common_root = common_root or ROOT / 'contracts/vendor/ocloudmodelfusion'
    common_manifest = common_root / 'bundle-manifest.json'
    if sha256(common_manifest.read_bytes()) != '09d0906d9f09b04f24266dee6bfab05785c5245373cc222612ec5b3b11e036f5':
        raise ContractError('E_DIGEST', 'common input differs from fixed release')
    common = mf.Bundle(common_manifest)
    art = mf.Bundle(root / 'bundle-manifest.json', defer_refs=True)
    if art.schemas.keys() & common.schemas.keys():
        raise ContractError('E_DUPLICATE_URN', 'duplicate package schema source')
    schemas = {**common.schemas, **art.schemas}
    art.schemas = schemas
    art.anchors = {**common.anchors, **art.anchors}
    art.registry = mf.registry_for(schemas)
    art.check_references()
    return art
