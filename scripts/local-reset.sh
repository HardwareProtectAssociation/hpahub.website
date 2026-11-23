#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)

if [ ! -t 0 ]; then
  echo '请在交互式终端中运行此命令。' >&2
  exit 1
fi
printf '警告：将停止本地测试项目并永久删除卷 hpahub-local_pb_data。输入 DELETE 确认：'
IFS= read -r answer
if [ "$answer" != DELETE ]; then
  echo '已取消。'
  exit 1
fi

docker compose -p hpahub-local -f "$root/compose.yaml" -f "$root/scripts/compose.local.yaml" down
if docker volume inspect hpahub-local_pb_data >/dev/null 2>&1; then
  docker volume rm hpahub-local_pb_data
else
  echo '本地测试数据卷不存在。'
fi
