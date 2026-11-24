#!/bin/sh
# One-time legacy migration. Run the Go command in the application's Linux image.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
gpu= warehouse= auth= output=
usage() {
  echo "用法：$0 --gpu-db FILE --warehouse-root DIR --auth-db FILE --output NEW_DIR" >&2
}
while [ "$#" -gt 0 ]; do
  case "$1" in
    --gpu-db|--warehouse-root|--auth-db|--output)
      option=$1
      [ "$#" -ge 2 ] || { usage; exit 1; }
      value=$2
      shift 2
      ;;
    --gpu-db=*|--warehouse-root=*|--auth-db=*|--output=*)
      option=${1%%=*}
      value=${1#*=}
      shift
      ;;
    --help|-h) usage; exit 0 ;;
    *) usage; exit 1 ;;
  esac
  case "$option" in
    --gpu-db) gpu=$value ;;
    --warehouse-root) warehouse=$value ;;
    --auth-db) auth=$value ;;
    --output) output=$value ;;
  esac
done
[ -n "$gpu" ] && [ -n "$warehouse" ] && [ -n "$auth" ] && [ -n "$output" ] || { usage; exit 1; }

# Docker resolves host bind paths before Go sees them. Reject host symlinks too.
absolute_path() {
  case "$1" in /*) candidate=$1 ;; *) candidate=$PWD/$1 ;; esac
  while [ "$candidate" != / ] && [ "${candidate%/}" != "$candidate" ]; do candidate=${candidate%/}; done
  current=$candidate
  while [ "$current" != / ]; do
    [ ! -L "$current" ] || { echo "symlink not allowed: $current" >&2; exit 1; }
    current=$(dirname -- "$current")
  done
  parent=$(CDPATH= cd -- "$(dirname -- "$candidate")" && pwd -P) || exit 1
  printf '%s/%s\n' "$parent" "$(basename -- "$candidate")"
}
gpu=$(absolute_path "$gpu")
auth=$(absolute_path "$auth")
warehouse=$(absolute_path "$warehouse")
warehouse=$(CDPATH= cd -- "$warehouse" && pwd -P)
output=$(absolute_path "$output")
[ -f "$gpu" ] && [ -f "$auth" ] && [ -d "$warehouse" ] || { echo 'snapshot failed: missing source file or warehouse directory' >&2; exit 1; }
[ ! -e "$output" ] && [ ! -L "$output" ] || { echo "snapshot failed: output already exists: $output" >&2; exit 1; }
case "$output" in "$warehouse/projects"|"$warehouse/projects/"*) echo 'snapshot failed: output cannot be inside warehouse projects' >&2; exit 1 ;; esac
image=${HPA_MAINTENANCE_IMAGE:-hpahub-local-app}
if ! docker image inspect "$image" >/dev/null 2>&1; then
  echo "缺少应用镜像 $image。请先运行：" >&2
  echo "docker compose -p hpahub-local -f \"$root/compose.yaml\" -f \"$root/scripts/compose.local.yaml\" build app" >&2
  exit 1
fi
# Mount database directories, not just .db files: SQLite must read live WAL files.
exec docker run --rm --network none --user "$(id -u):$(id -g)" \
  --entrypoint /app/pocketbase \
  --mount type=bind,src="$(dirname -- "$gpu")",dst=/gpu-source,readonly \
  --mount type=bind,src="$(dirname -- "$auth")",dst=/auth-source,readonly \
  --mount type=bind,src="$warehouse",dst=/warehouse-source,readonly \
  --mount type=bind,src="$(dirname -- "$output")",dst=/output \
  "$image" hpa-legacy-snapshot \
  --gpu-db "/gpu-source/$(basename -- "$gpu")" \
  --auth-db "/auth-source/$(basename -- "$auth")" \
  --warehouse-root /warehouse-source --output "/output/$(basename -- "$output")"
