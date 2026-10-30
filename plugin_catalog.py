"""Resolve maintained plugin implementations and their declared release versions."""

from dataclasses import dataclass
from functools import total_ordering
import hashlib
import json
from pathlib import Path
import re


@total_ordering
@dataclass(frozen=True)
class Version:
    major: int
    minor: int
    patch: int
    prerelease: tuple[str, ...] = ()

    @classmethod
    def parse(cls, value: str) -> "Version":
        match = re.fullmatch(
            r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)"
            r"(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?"
            r"(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?",
            value,
        )
        if match is None:
            raise ValueError("invalid semantic version: " + value)
        prerelease = tuple(match[4].split(".")) if match[4] else ()
        if any(item.isdigit() and len(item) > 1 and item.startswith("0") for item in prerelease):
            raise ValueError("numeric prerelease identifiers cannot have leading zeros: " + value)
        return cls(int(match[1]), int(match[2]), int(match[3]), prerelease)

    def precedence(self) -> tuple:
        identifiers = tuple((0, int(item)) if item.isdigit() else (1, item) for item in self.prerelease)
        return self.major, self.minor, self.patch, not self.prerelease, identifiers

    def __lt__(self, other: "Version") -> bool:
        if not isinstance(other, Version):
            return NotImplemented
        return self.precedence() < other.precedence()


@dataclass(frozen=True)
class PluginSource:
    name: str
    implementation: str
    directory: Path
    version: str | None
    release_eligible: bool
    distribution: str
    reason: str | None
    metadata_directory: Path | None


def load_catalog(root: Path) -> tuple[dict, list[PluginSource]]:
    """Read the sole release catalog and validate its declared source entries."""
    root = root.resolve()
    catalog = json.loads((root / "plugins/release/catalog.json").read_bytes())
    if catalog.get("schemaVersion") != 1 or not isinstance(catalog.get("plugins"), list):
        raise ValueError("unsupported plugin source catalog")
    sources = []
    identities = set()
    for entry in catalog["plugins"]:
        name, implementation, relative = entry["logicalId"], entry["implementation"], entry["sourceDir"]
        if not re.fullmatch(r"[a-z0-9][a-z0-9-]*", name):
            raise ValueError("invalid logical plugin name: " + name)
        directory = (root / relative).resolve()
        if not directory.is_relative_to(root / "plugins") or not directory.is_dir():
            raise ValueError("catalog source is missing or outside plugins/: " + relative)
        identity = name, implementation
        if identity in identities:
            raise ValueError("duplicate plugin implementation: " + "/".join(identity))
        identities.add(identity)
        version_file = directory / "VERSION"
        version = version_file.read_text().strip() if version_file.is_file() else None
        if version is not None:
            Version.parse(version)
        if entry.get("declaredVersion") != version:
            raise ValueError("catalog version differs from source VERSION: " + relative)
        eligible = entry["releaseEligible"]
        reason = entry.get("unmanagedReason")
        if not isinstance(eligible, bool) or not eligible and not reason:
            raise ValueError("each non-release implementation needs an explicit classification: " + relative)
        if eligible and version is None:
            raise ValueError("required release implementation has no VERSION: " + relative)
        metadata = entry.get("metadataDir")
        metadata_directory = (root / metadata).resolve() if metadata else None
        if metadata_directory is not None and (
            not metadata_directory.is_relative_to(root / "plugins") or not metadata_directory.is_dir()
        ):
            raise ValueError("catalog metadata directory is missing: " + metadata)
        distribution = entry.get("distribution", "wasm")
        if distribution not in {"wasm", "native"}:
            raise ValueError("unknown plugin distribution: " + distribution)
        sources.append(PluginSource(name, implementation, directory, version, eligible,
                                    distribution, reason, metadata_directory))
    return catalog, sources


def selected_entries(catalog: dict) -> list[dict]:
    """Resolve the same release inventory from source or registered metadata."""
    selected = {}
    for entry in catalog["plugins"]:
        if not entry["releaseEligible"] or entry.get("distribution", "wasm") != "wasm":
            continue
        name = entry["logicalId"]
        previous = selected.get(name)
        if previous is not None:
            current_version = Version.parse(entry["declaredVersion"])
            previous_version = Version.parse(previous["declaredVersion"])
            if current_version == previous_version:
                raise ValueError("same-version implementations require reconciliation: " + name)
            if current_version < previous_version:
                continue
        Version.parse(entry["declaredVersion"])
        selected[name] = entry
    required = set(catalog["requiredPlugins"])
    if not required or required != set(selected):
        raise ValueError("formal plugin inventory differs: missing=" + ",".join(sorted(required - selected.keys()))
                         + "; unlisted=" + ",".join(sorted(selected.keys() - required)))
    return [selected[name] for name in sorted(selected)]


def select_releases(root: Path) -> list[PluginSource]:
    """Select the highest SemVer implementation for every required Wasm plugin."""
    catalog, sources = load_catalog(root)
    entries = {(entry["logicalId"], entry["implementation"]) for entry in selected_entries(catalog)}
    return sorted((source for source in sources if (source.name, source.implementation) in entries),
                  key=lambda source: source.name)


def source_inputs(root: Path, source: PluginSource) -> dict[str, str]:
    """Bind compiled bytes and prepared metadata to the same complete inputs."""
    catalog, _ = load_catalog(root)
    entry = next(item for item in catalog["plugins"]
                 if item["logicalId"] == source.name and item["implementation"] == source.implementation)
    patterns = list(entry["artifactInputs"]) + ["plugin_catalog.py", "scripts/build-plugins.py",
                                              "plugins/release/toolchains.json",
                                              "scripts/toolchains/*"]
    for group in entry.get("sharedInputGroups", []):
        patterns.extend(catalog["sharedInputGroups"][group])
    files = set()
    for pattern in patterns:
        paths = [(root / pattern[:-3])] if pattern.endswith("/**") else root.glob(pattern)
        for path in paths:
            candidates = path.rglob("*") if path.is_dir() else [path]
            files.update(item for item in candidates if item.is_file()
                         and not {"node_modules", "target", "__pycache__", ".git"}.intersection(item.parts))
    return {str(path.relative_to(root)): hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted(files)}


if __name__ == "__main__":
    import argparse
    from dataclasses import asdict

    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--all-sources", action="store_true", help="Include references, examples and native code")
    parser.add_argument("--json", action="store_true")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    sources = load_catalog(root)[1] if args.all_sources else select_releases(root)
    if args.json:
        print(json.dumps([asdict(source) for source in sources], default=str, indent=2))
    else:
        for source in sources:
            print("\t".join([source.name, source.version or "unversioned", source.implementation,
                             str(source.directory.relative_to(root))]))
