#!/usr/bin/env python3
"""Seed complete image packages through the production immutable commit path."""
import argparse
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_contract import json_bytes, load_policy
from artifact_import import sync_packages
from artifact_storage import ArtifactStore


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--destination", type=Path, required=True)
    parser.add_argument("--policy", type=Path)
    parser.add_argument("--public-base-url", default="http://higress-plugin-server.higress-system.svc:8080")
    args = parser.parse_args()
    store = ArtifactStore(args.destination, public_base_url=args.public_base_url, policy=load_policy(args.policy))
    results = sync_packages(args.source, store) if any(args.source.glob("*.tgz")) else []
    print(json_bytes({"seeded_complete_packages": str(len(results)), "uploads": results}).decode())


if __name__ == "__main__":
    main()
