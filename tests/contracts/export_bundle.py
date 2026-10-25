"""Explicit source map exporter for inert ART fixture packages."""
from __future__ import annotations

import argparse
import json
import os
import tempfile
from pathlib import Path

from artifact_contract_support import ROOT, ContractError, contract_registry, manifest_bundle, mf, sha256


def export_bundle(manifest: Path, export_map: Path, output: Path, source_root: Path = ROOT) -> str:
    if output.exists() or output.is_symlink():
        raise ContractError('E_EXPORT_EXISTS', 'immutable export already exists')
    contract_registry()
    raw = manifest.read_bytes()
    spec = mf.parse_json(raw)
    mapping = mf.parse_json(export_map.read_bytes())
    if mapping.get('export_format_version') != 'd0-export-v1':
        raise ContractError('E_EXPORT_MAP', 'unsupported export mapping')
    registered = {e['path']: e['sha256'] for group in ('entries', 'fixtures', 'assets') for e in spec[group]}
    if len(registered) != sum(len(spec[g]) for g in ('entries', 'fixtures', 'assets')):
        raise ContractError('E_DUPLICATE_PATH', 'duplicate package path')
    sources = set()
    by_target = {}
    for entry in mapping['files']:
        source, target = entry['source'], entry['path']
        expected = ('v1/' + source.removeprefix('contracts/v1/')) if source.startswith('contracts/v1/') else source.replace('contracts/fixtures/v1/', 'fixtures/', 1)
        if (target != expected or not source.startswith(('contracts/v1/', 'contracts/fixtures/v1/artifact/'))
                or not source.endswith(('.json', '.md', '.ndjson', '.txt', '.bin', '.tgz', '.tar', '.gz'))
                or target in by_target or source in sources):
            raise ContractError('E_EXPORT_MAP', 'non-whitelisted or duplicate export')
        mf.safe_path(source_root, source)
        by_target[target] = source
        sources.add(source)
    if set(registered) != set(by_target):
        raise ContractError('E_EXPORT_MAP', 'mapping differs from full manifest')
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='art-export-', dir=output.parent) as directory:
        stage = Path(directory) / 'package'
        stage.mkdir()
        for target, source in by_target.items():
            content = mf.read_bytes(mf.safe_path(source_root, source))
            if sha256(content) != registered[target]:
                raise ContractError('E_DIGEST', 'source asset bytes differ')
            destination = mf.safe_path(stage, target)
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(content)
        (stage / 'bundle-manifest.json').write_bytes(raw)
        bundle = manifest_bundle(stage)
        rows = bundle.run_cases()
        if not rows or not all(row['matched'] for row in rows):
            bad = [row for row in rows if not row['matched']]
            raise ContractError('E_CASE_MISMATCH', json.dumps(bad, default=str), 'tool')
        os.rename(stage, output)
    return sha256(raw)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--offline', action='store_true', required=True)
    parser.add_argument('--manifest', type=Path, default=ROOT / 'contracts/v1/bundle-manifest.json')
    parser.add_argument('--export-map', type=Path, default=ROOT / 'contracts/v1/export-map.json')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    try:
        print('manifest_sha256=' + export_bundle(args.manifest, args.export_map, args.output))
        return 0
    except ContractError as error:
        print(error.code + ': ' + str(error))
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
