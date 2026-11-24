#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
compose() {
    docker compose -p hpahub-local -f "$root/compose.yaml" -f "$root/scripts/compose.local.yaml" "$@"
}

new_database=false
if ! docker volume inspect hpahub-local_pb_data >/dev/null 2>&1; then
    new_database=true
fi

compose up -d --build --remove-orphans

if [ "$new_database" = true ]; then
    email='admin@hpahub.local'
    password='admin123'
    if ! compose exec -T app ./pocketbase superuser create "$email" "$password"; then
        echo '无法创建测试超级管理员。数据库卷已保留，请检查容器日志。' >&2
        exit 1
    fi
    printf '测试管理员账号：admin\n测试管理员密码：%s\n' "$password"
else
    echo '检测到现有数据库，未修改超级管理员。'
fi

for image in $(docker image ls --quiet --filter 'label=org.hpahub.local-image=true' --filter dangling=true); do
    docker image rm "$image" >/dev/null 2>&1 || :
done

address=$(compose port proxy 8090)
port=${address##*:}
case "$port" in
    ''|0|*[!0-9]*) echo "无法获取本地端口：$address" >&2; exit 1 ;;
esac
url="http://localhost:$port"
printf '本地测试地址：%s\n' "$url"
if command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$url" >/dev/null 2>&1 || printf '无法自动打开浏览器，请手动访问：%s\n' "$url"
elif command -v open >/dev/null 2>&1; then
    open "$url" >/dev/null 2>&1 || printf '无法自动打开浏览器，请手动访问：%s\n' "$url"
else
    printf '无法自动打开浏览器，请手动访问：%s\n' "$url"
fi
