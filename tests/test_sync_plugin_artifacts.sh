#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
sync_script="$repo_root/scripts/sync-plugin-artifacts.sh"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

write_plugin() {
  local root="$1"
  local name="$2"
  local version="$3"
  local content="$4"
  local version_dir="$root/plugins/$name/$version"
  mkdir -p "$version_dir"
  printf '%s' "$content" >"$version_dir/plugin.wasm"
  printf 'Plugin Name: %s\nVersion: %s\n' "$name" "$version" >"$version_dir/metadata.txt"
}

source_root="$work_dir/wasm-go-build"
destination="$work_dir/plugin-server/plugins"
write_plugin "$source_root" ai-cache 1.0.1 'new-cache'
write_plugin "$work_dir/previous" ai-cache 1.0.0 'old-cache'
mkdir -p "$(dirname "$destination")"
mv "$work_dir/previous/plugins" "$destination"

"$sync_script" --source "$source_root" --destination "$destination"
[[ "$(<"$destination/ai-cache/1.0.1/plugin.wasm")" = 'new-cache' ]]
[[ "$(<"$destination/ai-cache/1.0.0/plugin.wasm")" = 'old-cache' ]]

"$sync_script" --source "$source_root" --destination "$destination" --dry-run
[[ "$(<"$destination/ai-cache/1.0.1/plugin.wasm")" = 'new-cache' ]]
[[ "$(<"$destination/ai-cache/1.0.0/plugin.wasm")" = 'old-cache' ]]
[[ -z "$(find "$(dirname "$destination")" -maxdepth 1 -type d -name '.plugins-sync.*' -print -quit)" ]]

nested_source="$work_dir/nested-wasm-go-build"
nested_version_dir="$nested_source/plugins/ai-cache/1.0.2/extra"
mkdir -p "$nested_version_dir"
printf 'invalid-nested-cache' >"$nested_version_dir/plugin.wasm"
printf 'Plugin Name: ai-cache\nVersion: 1.0.2\n' >"$nested_version_dir/metadata.txt"
if "$sync_script" --source "$nested_source" --destination "$work_dir/nested-target/plugins" --dry-run; then
  echo 'nested plugin artifact layout must be rejected' >&2
  exit 1
fi

absent_parent_destination="$work_dir/absent-parent/plugins"
"$sync_script" --source "$source_root" --destination "$absent_parent_destination" --dry-run
[[ ! -e "$(dirname "$absent_parent_destination")" ]]

rollback_source="$work_dir/rollback-wasm-go-build"
rollback_destination="$work_dir/rollback-plugin-server/plugins"
write_plugin "$rollback_source" ai-cache 1.0.1 'rollback-new-cache'
write_plugin "$work_dir/rollback-previous" ai-cache 1.0.0 'rollback-old-cache'
mkdir -p "$(dirname "$rollback_destination")"
mv "$work_dir/rollback-previous/plugins" "$rollback_destination"
rollback_fake_bin="$work_dir/rollback-fake-bin"
rollback_mv_counter="$work_dir/rollback-mv-counter"
mkdir -p "$rollback_fake_bin"
printf '%s\n' \
  '#!/usr/bin/env bash' \
  'set -euo pipefail' \
  'count=0' \
  '[[ -f "$MV_COUNTER_FILE" ]] && count="$(<"$MV_COUNTER_FILE")"' \
  'count=$((count + 1))' \
  'printf "%s\\n" "$count" >"$MV_COUNTER_FILE"' \
  'if [[ "$count" = 2 || "$count" = 3 ]]; then exit 1; fi' \
  'exec /bin/mv "$@"' \
  >"$rollback_fake_bin/mv"
chmod +x "$rollback_fake_bin/mv"
if MV_COUNTER_FILE="$rollback_mv_counter" PATH="$rollback_fake_bin:$PATH" \
  "$sync_script" --source "$rollback_source" --destination "$rollback_destination"; then
  echo 'forced replacement failure must make sync fail' >&2
  exit 1
fi
[[ "$(<"$rollback_destination/ai-cache/1.0.0/plugin.wasm")" = 'rollback-old-cache' ]]
[[ -z "$(find "$(dirname "$rollback_destination")" -maxdepth 1 -type d -name '.plugins-backup.*' -print -quit)" ]]

"$sync_script" --source "$source_root" --destination "$destination" --prune
[[ -f "$destination/ai-cache/1.0.1/plugin.wasm" ]]
[[ ! -e "$destination/ai-cache/1.0.0" ]]

echo "plugin artifact sync tests passed"
