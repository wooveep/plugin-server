#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="${PLUGIN_ARTIFACTS_DIR:-$repo_root/../higress-plugin/wasm-go/build}"
destination_dir="${PLUGIN_DIR:-$repo_root/plugins}"
prune=false
dry_run=false

usage() {
  cat <<'EOF'
Usage: sync-plugin-artifacts.sh [options]

Synchronize built Wasm plugins into the plugin-server Docker build context.

The source may be either the wasm-go build directory or its plugins child.
Existing plugin versions are retained by default so a rolling plugin-server
upgrade can continue to serve WasmPlugin resources that still reference them.

Options:
  --source DIR        Wasm artifact source. Default: ../higress-plugin/wasm-go/build
  --destination DIR   plugin-server plugins directory. Default: ./plugins
  --prune             Remove versions absent from the source after a successful sync.
  --dry-run           Validate and show the sync plan without changing the destination.
  -h, --help          Show this help.
EOF
}

log() {
  printf '[plugin-artifacts] %s\n' "$*"
}

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --source)
        [[ $# -ge 2 ]] || die "--source requires a value"
        source_dir="$2"
        shift 2
        ;;
      --destination)
        [[ $# -ge 2 ]] || die "--destination requires a value"
        destination_dir="$2"
        shift 2
        ;;
      --prune)
        prune=true
        shift
        ;;
      --dry-run)
        dry_run=true
        shift
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

resolve_source_dir() {
  if [[ -d "$source_dir/plugins" ]]; then
    source_dir="$source_dir/plugins"
  fi

  [[ -d "$source_dir" ]] || die "plugin artifact source does not exist: $source_dir"
  [[ ! -L "$source_dir" ]] || die "plugin artifact source must not be a symlink: $source_dir"
  source_dir="$(cd "$source_dir" && pwd -P)"
}

validate_artifacts() {
  local root="$1"
  local -a wasm_files
  mapfile -d '' -t wasm_files < <(find "$root" -type f -name plugin.wasm -print0 | sort -z)
  [[ "${#wasm_files[@]}" -gt 0 ]] || die "no plugin.wasm artifacts found under $root"

  local wasm version_dir metadata relative_path plugin_name plugin_version filename extra_path
  for wasm in "${wasm_files[@]}"; do
    [[ -s "$wasm" ]] || die "empty plugin artifact: $wasm"
    version_dir="$(dirname "$wasm")"
    metadata="$version_dir/metadata.txt"
    [[ -s "$metadata" ]] || die "missing or empty metadata.txt beside $wasm"
    relative_path="${wasm#"$root"/}"
    IFS=/ read -r plugin_name plugin_version filename extra_path <<<"$relative_path"
    [[ -n "$plugin_name" && -n "$plugin_version" && "$filename" = plugin.wasm && -z "$extra_path" ]] ||
      die "plugin artifacts must use <name>/<version>/plugin.wasm: $wasm"
  done

  printf '%s\n' "${#wasm_files[@]}"
}

sync_artifacts() {
  resolve_source_dir
  [[ ! -L "$destination_dir" ]] || die "plugin destination must not be a symlink: $destination_dir"

  local source_count
  source_count="$(validate_artifacts "$source_dir")"
  local destination_parent destination_name
  destination_parent="$(dirname "$destination_dir")"
  destination_name="$(basename "$destination_dir")"
  if [[ -d "$destination_parent" ]]; then
    destination_parent="$(cd "$destination_parent" && pwd -P)"
  elif ! "$dry_run"; then
    mkdir -p "$destination_parent"
    destination_parent="$(cd "$destination_parent" && pwd -P)"
  fi
  destination_dir="$destination_parent/$destination_name"

  if [[ -d "$destination_dir" && "$(cd "$destination_dir" && pwd -P)" = "$source_dir" ]]; then
    die "plugin source and destination must be different directories"
  fi

  local staging_dir backup_dir
  if "$dry_run"; then
    staging_dir="$(mktemp -d)"
  else
    staging_dir="$(mktemp -d "$destination_parent/.plugins-sync.XXXXXX")"
  fi
  backup_dir=""
  trap '
    status=$?
    if [[ -n "${backup_dir:-}" && -e "${backup_dir:-}" && ! -e "${destination_dir:-}" ]]; then
      mv "$backup_dir" "$destination_dir" || true
    fi
    [[ -n "${staging_dir:-}" && -d "${staging_dir:-}" ]] && rm -rf "${staging_dir:-}"
    exit "$status"
  ' EXIT

  if [[ -d "$destination_dir" && "$prune" = false ]]; then
    cp -a "$destination_dir/." "$staging_dir/"
  fi
  cp -a "$source_dir/." "$staging_dir/"
  local staged_count
  staged_count="$(validate_artifacts "$staging_dir")"

  if "$dry_run"; then
    log "validated $source_count source plugins; destination would contain $staged_count versions"
    rm -rf "$staging_dir"
    trap - EXIT
    return
  fi

  if [[ -e "$destination_dir" ]]; then
    backup_dir="$(mktemp -d "$destination_parent/.plugins-backup.XXXXXX")"
    rmdir "$backup_dir"
    mv "$destination_dir" "$backup_dir"
  fi

  if ! mv "$staging_dir" "$destination_dir"; then
    if [[ -n "$backup_dir" && -e "$backup_dir" ]] && ! mv "$backup_dir" "$destination_dir"; then
      log "initial plugin destination restore failed; exit cleanup will retry"
    fi
    die "could not replace plugin destination: $destination_dir"
  fi

  if [[ -n "$backup_dir" ]]; then
    rm -rf "$backup_dir"
    backup_dir=""
  fi
  trap - EXIT
  log "synchronized $source_count built plugins into $destination_dir ($staged_count versions available)"
}

parse_args "$@"
sync_artifacts
