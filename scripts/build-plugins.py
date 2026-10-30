#!/usr/bin/env python3
"""Build the catalog's full formal set, or one explicitly selected implementation."""

import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import uuid

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT))
from plugin_catalog import load_catalog, select_releases, source_inputs


def build_one(root: Path, source, *, host: bool, images: dict, run: Path) -> dict:
    version = source.version or "unversioned"
    output = root / "build" / ("native" if source.distribution == "native" else "plugins") / source.name / version
    output.mkdir(parents=True, exist_ok=True)
    artifact = output / ("filter.so" if source.distribution == "native" else "plugin.wasm")
    receipt_file = output / "build-receipt.json"
    inputs = source_inputs(root, source)
    inputs_digest = hashlib.sha256(json.dumps(inputs, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    # Evidence-only commits must not mint a different source identity for an
    # unchanged immutable plugin. Bind to the latest commit touching its inputs.
    commit = subprocess.check_output(["git", "log", "-1", "--format=%H", "--", *inputs], cwd=root, text=True).strip()
    toolchain_key = {"go": "go", "native-go": "go", "cpp": "cpp", "rust": "rust", "assemblyscript": "node"}[source.implementation]
    if host:
        version_command = {"go": ["go", "version"], "cpp": ["bazel", "--version"],
                           "rust": ["rustc", "--version"], "node": ["node", "--version"]}[toolchain_key]
        toolchain = "host: " + subprocess.check_output(version_command, text=True).strip()
    else:
        toolchain = images[toolchain_key]
    if receipt_file.exists() and artifact.exists():
        previous = json.loads(receipt_file.read_bytes())
        if (previous["source_commit"] == commit and previous["source_inputs"] == inputs
                and previous["toolchain"] == toolchain
                and previous["sha256"] == "sha256:" + hashlib.sha256(artifact.read_bytes()).hexdigest()):
            print(json.dumps({"plugin_name": source.name, "plugin_version": source.version, "cached": True}), flush=True)
            return previous
    receipt_file.unlink(missing_ok=True)
    # Bazel outputs may be read-only. A cache miss replaces this disposable
    # build output instead of attempting to overwrite the previous file.
    artifact.unlink(missing_ok=True)
    environment = dict(os.environ)
    commands = []
    started = datetime.now(timezone.utc).isoformat()
    cache = root / "build/cache"
    cache.mkdir(parents=True, exist_ok=True)
    relative_output = artifact.relative_to(root / "build")
    container_output = "/output/" + relative_output.as_posix()

    def invoke(argv, *, directory=source.directory, env=environment):
        commands.append(argv)
        with (run / (source.name + ".log")).open("ab") as log:
            subprocess.run(argv, cwd=directory, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)

    def container(image, argv, *, directory=source.directory, variables=None, extra_mounts=None):
        return ["docker", "run", "--rm", "--platform", "linux/amd64",
                "--user", f"{os.getuid()}:{os.getgid()}",
                "-v", f"{root}:/workspace:ro", "-v", f"{root / 'build'}:/output",
                "-w", "/workspace/" + directory.relative_to(root).as_posix(),
                *[item for name, value in (variables or {}).items() for item in ["-e", name + "=" + value]],
                *[item for name in ["HTTPS_PROXY", "HTTP_PROXY", "NO_PROXY", "GOPROXY"]
                  if name in environment for item in ["-e", name]],
                *(extra_mounts or []), image, *argv]

    if source.implementation in {"go", "native-go"}:
        native = source.distribution == "native"
        variables = {"GOOS": "linux" if native else "wasip1", "GOARCH": "amd64" if native else "wasm",
                     "CGO_ENABLED": "1" if native else "0", "GOTOOLCHAIN": "local"}
        argv = ["go", "build", "-p=2", "-mod=readonly", "-trimpath", "-buildvcs=false",
                "-buildmode=c-shared", "-o", str(artifact) if host else container_output, "."]
        if host:
            invoke(argv, env=dict(environment, **variables))
        else:
            modules = Path(subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip())
            go_cache = Path(subprocess.check_output(["go", "env", "GOCACHE"], text=True).strip())
            modules.mkdir(parents=True, exist_ok=True)
            go_cache.mkdir(parents=True, exist_ok=True)
            variables.update({"GOMODCACHE": "/cache/modules", "GOCACHE": "/cache/build"})
            invoke(container(images["go"], argv, variables=variables,
                             extra_mounts=["-v", f"{modules}:/cache/modules", "-v", f"{go_cache}:/cache/build"]))
    elif source.implementation == "cpp":
        name = source.directory.name
        if not re.fullmatch(r"[a-z0-9_]+", name):
            raise ValueError("invalid C++ build target: " + name)
        workspace = root / "plugins/wasm-cpp"
        prefix = str(cache / "cpp-links-") if host else "/output/cache/cpp-links-"
        user_root = str(cache / "bazel") if host else "/output/cache/bazel"
        build = ["bazel", "--output_user_root=" + user_root, "build", "--jobs=2",
                 "--symlink_prefix=" + prefix, f"//extensions/{name}:{name}.wasm"]
        compiled = prefix + f"bin/extensions/{name}/{name}.wasm"
        if host:
            invoke(build, directory=workspace)
            shutil.copyfile(compiled, artifact)
        else:
            command = shlex.join(build) + " && " + shlex.join(["cp", compiled, container_output])
            invoke(container(images["cpp"], ["bash", "-ec", command], directory=workspace))
    elif source.implementation == "rust":
        target = output / "target"
        argv = ["cargo", "build", "--locked", "--release", "--target", "wasm32-wasip1"]
        if host:
            invoke(argv, env=dict(environment, CARGO_TARGET_DIR=str(target)))
            wasm = list((target / "wasm32-wasip1/release").glob("*.wasm"))
            if len(wasm) != 1:
                raise ValueError("Rust build must produce one Wasm module")
            shutil.copyfile(wasm[0], artifact)
        else:
            cargo_cache = cache / "cargo"
            cargo_cache.mkdir(exist_ok=True)
            variables = {"CARGO_HOME": "/output/cache/cargo", "CARGO_TARGET_DIR": container_output.rsplit("/", 1)[0] + "/target"}
            command = shlex.join(argv)
            command += " && cp " + variables["CARGO_TARGET_DIR"] + "/wasm32-wasip1/release/*.wasm " + shlex.quote(container_output)
            invoke(container(images["rust"], ["bash", "-ec", command], variables=variables))
    elif source.implementation == "assemblyscript":
        workspace = root / "build/assemblyscript"
        shutil.copytree(root / "plugins/wasm-assemblyscript", workspace, dirs_exist_ok=True,
                        ignore=shutil.ignore_patterns("node_modules", "build"))
        candidates = [workspace / "extensions" / source.directory.name / name
                      for name in ["index.ts", "assembly/index.ts", "main.ts"]]
        entry = next((path for path in candidates if path.is_file()), None)
        if entry is None:
            raise ValueError("AssemblyScript entry point is missing")
        relative_entry = str(entry.relative_to(workspace))
        argv = ["./node_modules/.bin/asc", relative_entry, "--target", "release", "--outFile",
                str(artifact) if host else container_output]
        if host:
            invoke(["npm", "ci", "--ignore-scripts"], directory=workspace)
            invoke(argv, directory=workspace)
        else:
            # This is a generated build workspace, not a second maintained source tree.
            command = "npm ci --ignore-scripts --cache /output/cache/npm && " + shlex.join(argv)
            docker = container(images["node"], ["bash", "-ec", command])
            at = docker.index("-w")
            docker[at + 1] = "/output/assemblyscript"
            invoke(docker)
    else:
        raise ValueError("unsupported build implementation: " + source.implementation)
    raw = artifact.read_bytes()
    if raw[:4] != (b"\x7fELF" if source.distribution == "native" else b"\0asm"):
        raise ValueError("compiler did not produce the expected plugin binary: " + source.name)
    receipt = {"plugin_name": source.name, "plugin_version": source.version, "implementation": source.implementation,
               "distribution": source.distribution, "started_at": started,
               "completed_at": datetime.now(timezone.utc).isoformat(), "toolchain": toolchain,
               "source_commit": commit,
               "source_inputs": inputs, "source_inputs_digest": "sha256:" + inputs_digest,
               "commands": commands, "artifact": str(artifact.relative_to(root)),
               "sha256": "sha256:" + hashlib.sha256(raw).hexdigest(), "size_bytes": len(raw)}
    receipt_file.write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps({key: receipt[key] for key in ["plugin_name", "plugin_version", "implementation", "sha256", "size_bytes"]}), flush=True)
    return receipt


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--plugin")
    parser.add_argument("--implementation")
    parser.add_argument("--host", action="store_true", help="Use the developer's installed toolchain instead of pinned containers")
    parser.add_argument("--keep-going", action="store_true", help="Collect independent compiler failures in one batch")
    args = parser.parse_args()
    root = args.root.resolve()
    _, all_sources = load_catalog(root)
    sources = select_releases(root)
    if args.plugin:
        if args.implementation:
            sources = [item for item in all_sources if item.name == args.plugin and item.implementation == args.implementation]
        else:
            sources = [item for item in sources if item.name == args.plugin]
        if len(sources) != 1:
            parser.error("select one known plugin and, for examples/native code, its implementation")
    elif args.implementation:
        parser.error("--implementation requires --plugin")
    images = json.loads((root / "plugins/release/toolchains.json").read_bytes())["images"]
    run = root / "build/runs" / (datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ-") + uuid.uuid4().hex[:8])
    run.mkdir(parents=True)
    if not args.host and any(source.implementation == "rust" for source in sources):
        tag = "plugin-server-rust-toolchain:" + images["rust"].rsplit("sha256:", 1)[1][:16]
        with (run / "toolchain-rust.log").open("wb") as log:
            subprocess.run(["docker", "build", "--platform", "linux/amd64", "--build-arg", "RUST_IMAGE=" + images["rust"],
                            "-f", str(root / "scripts/toolchains/Rust.Dockerfile"), "-t", tag,
                            str(root / "scripts/toolchains")], stdout=log, stderr=subprocess.STDOUT, check=True)
        images["rust"] = subprocess.check_output(["docker", "image", "inspect", "--format", "{{.Id}}", tag], text=True).strip()
    receipts, failures = [], []
    for source in sources:
        try:
            receipts.append(build_one(root, source, host=args.host, images=images, run=run))
        except (subprocess.CalledProcessError, OSError, ValueError) as error:
            if not args.keep_going:
                raise
            failure = {"plugin": source.name, "error": str(error),
                       "log": str((run / (source.name + ".log")).relative_to(root))}
            failures.append(failure)
            print(json.dumps(failure), flush=True)
    (run / "batch.json").write_text(json.dumps({"complete": not failures, "plugins": receipts,
                                               "failures": failures}, indent=2) + "\n")
    print(json.dumps({"complete": not failures, "count": len(receipts), "failed": len(failures),
                      "batch": str(run.relative_to(root))}))
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
