#!/usr/bin/env python3
"""Require the full source-selected inventory using registered metadata only."""

import argparse
import json
from pathlib import Path
import sys

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from artifact_contract import digest, json_bytes
from artifact_storage import ArtifactStore
from mf_canonical import parse_json
from plugin_catalog import selected_entries


def release_catalog(store: ArtifactStore, source_catalog: dict) -> dict:
    selected = {(entry["logicalId"], entry["declaredVersion"]): entry for entry in selected_entries(source_catalog)}
    registered = {(entry["manifest"]["plugin_name"], entry["manifest"]["plugin_version"]): entry
                  for entry in store.list_metadata()}
    # The selected release must be complete. Previously registered versions
    # remain immutable and available without becoming this release's defaults.
    if missing := selected.keys() - registered.keys():
        raise ValueError("registered release differs from the full source catalog: missing="
                         + repr(sorted(missing)))
    artifacts = []
    for identity, entry in sorted(selected.items()):
        item = registered[identity]
        metadata = item["metadata"]
        if metadata is None or (metadata["info"]["name"], metadata["info"]["version"]) != identity:
            raise ValueError("registered plugin metadata identity differs: " + repr(identity))
        artifacts.append({"implementation": entry["implementation"], **item})
    return {"catalog_version": "plugin-release.v1", "source_catalog_digest": digest(json_bytes(source_catalog)),
            "count": len(artifacts), "artifacts": artifacts}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--plugins-dir", type=Path, required=True)
    parser.add_argument("--catalog", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    value = release_catalog(ArtifactStore(args.plugins_dir), parse_json(args.catalog.read_bytes()))
    args.output.write_bytes(json_bytes(value))
    print(json.dumps({"complete": True, "count": value["count"], "output": str(args.output)}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
