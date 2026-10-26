"""Read-only compatibility inventory; complete packages use artifact_packaging.py."""
import argparse
import json
from pathlib import Path

from artifact_contract import digest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plugins-dir", type=Path, default=Path("plugins"))
    args = parser.parse_args()
    rows = [{"path": str(path.relative_to(args.plugins_dir)), "sha256": digest(path.read_bytes())}
            for path in sorted(args.plugins_dir.glob("*/*/plugin.wasm")) if not path.is_symlink()]
    print(json.dumps({"legacy_inventory": rows, "provenance": "unknown"}))


if __name__ == "__main__":
    main()
