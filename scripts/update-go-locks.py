#!/usr/bin/env python3
"""Explicitly reconcile Go module locks after an upstream/custom source merge."""

from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from plugin_catalog import select_releases


def main() -> int:
    image = json.loads((ROOT / "plugins/release/toolchains.json").read_bytes())["images"]["go"]
    directories = [ROOT / "plugins/proxy-wasm-go-sdk", ROOT / "plugins/wasm-go"]
    directories += [source.directory for source in select_releases(ROOT) if source.implementation == "go"]
    modules = Path(subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip())
    cache = Path(subprocess.check_output(["go", "env", "GOCACHE"], text=True).strip())
    run = ROOT / "build/module-locks" / datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    run.mkdir(parents=True)
    changes = []
    for directory in directories:
        paths = [directory / "go.mod", directory / "go.sum"]
        before = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() if path.exists() else None for path in paths}
        argv = ["docker", "run", "--rm", "--platform", "linux/amd64", "--user", f"{os.getuid()}:{os.getgid()}",
                "-v", f"{ROOT / 'plugins'}:/workspace/plugins", "-v", f"{modules}:/cache/modules",
                "-v", f"{cache}:/cache/build", "-w", "/workspace/" + directory.relative_to(ROOT).as_posix(),
                "-e", "GOMODCACHE=/cache/modules", "-e", "GOCACHE=/cache/build", "-e", "GOTOOLCHAIN=local",
                *[item for name in ["HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "GOPROXY"] if name in os.environ
                  for item in ["-e", name]], image, "go", "mod", "tidy"]
        with (run / (directory.name + ".log")).open("ab") as log:
            subprocess.run(argv, stdout=log, stderr=subprocess.STDOUT, check=True)
        after = {path.name: hashlib.sha256(path.read_bytes()).hexdigest() if path.exists() else None for path in paths}
        changes.append({"module": str(directory.relative_to(ROOT)), "before": before, "after": after})
        print(json.dumps({"module": str(directory.relative_to(ROOT)), "changed": before != after}), flush=True)
    (run / "receipt.json").write_text(json.dumps({"toolchain": image, "modules": changes}, indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
