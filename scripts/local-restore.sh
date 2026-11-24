#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
if [ "$#" -ne 1 ]; then
  echo "用法：$0 PocketBase备份.zip" >&2
  exit 1
fi
if [ ! -t 0 ]; then
  echo '请在交互式终端中运行此命令。' >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT HUP INT TERM

# The validator is /app/pocketbase in the locally built application image.
# Build first; validation never builds, starts PocketBase, or stops services.
image=${HPA_MAINTENANCE_IMAGE:-hpahub-local-app}
if ! docker image inspect "$image" >/dev/null 2>&1; then
  echo "缺少应用镜像 $image。请先运行：" >&2
  echo "docker compose -p hpahub-local -f \"$root/compose.yaml\" -f \"$root/scripts/compose.local.yaml\" build app" >&2
  exit 1
fi
archive=$(CDPATH= cd -- "$(dirname -- "$1")" && pwd)/$(basename -- "$1")
docker run --rm --network none --user "$(id -u):$(id -g)" \
  --entrypoint /app/pocketbase \
  --mount type=bind,src="$archive",dst=/backup.zip,readonly \
  --mount type=bind,src="$work",dst=/restore \
  "$image" hpa-validate-restore /backup.zip /restore

printf '警告：将停止本地测试项目并替换其数据，原数据会保留。输入 RESTORE 确认：'
IFS= read -r answer
if [ "$answer" != RESTORE ]; then
  echo '已取消。'
  exit 1
fi

compose() {
  docker compose -p hpahub-local -f "$root/compose.yaml" -f "$root/scripts/compose.local.yaml" "$@"
}
compose stop
if ! docker volume inspect hpahub-local_pb_data >/dev/null 2>&1; then
  docker volume create --label com.docker.compose.project=hpahub-local \
    --label com.docker.compose.volume=pb_data hpahub-local_pb_data >/dev/null
fi

docker run --rm -i --network none \
  --mount type=volume,src=hpahub-local_pb_data,dst=/data \
  --mount type=bind,src="$work",dst=/restore,readonly \
  alpine:3.22 sh -s <<'SH'
[ ! -L /data/backups ] || { echo '数据卷中的 backups 不得是符号链接。' >&2; exit 1; }
set -eu
stage=$(mktemp -d /data/.local-restore-stage-XXXXXX)
mkdir "$stage/new"
cp -a /restore/. "$stage/new/"
mkdir -p /data/backups
old=$(mktemp -d /data/backups/local-restore-XXXXXX)
phase=old
rollback() {
  status=$?
  trap - EXIT HUP INT TERM
  if [ "$status" -ne 0 ]; then
    echo "恢复失败，正在还原原数据。原数据目录：$old" >&2
    if [ "$phase" = new ]; then
      for item in /data/* /data/.[!.]* /data/..?*; do
        [ -e "$item" ] || [ -L "$item" ] || continue
        case "$item" in /data/backups|"$stage") continue ;; esac
        mv "$item" "$stage/new/" || echo "无法移回：$item" >&2
      done
    fi
    for item in "$old"/* "$old"/.[!.]* "$old"/..?*; do
      [ -e "$item" ] || [ -L "$item" ] || continue
      mv "$item" /data/ || echo "无法还原：$item；原数据保留在 $old" >&2
    done
  fi
  exit "$status"
}
trap rollback EXIT
trap 'exit 1' HUP INT TERM
for item in /data/* /data/.[!.]* /data/..?*; do
  [ -e "$item" ] || [ -L "$item" ] || continue
  case "$item" in /data/backups|"$stage") continue ;; esac
  mv "$item" "$old/"
done
phase=new
for item in "$stage/new"/* "$stage/new"/.[!.]* "$stage/new"/..?*; do
  [ -e "$item" ] || [ -L "$item" ] || continue
  mv "$item" /data/
done
rmdir "$stage/new" "$stage" 2>/dev/null || :
trap - EXIT HUP INT TERM
echo "原数据保留在卷 hpahub-local_pb_data 的 ${old#/data/}。"
SH

echo "恢复完成。启动本地项目：sh \"$root/scripts/local-up.sh\""
