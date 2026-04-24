from __future__ import annotations

import hashlib
from datetime import datetime
from pathlib import Path


def calculate_md5(file_path: str | Path, chunk_size: int = 4096) -> str:
    md5_hash = hashlib.md5()
    with open(file_path, "rb") as file_obj:
        while chunk := file_obj.read(chunk_size):
            md5_hash.update(chunk)
    return md5_hash.hexdigest()


def build_metadata_text(wasm_path: str | Path) -> str:
    wasm_path = Path(wasm_path)
    parent_name = wasm_path.parent.name
    grandparent_name = wasm_path.parent.parent.name
    file_label = f"{grandparent_name}:{parent_name}"
    stat_info = wasm_path.stat()
    size = stat_info.st_size
    mtime = datetime.fromtimestamp(stat_info.st_mtime).isoformat()
    ctime = datetime.fromtimestamp(stat_info.st_ctime).isoformat()
    md5_value = calculate_md5(wasm_path)

    return (
        f"File: {file_label}\n"
        f"Size: {size} bytes\n"
        f"Last Modified: {mtime}\n"
        f"Created: {ctime}\n"
        f"MD5: {md5_value}\n"
    )


def generate_metadata_file(wasm_path: str | Path) -> Path:
    wasm_path = Path(wasm_path)
    metadata_path = wasm_path.parent / "metadata.txt"
    metadata_path.write_text(build_metadata_text(wasm_path), encoding="utf-8")
    return metadata_path


def generate_all_metadata(plugins_dir: str | Path = "plugins") -> list[Path]:
    plugins_dir = Path(plugins_dir)
    generated: list[Path] = []
    for wasm_path in plugins_dir.rglob("plugin.wasm"):
        generated.append(generate_metadata_file(wasm_path))
    return generated


def main() -> None:
    generate_all_metadata()


if __name__ == "__main__":
    main()
