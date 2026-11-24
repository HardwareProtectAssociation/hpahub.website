# HPAHub 网站模板

基于 Svelte 和 PocketBase 的登录制网站模板。

## 部署

安装 Docker 和 Docker Compose，然后运行：

```sh
docker compose up -d --build --remove-orphans
```

构建时可通过 Docker 构建参数 `GOPROXY` 和 `ALPINE_MIRROR` 设置依赖镜像源。

访问 `http://localhost:18763`。设置 `PORT` 环境变量可更改宿主机端口。PocketBase 数据保存在 Docker 卷 `pb_data` 中，更新容器不会清除数据。

显卡数据、仓库项目与文件由 PocketBase 管理，保存在 `pb_data`；默认本地文件存储会随 PocketBase 备份一起保存。视频缩略图和预览依赖容器内的 FFmpeg、ffprobe。旧 `gpu_data`、`warehouse_data` 卷仅以只读方式挂载用于迁移，不会自动删除。

## 从旧版本迁移

先备份三个数据卷并验证恢复。停止所有旧服务和写入；新版本导入期间不能运行其他应用实例。以下命令保留旧卷；不要使用 `down -v`。命令失败时停止操作，保留备份及旧卷，修复错误后可重试。

```sh
docker compose down --remove-orphans
docker compose build app
# 仅当旧显卡卷包含 gpu.db 时运行：
docker compose run --rm --no-deps app ./pocketbase hpa-import-gpu --source /legacy/gpu/gpu.db
# 仅当旧仓库卷包含数据时运行：
docker compose run --rm --no-deps app ./pocketbase hpa-import-warehouse --source /legacy/warehouse
docker compose up -d --build --remove-orphans
```

导入读取旧卷，不修改旧数据；重复导入会检查冲突。发现未导入的旧卷时，服务拒绝启动，避免显示空数据。切换后检查显卡、项目、文件、预览和访问权限，并创建、验证新的 PocketBase 备份。新服务产生写入后，直接回退旧卷会丢失这些写入；恢复前先停写并确认回滚方案。若使用 PocketBase 外部对象存储，须单独备份和验证对象；不能假定 PocketBase ZIP 包含远程文件。

创建 PocketBase 超级管理员（将邮箱和密码替换为自己的值）：

```sh
docker compose exec app ./pocketbase superuser upsert admin@example.com '替换为强密码'
```

PocketBase 超级管理员还可在控制台设置备份间隔和保留份数、手动创建及下载备份。自动备份默认关闭；保留份数仅限制自动备份，不删除手动备份。

## 本地测试

需要 Docker Compose 2.24.4 或更新版本。以下命令使用独立的 `hpahub-local` 项目和数据卷，不影响普通部署；无需宿主机 Python 或 Go：

```sh
sh scripts/local-up.sh                      # 构建、启动并打开页面
sh scripts/local-reset.sh                   # 确认后删除本地测试数据库及备份
sh scripts/local-restore.sh /path/backup.zip # 确认后从 PocketBase ZIP 恢复
```

测试地址默认为 `http://localhost:18764`，可通过 `PORT` 覆盖。首次创建数据卷时，测试管理员账号为 `admin`，密码为 `admin123`。已有或恢复的数据不会重置管理员。

其他检查：

```sh
(cd backend && go test ./...)
(cd frontend && npm ci && npm run build)
# Go 测试同时覆盖备份验证、迁移快照与仓库文件安全。
```

恢复会停止测试服务，并将原数据保留在测试卷的 `backups/local-restore-*` 目录。恢复后运行启动命令。本地使用 Caddy 反向代理，仅监听本机；应用运行在隔离网络中。生产备份仍包含敏感数据和生产账户，请妥善保管。

## 一次性旧数据快照

先构建本地应用镜像，再从已授权的隔离测试数据创建快照：

```sh
docker compose -p hpahub-local -f compose.yaml -f scripts/compose.local.yaml build app
sh scripts/hpa-legacy-snapshot.sh \
  --gpu-db /path/to/gpu.db \
  --warehouse-root /path/to/warehouse \
  --auth-db /path/to/users.db \
  --output /path/to/new-snapshot
```

脚本使用 Go 维护命令，不需要宿主机语言运行时。默认镜像为 `hpahub-local-app`；可通过 `HPA_MAINTENANCE_IMAGE` 指定已构建的镜像。恢复脚本也使用此镜像。快照输出必须不存在；数据库通过 SQLite 在线备份包含 WAL 数据，文件清单记录大小及 SHA-256。输出含敏感账户数据，不得提交或公开。该工具面向 Linux Docker 环境，Windows 请在 WSL 2 中执行。


## 备份接口

所有接口均需 PocketBase 超级管理员身份：

- `GET /api/backup-policy`：读取策略。
- `PUT /api/backup-policy`：设置 `{"intervalHours": 24, "retainCount": 5}`。间隔为 0 时关闭自动备份；范围为 0–8760 小时，保留份数为 1–100。
- `GET /api/backups`、`POST /api/backups`：列出和创建备份。
- `GET /api/backups/{name}?token={fileToken}`：下载备份；通过 `POST /api/files/token` 获取短期文件令牌。

自动备份保存在 PocketBase 备份存储中。策略更改后，从保存时起计算下次备份时间。

## 数据安全

备份包含敏感数据，请妥善保管并定期下载到独立存储。生产环境应在可信反向代理后启用 HTTPS。删除 Docker 卷会永久删除数据库及保存在卷中的备份。
