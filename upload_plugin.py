"""Upload a complete artifact.v1 package with durable, explicit idempotency."""
import argparse
import json
import os
from pathlib import Path
from urllib.parse import quote

from artifact_contract import load_policy, validate_package
from artifact_import import ArtifactClient


def build_upload_url(server, name, version):
    return f"{server.rstrip('/')}/plugins/{quote(name, safe='')}/{quote(version, safe='')}/package.tgz"


def upload_plugin(server, token, plugin_file, name, version, *, upload_id, policy=None):
    path = Path(plugin_file)
    raw = path.read_bytes()
    validated = validate_package(raw, policy)
    if (validated.descriptor["plugin_name"], validated.descriptor["plugin_version"]) != (name, version):
        raise ValueError("package identity differs from requested name/version")
    receipt = ArtifactClient(server, token, policy=policy).commit(upload_id, raw)
    return 201, json.dumps(receipt) + "\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--server", required=True)
    parser.add_argument("--token", default=os.getenv("PLUGIN_SERVER_UPLOAD_TOKEN"))
    parser.add_argument("--file", type=Path, required=True, help="complete package .tgz")
    parser.add_argument("--name", required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--upload-id", required=True, help="reuse this key after unknown outcomes")
    parser.add_argument("--policy", type=Path)
    args = parser.parse_args()
    if not args.token:
        parser.error("provide PLUGIN_SERVER_UPLOAD_TOKEN or --token")
    status, body = upload_plugin(args.server, args.token, args.file, args.name, args.version, upload_id=args.upload_id, policy=load_policy(args.policy))
    print(f"HTTP {status}: {body}", end="")


if __name__ == "__main__":
    main()
