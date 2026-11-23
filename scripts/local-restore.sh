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

# Extract only regular files and directories into an empty private directory.
python3 - "$1" "$work" <<'PY'
import os
from pathlib import PurePosixPath
import shutil
import sqlite3
import stat
import sys
import zipfile

archive, destination = sys.argv[1:]
try:
    with zipfile.ZipFile(archive) as z:
        seen = set()
        for entry in z.infolist():
            name = entry.filename
            parts = name.rstrip('/').split('/')
            mode = entry.external_attr >> 16
            kind = stat.S_IFMT(mode)
            if (not name or name.startswith('/') or '\\' in name or
                    any(part in ('', '.', '..') for part in parts) or
                    parts[0] == 'backups' or parts[0].startswith('.local-restore-') or
                    kind not in (0, stat.S_IFREG, stat.S_IFDIR) or
                    (entry.is_dir() and kind == stat.S_IFREG) or
                    (not entry.is_dir() and kind == stat.S_IFDIR)):
                raise ValueError(f'不安全的备份路径或文件类型：{name!r}')
            path = str(PurePosixPath(*parts))
            if path in seen:
                raise ValueError(f'备份包含重复路径：{name!r}')
            seen.add(path)
            target = os.path.join(destination, *parts)
            if entry.is_dir():
                os.makedirs(target, exist_ok=True)
            else:
                os.makedirs(os.path.dirname(target), exist_ok=True)
                with z.open(entry) as source, open(target, 'xb') as output:
                    shutil.copyfileobj(source, output)
    db_path = os.path.join(destination, 'data.db')
    if not os.path.isfile(db_path):
        raise ValueError('备份缺少 data.db')
    for filename in ('data.db', 'auxiliary.db'):
        path = os.path.join(destination, filename)
        if not os.path.exists(path):
            continue
        with sqlite3.connect(path) as db:
            if db.execute('PRAGMA integrity_check').fetchall() != [('ok',)]:
                raise ValueError(f'{filename} 完整性检查失败')
            if filename == 'data.db' and not db.execute(
                "SELECT 1 FROM sqlite_master WHERE type='table' AND name='_collections'"
            ).fetchone():
                raise ValueError('data.db 不是有效的 PocketBase 数据库')
except (OSError, ValueError, zipfile.BadZipFile, sqlite3.Error, RuntimeError) as exc:
    print(f'备份验证失败：{exc}', file=sys.stderr)
    sys.exit(1)
PY

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

echo "恢复完成。启动本地项目：$root/scripts/local-up.sh"
