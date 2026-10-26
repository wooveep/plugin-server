#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
export PYTHONPATH="$repo_root:$repo_root/tests"
"${PYTHON:-python3}" - "$work_dir" <<'PY'
import sys
from pathlib import Path
from test_artifact_contract import example_archive
root=Path(sys.argv[1])
(root/"source").mkdir()
(root/"source/one.tgz").write_bytes(example_archive())
legacy=root/"destination/plugins/legacy/0.1.0/plugin.wasm"
legacy.parent.mkdir(parents=True)
legacy.write_bytes(b"preserve me")
PY
"$repo_root/scripts/sync-plugin-artifacts.sh" --source "$work_dir/source" --destination "$work_dir/destination/plugins" > /dev/null
test -f "$work_dir/destination/plugins/test-plugin/1.0.0/manifest.json"
test "$(<"$work_dir/destination/plugins/legacy/0.1.0/plugin.wasm")" = 'preserve me'
"$repo_root/scripts/sync-plugin-artifacts.sh" --source "$work_dir/source" --destination "$work_dir/absent/plugins" --dry-run > /dev/null
test ! -e "$work_dir/absent"
if "$repo_root/scripts/sync-plugin-artifacts.sh" --source "$work_dir/source" --destination "$work_dir/destination/plugins" --prune > /dev/null 2>&1; then
  echo 'prune requires Console reference coordination' >&2
  exit 1
fi
test -f "$work_dir/destination/plugins/legacy/0.1.0/plugin.wasm"
echo "complete package sync and retention checks passed"
