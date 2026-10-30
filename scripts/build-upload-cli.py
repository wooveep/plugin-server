#!/usr/bin/env python3
"""Package only the runtime modules and fixed production schema into a zipapp."""
import argparse
import shutil
import tempfile
import zipapp
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MODULES = ("upload_plugin.py", "artifact_import.py", "artifact_contract.py", "artifact_storage.py", "artifact_retention.py",
           "artifact_packaging.py", "plugin_schema_conversion.py", "mf_canonical.py")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="artifact-cli-") as temporary:
        stage = Path(temporary)
        for name in MODULES:
            shutil.copyfile(ROOT / name, stage / name)
        shutil.copytree(ROOT / "artifact_schemas", stage / "artifact_schemas")
        shutil.copyfile(ROOT / "requirements.lock", stage / "requirements.lock")
        (stage / "__main__.py").write_text("from upload_plugin import main\nmain()\n")
        zipapp.create_archive(stage, args.output, interpreter="/usr/bin/env python3")


if __name__ == "__main__":
    main()
