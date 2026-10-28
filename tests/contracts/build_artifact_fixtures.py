"""Deterministically rebuild inert fixture archives from frozen JSON recipes."""
from __future__ import annotations

import argparse
import base64
import gzip
import io
import json
import tarfile
from pathlib import Path
from typing import Any


def build_archive(recipe: dict[str, Any]) -> bytes:
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w', format=tarfile.USTAR_FORMAT) as archive:
        for entry in recipe['members']:
            content = base64.b64decode(entry['base64'], validate=True) if 'base64' in entry else entry.get('text', '').encode()
            item = tarfile.TarInfo(entry['path'])
            item.size = len(content)
            item.mode = 0o644
            item.mtime = int(recipe.get('tar_mtime', 0))
            item.uid = item.gid = 0
            item.uname = item.gname = ''
            if entry.get('type') in ('symlink', 'hardlink'):
                item.type = tarfile.SYMTYPE if entry['type'] == 'symlink' else tarfile.LNKTYPE
                item.linkname = entry.get('target', 'outside')
                item.size = 0
                content = b''
            archive.addfile(item, io.BytesIO(content))
    tar_bytes = stream.getvalue()
    if recipe.get('tar_cut_bytes'):
        tar_bytes = tar_bytes[:-recipe['tar_cut_bytes']]
    if recipe.get('declared_size_delta'):
        header = bytearray(tar_bytes[:512])
        size = int(header[124:136].rstrip(b'\x00 '), 8) + recipe['declared_size_delta']
        header[124:136] = ('%011o\0' % size).encode()
        header[148:156] = b'        '
        header[148:156] = ('%06o\0 ' % sum(header)).encode()
        tar_bytes = bytes(header) + tar_bytes[512:]
    if recipe.get('tar_trailing_base64'):
        tar_bytes += base64.b64decode(recipe['tar_trailing_base64'], validate=True)
    # GzipFile keeps the frozen OS=255 header across supported Python versions.
    compressed = io.BytesIO()
    with gzip.GzipFile(filename='', mode='wb', fileobj=compressed, compresslevel=9, mtime=recipe.get('gzip_mtime', 0)) as output:
        output.write(tar_bytes)
    result = compressed.getvalue()
    if recipe.get('gzip_cut_bytes'):
        result = result[:-recipe['gzip_cut_bytes']]
    return result


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--recipe', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    content = build_archive(json.loads(args.recipe.read_bytes()))
    if args.output.exists():
        parser.error('output must be a new fixture path')
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_bytes(content)
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
