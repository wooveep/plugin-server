#!/usr/bin/env python3
"""Seed a PVC from an image's already validated immutable artifact store."""

import argparse
import json
from pathlib import Path
import sqlite3
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_storage import ArtifactStore
from mf_canonical import parse_json


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--destination", type=Path, required=True)
    parser.add_argument("--source-state", type=Path)
    parser.add_argument("--destination-state", type=Path)
    args = parser.parse_args()
    source_state = args.source_state or args.source.parent / ("." + args.source.name + "-artifact-state")
    # Read the image's committed registry without attempting to mutate its lock,
    # schema or package bytes. This also works with a read-only image filesystem.
    with sqlite3.connect((source_state / "state.sqlite3").resolve().as_uri() + "?mode=ro", uri=True) as db:
        manifests = [parse_json(row[0]) for row in db.execute(
            "SELECT manifest FROM artifacts a WHERE NOT EXISTS(SELECT 1 FROM retention_commands r "
            "WHERE r.artifact_id=a.artifact_id AND r.state IN ('unknown','deleted')) ORDER BY name,version")]
    destination = ArtifactStore(args.destination, state_dir=args.destination_state)
    print(json.dumps(destination.seed_registered(args.source, manifests)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
